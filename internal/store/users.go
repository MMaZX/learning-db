package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Roles de usuario. El rol no se persiste: los usuarios con token personal son
// siempre RoleUser; solo el token estático autentica como RoleAdmin.
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	TokenHash string    `json:"-"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// HashToken calcula el hash SHA-256 en formato hexadecimal de un token en claro.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateToken genera un token criptográfico seguro de 32 bytes en formato hexadecimal (64 caracteres).
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generando token aleatorio: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateUser registra un nuevo usuario con su token hash.
func (s *Store) CreateUser(ctx context.Context, username, tokenHash string) (*User, error) {
	if username == "" {
		return nil, errors.New("el nombre de usuario no puede estar vacío")
	}
	if tokenHash == "" {
		return nil, errors.New("el token hash no puede estar vacío")
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, token_hash, is_active)
		VALUES (?, ?, 1)
	`, username, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("insertando usuario: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obteniendo id de usuario insertado: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// GetUserByID busca un usuario por su ID interno.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, token_hash, is_active, created_at
		FROM users
		WHERE id = ?
	`, id)

	var u User
	var createdAtStr string
	var isActiveInt int
	if err := row.Scan(&u.ID, &u.Username, &u.TokenHash, &isActiveInt, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("escaneando usuario por id: %w", err)
	}
	u.IsActive = isActiveInt == 1
	u.Role = RoleUser
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	if u.CreatedAt.IsZero() {
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	}
	return &u, nil
}

// GetUserByTokenHash busca un usuario por el hash de su token.
func (s *Store) GetUserByTokenHash(ctx context.Context, tokenHash string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, token_hash, is_active, created_at
		FROM users
		WHERE token_hash = ?
	`, tokenHash)

	var u User
	var createdAtStr string
	var isActiveInt int
	if err := row.Scan(&u.ID, &u.Username, &u.TokenHash, &isActiveInt, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("escaneando usuario por token hash: %w", err)
	}
	u.IsActive = isActiveInt == 1
	u.Role = RoleUser
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	if u.CreatedAt.IsZero() {
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	}
	return &u, nil
}

// GetUserByUsername busca un usuario por su nombre.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, token_hash, is_active, created_at
		FROM users
		WHERE username = ?
	`, username)

	var u User
	var createdAtStr string
	var isActiveInt int
	if err := row.Scan(&u.ID, &u.Username, &u.TokenHash, &isActiveInt, &createdAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("escaneando usuario por username: %w", err)
	}
	u.IsActive = isActiveInt == 1
	u.Role = RoleUser
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
	if u.CreatedAt.IsZero() {
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	}
	return &u, nil
}

// ListUsers lista todos los usuarios ordenados por id ascendente.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, token_hash, is_active, created_at
		FROM users
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("consultando usuarios: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var createdAtStr string
		var isActiveInt int
		if err := rows.Scan(&u.ID, &u.Username, &u.TokenHash, &isActiveInt, &createdAtStr); err != nil {
			return nil, fmt.Errorf("escaneando fila de usuario: %w", err)
		}
		u.IsActive = isActiveInt == 1
		u.Role = RoleUser
		u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAtStr)
		if u.CreatedAt.IsZero() {
			u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
