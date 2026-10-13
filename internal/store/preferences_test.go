package store

import (
	"context"
	"testing"
)

func TestStore_UserPreferences(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	u, err := s.CreateUser(ctx, "persona_pref", HashToken("tok_pref"))
	if err != nil {
		t.Fatalf("falló creando usuario: %v", err)
	}

	// 1. Obtener sin preferencias previas
	prefs, err := s.GetUserPreferences(ctx, u.ID)
	if err != nil {
		t.Fatalf("error obteniendo preferencias inexistentes: %v", err)
	}
	if prefs != nil {
		t.Fatalf("se esperaba nil para usuario sin preferencias, obtenido: %+v", prefs)
	}

	// 2. Guardar preferencias
	saved, err := s.SaveUserPreferences(ctx, u.ID, `{"lenguaje":"es","modo":"conciso"}`)
	if err != nil {
		t.Fatalf("falló guardando preferencias: %v", err)
	}
	if saved == nil || saved.UserID != u.ID || saved.PreferencesJSON != `{"lenguaje":"es","modo":"conciso"}` {
		t.Fatalf("preferencias guardadas no coinciden: %+v", saved)
	}

	// 3. Recuperar preferencias guardadas
	retrieved, err := s.GetUserPreferences(ctx, u.ID)
	if err != nil {
		t.Fatalf("falló recuperando preferencias: %v", err)
	}
	if retrieved == nil || retrieved.PreferencesJSON != saved.PreferencesJSON {
		t.Fatalf("preferencias recuperadas no coinciden: %+v", retrieved)
	}

	// 4. Actualización idempotente (upsert)
	updated, err := s.SaveUserPreferences(ctx, u.ID, `{"lenguaje":"es","modo":"detallado"}`)
	if err != nil {
		t.Fatalf("falló actualizando preferencias: %v", err)
	}
	if updated.PreferencesJSON != `{"lenguaje":"es","modo":"detallado"}` {
		t.Fatalf("preferencias no se actualizaron: %+v", updated)
	}

	// 5. Guardar preferencias migradas (registra migrated_at)
	migrated, err := s.SaveMigratedUserPreferences(ctx, u.ID, `{"lenguaje":"es","migrado":true}`)
	if err != nil {
		t.Fatalf("falló migrando preferencias: %v", err)
	}
	if migrated.MigratedAt == nil {
		t.Fatal("se esperaba migrated_at no nulo")
	}

	// 6. Admin fallback con userID = 0
	adminPrefs, err := s.GetUserPreferences(ctx, 0)
	if err != nil {
		t.Fatalf("error obteniendo preferencias de admin: %v", err)
	}
	if adminPrefs != nil {
		t.Fatalf("admin no debería tener preferencias inicialmente: %+v", adminPrefs)
	}
	savedAdmin, err := s.SaveUserPreferences(ctx, 0, `{"admin_mode":true}`)
	if err != nil {
		t.Fatalf("falló guardando preferencias para admin: %v", err)
	}
	if savedAdmin.PreferencesJSON != `{"admin_mode":true}` {
		t.Fatalf("preferencias de admin guardadas inesperadas: %+v", savedAdmin)
	}
	retrievedAdmin, err := s.GetUserPreferences(ctx, 0)
	if err != nil || retrievedAdmin == nil || retrievedAdmin.PreferencesJSON != `{"admin_mode":true}` {
		t.Fatalf("falló recuperando preferencias de admin: %v, %+v", err, retrievedAdmin)
	}
}

func TestStore_StaticAdminPreferencesAreIsolated(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	named, err := s.CreateUser(ctx, "admin", HashToken("tok_admin_named"))
	if err != nil {
		t.Fatalf("creating named admin: %v", err)
	}
	if _, err := s.SaveUserPreferences(ctx, 0, `{"static":true}`); err != nil {
		t.Fatalf("saving static admin preferences: %v", err)
	}

	got, err := s.GetUserPreferences(ctx, named.ID)
	if err != nil || got != nil {
		t.Fatalf("named 'admin' user must not see static admin preferences: %v, %+v", err, got)
	}

	sys, err := s.GetUserByUsername(ctx, systemAdminUsername)
	if err != nil || sys == nil {
		t.Fatalf("system user not created: %v", err)
	}
	if sys.TokenHash == HashToken("system-admin-reserved-account") {
		t.Fatal("system user must not use a predictable token")
	}
}
