package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserversdk "github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func newTestHTTPServer(t *testing.T, token string) *httptest.Server {
	t.Helper()
	metadata, err := store.Open(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = metadata.Close() })

	mcpServer := New(&Deps{
		Store:     metadata,
		Knowledge: knowledge.NewStore(t.TempDir()),
	})
	streamableSrv := mcpserversdk.NewStreamableHTTPServer(
		mcpServer,
		mcpserversdk.WithEndpointPath("/mcp"),
		mcpserversdk.WithStateful(true),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", WithBearerAuth(token, streamableSrv))
	return httptest.NewServer(mux)
}

func newTestClient(t *testing.T, baseURL, token string) *client.Client {
	t.Helper()
	c, err := client.NewStreamableHttpClient(
		baseURL+"/mcp",
		transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	if _, err := c.Initialize(context.Background(), initRequest); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestHTTPTransportRejectsMissingOrWrongToken(t *testing.T) {
	srv := newTestHTTPServer(t, "secreto")
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperaba 401 sin token, obtuve %d", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader("{}"))
	req2.Header.Set("Authorization", "Bearer incorrecto")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperaba 401 con token incorrecto, obtuve %d", resp2.StatusCode)
	}
}

// TestHTTPTransportServesConcurrentSessions verifica que dos clientes MCP
// independientes, conectados al mismo servidor HTTP, pueden inicializarse y
// llamar tools en paralelo sin cruzarse ni pisarse (en particular, sin
// errores de "database is locked" en el store SQLite compartido).
func TestHTTPTransportServesConcurrentSessions(t *testing.T) {
	srv := newTestHTTPServer(t, "secreto")
	t.Cleanup(srv.Close)

	clientA := newTestClient(t, srv.URL, "secreto")
	clientB := newTestClient(t, srv.URL, "secreto")

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for _, c := range []*client.Client{clientA, clientB} {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(c *client.Client) {
				defer wg.Done()
				req := mcp.CallToolRequest{}
				req.Params.Name = "recordar_contexto"
				req.Params.Arguments = map[string]any{"tarea": "ping"}
				if _, err := c.CallTool(context.Background(), req); err != nil {
					errs <- err
				}
			}(c)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("llamada concurrente falló: %v", err)
	}
}
