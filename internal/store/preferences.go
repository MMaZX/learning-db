package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type UserPreferences struct {
	UserID          int64      `json:"user_id"`
	PreferencesJSON string     `json:"preferences"`
	MigratedAt      *time.Time `json:"migrated_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// systemAdminUsername is the reserved internal user backing the static administrator's preferences.
const systemAdminUsername = "__system_admin__"

func (s *Store) resolveUserID(ctx context.Context, userID int64) (int64, error) {
	if userID > 0 {
		return userID, nil
	}
	u, err := s.GetUserByUsername(ctx, systemAdminUsername)
	if err != nil {
		return 0, err
	}
	if u != nil {
		return u.ID, nil
	}
	// The backing account is never meant to authenticate: its token is random and discarded.
	token, err := GenerateToken()
	if err != nil {
		return 0, fmt.Errorf("generando token para la cuenta del sistema: %w", err)
	}
	created, err := s.CreateUser(ctx, systemAdminUsername, HashToken(token))
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

// GetUserPreferences devuelve las preferencias de un usuario por su ID, o nil si aún no tiene registradas.
func (s *Store) GetUserPreferences(ctx context.Context, userID int64) (*UserPreferences, error) {
	if userID <= 0 {
		u, err := s.GetUserByUsername(ctx, systemAdminUsername)
		if err != nil {
			return nil, err
		}
		if u == nil {
			return nil, nil
		}
		userID = u.ID
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT user_id, preferences_json, migrated_at, updated_at
		FROM user_preferences
		WHERE user_id = ?
	`, userID)

	var p UserPreferences
	var migratedAtStr sql.NullString
	var updatedAtStr string
	if err := row.Scan(&p.UserID, &p.PreferencesJSON, &migratedAtStr, &updatedAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("escaneando preferencias de usuario: %w", err)
	}

	if migratedAtStr.Valid && migratedAtStr.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, migratedAtStr.String); err == nil {
			p.MigratedAt = &t
		} else if t, err := time.Parse("2006-01-02 15:04:05", migratedAtStr.String); err == nil {
			p.MigratedAt = &t
		}
	}

	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAtStr)
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAtStr)
	}

	return &p, nil
}

// SaveUserPreferences guarda o actualiza (upsert) las preferencias crudas de un usuario.
func (s *Store) SaveUserPreferences(ctx context.Context, userID int64, rawJSON string) (*UserPreferences, error) {
	var err error
	userID, err = s.resolveUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("resolviendo usuario para preferencias: %w", err)
	}

	if rawJSON == "" {
		rawJSON = "{}"
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences_json, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			preferences_json = excluded.preferences_json,
			updated_at = excluded.updated_at
	`, userID, rawJSON, nowStr)
	if err != nil {
		return nil, fmt.Errorf("guardando preferencias de usuario: %w", err)
	}

	return s.GetUserPreferences(ctx, userID)
}

// SaveMigratedUserPreferences guarda o actualiza las preferencias registrando la fecha de migración.
func (s *Store) SaveMigratedUserPreferences(ctx context.Context, userID int64, rawJSON string) (*UserPreferences, error) {
	var err error
	userID, err = s.resolveUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("resolviendo usuario para preferencias: %w", err)
	}

	if rawJSON == "" {
		rawJSON = "{}"
	}

	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO user_preferences (user_id, preferences_json, migrated_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			preferences_json = excluded.preferences_json,
			migrated_at = COALESCE(user_preferences.migrated_at, excluded.migrated_at),
			updated_at = excluded.updated_at
	`, userID, rawJSON, nowStr, nowStr)
	if err != nil {
		return nil, fmt.Errorf("migrando preferencias de usuario: %w", err)
	}

	return s.GetUserPreferences(ctx, userID)
}
