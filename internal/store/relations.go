package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// relationScoreFloor es el score mínimo (-bm25, mayor = más parecido) para
// que una coincidencia FTS de otro topic_key cuente como relación. Se mide
// con los mismos pesos que recallViaFTS (subject 2, claim 1, context 3).
//
// Calibración: un término aporta ~idf*peso. Medido en tests, coincidir en
// subject+claim con términos poco frecuentes da 4-8; un término presente en
// casi todas las filas (idf ~ 0) da ~1e-6. 1.0 equivale a "al menos un
// término del subject/context razonablemente raro o varios de claim": deja
// fuera ruido de términos comunes. Limitación conocida: bm25 depende del
// corpus (la propia propuesta cuenta en N), así que con muy pocas filas
// (<~5) los idf colapsan y solo se detectan relaciones por topic_key.
const relationScoreFloor = 1.0

// maxFTSRelations acota cuántas relaciones derivadas de FTS
// (possible_conflict y previously_rejected por similitud) se guardan por
// propuesta, para que una propuesta genérica no inunde la revisión humana.
const maxFTSRelations = 5

// relationFTSCandidates es cuántas filas candidatas se leen del índice FTS
// antes de aplicar el umbral y el tope (ordenadas por mejor score).
const relationFTSCandidates = 50

// relationsForBatch es cuántos ids de origen se consultan por sentencia en
// RelationsFor (holgado bajo el límite de variables de SQLite).
const relationsForBatch = 500

// DetectKnowledgeRelations calcula y persiste las relaciones de la propuesta
// id con el resto del conocimiento:
//   - mismo topic_key: validated -> same_topic (+ supersedes_candidate si la
//     propuesta aún no apunta a esa fila con supersedes_id), proposed ->
//     same_topic, rejected -> previously_rejected;
//   - distinto topic_key con coincidencia FTS sobre subject/claim/context:
//     validated/proposed -> possible_conflict, rejected -> previously_rejected
//     (score >= relationScoreFloor, máximo maxFTSRelations).
//
// Las filas deprecated y la propia fila se ignoran. La inserción es
// idempotente (UNIQUE from_id,to_id,relation), así que redetectar no
// duplica. Nunca modifica knowledge.
func (s *Store) DetectKnowledgeRelations(ctx context.Context, id int64) ([]KnowledgeRelation, error) {
	k, err := s.GetKnowledge(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("cargando conocimiento %d: %w", id, err)
	}

	var found []KnowledgeRelation
	add := func(to int64, relation string, score *float64) {
		found = append(found, KnowledgeRelation{FromID: id, ToID: to, Relation: relation, Score: score})
	}

	if k.TopicKey != "" {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, status FROM knowledge
			WHERE topic_key = ? AND id <> ? AND status IN ('validated','proposed','rejected')
			ORDER BY id`, k.TopicKey, id)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var other int64
			var status string
			if err := rows.Scan(&other, &status); err != nil {
				rows.Close()
				return nil, err
			}
			switch status {
			case KnowledgeStatusValidated:
				add(other, RelationSameTopic, nil)
				if k.SupersedesID == nil || *k.SupersedesID != other {
					add(other, RelationSupersedesCandidate, nil)
				}
			case KnowledgeStatusProposed:
				add(other, RelationSameTopic, nil)
			case KnowledgeStatusRejected:
				add(other, RelationPreviouslyRejected, nil)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	if ftsQuery := buildFTSQuery(k.Subject, k.Context+" "+k.Claim); ftsQuery != "" {
		fts, err := s.relationsViaFTS(ctx, k, ftsQuery)
		if err != nil {
			return nil, err
		}
		found = append(found, fts...)
	}

	if len(found) == 0 {
		return []KnowledgeRelation{}, nil
	}
	return s.persistRelations(ctx, found)
}

// relationsViaFTS devuelve las relaciones por similitud textual con filas de
// OTRO topic_key, ya filtradas por umbral y tope.
func (s *Store) relationsViaFTS(ctx context.Context, k *Knowledge, ftsQuery string) ([]KnowledgeRelation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT knowledge.id, knowledge.status, bm25(knowledge_fts, 2.0, 1.0, 3.0)
		FROM knowledge_fts
		JOIN knowledge ON knowledge.id = knowledge_fts.rowid
		WHERE knowledge_fts MATCH ?
		  AND knowledge.id <> ?
		  AND IFNULL(knowledge.topic_key,'') <> ?
		  AND knowledge.status IN ('validated','proposed','rejected')
		ORDER BY bm25(knowledge_fts, 2.0, 1.0, 3.0)
		LIMIT ?`, ftsQuery, k.ID, k.TopicKey, relationFTSCandidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KnowledgeRelation
	for rows.Next() {
		var other int64
		var status string
		var rank float64
		if err := rows.Scan(&other, &status, &rank); err != nil {
			return nil, err
		}
		score := -rank // bm25 es más negativo cuanto mejor coincide
		if score < relationScoreFloor {
			// Ordenado por mejor score: el resto está por debajo del umbral.
			break
		}
		if len(out) >= maxFTSRelations {
			break
		}
		relation := RelationPossibleConflict
		if status == KnowledgeStatusRejected {
			relation = RelationPreviouslyRejected
		}
		sc := score
		out = append(out, KnowledgeRelation{FromID: k.ID, ToID: other, Relation: relation, Score: &sc})
	}
	return out, rows.Err()
}

// persistRelations inserta con INSERT OR IGNORE y devuelve cada relación con
// su id/created_at reales (también las que ya existían).
func (s *Store) persistRelations(ctx context.Context, found []KnowledgeRelation) ([]KnowledgeRelation, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for i := range found {
		r := &found[i]
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO knowledge_relations (from_id, to_id, relation, score)
			VALUES (?, ?, ?, ?)`, r.FromID, r.ToID, r.Relation, r.Score); err != nil {
			return nil, fmt.Errorf("guardando relación: %w", err)
		}
		var createdAt string
		if err := tx.QueryRowContext(ctx, `
			SELECT id, created_at FROM knowledge_relations
			WHERE from_id = ? AND to_id = ? AND relation = ?`, r.FromID, r.ToID, r.Relation).Scan(&r.ID, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt = parseTime(createdAt)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return found, nil
}

// RelationsFor devuelve, para cada id de origen, sus relaciones con el
// estado ACTUAL de la fila relacionada (JOIN con knowledge en una sola
// consulta, sin N+1). Los ids sin relaciones no aparecen en el mapa.
func (s *Store) RelationsFor(ctx context.Context, ids []int64) (map[int64][]RelatedKnowledge, error) {
	seen := make(map[int64]struct{}, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		args = append(args, id)
	}
	out := map[int64][]RelatedKnowledge{}
	// SQLite limita las variables por consulta (32766 desde 3.32, 999 antes):
	// se consulta por lotes; cada id de origen cae en un único lote.
	for start := 0; start < len(args); start += relationsForBatch {
		end := min(start+relationsForBatch, len(args))
		if err := s.relationsForBatch(ctx, args[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// relationsForBatch resuelve un lote de ids de origen y acumula en out.
func (s *Store) relationsForBatch(ctx context.Context, args []any, out map[int64][]RelatedKnowledge) error {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.from_id, r.relation, r.score,
		       k.id, k.status, k.subject, IFNULL(k.context,''), k.claim,
		       IFNULL(k.decided_by,''), k.decided_at, IFNULL(k.decision_note,'')
		FROM knowledge_relations r
		JOIN knowledge k ON k.id = r.to_id
		WHERE r.from_id IN (`+placeholders+`)
		ORDER BY r.from_id, r.id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var from int64
		var rel RelatedKnowledge
		var score sql.NullFloat64
		var decidedAt sql.NullString
		if err := rows.Scan(&from, &rel.Relation, &score, &rel.ID, &rel.Status, &rel.Subject, &rel.Context, &rel.Claim,
			&rel.DecidedBy, &decidedAt, &rel.DecisionNote); err != nil {
			return err
		}
		if score.Valid {
			rel.Score = &score.Float64
		}
		if decidedAt.Valid {
			t := parseTime(decidedAt.String)
			rel.DecidedAt = &t
		}
		out[from] = append(out[from], rel)
	}
	return rows.Err()
}
