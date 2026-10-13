package mcpserver

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

type userContextKey struct{}

var currentUserKey = userContextKey{}

// ContextWithUser inyecta el usuario autenticado en el contexto.
func ContextWithUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, currentUserKey, u)
}

// UserFromContext extrae el usuario autenticado del contexto si existe.
func UserFromContext(ctx context.Context) (*store.User, bool) {
	u, ok := ctx.Value(currentUserKey).(*store.User)
	return u, ok && u != nil
}

// UserStore define las operaciones necesarias del store para autenticar usuarios.
type UserStore interface {
	GetUserByTokenHash(ctx context.Context, tokenHash string) (*store.User, error)
}

// AdminUser representa el usuario administrador cuando se autentica con el token estático.
var AdminUser = &store.User{
	ID:       0,
	Username: "admin",
	Role:     store.RoleAdmin,
	IsActive: true,
}

// WithUserAuth envuelve un handler HTTP autenticando al usuario a partir de su token.
// Si el token coincide con staticToken (A1), autentica como admin.
// Si coincide con el token personal de un usuario en UserStore (S1), autentica como dicho usuario.
// En ambos casos inyecta el usuario en el contexto de la petición.
func WithUserAuth(userStore UserStore, staticToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokens := extraerTokens(r)
		if len(tokens) == 0 {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}

		for _, token := range tokens {
			// A1: El token estático actual sigue valiendo como administrador
			if staticToken != "" && iguales(token, staticToken) {
				ctx := ContextWithUser(r.Context(), AdminUser)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// S1: Buscar por token personal
			if userStore != nil {
				hash := store.HashToken(token)
				u, err := userStore.GetUserByTokenHash(r.Context(), hash)
				if err == nil && u != nil && u.IsActive {
					ctx := ContextWithUser(r.Context(), u)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		http.Error(w, "no autorizado", http.StatusUnauthorized)
	})
}

// WithBearerAuth envuelve un handler HTTP exigiendo el token estático, para
// compatibilidad con invocaciones previas.
func WithBearerAuth(token string, next http.Handler) http.Handler {
	return WithUserAuth(nil, token, next)
}

func extraerTokens(r *http.Request) []string {
	var tokens []string
	const prefix = "Bearer "
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, prefix) {
		if tok := strings.TrimSpace(strings.TrimPrefix(header, prefix)); tok != "" {
			tokens = append(tokens, tok)
		}
	}
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" && !strings.HasPrefix(key, prefix) {
		tokens = append(tokens, key)
	}
	return tokens
}

func iguales(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
