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
