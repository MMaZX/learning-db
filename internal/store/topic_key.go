package store

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode"
)

var topicKeyAccents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ñ", "n", "ç", "c",
)

// TopicKey devuelve la identidad estable de un concepto: subject y context
// normalizados (minúsculas, sin tildes, cada racha de espacios pasa a "-",
// sin espacios en los extremos) unidos por "@". Un context vacío da
// "subject@". El "@" dentro de cada parte se reemplaza por "-" para que la
// clave sea inequívoca.
func TopicKey(subject, context string) string {
	return normalizeTopicPart(subject) + "@" + normalizeTopicPart(context)
}

func normalizeTopicPart(v string) string {
	v = topicKeyAccents.Replace(strings.ToLower(v))
	v = strings.ReplaceAll(v, "@", "-")
	return strings.Join(strings.FieldsFunc(v, unicode.IsSpace), "-")
}

// ensureTopicKeys rellena topic_key en las filas que aún no lo tienen y crea
// el índice único parcial de conceptos validados. Es idempotente y nunca
// hace fallar el arranque por datos previos duplicados.
//
// Duplicados validados preexistentes (mismo topic_key, p.ej. una cadena v1 y
// v2 aprobadas antes de que existiera el índice): se conserva la clave en la
// fila validada de mayor version (desempate por id mayor) y las demás quedan
// con topic_key NULL. No se borra ni se cambia el status de nada; las filas
// sin clave siguen siendo visibles para recall y FindValidatedKnowledge
// mediante el fallback por subject/context.
func (s *Store) ensureTopicKeys(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	held := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT topic_key FROM knowledge WHERE status = 'validated' AND topic_key IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("leyendo topic_key existentes: %w", err)
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return err
		}
		held[key] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	type pending struct {
		id        int64
		key       string
		validated bool
	}
	var todo []pending
	rows, err = tx.QueryContext(ctx, `
		SELECT id, subject, IFNULL(context,''), status = 'validated'
		FROM knowledge WHERE topic_key IS NULL
		ORDER BY (status = 'validated') DESC, version DESC, id DESC`)
	if err != nil {
		return fmt.Errorf("leyendo filas sin topic_key: %w", err)
	}
	for rows.Next() {
		var p pending
		var subject, context_ string
		if err := rows.Scan(&p.id, &subject, &context_, &p.validated); err != nil {
			rows.Close()
			return err
		}
		p.key = TopicKey(subject, context_)
		todo = append(todo, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	skipped := 0
	for _, p := range todo {
		if p.validated {
			if held[p.key] {
				skipped++
				continue
			}
			held[p.key] = true
		}
		if _, err := tx.ExecContext(ctx, `UPDATE knowledge SET topic_key = ? WHERE id = ?`, p.key, p.id); err != nil {
			return fmt.Errorf("backfill topic_key #%d: %w", p.id, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_topic_key_validated
		ON knowledge(topic_key) WHERE status = 'validated'`); err != nil {
		return fmt.Errorf("creando índice único de topic_key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if skipped > 0 {
		log.Printf("store: %d fila(s) validada(s) duplicada(s) quedan sin topic_key (se conserva la de mayor version)", skipped)
	}
	return nil
}
