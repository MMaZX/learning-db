package store

import (
	"context"
	"testing"
)

func TestMigration0010_UsersAndPreferences(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 1. Insert user
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, token_hash, is_active)
		VALUES ('persona 1', 'hash_token_123', 1)
	`)
	if err != nil {
		t.Fatalf("falló insertar usuario: %v", err)
	}
	userID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("error obteniendo LastInsertId: %v", err)
	}

	// 2. Insert preferences
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences_json, migrated_at)
		VALUES (?, '{"theme":"dark"}', '2026-10-13T12:00:00Z')
	`, userID)
	if err != nil {
		t.Fatalf("falló insertar preferencias: %v", err)
	}

	// 3. Unique username constraint
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (username, token_hash, is_active)
		VALUES ('persona 1', 'another_hash', 1)
	`)
	if err == nil {
		t.Fatalf("se esperaba error de unicidad en username")
	}

	// 4. Unique token_hash constraint
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO users (username, token_hash, is_active)
		VALUES ('persona 2', 'hash_token_123', 1)
	`)
	if err == nil {
		t.Fatalf("se esperaba error de unicidad en token_hash")
	}

	// 5. Foreign key constraint on user_preferences
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences_json)
		VALUES (999999, '{}')
	`)
	if err == nil {
		t.Fatalf("se esperaba error de foreign key en user_preferences")
	}
}

func TestStore_CreateAndGetUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	token := "mi-token-secreto"
	hash := HashToken(token)

	u, err := s.CreateUser(ctx, "persona 2", hash)
	if err != nil {
		t.Fatalf("falló CreateUser: %v", err)
	}
	if u.ID <= 0 || u.Username != "persona 2" || !u.IsActive {
		t.Fatalf("usuario retornado inválido: %+v", u)
	}

	// Recuperar por hash
	byHash, err := s.GetUserByTokenHash(ctx, hash)
	if err != nil {
		t.Fatalf("falló GetUserByTokenHash: %v", err)
	}
	if byHash.ID != u.ID || byHash.Username != "persona 2" {
		t.Fatalf("usuario no coincide: %+v", byHash)
	}

	// Recuperar por username
	byName, err := s.GetUserByUsername(ctx, "persona 2")
	if err != nil {
		t.Fatalf("falló GetUserByUsername: %v", err)
	}
	if byName.ID != u.ID {
		t.Fatalf("usuario por nombre no coincide: %+v", byName)
	}

	// Hash inexistente
	notFound, err := s.GetUserByTokenHash(ctx, HashToken("inexistente"))
	if err != nil {
		t.Fatalf("error buscando token inexistente: %v", err)
	}
	if notFound != nil {
		t.Fatalf("esperaba nil para token inexistente, obtuve: %+v", notFound)
	}

	// Listar usuarios
	users, err := s.ListUsers(ctx)
	if err != nil {
		t.Fatalf("falló ListUsers: %v", err)
	}
	if len(users) != 1 || users[0].Username != "persona 2" {
		t.Fatalf("lista de usuarios inesperada: %+v", users)
	}
}

func TestGenerateAndHashToken(t *testing.T) {
	tok1, err := GenerateToken()
	if err != nil {
		t.Fatalf("falló GenerateToken: %v", err)
	}
	tok2, err := GenerateToken()
	if err != nil {
		t.Fatalf("falló GenerateToken: %v", err)
	}
	if tok1 == tok2 {
		t.Fatalf("tokens generados deben ser únicos")
	}
	if len(tok1) < 32 {
		t.Fatalf("longitud de token insuficiente: %d", len(tok1))
	}

	h1 := HashToken(tok1)
	h2 := HashToken(tok1)
	if h1 != h2 {
		t.Fatalf("HashToken debe ser determinista")
	}
	if h1 == tok1 {
		t.Fatalf("hash no debe ser el token en claro")
	}
}
