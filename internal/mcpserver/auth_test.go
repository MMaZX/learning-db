package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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
