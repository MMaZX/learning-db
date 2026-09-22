package schema

import "testing"

func TestComputeDiff_NilPreviousMeansNoChanges(t *testing.T) {
	curr := &Snapshot{Tables: map[string]*Table{"pedido": {Name: "pedido"}}}
	diff := computeDiff(nil, curr)
	if diff.HasChanges() {
		t.Error("sin snapshot previo no debería reportar cambios (es la primera inspección)")
	}
}

func TestComputeDiff_DetectsAddedAndRemovedTables(t *testing.T) {
	prev := &Snapshot{Tables: map[string]*Table{
		"pedido":  {Name: "pedido"},
		"cliente": {Name: "cliente"},
	}}
	curr := &Snapshot{Tables: map[string]*Table{
		"pedido": {Name: "pedido"},
		"pago":   {Name: "pago"},
	}}
	diff := computeDiff(prev, curr)

	if !diff.HasChanges() {
		t.Fatal("esperaba cambios")
	}
	if len(diff.TablesAdded) != 1 || diff.TablesAdded[0] != "pago" {
		t.Errorf("TablesAdded = %v, want [pago]", diff.TablesAdded)
	}
	if len(diff.TablesRemoved) != 1 || diff.TablesRemoved[0] != "cliente" {
		t.Errorf("TablesRemoved = %v, want [cliente]", diff.TablesRemoved)
	}
}

func TestComputeDiff_DetectsColumnChanges(t *testing.T) {
	prev := &Snapshot{Tables: map[string]*Table{
		"pedido": {Name: "pedido", Columns: []Column{
			{Name: "id", Type: "int"},
			{Name: "viejo", Type: "varchar(10)"},
		}},
	}}
	curr := &Snapshot{Tables: map[string]*Table{
		"pedido": {Name: "pedido", Columns: []Column{
			{Name: "id", Type: "bigint"}, // cambió de tipo
			{Name: "nuevo", Type: "int"}, // columna añadida
		}},
	}}
	diff := computeDiff(prev, curr)

	if len(diff.TablesChanged) != 1 {
		t.Fatalf("TablesChanged = %v, want 1 tabla", diff.TablesChanged)
	}
	change := diff.TablesChanged[0]
	if len(change.ColumnsAdded) != 1 || change.ColumnsAdded[0] != "nuevo" {
		t.Errorf("ColumnsAdded = %v, want [nuevo]", change.ColumnsAdded)
	}
	if len(change.ColumnsRemoved) != 1 || change.ColumnsRemoved[0] != "viejo" {
		t.Errorf("ColumnsRemoved = %v, want [viejo]", change.ColumnsRemoved)
	}
	if len(change.ColumnsRetyped) != 1 || change.ColumnsRetyped[0] != "id" {
		t.Errorf("ColumnsRetyped = %v, want [id]", change.ColumnsRetyped)
	}
}

func TestComputeDiff_NoChangeWhenIdentical(t *testing.T) {
	table := &Table{Name: "pedido", Columns: []Column{{Name: "id", Type: "int"}}}
	prev := &Snapshot{Tables: map[string]*Table{"pedido": table}}
	curr := &Snapshot{Tables: map[string]*Table{"pedido": table}}

	diff := computeDiff(prev, curr)
	if diff.HasChanges() {
		t.Errorf("no esperaba cambios, got %+v", diff)
	}
}
