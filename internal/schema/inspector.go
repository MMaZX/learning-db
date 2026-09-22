package schema

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

// Inspector consulta INFORMATION_SCHEMA y mantiene una caché en memoria del
// schema descubierto. Es seguro para uso concurrente.
type Inspector struct {
	pool *sql.DB

	excludedTables  map[string]bool
	excludedColumns map[string]bool

	mu       sync.RWMutex
	snapshot *Snapshot
}

func NewInspector(pool *sql.DB, excludedTables, excludedColumns map[string]bool) *Inspector {
	return &Inspector{
		pool:            pool,
		excludedTables:  excludedTables,
		excludedColumns: excludedColumns,
	}
}

// Current devuelve la última snapshot cacheada, inspeccionando por primera
// vez si aún no existe.
func (i *Inspector) Current(ctx context.Context) (*Snapshot, error) {
	i.mu.RLock()
	snap := i.snapshot
	i.mu.RUnlock()
	if snap != nil {
		return snap, nil
	}
	return i.Refresh(ctx)
}

// Refresh vuelve a consultar INFORMATION_SCHEMA, reemplaza la caché y
// devuelve un Diff contra la snapshot anterior (nil si es la primera vez).
func (i *Inspector) Refresh(ctx context.Context) (*Snapshot, error) {
	snap, err := i.inspect(ctx)
	if err != nil {
		return nil, err
	}
	i.mu.Lock()
	i.snapshot = snap
	i.mu.Unlock()
	return snap, nil
}

// RefreshWithDiff hace lo mismo que Refresh pero además calcula el Diff
// contra la snapshot previa.
func (i *Inspector) RefreshWithDiff(ctx context.Context) (*Snapshot, *Diff, error) {
	i.mu.RLock()
	previous := i.snapshot
	i.mu.RUnlock()

	snap, err := i.inspect(ctx)
	if err != nil {
		return nil, nil, err
	}
	i.mu.Lock()
	i.snapshot = snap
	i.mu.Unlock()

	return snap, computeDiff(previous, snap), nil
}

func (i *Inspector) isExcluded(table string) bool {
	return i.excludedTables[table]
}

func (i *Inspector) inspect(ctx context.Context) (*Snapshot, error) {
	var dbName string
	if err := i.pool.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&dbName); err != nil {
		return nil, fmt.Errorf("obteniendo base de datos activa: %w", err)
	}

	tables, err := i.loadTables(ctx, dbName)
	if err != nil {
		return nil, err
	}
	if err := i.loadColumns(ctx, dbName, tables); err != nil {
		return nil, err
	}
	if err := i.loadPrimaryKeys(ctx, dbName, tables); err != nil {
		return nil, err
	}
	if err := i.loadForeignKeys(ctx, dbName, tables); err != nil {
		return nil, err
	}
	if err := i.loadIndexes(ctx, dbName, tables); err != nil {
		return nil, err
	}

	// Construir ReferencedBy (FKs entrantes) a partir de las FKs salientes ya
	// cargadas, para poder responder get_relationships en ambas direcciones.
	for tname, t := range tables {
		for _, fk := range t.ForeignKeys {
			if target, ok := tables[fk.Table]; ok {
				target.ReferencedBy = append(target.ReferencedBy, Reference{Table: tname, Column: fk.Column})
			}
		}
	}

	snap := &Snapshot{
		Database: dbName,
		Tables:   tables,
	}
	snap.RefreshedAt = nowUTC()
	return snap, nil
}

func (i *Inspector) loadTables(ctx context.Context, dbName string) (map[string]*Table, error) {
	rows, err := i.pool.QueryContext(ctx, `
		SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_COMMENT, ''), IFNULL(TABLE_ROWS, 0)
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME`, dbName)
	if err != nil {
		return nil, fmt.Errorf("listando tablas: %w", err)
	}
	defer rows.Close()

	tables := make(map[string]*Table)
	for rows.Next() {
		var name, tableType, comment string
		var rowEstimate int64
		if err := rows.Scan(&name, &tableType, &comment, &rowEstimate); err != nil {
			return nil, err
		}
		if i.isExcluded(name) {
			continue
		}
		tables[name] = &Table{
			Name:        name,
			Type:        tableType,
			Comment:     comment,
			RowEstimate: rowEstimate,
		}
	}
	return tables, rows.Err()
}

func (i *Inspector) loadColumns(ctx context.Context, dbName string, tables map[string]*Table) error {
	rows, err := i.pool.QueryContext(ctx, `
		SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT,
		       IFNULL(COLUMN_COMMENT, ''), ORDINAL_POSITION
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, ORDINAL_POSITION`, dbName)
	if err != nil {
		return fmt.Errorf("listando columnas: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, colType, isNullable, comment string
		var colDefault sql.NullString
		var ordinal int
		if err := rows.Scan(&tableName, &colName, &colType, &isNullable, &colDefault, &comment, &ordinal); err != nil {
			return err
		}
		t, ok := tables[tableName]
		if !ok {
			continue
		}
		if i.excludedColumns[colName] {
			continue
		}
		col := Column{
			Name:       colName,
			Type:       colType,
			Nullable:   isNullable == "YES",
			Comment:    comment,
			OrdinalPos: ordinal,
		}
		if colDefault.Valid {
			col.Default = &colDefault.String
		}
		t.Columns = append(t.Columns, col)
	}
	return rows.Err()
}

func (i *Inspector) loadPrimaryKeys(ctx context.Context, dbName string, tables map[string]*Table) error {
	rows, err := i.pool.QueryContext(ctx, `
		SELECT TABLE_NAME, COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY TABLE_NAME, ORDINAL_POSITION`, dbName)
	if err != nil {
		return fmt.Errorf("listando primary keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName string
		if err := rows.Scan(&tableName, &colName); err != nil {
			return err
		}
		t, ok := tables[tableName]
		if !ok {
			continue
		}
		for idx := range t.Columns {
			if t.Columns[idx].Name == colName {
				t.Columns[idx].PrimaryKey = true
			}
		}
	}
	return rows.Err()
}

func (i *Inspector) loadForeignKeys(ctx context.Context, dbName string, tables map[string]*Table) error {
	rows, err := i.pool.QueryContext(ctx, `
		SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
		FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY TABLE_NAME, ORDINAL_POSITION`, dbName)
	if err != nil {
		return fmt.Errorf("listando foreign keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, colName, constraintName, refTable, refColumn string
		if err := rows.Scan(&tableName, &colName, &constraintName, &refTable, &refColumn); err != nil {
			return err
		}
		t, ok := tables[tableName]
		if !ok {
			continue
		}
		if _, ok := tables[refTable]; !ok {
			continue // tabla referenciada excluida/no visible
		}
		fk := ForeignKey{ConstraintName: constraintName, Table: refTable, Column: refColumn}
		t.ForeignKeys = append(t.ForeignKeys, fk)
		for idx := range t.Columns {
			if t.Columns[idx].Name == colName {
				fkCopy := fk
				t.Columns[idx].ForeignKey = &fkCopy
			}
		}
	}
	return rows.Err()
}

func (i *Inspector) loadIndexes(ctx context.Context, dbName string, tables map[string]*Table) error {
	rows, err := i.pool.QueryContext(ctx, `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, dbName)
	if err != nil {
		return fmt.Errorf("listando índices: %w", err)
	}
	defer rows.Close()

	type key struct{ table, index string }
	byIndex := make(map[key]*Index)
	order := make(map[string][]string) // table -> ordered index names

	for rows.Next() {
		var tableName, indexName, colName string
		var nonUnique int
		if err := rows.Scan(&tableName, &indexName, &nonUnique, &colName); err != nil {
			return err
		}
		if _, ok := tables[tableName]; !ok {
			continue
		}
		k := key{tableName, indexName}
		idx, ok := byIndex[k]
		if !ok {
			idx = &Index{Name: indexName, Unique: nonUnique == 0}
			byIndex[k] = idx
			order[tableName] = append(order[tableName], indexName)
		}
		idx.Columns = append(idx.Columns, colName)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for tableName, names := range order {
		t := tables[tableName]
		seen := map[string]bool{}
		for _, n := range names {
			if seen[n] {
				continue
			}
			seen[n] = true
			t.Indexes = append(t.Indexes, *byIndex[key{tableName, n}])
		}
	}
	return nil
}

func computeDiff(prev, curr *Snapshot) *Diff {
	diff := &Diff{}
	if prev == nil {
		return diff
	}
	for name := range curr.Tables {
		if _, ok := prev.Tables[name]; !ok {
			diff.TablesAdded = append(diff.TablesAdded, name)
		}
	}
	for name := range prev.Tables {
		if _, ok := curr.Tables[name]; !ok {
			diff.TablesRemoved = append(diff.TablesRemoved, name)
		}
	}
	for name, currT := range curr.Tables {
		prevT, ok := prev.Tables[name]
		if !ok {
			continue
		}
		change := diffTable(prevT, currT)
		if change != nil {
			diff.TablesChanged = append(diff.TablesChanged, *change)
		}
	}
	sort.Strings(diff.TablesAdded)
	sort.Strings(diff.TablesRemoved)
	sort.Slice(diff.TablesChanged, func(a, b int) bool { return diff.TablesChanged[a].Table < diff.TablesChanged[b].Table })
	return diff
}

func diffTable(prev, curr *Table) *TableChange {
	prevCols := colMap(prev.Columns)
	currCols := colMap(curr.Columns)

	change := TableChange{Table: curr.Name}
	for name, col := range currCols {
		prevCol, ok := prevCols[name]
		if !ok {
			change.ColumnsAdded = append(change.ColumnsAdded, name)
			continue
		}
		if prevCol.Type != col.Type {
			change.ColumnsRetyped = append(change.ColumnsRetyped, name)
		}
	}
	for name := range prevCols {
		if _, ok := currCols[name]; !ok {
			change.ColumnsRemoved = append(change.ColumnsRemoved, name)
		}
	}
	change.IndexesChanged = fmt.Sprintf("%v", prev.Indexes) != fmt.Sprintf("%v", curr.Indexes)
	change.ForeignKeysChanged = fmt.Sprintf("%v", prev.ForeignKeys) != fmt.Sprintf("%v", curr.ForeignKeys)

	if len(change.ColumnsAdded) == 0 && len(change.ColumnsRemoved) == 0 &&
		len(change.ColumnsRetyped) == 0 && !change.IndexesChanged && !change.ForeignKeysChanged {
		return nil
	}
	sort.Strings(change.ColumnsAdded)
	sort.Strings(change.ColumnsRemoved)
	sort.Strings(change.ColumnsRetyped)
	return &change
}

func colMap(cols []Column) map[string]Column {
	m := make(map[string]Column, len(cols))
	for _, c := range cols {
		m[c.Name] = c
	}
	return m
}
