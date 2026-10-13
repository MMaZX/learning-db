package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerPreferencesTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("recordando_configuracion",
			mcp.WithDescription("Herramienta de inicio de sesión de una sola ejecución. Lee y devuelve las preferencias de comportamiento y configuración del usuario autenticado desde el servidor. Si el usuario aún no tiene preferencias guardadas, indica que se deben migrar desde preferences.json mediante migrar_preferencias."),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, error) {
			user, ok := UserFromContext(ctx)
			userID := int64(0)
			username := "admin"
			if ok && user != nil {
				userID = user.ID
				username = user.Username
			}

			prefs, err := deps.Store.GetUserPreferences(ctx, userID)
			if err != nil {
				return toolError(err)
			}

			if prefs == nil {
				return toJSONResult(map[string]any{
					"tiene_preferencias": false,
					"migrado":            false,
					"usuario":            username,
					"preferencias":       nil,
					"mensaje":            "El usuario no tiene preferencias guardadas en el servidor. Si tienes un archivo preferences.json local o preferencias previas, llama a migrar_preferencias para guardarlas en el servidor.",
					"instrucciones":      "No hay preferencias registradas en el servidor para este usuario. Si existe un preferences.json local, sugiere migrarlo mediante migrar_preferencias (se incorporará en el flujo del sistema); de lo contrario, procede con la configuración predeterminada de Jarvis.",
				})
			}

			var parsed any
			if err := json.Unmarshal([]byte(prefs.PreferencesJSON), &parsed); err != nil {
				parsed = prefs.PreferencesJSON
			}

			return toJSONResult(map[string]any{
				"tiene_preferencias": true,
				"migrado":            prefs.MigratedAt != nil,
				"usuario":            username,
				"preferencias":       parsed,
				"migrado_en":         prefs.MigratedAt,
				"actualizado_en":     prefs.UpdatedAt,
				"mensaje":            "Preferencias de usuario recuperadas exitosamente del servidor.",
				"instrucciones":      "Aplica estas preferencias de comportamiento en todas tus respuestas durante la conversación.",
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("migrar_preferencias",
			mcp.WithDescription("Migra y persiste el contenido completo de preferences.json en el servidor para el usuario autenticado. La herramienta es idempotente, conserva el JSON completo sin alterar su esquema y registra la fecha de migración."),
			mcp.WithString("contenido_json", mcp.Required(), mcp.Description("Contenido completo de preferences.json en formato JSON válido")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			ContenidoJSON string `json:"contenido_json"`
		}) (*mcp.CallToolResult, error) {
			raw := strings.TrimSpace(args.ContenidoJSON)
			if raw == "" {
				return toolError(errors.New("el contenido_json no puede estar vacío"))
			}
			if !json.Valid([]byte(raw)) {
				return toolError(errors.New("el contenido_json proporcionado no es un JSON válido"))
			}

			user, ok := UserFromContext(ctx)
			userID := int64(0)
			username := "admin"
			if ok && user != nil {
				userID = user.ID
				username = user.Username
			}

			saved, err := deps.Store.SaveMigratedUserPreferences(ctx, userID, raw)
			if err != nil {
				return toolError(err)
			}

			return toJSONResult(map[string]any{
				"exito":          true,
				"migrado":        true,
				"usuario":        username,
				"mensaje":        "Preferencias migradas y guardadas exitosamente en el servidor.",
				"migrado_en":     saved.MigratedAt,
				"actualizado_en": saved.UpdatedAt,
			})
		}),
	)
}
