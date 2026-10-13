package main

import (
	"context"
	"testing"

	"github.com/fulanito/db-intelligence-mcp/internal/config"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func TestHandleCLI_CreateAndListUsers(t *testing.T) {
	dataPath := t.TempDir()
	cfg := &config.Config{
		DataPath: dataPath,
	}

	// 1. Crear usuario vía CLI
	handled, err := handleCLI(cfg, []string{"user", "create", "testuser"})
	if err != nil {
		t.Fatalf("error en handleCLI user create: %v", err)
	}
	if !handled {
		t.Fatalf("se esperaba que handleCLI manejara el comando")
	}

	// 2. Verificar que el usuario se guardó en la base de datos
	s, err := store.Open(context.Background(), dataPath)
	if err != nil {
		t.Fatalf("abriendo store: %v", err)
	}
	defer s.Close()

	u, err := s.GetUserByUsername(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("error buscando usuario creado: %v", err)
	}
	if u == nil || u.Username != "testuser" || u.TokenHash == "" {
		t.Fatalf("usuario creado no coincide: %+v", u)
	}

	// 3. Listar usuarios vía CLI
	handled, err = handleCLI(cfg, []string{"user", "list"})
	if err != nil {
		t.Fatalf("error en handleCLI user list: %v", err)
	}
	if !handled {
		t.Fatalf("se esperaba que handleCLI manejara user list")
	}

	// 4. Subcomando alternativo create-user
	handled, err = handleCLI(cfg, []string{"create-user", "anotheruser"})
	if err != nil {
		t.Fatalf("error en handleCLI create-user: %v", err)
	}
	if !handled {
		t.Fatalf("se esperaba que handleCLI manejara create-user")
	}

	// 5. Help
	handled, err = handleCLI(cfg, []string{"--help"})
	if err != nil || !handled {
		t.Fatalf("error en help: %v, handled=%v", err, handled)
	}

	// 6. Sin argumentos
	handled, err = handleCLI(cfg, nil)
	if err != nil || handled {
		t.Fatalf("sin argumentos no debe manejarse como CLI especial: handled=%v", handled)
	}
}
