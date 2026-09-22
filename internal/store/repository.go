package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const sqliteTimeLayout = "2006-01-02T15:04:05.000Z"

func parseTime(s string) time.Time {
	t, err := time.Parse(sqliteTimeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// --- query_history -----------------------------------------------------

func (s *Store) RecordQuery(ctx context.Context, e QueryHistoryEntry) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO query_history (tool, sql_text, agent, duration_ms, row_count, truncated, error)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Tool, e.SQL, nullIfEmpty(e.Agent), e.DurationMS, e.RowCount, boolToInt(e.Truncated), nullIfEmpty(e.Error))
	if err != nil {
		return 0, fmt.Errorf("guardando query_history: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) RecentQueries(ctx context.Context, limit int) ([]QueryHistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tool, sql_text, IFNULL(agent,''), duration_ms, row_count, truncated, IFNULL(error,''), executed_at
		FROM query_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QueryHistoryEntry
	for rows.Next() {
		var e QueryHistoryEntry
		var truncated int
		var executedAt string
		if err := rows.Scan(&e.ID, &e.Tool, &e.SQL, &e.Agent, &e.DurationMS, &e.RowCount, &truncated, &e.Error, &executedAt); err != nil {
			return nil, err
		}
		e.Truncated = truncated != 0
		e.ExecutedAt = parseTime(executedAt)
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- audit_log -----------------------------------------------------------

func (s *Store) RecordAudit(ctx context.Context, e AuditEntry) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (tool, agent, action, detail_json, error)
		VALUES (?, ?, ?, ?, ?)`,
		e.Tool, nullIfEmpty(e.Agent), e.Action, nullIfEmpty(e.Detail), nullIfEmpty(e.Error))
	if err != nil {
		return fmt.Errorf("guardando audit_log: %w", err)
	}
	return nil
}

// --- tool_usage --------------------------------------------------------

// RecordToolUsage incrementa el contador de uso de una tool en una sola
// escritura atómica (UPSERT), para que llamadas concurrentes nunca se
// pisen entre sí como podría pasar con un read-then-write desde Go.
func (s *Store) RecordToolUsage(ctx context.Context, tool string, isError bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tool_usage (tool, call_count, error_count, first_used_at, last_used_at)
		VALUES (?, 1, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'), strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		ON CONFLICT (tool) DO UPDATE SET
			call_count = call_count + 1,
			error_count = error_count + excluded.error_count,
			last_used_at = excluded.last_used_at`,
		tool, boolToInt(isError))
	if err != nil {
		return fmt.Errorf("registrando uso de tool %q: %w", tool, err)
	}
	return nil
}

// SeedToolUsage crea una fila en 0 para cada tool registrada que todavía no
// tenga contador (INSERT OR IGNORE), para que las estadísticas siempre
// muestren las 19 tools del catálogo -- incluidas las que nunca se han
// llamado -- y no solo las que ya generaron al menos un uso.
func (s *Store) SeedToolUsage(ctx context.Context, tools []string) error {
	for _, tool := range tools {
		if _, err := s.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO tool_usage (tool, call_count, error_count) VALUES (?, 0, 0)`, tool); err != nil {
			return fmt.Errorf("sembrando tool_usage para %q: %w", tool, err)
		}
	}
	return nil
}

// ToolUsageStats devuelve el uso de todas las tools, ordenado de más a
// menos usada (empatadas, alfabético) para que la tool menos usada -- o
// nunca usada -- sea fácil de encontrar al final del listado.
func (s *Store) ToolUsageStats(ctx context.Context) ([]ToolUsage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tool, call_count, error_count, first_used_at, last_used_at
		FROM tool_usage
		ORDER BY call_count DESC, tool ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ToolUsage
	for rows.Next() {
		var u ToolUsage
		var firstUsedAt, lastUsedAt sql.NullString
		if err := rows.Scan(&u.Tool, &u.CallCount, &u.ErrorCount, &firstUsedAt, &lastUsedAt); err != nil {
			return nil, err
		}
		if firstUsedAt.Valid {
			t := parseTime(firstUsedAt.String)
			u.FirstUsedAt = &t
		}
		if lastUsedAt.Valid {
			t := parseTime(lastUsedAt.String)
			u.LastUsedAt = &t
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- observations ----------------------------------------------------------

func (s *Store) RecordObservation(ctx context.Context, o Observation) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO observations (subject, kind, statement, context, evidence_json, related_queries_json, agent)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.Subject, o.Kind, o.Statement, nullIfEmpty(o.Context), nullIfEmpty(o.EvidenceJSON), nullIfEmpty(o.RelatedQueriesJSON), nullIfEmpty(o.Agent))
	if err != nil {
		return 0, fmt.Errorf("guardando observation: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) SearchObservations(ctx context.Context, subjectLike string, limit int) ([]Observation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, subject, kind, statement, IFNULL(context,''), IFNULL(evidence_json,''),
		       IFNULL(related_queries_json,''), IFNULL(agent,''), created_at
		FROM observations
		WHERE subject LIKE ? OR statement LIKE ?
		ORDER BY id DESC LIMIT ?`, "%"+subjectLike+"%", "%"+subjectLike+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Observation
	for rows.Next() {
		var o Observation
		var createdAt string
		if err := rows.Scan(&o.ID, &o.Subject, &o.Kind, &o.Statement, &o.Context, &o.EvidenceJSON,
			&o.RelatedQueriesJSON, &o.Agent, &createdAt); err != nil {
			return nil, err
		}
		o.CreatedAt = parseTime(createdAt)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) GetObservations(ctx context.Context, ids []int64) ([]Observation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []Observation
	for _, id := range ids {
		row := s.db.QueryRowContext(ctx, `
			SELECT id, subject, kind, statement, IFNULL(context,''), IFNULL(evidence_json,''),
			       IFNULL(related_queries_json,''), IFNULL(agent,''), created_at
			FROM observations WHERE id = ?`, id)
		var o Observation
		var createdAt string
		if err := row.Scan(&o.ID, &o.Subject, &o.Kind, &o.Statement, &o.Context, &o.EvidenceJSON,
			&o.RelatedQueriesJSON, &o.Agent, &createdAt); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		o.CreatedAt = parseTime(createdAt)
		out = append(out, o)
	}
	return out, nil
}

// --- knowledge -------------------------------------------------------------

const knowledgeColumns = `id, subject, IFNULL(context,''), claim, IFNULL(evidence_json,''), IFNULL(source_json,''),
	       confidence, status, version, supersedes_id, IFNULL(source_observation_ids_json,''), created_at,
	       IFNULL(taught_by,''), IFNULL(decided_by,''), decided_at, IFNULL(decision_note,'')`

func (s *Store) ProposeKnowledge(ctx context.Context, k Knowledge) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO knowledge (subject, context, claim, evidence_json, source_json, confidence, status, version, supersedes_id, source_observation_ids_json, taught_by)
		VALUES (?, ?, ?, ?, ?, ?, 'proposed', ?, ?, ?, ?)`,
		k.Subject, nullIfEmpty(k.Context), k.Claim, nullIfEmpty(k.EvidenceJSON), nullIfEmpty(k.SourceJSON),
		k.Confidence, k.Version, k.SupersedesID, nullIfEmpty(k.SourceObservationIDsJSON), nullIfEmpty(k.TaughtBy))
	if err != nil {
		return 0, fmt.Errorf("guardando propuesta de conocimiento: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) GetKnowledge(ctx context.Context, id int64) (*Knowledge, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+knowledgeColumns+` FROM knowledge WHERE id = ?`, id)
	return scanKnowledge(row)
}

// FindValidatedKnowledge busca conocimiento YA VALIDADO para un concepto en
// un contexto concreto. Es la consulta que debe hacer un agente antes de
// asumir o inventar de dónde sale un dato: si no devuelve nada, la
// respuesta correcta es preguntarle al usuario (ver tool aprender_del_usuario),
// no adivinar.
func (s *Store) FindValidatedKnowledge(ctx context.Context, subject, context_ string) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+knowledgeColumns+`
		FROM knowledge
		WHERE status = 'validated' AND subject = ? AND (context = ? OR ? = '' OR context IS NULL)
		ORDER BY version DESC`, subject, context_, context_)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Knowledge
	for rows.Next() {
		k, err := scanKnowledgeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (s *Store) PendingKnowledge(ctx context.Context, limit int) ([]Knowledge, error) {
	return s.knowledgeByStatus(ctx, KnowledgeStatusProposed, limit)
}

func (s *Store) knowledgeByStatus(ctx context.Context, status string, limit int) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+knowledgeColumns+`
		FROM knowledge WHERE status = ? ORDER BY id DESC LIMIT ?`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Knowledge
	for rows.Next() {
		k, err := scanKnowledgeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (s *Store) SearchKnowledge(ctx context.Context, q string, limit int) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+knowledgeColumns+`
		FROM knowledge WHERE subject LIKE ? OR claim LIKE ? OR context LIKE ?
		ORDER BY (status = 'validated') DESC, id DESC LIMIT ?`, "%"+q+"%", "%"+q+"%", "%"+q+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Knowledge
	for rows.Next() {
		k, err := scanKnowledgeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

// ApproveKnowledge marca una propuesta como validada. Crea una nueva fila en
// lugar de sobrescribir cuando existe evolución (ver propose_knowledge con
// supersedes_id), preservando el historial completo de versiones.
func (s *Store) ApproveKnowledge(ctx context.Context, id int64, approvedBy, note string) (*Knowledge, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE knowledge
		SET status = 'validated', decided_by = ?, decided_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'), decision_note = ?
		WHERE id = ? AND status = 'proposed'`, approvedBy, nullIfEmpty(note), id)
	if err != nil {
		return nil, fmt.Errorf("aprobando conocimiento: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("no existe una propuesta #%d en estado 'proposed'", id)
	}
	return s.GetKnowledge(ctx, id)
}

// RejectKnowledge rechaza una propuesta conservando el registro (nunca se
// elimina silenciosamente, según sección 9 / reject_knowledge del spec).
func (s *Store) RejectKnowledge(ctx context.Context, id int64, rejectedBy, note string) (*Knowledge, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE knowledge
		SET status = 'rejected', decided_by = ?, decided_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'), decision_note = ?
		WHERE id = ? AND status = 'proposed'`, rejectedBy, nullIfEmpty(note), id)
	if err != nil {
		return nil, fmt.Errorf("rechazando conocimiento: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("no existe una propuesta #%d en estado 'proposed'", id)
	}
	return s.GetKnowledge(ctx, id)
}

// --- entity_aliases ---------------------------------------------------------

// UpsertEntityAlias crea o REEMPLAZA el alias de una tabla (columnName="")
// o columna. Se aplica de inmediato: no requiere aprobación humana, porque
// es solo el nombre con el que se presenta la entidad, no una afirmación de
// negocio sobre de dónde sale un dato. Al resolverse en tiempo de lectura
// (ver AllEntityAliases), corregir un alias existente actualiza
// automáticamente cómo se muestra todo el conocimiento ya enseñado que
// referencia esa tabla/columna.
func (s *Store) UpsertEntityAlias(ctx context.Context, tableName, columnName, alias, taughtBy string) (*EntityAlias, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO entity_aliases (table_name, column_name, alias, taught_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (table_name, COALESCE(column_name, '')) DO UPDATE SET
			alias = excluded.alias,
			taught_by = excluded.taught_by,
			updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
		tableName, nullIfEmpty(columnName), alias, nullIfEmpty(taughtBy))
	if err != nil {
		return nil, fmt.Errorf("guardando alias: %w", err)
	}
	return s.GetEntityAlias(ctx, tableName, columnName)
}

func (s *Store) GetEntityAlias(ctx context.Context, tableName, columnName string) (*EntityAlias, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, table_name, IFNULL(column_name,''), alias, IFNULL(taught_by,''), created_at, updated_at
		FROM entity_aliases WHERE table_name = ? AND IFNULL(column_name,'') = ?`,
		tableName, columnName)
	var a EntityAlias
	var createdAt, updatedAt string
	if err := row.Scan(&a.ID, &a.TableName, &a.ColumnName, &a.Alias, &a.TaughtBy, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return &a, nil
}

// AllEntityAliases devuelve todos los alias enseñados, usados para resolver
// nombre real -> alias en cada respuesta de las demás tools.
func (s *Store) AllEntityAliases(ctx context.Context) ([]EntityAlias, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, table_name, IFNULL(column_name,''), alias, IFNULL(taught_by,''), created_at, updated_at
		FROM entity_aliases ORDER BY table_name, column_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EntityAlias
	for rows.Next() {
		var a EntityAlias
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.TableName, &a.ColumnName, &a.Alias, &a.TaughtBy, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKnowledge(row *sql.Row) (*Knowledge, error) {
	return scanKnowledgeRows(row)
}

func scanKnowledgeRows(sc rowScanner) (*Knowledge, error) {
	var k Knowledge
	var createdAt string
	var decidedAt sql.NullString
	var confidence sql.NullFloat64
	var supersedes sql.NullInt64

	if err := sc.Scan(&k.ID, &k.Subject, &k.Context, &k.Claim, &k.EvidenceJSON, &k.SourceJSON, &confidence, &k.Status, &k.Version,
		&supersedes, &k.SourceObservationIDsJSON, &createdAt, &k.TaughtBy, &k.DecidedBy, &decidedAt, &k.DecisionNote); err != nil {
		return nil, err
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
	return &k, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
