package security

import "testing"

func TestValidate_RejectsWriteStatements(t *testing.T) {
	cases := []string{
		"INSERT INTO pedido (id) VALUES (1)",
		"UPDATE pedido SET estado = 1",
		"DELETE FROM pedido",
		"DROP TABLE pedido",
		"ALTER TABLE pedido ADD COLUMN x INT",
		"CREATE TABLE x (id INT)",
		"TRUNCATE TABLE pedido",
		"RENAME TABLE pedido TO pedido2",
		"GRANT ALL ON *.* TO 'x'@'%'",
		"REVOKE ALL ON *.* FROM 'x'@'%'",
	}
	for _, sql := range cases {
		if _, err := Validate(sql); err == nil {
			t.Errorf("esperaba rechazo para %q, pero fue aceptado", sql)
		}
	}
}

func TestValidate_RejectsMultipleStatements(t *testing.T) {
	cases := []string{
		"SELECT * FROM pedido; DROP TABLE pedido",
		"SELECT 1; SELECT 2",
		"SELECT 1;;",
	}
	for _, sql := range cases {
		if _, err := Validate(sql); err == nil {
			t.Errorf("esperaba rechazo por múltiples sentencias para %q", sql)
		}
	}
}

func TestValidate_AcceptsSelect(t *testing.T) {
	cases := []string{
		"SELECT * FROM pedido",
		"select id, total from pedido where estado = 1",
		"SELECT p.id FROM pedido p JOIN cliente c ON c.id = p.cliente_id",
		"SELECT * FROM pedido UNION SELECT * FROM pedido_historico",
	}
	for _, sql := range cases {
		if _, err := Validate(sql); err != nil {
			t.Errorf("esperaba aceptar %q, error: %v", sql, err)
		}
	}
}

func TestValidate_RejectsEmpty(t *testing.T) {
	if _, err := Validate("   "); err == nil {
		t.Error("esperaba rechazo para consulta vacía")
	}
}

func TestValidate_RejectsForUpdate(t *testing.T) {
	if _, err := Validate("SELECT * FROM pedido FOR UPDATE"); err == nil {
		t.Error("esperaba rechazo para SELECT ... FOR UPDATE")
	}
}

func TestEnforceLimit_AddsLimitWhenMissing(t *testing.T) {
	stmt, err := Validate("SELECT * FROM pedido")
	if err != nil {
		t.Fatal(err)
	}
	limited := EnforceLimit(stmt, 100)
	got := String(limited)
	want := "select * from pedido limit 100"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnforceLimit_KeepsExistingLimit(t *testing.T) {
	stmt, err := Validate("SELECT * FROM pedido LIMIT 5")
	if err != nil {
		t.Fatal(err)
	}
	limited := EnforceLimit(stmt, 100)
	got := String(limited)
	want := "select * from pedido limit 5"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
