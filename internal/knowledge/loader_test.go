package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeIndex(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseIndex_ParsesEntitiesAndRelations(t *testing.T) {
	dir := t.TempDir()
	writeIndex(t, dir, `# Índice

## Entidades

### Venta
- tablas: pedido, detallepedido, cliente
- documento: ventas.md
- descripcion: Proceso de venta
- relacion:pertenece: Cliente
- relacion:genera: Pago

### Cliente
- tablas: cliente
- documento: clientes.md
`)

	s := NewStore(dir)
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}

	venta, ok := s.Entity("venta")
	if !ok {
		t.Fatal("esperaba encontrar la entidad Venta (case-insensitive)")
	}
	if venta.Name != "Venta" {
		t.Errorf("nombre = %q, want Venta", venta.Name)
	}
	if len(venta.Tables) != 3 {
		t.Errorf("tablas = %v, want 3 elementos", venta.Tables)
	}
	if venta.Relations["pertenece"] != "Cliente" {
		t.Errorf("relacion pertenece = %q, want Cliente", venta.Relations["pertenece"])
	}
	if venta.Relations["genera"] != "Pago" {
		t.Errorf("relacion genera = %q, want Pago", venta.Relations["genera"])
	}

	if _, ok := s.Entity("inexistente"); ok {
		t.Error("no debería encontrar una entidad inexistente")
	}
}

func TestStore_Document_LoadsMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	businessDir := filepath.Join(dir, "business")
	if err := os.MkdirAll(businessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(businessDir, "ventas.md"), []byte("# Ventas\n\nContenido de prueba sobre cortesía.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewStore(dir)
	if err := s.Reload(); err != nil {
		t.Fatal(err)
	}

	doc, ok := s.Document("ventas")
	if !ok {
		t.Fatal("esperaba encontrar el documento ventas.md")
	}
	if doc.Title != "Ventas" {
		t.Errorf("title = %q, want Ventas", doc.Title)
	}

	// Búsqueda por contenido también debe funcionar (ej. "cortesía").
	if _, ok := s.Document("cortesía"); !ok {
		t.Error("esperaba encontrar el documento buscando por contenido 'cortesía'")
	}
}

func TestStore_Reload_IsSafeWhenIndexMissing(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload no debería fallar si index.md no existe: %v", err)
	}
	if _, ok := s.Entity("venta"); ok {
		t.Error("no debería haber entidades sin index.md")
	}
}
