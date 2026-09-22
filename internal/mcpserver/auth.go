package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// WithBearerAuth envuelve un handler HTTP exigiendo
// "Authorization: Bearer <token>" con un token estático, para proteger el
// transporte MCP Streamable HTTP (multi-sesión). Comparación en tiempo
// constante para no filtrar el token por timing.
func WithBearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		got := strings.TrimPrefix(header, prefix)
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "no autorizado", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
