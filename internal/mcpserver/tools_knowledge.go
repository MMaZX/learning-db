package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerKnowledgeTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("recordar_contexto",
			mcp.WithDescription("PRIMERA tool que debes llamar ante cualquier petición sobre datos o procesos de negocio, especialmente al comenzar una sesión nueva. Recupera en una sola llamada la memoria VALIDADA relevante para la petición completa, aunque el usuario no use exactamente las mismas palabras con las que se guardó. Si devuelve resultados, reutiliza directamente sus fuentes, relaciones y fórmulas y evita redescubrir el schema o volver a preguntar lo ya aprendido."),
			mcp.WithString("tarea", mcp.Required(), mcp.Description("Petición completa del usuario, en lenguaje natural")),
			mcp.WithString("contexto", mcp.Description("Área de negocio si se conoce, ej. 'ventas' o 'movimientos de stock'")),
			mcp.WithNumber("limite", mcp.Description("Máximo de recuerdos validados a devolver"), mcp.DefaultNumber(20)),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Tarea    string `json:"tarea"`
			Contexto string `json:"contexto"`
			Limite   int    `json:"limite"`
		}) (*mcp.CallToolResult, error) {
			if args.Limite <= 0 || args.Limite > 50 {
				args.Limite = 20
			}
			found, err := deps.Store.RecallValidatedKnowledge(ctx, args.Tarea, args.Contexto, args.Limite)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			if len(found) == 0 {
				return toJSONResult(map[string]any{
					"memoria_encontrada": false,
					"nota":               "No encontré memoria validada relevante. Explora solo el schema necesario y consulta conocimiento específico antes de asumir reglas de negocio.",
				})
			}
			return toJSONResult(map[string]any{
				"memoria_encontrada":    true,
				"conocimiento_validado": conocimientosAJSON(found, aliases),
				"instruccion":           "Aplica esta memoria directamente. No redescubras estas relaciones ni vuelvas a preguntarlas; explora únicamente lo que falte para completar la tarea.",
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_conocimiento_negocio",
			mcp.WithDescription("Consulta la documentación funcional (Markdown) sobre cómo funciona el sistema: qué significan estados, qué representa una entidad de negocio, reglas de negocio, etc. Busca por tema/nombre de archivo."),
			mcp.WithString("tema", mcp.Required(), mcp.Description("Tema a consultar, ej. 'cortesía', 'ventas', 'kardex'")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Tema string `json:"tema"`
		}) (*mcp.CallToolResult, error) {
			doc, ok := deps.Knowledge.Document(args.Tema)
			if !ok {
				return toJSONResult(map[string]any{
					"encontrado": false,
					"nota":       fmt.Sprintf("No hay documentación funcional para %q todavía. Puedes registrar una observación/propuesta, o un humano puede añadir knowledge/business/%s.md.", args.Tema, args.Tema),
				})
			}
			return toJSONResult(map[string]any{"encontrado": true, "documento": doc})
		}),
	)

	s.AddTool(
		mcp.NewTool("buscar_conocimiento",
			mcp.WithDescription("Busca en todo el conocimiento acumulado: documentación funcional, conocimiento validado, propuestas pendientes y observaciones. Cada resultado indica su nivel de confianza (documentacion | validado | propuesto | observacion/hipotesis) para que nunca se confunda una inferencia con un hecho validado. Si no aparece nada con estado 'validado' para lo que necesitas, NO asumas ni inventes de dónde sale el dato: usa aprender_del_usuario para preguntarle al humano."),
			mcp.WithString("consulta", mcp.Required(), mcp.Description("Término de búsqueda")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Consulta string `json:"consulta"`
		}) (*mcp.CallToolResult, error) {
			_, documents := deps.Knowledge.Search(args.Consulta)
			knowledgeRows, err := deps.Store.SearchKnowledge(ctx, args.Consulta, 20)
			if err != nil {
				return toolError(err)
			}
			observations, err := deps.Store.SearchObservations(ctx, args.Consulta, 20)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}

			return toJSONResult(map[string]any{
				"consulta":      args.Consulta,
				"documentacion": docTitles(documents),
				"conocimiento":  conocimientosAJSON(knowledgeRows, aliases), // estado: propuesto | validado | rechazado | obsoleto
				"observaciones": observacionesAJSON(observations),           // tipo: observacion | hipotesis — nunca confundir con conocimiento validado
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_conocimiento_validado",
			mcp.WithDescription("Busca conocimiento YA VALIDADO por un humano para un concepto en un contexto concreto (ej. concepto='vendedor', contexto='ventas'). Es la consulta que un agente debe hacer ANTES de generar SQL que dependa de un concepto de negocio ambiguo. Si devuelve una lista vacía, NO existe una fuente confiable para ese concepto en ese contexto: la respuesta correcta es preguntarle al usuario cómo se obtiene y usar aprender_del_usuario, nunca adivinar una columna por similitud de nombre."),
			mcp.WithString("concepto", mcp.Required(), mcp.Description("Concepto de negocio, ej. 'vendedor', 'utilidad'")),
			mcp.WithString("contexto", mcp.Description("Ámbito de aplicación, ej. 'ventas', 'compras', 'reportes de ventas'. El conocimiento de un contexto nunca se reutiliza automáticamente en otro.")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Concepto string `json:"concepto"`
			Contexto string `json:"contexto"`
		}) (*mcp.CallToolResult, error) {
			found, err := deps.Store.FindValidatedKnowledge(ctx, args.Concepto, args.Contexto)
			if err != nil {
				return toolError(err)
			}
			if len(found) == 0 {
				return toJSONResult(map[string]any{
					"encontrado": false,
					"nota": fmt.Sprintf(
						"No hay conocimiento VALIDADO para el concepto %q en el contexto %q. No asumas ni inventes una columna/relación: pregúntale al usuario cómo se obtiene este dato y regístralo con la tool aprender_del_usuario.",
						args.Concepto, args.Contexto),
				})
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(map[string]any{"encontrado": true, "conocimiento": conocimientosAJSON(found, aliases)})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_entidad",
			mcp.WithDescription("Consulta una entidad conceptual de negocio (ej. 'venta', 'cliente') declarada en knowledge/index.md: qué tablas la componen, su documentación y sus relaciones semánticas explícitas con otras entidades."),
			mcp.WithString("nombre", mcp.Required(), mcp.Description("Nombre de la entidad, ej. 'Venta'")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Nombre string `json:"nombre"`
		}) (*mcp.CallToolResult, error) {
			e, ok := deps.Knowledge.Entity(args.Nombre)
			if !ok {
				return toJSONResult(map[string]any{
					"encontrado": false,
					"nota":       fmt.Sprintf("La entidad %q no está declarada en knowledge/index.md todavía. Usa buscar_en_base_datos para explorar tablas relacionadas y, si corresponde, añade la entidad al índice.", args.Nombre),
				})
			}
			return toJSONResult(map[string]any{"encontrado": true, "entidad": e})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_relaciones",
			mcp.WithDescription("Explora las relaciones de una tabla: claves foráneas salientes (a qué tablas apunta) y entrantes (qué tablas la referencian), basado en el schema real de la base de datos (hechos, no inferencias)."),
			mcp.WithString("tabla", mcp.Required(), mcp.Description("Nombre de la tabla")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Tabla string `json:"tabla"`
		}) (*mcp.CallToolResult, error) {
			snap, err := deps.Inspector.Current(ctx)
			if err != nil {
				return toolError(err)
			}
			t, ok := snap.Tables[args.Tabla]
			if !ok {
				return toolError(fmt.Errorf("la tabla %q no existe (o está excluida)", args.Tabla))
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			out := map[string]any{
				"tabla":            t.Name,
				"referencia_a":     t.ForeignKeys,  // esta tabla -> otras
				"referenciada_por": t.ReferencedBy, // otras tablas -> esta
			}
			if alias := resolveAlias(aliases, t.Name); alias != "" {
				out["alias"] = alias
			}
			return toJSONResult(out)
		}),
	)
}
