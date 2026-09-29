package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// PassiveAgent marca en observations.agent las filas generadas solas por
	// la captura pasiva (no las escribió una persona ni un asistente).
	PassiveAgent = "passive"

	// passiveWindow: un mismo conjunto de tablas vuelve a generar observación
	// solo si pasó al menos esta ventana desde la última observación.
	passiveWindow = 24 * time.Hour

	// passiveMaxPerHour: tope global de observaciones pasivas nuevas por hora
	// (red de seguridad si aparecen muchos conjuntos de tablas distintos).
	passiveMaxPerHour = 20

	// passiveMaxTables acota el conjunto guardado (subject/evidencia).
	passiveMaxTables = 20

	// Mismo formato que el DEFAULT de created_at en las migraciones, para que
	// las comparaciones de texto entre ambos sean consistentes.
	passiveTimeFormat = "2006-01-02T15:04:05.000Z"
)

// clock devuelve la hora actual; se puede inyectar en tests.
func (s *Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// CapturePassiveQuery registra el paso de una consulta exitosa por un conjunto
// de tablas. Siempre suma hits/last_seen; inserta una OBSERVACIÓN (nunca
// conocimiento, jamás validado) solo la primera vez que se ve el conjunto o
// tras passiveWindow, y respeta el tope global passiveMaxPerHour. Guarda
// únicamente nombres de tablas: nunca SQL, literales ni filas (PII).
// Devuelve true si se creó una observación.
func (s *Store) CapturePassiveQuery(ctx context.Context, tables []string) (bool, error) {
	set := normalizeTableSet(tables)
	if len(set) == 0 {
		return false, nil
	}
	key := strings.Join(set, ",")
	now := s.clock().UTC()
	nowStr := now.Format(passiveTimeFormat)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("iniciando captura pasiva: %w", err)
	}
	defer tx.Rollback()

	var lastObserved *string
	err = tx.QueryRowContext(ctx, `SELECT last_observed FROM passive_capture WHERE table_set = ?`, key).Scan(&lastObserved)
	seen := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("leyendo passive_capture: %w", err)
	}

	due := !seen || lastObserved == nil
	if !due {
		if t, perr := time.Parse(passiveTimeFormat, *lastObserved); perr != nil || now.Sub(t) >= passiveWindow {
			due = true
		}
	}

	created := false
	if due {
		var recent int
		since := now.Add(-time.Hour).Format(passiveTimeFormat)
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM observations WHERE agent = ? AND created_at >= ?`, PassiveAgent, since).Scan(&recent); err != nil {
			return false, fmt.Errorf("contando observaciones pasivas: %w", err)
		}
		if recent < passiveMaxPerHour {
			evidence, _ := json.Marshal(map[string][]string{"tablas": set})
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO observations (subject, kind, statement, evidence_json, agent, created_at)
				VALUES (?, 'observation', ?, ?, ?, ?)`,
				"consulta:"+key,
				"Consulta exitosa que involucra las tablas: "+strings.Join(set, ", "),
				string(evidence), PassiveAgent, nowStr); err != nil {
				return false, fmt.Errorf("guardando observación pasiva: %w", err)
			}
			created = true
		}
	}

	var observed any
	if created {
		observed = nowStr
	}
	// En el conflicto, last_observed solo se pisa si se creó observación ahora.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO passive_capture (table_set, first_seen, last_seen, last_observed, hits)
		VALUES (?, ?, ?, ?, 1)
		ON CONFLICT(table_set) DO UPDATE SET
			last_seen = excluded.last_seen,
			hits = hits + 1,
			last_observed = COALESCE(excluded.last_observed, last_observed)`,
		key, nowStr, nowStr, observed); err != nil {
		return false, fmt.Errorf("actualizando passive_capture: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("confirmando captura pasiva: %w", err)
	}
	return created, nil
}

// normalizeTableSet pasa a minúsculas, quita vacíos y duplicados, ordena y
// acota el conjunto para que la misma combinación siempre dé la misma clave.
func normalizeTableSet(tables []string) []string {
	seen := make(map[string]struct{}, len(tables))
	out := make([]string, 0, len(tables))
	for _, t := range tables {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	if len(out) > passiveMaxTables {
		out = out[:passiveMaxTables]
	}
	return out
}
