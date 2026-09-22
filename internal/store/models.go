package store

import "time"

type QueryHistoryEntry struct {
	ID         int64     `json:"id"`
	Tool       string    `json:"tool"`
	SQL        string    `json:"sql"`
	Agent      string    `json:"agent,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	RowCount   int       `json:"row_count"`
	Truncated  bool      `json:"truncated"`
	Error      string    `json:"error,omitempty"`
	ExecutedAt time.Time `json:"executed_at"`
}

// ToolUsage es el contador agregado de llamadas a una tool MCP. CallCount
// incluye tanto llamadas exitosas como fallidas; ErrorCount es el subconjunto
// que terminó en error, para distinguir "no se usa" de "se usa pero falla".
type ToolUsage struct {
	Tool        string     `json:"tool"`
	CallCount   int64      `json:"call_count"`
	ErrorCount  int64      `json:"error_count"`
	FirstUsedAt *time.Time `json:"first_used_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}

type AuditEntry struct {
	ID     int64     `json:"id"`
	TS     time.Time `json:"ts"`
	Tool   string    `json:"tool"`
	Agent  string    `json:"agent,omitempty"`
	Action string    `json:"action"`
	Detail string    `json:"detail,omitempty"` // JSON crudo
	Error  string    `json:"error,omitempty"`
}

type Observation struct {
	ID                 int64     `json:"id"`
	Subject            string    `json:"subject"`
	Kind               string    `json:"kind"` // observation | hypothesis
	Statement          string    `json:"statement"`
	Context            string    `json:"context,omitempty"`
	EvidenceJSON       string    `json:"evidence,omitempty"`
	RelatedQueriesJSON string    `json:"related_queries,omitempty"`
	Agent              string    `json:"agent,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

const (
	KnowledgeStatusProposed   = "proposed"
	KnowledgeStatusValidated  = "validated"
	KnowledgeStatusRejected   = "rejected"
	KnowledgeStatusDeprecated = "deprecated"
)

// Knowledge es un concepto de negocio con estado (proposed/validated/
// rejected/deprecated). Context distingue el mismo concepto en distintos
// ámbitos (ej. "vendedor" en "ventas" puede tener una fuente distinta que
// "vendedor" en "compras"): nunca se extrapola entre contextos sin
// evidencia propia. SourceJSON guarda de forma estructurada lo enseñado por
// un humano (tablas/columnas/relación/fórmula) y lo verificado contra el
// schema real — ver internal/mcpserver/knowledge_source.go.
// EntityAlias es el nombre legible que un humano le da a una tabla o
// columna real, para que los asistentes MCP hablen de ella sin exponer el
// nombre crudo de base de datos. ColumnName vacío significa alias de tabla
// completa. Se aplica de inmediato (sin aprobación): es solo un nombre de
// presentación, no una afirmación de negocio.
type EntityAlias struct {
	ID         int64     `json:"id"`
	TableName  string    `json:"table_name"`
	ColumnName string    `json:"column_name,omitempty"`
	Alias      string    `json:"alias"`
	TaughtBy   string    `json:"taught_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Knowledge struct {
	ID                       int64      `json:"id"`
	Subject                  string     `json:"subject"`
	Context                  string     `json:"context,omitempty"`
	Claim                    string     `json:"claim"`
	EvidenceJSON             string     `json:"evidence,omitempty"`
	SourceJSON               string     `json:"source,omitempty"`
	Confidence               *float64   `json:"confidence,omitempty"`
	Status                   string     `json:"status"`
	Version                  int        `json:"version"`
	SupersedesID             *int64     `json:"supersedes_id,omitempty"`
	SourceObservationIDsJSON string     `json:"source_observation_ids,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	TaughtBy                 string     `json:"taught_by,omitempty"`
	DecidedBy                string     `json:"decided_by,omitempty"`
	DecidedAt                *time.Time `json:"decided_at,omitempty"`
	DecisionNote             string     `json:"decision_note,omitempty"`
}
