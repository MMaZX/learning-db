package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

type mockUserStore struct {
	users map[string]*store.User
}

func (m *mockUserStore) GetUserByTokenHash(ctx context.Context, hash string) (*store.User, error) {
	return m.users[hash], nil
}

func TestWithBearerAuth(t *testing.T) {
	const token = "secreto-123"
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := WithBearerAuth(token, ok)

	cases := []struct {
		name    string
		headers map[string]string
		want    int
	}{
		{"sin headers", nil, http.StatusUnauthorized},
		{"bearer correcto", map[string]string{"Authorization": "Bearer " + token}, http.StatusOK},
		{"bearer incorrecto", map[string]string{"Authorization": "Bearer otro"}, http.StatusUnauthorized},
		{"authorization sin Bearer", map[string]string{"Authorization": token}, http.StatusUnauthorized},
		{"x-api-key correcto", map[string]string{"X-API-Key": token}, http.StatusOK},
		{"x-api-key minúsculas", map[string]string{"x-api-key": token}, http.StatusOK},
		{"x-api-key incorrecto", map[string]string{"X-API-Key": "otro"}, http.StatusUnauthorized},
		{"x-api-key con prefijo Bearer", map[string]string{"X-API-Key": "Bearer " + token}, http.StatusUnauthorized},
		// Un conector puede mandar su propio Bearer de OAuth junto a X-API-Key.
		{"bearer ajeno + x-api-key correcto", map[string]string{"Authorization": "Bearer oauth-ajeno", "X-API-Key": token}, http.StatusOK},
		{"x-api-key vacío", map[string]string{"X-API-Key": ""}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d, quería %d", rec.Code, tc.want)
			}
		})
	}
}

func TestWithBearerAuth_EmptyServerTokenNeverAuthorizes(t *testing.T) {
	h := WithBearerAuth("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	for _, hdr := range []map[string]string{{"Authorization": "Bearer "}, {"X-API-Key": ""}} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("token vacío no debe autorizar: %v -> %d", hdr, rec.Code)
		}
	}
}

func TestWithUserAuth(t *testing.T) {
	const staticAdminToken = "admin-static-token"
	const userToken = "user-alice-token"
	userHash := store.HashToken(userToken)

	const inactiveToken = "user-bob-inactive-token"
	inactiveHash := store.HashToken(inactiveToken)

	mockStore := &mockUserStore{
		users: map[string]*store.User{
			userHash: {
				ID:        1,
				Username:  "alice",
				TokenHash: userHash,
				IsActive:  true,
				CreatedAt: time.Now(),
			},
			inactiveHash: {
				ID:        2,
				Username:  "bob",
				TokenHash: inactiveHash,
				IsActive:  false,
				CreatedAt: time.Now(),
			},
		},
	}

	var capturedUser *store.User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	})

	h := WithUserAuth(mockStore, staticAdminToken, handler)

	t.Run("token personal valido en Bearer", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+userToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusOK)
		}
		if capturedUser == nil || capturedUser.Username != "alice" {
			t.Fatalf("usuario en contexto inesperado: %+v", capturedUser)
		}
	})

	t.Run("token personal valido en X-API-Key", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("X-API-Key", userToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusOK)
		}
		if capturedUser == nil || capturedUser.Username != "alice" {
			t.Fatalf("usuario en contexto inesperado: %+v", capturedUser)
		}
	})

	t.Run("token estatico conserva rol admin (A1)", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+staticAdminToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusOK)
		}
		if capturedUser == nil || capturedUser.Username != "admin" {
			t.Fatalf("usuario admin esperado, obtenido: %+v", capturedUser)
		}
	})

	t.Run("usuario inactivo rechazado", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+inactiveToken)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("token desconocido rechazado", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		req.Header.Set("Authorization", "Bearer token-desconocido")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("sin token rechazado", func(t *testing.T) {
		capturedUser = nil
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d, quería %d", rec.Code, http.StatusUnauthorized)
		}
	})
}
