package query

import (
	"strings"
	"testing"

	"github.com/xwb1989/sqlparser"
)

func analyze(t *testing.T, sql string) *ExplainResult {
	t.Helper()
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		t.Fatalf("parseando %q: %v", sql, err)
	}
	selStmt, ok := stmt.(sqlparser.SelectStatement)
	if !ok {
		t.Fatalf("%q no es un SELECT", sql)
	}
	res := &ExplainResult{SQL: sql}
	analyzeSelect(selStmt, res)
	return res
}

func containsWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestAnalyzeSelect_StarExpr(t *testing.T) {
	res := analyze(t, "SELECT * FROM pedido WHERE id = 1")
	if !containsWarning(res.Warnings, "SELECT *") {
		t.Fatalf("esperaba advertencia de SELECT *, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_ExplicitColumnsNoWarning(t *testing.T) {
	res := analyze(t, "SELECT id, nombre FROM pedido WHERE id = 1")
	if containsWarning(res.Warnings, "SELECT *") {
		t.Fatalf("no esperaba advertencia de SELECT *, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_FunctionWrappedColumn(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido WHERE UPPER(email) = 'JOHN@EXAMPLE.COM'")
	if !containsWarning(res.Warnings, "función sobre una columna") {
		t.Fatalf("esperaba advertencia de función sobre columna, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_PlainWhereNoFunctionWarning(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido WHERE email = 'john@example.com' AND estado = 1")
	if containsWarning(res.Warnings, "función sobre una columna") {
		t.Fatalf("no esperaba advertencia de función sobre columna, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_FunctionOnLiteralNoWarning(t *testing.T) {
	// NOW() no envuelve una columna, es solo un valor: no debe dispararla.
	res := analyze(t, "SELECT id FROM pedido WHERE creado_en > NOW()")
	if containsWarning(res.Warnings, "función sobre una columna") {
		t.Fatalf("no esperaba advertencia de función sobre columna, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_LargeOffsetWarns(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido ORDER BY id LIMIT 20 OFFSET 5000")
	if !containsWarning(res.Warnings, "OFFSET 5000") {
		t.Fatalf("esperaba advertencia de OFFSET grande, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_SmallOffsetNoWarning(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido ORDER BY id LIMIT 20 OFFSET 40")
	if containsWarning(res.Warnings, "OFFSET") {
		t.Fatalf("no esperaba advertencia de OFFSET pequeño, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_NoOffsetNoWarning(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido WHERE id = 1 LIMIT 20")
	if containsWarning(res.Warnings, "OFFSET") {
		t.Fatalf("no esperaba advertencia de OFFSET, warnings=%v", res.Warnings)
	}
}

func TestAnalyzeSelect_UnionStillHandled(t *testing.T) {
	res := analyze(t, "SELECT id FROM pedido UNION SELECT id FROM pedido_historico")
	if !containsWarning(res.Warnings, "UNION") {
		t.Fatalf("esperaba advertencia de UNION, warnings=%v", res.Warnings)
	}
}

func TestExtractTables(t *testing.T) {
	cases := map[string][]string{
		"SELECT * FROM pedido": {"pedido"},
		"SELECT p.id FROM Pedido p JOIN cliente c ON c.id = p.cliente_id":               {"cliente", "pedido"},
		"SELECT * FROM a WHERE id IN (SELECT a_id FROM b WHERE x IN (SELECT 1 FROM c))": {"a", "b", "c"},
		"SELECT id FROM a UNION SELECT id FROM b UNION SELECT id FROM a":                {"a", "b"},
		"SELECT * FROM (SELECT id FROM t1) sub JOIN t2 ON t2.id = sub.id":               {"t1", "t2"},
		"SELECT 1": {},
	}
	for sql, want := range cases {
		stmt, err := sqlparser.Parse(sql)
		if err != nil {
			t.Fatalf("parseando %q: %v", sql, err)
		}
		got := ExtractTables(stmt)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%q: got %v, want %v", sql, got, want)
		}
	}
}
