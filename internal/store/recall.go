package store

import (
	"context"
	"sort"
	"strings"
	"unicode"
)

const maxRecallCandidates = 1000

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

// RecallValidatedKnowledge recupera el conocimiento validado relevante para
// una petición escrita en lenguaje natural. A diferencia de SearchKnowledge,
// no exige que la petición completa aparezca literalmente en subject/claim:
// separa términos, normaliza tildes y ordena por coincidencia. Esto permite
// reconstruir el contexto al comenzar una sesión sin volver a explorar el
// schema para una tarea que el MCP ya conoce.
func (s *Store) RecallValidatedKnowledge(ctx context.Context, task, contextHint string, limit int) ([]Knowledge, error) {
	if limit <= 0 {
		limit = 20
	}

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

	query := normalizeRecallText(strings.TrimSpace(task + " " + contextHint))
	if query == "" {
		return []Knowledge{}, nil
	}
	terms := recallTerms(query)
	candidates := make([]recallCandidate, 0)
	for rows.Next() {
		k, err := scanKnowledgeRows(rows)
		if err != nil {
			return nil, err
		}
		score := scoreRecallCandidate(*k, query, normalizeRecallText(contextHint), terms)
		if score > 0 {
			candidates = append(candidates, recallCandidate{knowledge: *k, score: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].knowledge.Version != candidates[j].knowledge.Version {
			return candidates[i].knowledge.Version > candidates[j].knowledge.Version
		}
		return candidates[i].knowledge.ID > candidates[j].knowledge.ID
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]Knowledge, len(candidates))
	for i, candidate := range candidates {
		out[i] = candidate.knowledge
	}
	return out, nil
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
