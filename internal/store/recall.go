package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"unicode"
)

const maxRecallCandidates = 1000
const recallLimitMax = 50

var recallStopWords = map[string]bool{
	"algo": true, "como": true, "cual": true, "cuando": true,
	"dame": true, "datos": true, "desde": true, "donde": true,
	"esta": true, "este": true, "esto": true, "hacer": true,
	"para": true, "pero": true, "quiero": true, "sobre": true,
	"tiene": true, "todos": true, "ultima": true, "ultimo": true,
	"unos": true, "unas": true,
}

type recallCandidate struct {
	knowledge Knowledge
	score     int
}

// knowledgeColumnsQualified es knowledgeColumns con cada columna prefijada
// por "knowledge.": se necesita al hacer JOIN con knowledge_fts, que también
// tiene columnas subject/claim/context y produciría "ambiguous column name"
// con los nombres sin prefijo.
const knowledgeColumnsQualified = `knowledge.id, knowledge.subject, IFNULL(knowledge.context,''), knowledge.claim, IFNULL(knowledge.evidence_json,''), IFNULL(knowledge.source_json,''),
	       knowledge.confidence, knowledge.status, knowledge.version, knowledge.supersedes_id, IFNULL(knowledge.source_observation_ids_json,''), knowledge.created_at,
	       IFNULL(knowledge.taught_by,''), IFNULL(knowledge.decided_by,''), knowledge.decided_at, IFNULL(knowledge.decision_note,''), IFNULL(knowledge.topic_key,'')`

// RecallValidatedKnowledge recupera el conocimiento validado relevante para
// una petición escrita en lenguaje natural. A diferencia de SearchKnowledge,
// no exige que la petición completa aparezca literalmente en subject/claim:
// separa términos, normaliza tildes y ordena por coincidencia. Esto permite
// reconstruir el contexto al comenzar una sesión sin volver a explorar el
// schema para una tarea que el MCP ya conoce.
//
// Primero intenta el índice FTS5 (knowledge_fts, ver migración
// 0005_knowledge_fts.sql), rankeado por bm25() combinado con los mismos
// pesos de coincidencia que el scoring en Go. Si la consulta FTS falla
// (índice ausente/corrupto) o no devuelve resultados, cae al scoring en Go
// original sobre LIKE/contains, que sigue funcionando igual que antes.
func (s *Store) RecallValidatedKnowledge(ctx context.Context, task, contextHint string, limit int) ([]Knowledge, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > recallLimitMax {
		limit = recallLimitMax
	}

	query := normalizeRecallText(strings.TrimSpace(task + " " + contextHint))
	if query == "" {
		return []Knowledge{}, nil
	}
	terms := recallTerms(query)
	normalizedContext := normalizeRecallText(contextHint)

	if ftsQuery := buildFTSQuery(task, contextHint); ftsQuery != "" {
		out, err := s.recallViaFTS(ctx, ftsQuery, query, normalizedContext, terms, limit)
		if err == nil && len(out) > 0 {
			return out, nil
		}
		// Cae al scoring en Go si FTS falla (p.ej. índice inexistente en una
		// base vieja) o no encuentra nada: nunca dejamos que un límite del
		// índice haga parecer vacío un recall que el scoring original sí
		// resolvería.
	}

	return s.recallViaGoScoring(ctx, query, normalizedContext, terms, limit)
}

// buildFTSQuery convierte texto libre del usuario en una expresión MATCH de
// FTS5 segura: cada token se normaliza (esto ya elimina cualquier carácter
// que no sea letra/dígito, incluyendo operadores de FTS5 como *, -, (), :) y
// además se envuelve entre comillas dobles (literal de string de FTS5) como
// segunda capa de seguridad, así que texto de usuario arbitrario nunca puede
// producir un error de sintaxis FTS5. Los tokens se combinan con OR: basta
// con que coincida un término para que la fila sea candidata.
func buildFTSQuery(task, contextHint string) string {
	normalized := normalizeRecallText(strings.TrimSpace(task + " " + contextHint))
	if normalized == "" {
		return ""
	}
	seen := map[string]bool{}
	tokens := make([]string, 0)
	for _, field := range strings.Fields(normalized) {
		// Mismo filtro que recallTerms: sin stopwords/tokens cortos, un "de"
		// haría que cualquier tarea acierte conceptos como "movimientos de
		// stock" (falso positivo con memoria_encontrada=true).
		if seen[field] || len([]rune(field)) < 3 || recallStopWords[field] {
			continue
		}
		seen[field] = true
		escaped := strings.ReplaceAll(field, `"`, `""`)
		tokens = append(tokens, `"`+escaped+`"`)
	}
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " OR ")
}

// recallViaFTS busca candidatos usando el índice knowledge_fts, filtrando a
// status='validated' en el JOIN (el índice FTS no conoce status). El score
// final combina el mismo scoreRecallCandidate usado por el fallback en Go
// (para preservar "context exacto > subject > términos") con bm25() como
// desempate adicional: bm25 es más negativo cuanto mejor es la coincidencia,
// así que se invierte el signo antes de sumarlo.
func (s *Store) recallViaFTS(ctx context.Context, ftsQuery, query, normalizedContext string, terms []string, limit int) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+knowledgeColumnsQualified+`, bm25(knowledge_fts, 2.0, 1.0, 3.0)
		FROM knowledge_fts
		JOIN knowledge ON knowledge.id = knowledge_fts.rowid
		WHERE knowledge_fts MATCH ? AND knowledge.status = 'validated'
		LIMIT ?`, ftsQuery, maxRecallCandidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]recallCandidate, 0)
	for rows.Next() {
		k, rank, err := scanKnowledgeWithRank(rows)
		if err != nil {
			return nil, err
		}
		score := scoreRecallCandidate(*k, query, normalizedContext, terms)
		score += int(-rank * 10)
		candidates = append(candidates, recallCandidate{knowledge: *k, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortRecallCandidates(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return toKnowledgeSlice(candidates), nil
}

// recallViaGoScoring es el scoring original: escanea hasta
// maxRecallCandidates filas validadas y puntúa cada una en Go. Se mantiene
// como fallback cuando FTS falla o no encuentra nada.
func (s *Store) recallViaGoScoring(ctx context.Context, query, normalizedContext string, terms []string, limit int) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+knowledgeColumns+`
		FROM knowledge
		WHERE status = 'validated'
		ORDER BY id DESC
		LIMIT ?`, maxRecallCandidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]recallCandidate, 0)
	for rows.Next() {
		k, err := scanKnowledgeRows(rows)
		if err != nil {
			return nil, err
		}
		score := scoreRecallCandidate(*k, query, normalizedContext, terms)
		if score > 0 {
			candidates = append(candidates, recallCandidate{knowledge: *k, score: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sortRecallCandidates(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return toKnowledgeSlice(candidates), nil
}

func sortRecallCandidates(candidates []recallCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		// Desempate: una fila con topic_key canónico gana a una legacy sin clave.
		if ki, kj := candidates[i].knowledge.TopicKey != "", candidates[j].knowledge.TopicKey != ""; ki != kj {
			return ki
		}
		if candidates[i].knowledge.Version != candidates[j].knowledge.Version {
			return candidates[i].knowledge.Version > candidates[j].knowledge.Version
		}
		return candidates[i].knowledge.ID > candidates[j].knowledge.ID
	})
}

func toKnowledgeSlice(candidates []recallCandidate) []Knowledge {
	out := make([]Knowledge, len(candidates))
	for i, candidate := range candidates {
		out[i] = candidate.knowledge
	}
	return out
}

// scanKnowledgeWithRank es scanKnowledgeRows más la columna bm25() extra que
// devuelve recallViaFTS. Se duplica el escaneo campo a campo (en vez de
// reusar scanKnowledgeRows) porque Scan necesita recibir todos los
// destinos, incluido rank, en una sola llamada.
func scanKnowledgeWithRank(rows *sql.Rows) (*Knowledge, float64, error) {
	var k Knowledge
	var createdAt string
	var decidedAt sql.NullString
	var confidence sql.NullFloat64
	var supersedes sql.NullInt64
	var rank float64

	if err := rows.Scan(&k.ID, &k.Subject, &k.Context, &k.Claim, &k.EvidenceJSON, &k.SourceJSON, &confidence, &k.Status, &k.Version,
		&supersedes, &k.SourceObservationIDsJSON, &createdAt, &k.TaughtBy, &k.DecidedBy, &decidedAt, &k.DecisionNote, &k.TopicKey, &rank); err != nil {
		return nil, 0, err
	}
	k.CreatedAt = parseTime(createdAt)
	if confidence.Valid {
		k.Confidence = &confidence.Float64
	}
	if supersedes.Valid {
		k.SupersedesID = &supersedes.Int64
	}
	if decidedAt.Valid {
		t := parseTime(decidedAt.String)
		k.DecidedAt = &t
	}
	return &k, rank, nil
}

func scoreRecallCandidate(k Knowledge, query, normalizedContext string, terms []string) int {
	subject := normalizeRecallText(k.Subject)
	contextValue := normalizeRecallText(k.Context)
	claim := normalizeRecallText(k.Claim)
	source := normalizeRecallText(k.SourceJSON)

	score := 0
	if subject != "" && (strings.Contains(query, subject) || strings.Contains(subject, query)) {
		score += 50
	}
	if contextValue != "" && strings.Contains(query, contextValue) {
		score += 35
	}
	if normalizedContext != "" {
		if contextValue == normalizedContext {
			score += 70
		} else if strings.Contains(contextValue, normalizedContext) || strings.Contains(normalizedContext, contextValue) {
			score += 40
		}
	}

	for _, term := range terms {
		if recallContains(subject, term) {
			score += 14
		}
		if recallContains(contextValue, term) {
			score += 9
		}
		if recallContains(claim, term) {
			score += 4
		}
		if recallContains(source, term) {
			score++
		}
	}
	return score
}

func recallContains(text, term string) bool {
	return strings.Contains(text, term) || strings.Contains(term, text)
}

func recallTerms(normalized string) []string {
	seen := map[string]bool{}
	var terms []string
	for _, term := range strings.Fields(normalized) {
		if len([]rune(term)) < 3 || recallStopWords[term] || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func normalizeRecallText(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	).Replace(value)
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}
