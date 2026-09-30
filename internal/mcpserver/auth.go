package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// WithBearerAuth envuelve un handler HTTP exigiendo el token estático, para
// proteger el transporte MCP Streamable HTTP (multi-sesión). Acepta dos
// formas equivalentes:
//   - "Authorization: Bearer <token>" (Claude Code, Codex, OpenCode, etc.)
//   - "X-API-Key: <token>" (clientes cuyo header Authorization queda
//     reservado para OAuth, p. ej. conectores web)
//
// Comparación en tiempo constante para no filtrar el token por timing.
func WithBearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tokenValido(r, token) {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenValido indica si la petición trae el token correcto en alguno de los
// headers aceptados. Un token vacío en el servidor nunca autoriza.
func tokenValido(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	const prefix = "Bearer "
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, prefix) {
		if iguales(strings.TrimPrefix(header, prefix), token) {
			return true
		}
	}
	if key := r.Header.Get("X-API-Key"); key != "" {
		return iguales(key, token)
	}
	return false
}

func iguales(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
