// Package schema descubre e indexa automáticamente la estructura de la base
// de datos (tablas, columnas, PK, FK, índices) y mantiene una caché en
// memoria para no golpear INFORMATION_SCHEMA en cada pregunta.
package schema

import "time"

type Column struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Nullable   bool        `json:"nullable"`
	Default    *string     `json:"default,omitempty"`
	Comment    string      `json:"comment,omitempty"`
	PrimaryKey bool        `json:"primary_key,omitempty"`
	ForeignKey *ForeignKey `json:"foreign_key,omitempty"`
	OrdinalPos int         `json:"-"`
}

type ForeignKey struct {
	ConstraintName string `json:"constraint_name"`
	Table          string `json:"table"`
	Column         string `json:"column"`
}

type Index struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

type Table struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"` // BASE TABLE | VIEW
	Comment     string       `json:"comment,omitempty"`
	Columns     []Column     `json:"columns"`
	Indexes     []Index      `json:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
	// ReferencedBy lista qué tablas.columnas apuntan hacia esta tabla (FKs
	// entrantes), útil para get_relationships sin tener que recorrer todo
	// el schema en cada consulta.
	ReferencedBy []Reference `json:"referenced_by,omitempty"`
	RowEstimate  int64       `json:"row_estimate,omitempty"`
}

type Reference struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// Snapshot es la representación completa e inmutable del schema en un
// momento dado.
type Snapshot struct {
	Database    string            `json:"database"`
	Tables      map[string]*Table `json:"tables"`
	RefreshedAt time.Time         `json:"refreshed_at"`
}

// Diff describe los cambios detectados entre dos snapshots (sección 22 del
// spec: refresh_schema debe poder informar "Schema changed since last scan").
type Diff struct {
	TablesAdded   []string      `json:"tables_added,omitempty"`
	TablesRemoved []string      `json:"tables_removed,omitempty"`
	TablesChanged []TableChange `json:"tables_changed,omitempty"`
}

type TableChange struct {
	Table              string   `json:"table"`
	ColumnsAdded       []string `json:"columns_added,omitempty"`
	ColumnsRemoved     []string `json:"columns_removed,omitempty"`
	ColumnsRetyped     []string `json:"columns_retyped,omitempty"`
	IndexesChanged     bool     `json:"indexes_changed,omitempty"`
	ForeignKeysChanged bool     `json:"foreign_keys_changed,omitempty"`
}

func (d *Diff) HasChanges() bool {
	return len(d.TablesAdded) > 0 || len(d.TablesRemoved) > 0 || len(d.TablesChanged) > 0
}
