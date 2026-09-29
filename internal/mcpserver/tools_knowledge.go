package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// maxIDsPorConsulta acota obtener_conocimiento_por_id.
const maxIDsPorConsulta = 20

func registerKnowledgeTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("recordar_contexto",
			mcp.WithDescription("ÚNICA y PRIMERA tool que debes llamar para recuperar memoria de negocio al comenzar una sesión o tarea nueva, antes de explorar el schema o escribir SQL. Pásale la petición completa del usuario tal cual (parámetro 'tarea'), no un resumen ni una sola palabra clave: la búsqueda tolera variaciones de redacción. Resuelve el recuerdo en una sola llamada, incluida la fuente (tablas/columnas/relación/fórmula) y el alias legible de cada dato, así que no necesitas una segunda llamada a buscar_conocimiento ni a obtener_conocimiento_validado para recordar contexto: esas dos tools sirven para otra cosa (auditoría amplia y verificación puntual antes de un SQL concreto), no para el recuerdo de sesión. Si devuelve resultados, reutiliza directamente sus fuentes, relaciones y fórmulas y evita redescubrir el schema o volver a preguntar lo ya aprendido. Parámetro opcional 'detalle': 'completo' (por defecto) devuelve cada recuerdo entero; 'compacto' devuelve solo id, concepto, contexto, una vista previa de la afirmación, las tablas y la versión (mucho más barato cuando hay muchos recuerdos) y luego pides el detalle de los que necesites con obtener_conocimiento_por_id."),
			mcp.WithString("tarea", mcp.Required(), mcp.Description("Petición completa del usuario, en lenguaje natural")),
			mcp.WithString("contexto", mcp.Description("Área de negocio si se conoce, ej. 'ventas' o 'movimientos de stock'")),
			mcp.WithNumber("limite", mcp.Description("Máximo de recuerdos validados a devolver"), mcp.DefaultNumber(20)),
			mcp.WithString("detalle", mcp.Description("Nivel de detalle: 'completo' (por defecto) o 'compacto' (vista previa; pide el detalle con obtener_conocimiento_por_id)"), mcp.Enum("completo", "compacto")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Tarea    string `json:"tarea"`
			Contexto string `json:"contexto"`
			Limite   int    `json:"limite"`
			Detalle  string `json:"detalle"`
		}) (*mcp.CallToolResult, error) {
			if args.Detalle != "" && args.Detalle != "completo" && args.Detalle != "compacto" {
				return mcp.NewToolResultError("detalle debe ser 'completo' o 'compacto'"), nil
			}
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
					"nota":               "No encontré memoria validada relevante. No asumas ni inventes tablas/columnas: pregúntale al usuario cómo se obtiene el dato y regístralo con aprender_del_usuario. Explora solo el schema necesario antes de asumir reglas de negocio.",
				})
			}
			if args.Detalle == "compacto" {
				return toJSONResult(map[string]any{
					"memoria_encontrada":    true,
					"conocimiento_validado": conocimientosCompactosAJSON(found, aliases),
					"siguiente_paso":        "Esto es una vista compacta. Llama a obtener_conocimiento_por_id con los ids que necesites para ver la fuente completa (columnas, relación, fórmula) y los alias antes de escribir SQL; no hace falta pedir los que no te sirvan.",
					"instruccion":           "Aplica esta memoria directamente. No redescubras estas relaciones ni vuelvas a preguntarlas; explora únicamente lo que falte para completar la tarea.",
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
		mcp.NewTool("obtener_conocimiento_por_id",
			mcp.WithDescription("Follow-up de recordar_contexto(detalle='compacto'): devuelve el detalle COMPLETO (afirmación entera, fuente, fuente_alias, evidencia, versión) de los conocimientos cuyos ids elijas de la vista compacta. Solo devuelve conocimiento VALIDADO; los ids inexistentes o no validados se omiten y se listan en 'no_encontrados'. No es una vía de recuerdo alternativa: recordar_contexto sigue siendo el único punto de entrada para recuperar memoria."),
			mcp.WithArray("ids", mcp.Required(), mcp.Description("Ids de conocimiento a expandir (máximo 20)"), mcp.WithNumberItems()),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			IDs []int64 `json:"ids"`
		}) (*mcp.CallToolResult, error) {
			if len(args.IDs) == 0 {
				return mcp.NewToolResultError("ids no puede estar vacío"), nil
			}
			if len(args.IDs) > maxIDsPorConsulta {
				return mcp.NewToolResultError(fmt.Sprintf("máximo %d ids por llamada", maxIDsPorConsulta)), nil
			}
			found, err := deps.Store.GetKnowledgeByIDs(ctx, args.IDs)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			got := make(map[int64]struct{}, len(found))
			for _, k := range found {
				got[k.ID] = struct{}{}
			}
			missing := []int64{}
			for _, id := range args.IDs {
				if _, ok := got[id]; !ok {
					missing = append(missing, id)
				}
			}
			return toJSONResult(map[string]any{
				"conocimiento_validado": conocimientosAJSON(found, aliases),
				"no_encontrados":        missing,
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
			mcp.WithDescription("Búsqueda AMPLIA de auditoría sobre todo el conocimiento acumulado: documentación funcional, conocimiento validado, propuestas pendientes y observaciones. No es la tool de recuerdo de sesión (para eso usa recordar_contexto con la petición completa): úsala solo cuando necesites explorar qué se ha propuesto/rechazado/observado además de lo validado, ej. para revisar cobertura o depurar por qué algo no se recordó. Cada resultado indica su nivel de confianza (documentacion | validado | propuesto | observacion/hipotesis) para que nunca se confunda una inferencia con un hecho validado. Si no aparece nada con estado 'validado' para lo que necesitas, NO asumas ni inventes de dónde sale el dato: usa aprender_del_usuario para preguntarle al humano."),
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
			mcp.WithDescription("Verificación PUNTUAL, no recuerdo de sesión: confirma si existe conocimiento YA VALIDADO por un humano para un concepto exacto en un contexto exacto (ej. concepto='vendedor', contexto='ventas'), justo antes de escribir el SQL que depende de ese concepto de negocio ambiguo. Para recuperar contexto al iniciar una tarea usa recordar_contexto con la petición completa, no esta tool. Si devuelve una lista vacía, NO existe una fuente confiable para ese concepto en ese contexto: la respuesta correcta es preguntarle al usuario cómo se obtiene y usar aprender_del_usuario, nunca adivinar una columna por similitud de nombre."),
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
