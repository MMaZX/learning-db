package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/schema"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

var errTipo = errors.New("tipo debe ser 'observacion' o 'hipotesis'")

func registerLearningTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("registrar_observacion",
			mcp.WithDescription("Registra una observación o hipótesis generada durante una interacción. Una 'observacion' es un hecho notado (ej. 'estado=10 aparece junto con precio=0 en 40 filas'); una 'hipotesis' es una posible explicación, NUNCA un hecho validado. Esto es el primer escalón del flujo de aprendizaje: no crea conocimiento validado por sí mismo."),
			mcp.WithString("tema", mcp.Required(), mcp.Description("Tema/entidad al que se refiere, ej. 'pedido.estado'")),
			mcp.WithString("tipo", mcp.Required(), mcp.Description("'observacion' o 'hipotesis'"), mcp.Enum("observacion", "hipotesis")),
			mcp.WithString("enunciado", mcp.Required(), mcp.Description("Texto de la observación o hipótesis")),
			mcp.WithString("contexto", mcp.Description("Contexto de la pregunta/tarea que originó la observación")),
			mcp.WithArray("evidencia", mcp.Description("Evidencia: ids de consultas, rutas de documentación, etc."), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithArray("consultas_relacionadas", mcp.Description("SQL o ids de historial relacionados"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithString("agente", mcp.Description("Identificador del asistente/agente de origen, si se conoce")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Tema                  string   `json:"tema"`
			Tipo                  string   `json:"tipo"`
			Enunciado             string   `json:"enunciado"`
			Contexto              string   `json:"contexto"`
			Evidencia             []string `json:"evidencia"`
			ConsultasRelacionadas []string `json:"consultas_relacionadas"`
			Agente                string   `json:"agente"`
		}) (*mcp.CallToolResult, error) {
			kind, ok := tipoAIngles(args.Tipo)
			if !ok {
				return toolError(errTipo)
			}
			id, err := deps.Store.RecordObservation(ctx, store.Observation{
				Subject:            args.Tema,
				Kind:               kind,
				Statement:          args.Enunciado,
				Context:            args.Contexto,
				EvidenceJSON:       mustJSON(args.Evidencia),
				RelatedQueriesJSON: mustJSON(args.ConsultasRelacionadas),
				Agent:              args.Agente,
			})
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "registrar_observacion", "observacion_creada", mustJSON(map[string]any{"id": id, "tema": args.Tema}), "")
			return toJSONResult(map[string]any{"id": id, "estado": args.Tipo})
		}),
	)

	s.AddTool(
		mcp.NewTool("proponer_conocimiento",
			mcp.WithDescription("Convierte observaciones/evidencia en una propuesta formal de conocimiento. El estado inicial siempre es 'propuesto', NUNCA 'validado' automáticamente: requiere aprobación humana vía aprobar_conocimiento. Si 'reemplaza_a_id' se indica, esta propuesta es una nueva versión de un conocimiento existente (se conserva el historial completo, nunca se sobrescribe). Para conocimiento de un concepto+contexto enseñado directamente por el usuario en la conversación, prefiere la tool aprender_del_usuario: verifica automáticamente contra el schema real."),
			mcp.WithString("concepto", mcp.Required(), mcp.Description("Tema, ej. 'cortesia'")),
			mcp.WithString("contexto", mcp.Description("Ámbito de aplicación, ej. 'ventas' (evita que se extrapole a otros contextos)")),
			mcp.WithString("afirmacion", mcp.Required(), mcp.Description("Afirmación propuesta, ej. 'pedido.estado = 10 representa una cortesía'")),
			mcp.WithArray("evidencia", mcp.Description("Evidencia que sustenta la afirmación"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithNumber("confianza", mcp.Description("Confianza estimada 0.0-1.0 (nunca sustituye la validación humana)"), mcp.Min(0), mcp.Max(1)),
			mcp.WithArray("observaciones_origen_ids", mcp.Description("IDs de observaciones que originaron esta propuesta"), mcp.Items(map[string]any{"type": "number"})),
			mcp.WithNumber("reemplaza_a_id", mcp.Description("ID de un conocimiento previo que esta propuesta actualiza (versionado)")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Concepto               string   `json:"concepto"`
			Contexto               string   `json:"contexto"`
			Afirmacion             string   `json:"afirmacion"`
			Evidencia              []string `json:"evidencia"`
			Confianza              *float64 `json:"confianza"`
			ObservacionesOrigenIDs []int64  `json:"observaciones_origen_ids"`
			ReemplazaAID           *int64   `json:"reemplaza_a_id"`
		}) (*mcp.CallToolResult, error) {
			version := 1
			if args.ReemplazaAID != nil {
				if prev, err := deps.Store.GetKnowledge(ctx, *args.ReemplazaAID); err == nil && prev != nil {
					version = prev.Version + 1
				}
			}
			id, err := deps.Store.ProposeKnowledge(ctx, store.Knowledge{
				Subject:                  args.Concepto,
				Context:                  args.Contexto,
				Claim:                    args.Afirmacion,
				EvidenceJSON:             mustJSON(args.Evidencia),
				Confidence:               args.Confianza,
				Version:                  version,
				SupersedesID:             args.ReemplazaAID,
				SourceObservationIDsJSON: mustJSON(args.ObservacionesOrigenIDs),
			})
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "proponer_conocimiento", "propuesta_creada", mustJSON(map[string]any{"id": id, "concepto": args.Concepto}), "")
			return toJSONResult(map[string]any{"id": id, "estado": "propuesto", "version": version})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_conocimiento_pendiente",
			mcp.WithDescription("Lista las propuestas de conocimiento pendientes de validación humana (estado='propuesto')."),
			mcp.WithNumber("limite", mcp.Description("Máximo de propuestas a devolver"), mcp.DefaultNumber(50)),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Limite int `json:"limite"`
		}) (*mcp.CallToolResult, error) {
			if args.Limite <= 0 {
				args.Limite = 50
			}
			pending, err := deps.Store.PendingKnowledge(ctx, args.Limite)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(conocimientosAJSON(pending, aliases))
		}),
	)

	s.AddTool(
		mcp.NewTool("aprobar_conocimiento",
			mcp.WithDescription("Aprueba una propuesta de conocimiento (estado 'propuesto' -> 'validado'). Requiere identificar quién aprueba. A partir de este momento el conocimiento puede reutilizarse como fuente confiable por cualquier asistente MCP conectado."),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("ID de la propuesta a aprobar")),
			mcp.WithString("aprobado_por", mcp.Required(), mcp.Description("Quién aprueba (usuario/administrador)")),
			mcp.WithString("nota", mcp.Description("Nota opcional de la decisión")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			ID          int64  `json:"id"`
			AprobadoPor string `json:"aprobado_por"`
			Nota        string `json:"nota"`
		}) (*mcp.CallToolResult, error) {
			k, err := deps.Store.ApproveKnowledge(ctx, args.ID, args.AprobadoPor, args.Nota)
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "aprobar_conocimiento", "conocimiento_aprobado", mustJSON(map[string]any{"id": args.ID, "por": args.AprobadoPor}), "")
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(conocimientoAJSON(*k, aliases))
		}),
	)

	s.AddTool(
		mcp.NewTool("rechazar_conocimiento",
			mcp.WithDescription("Rechaza una propuesta de conocimiento (estado 'propuesto' -> 'rechazado'). El registro se conserva completo, nunca se elimina."),
			mcp.WithNumber("id", mcp.Required(), mcp.Description("ID de la propuesta a rechazar")),
			mcp.WithString("rechazado_por", mcp.Required(), mcp.Description("Quién rechaza")),
			mcp.WithString("nota", mcp.Description("Motivo del rechazo")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			ID           int64  `json:"id"`
			RechazadoPor string `json:"rechazado_por"`
			Nota         string `json:"nota"`
		}) (*mcp.CallToolResult, error) {
			k, err := deps.Store.RejectKnowledge(ctx, args.ID, args.RechazadoPor, args.Nota)
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "rechazar_conocimiento", "conocimiento_rechazado", mustJSON(map[string]any{"id": args.ID, "por": args.RechazadoPor}), "")
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}
			return toJSONResult(conocimientoAJSON(*k, aliases))
		}),
	)

	s.AddTool(
		mcp.NewTool("aprender_del_usuario",
			mcp.WithDescription(
				"Registra una enseñanza EXPLÍCITA de un humano sobre de dónde sale un concepto de negocio (ej. 'el vendedor se obtiene de pedidosventa.codUsuario -> usuario.codUsuario'). "+
					"Regla obligatoria: cuando no exista conocimiento validado para un concepto (usa primero obtener_conocimiento_validado), el agente NUNCA debe adivinar una columna por similitud de nombre ni inventar una relación. "+
					"Debe preguntarle al usuario cómo se obtiene el dato, y solo entonces llamar a esta tool con lo que el usuario respondió. "+
					"Esta tool verifica automáticamente contra el schema real las tablas/columnas/relación mencionadas (rechaza la enseñanza si ninguna referencia existe, para no guardar estructuras inventadas) y crea una propuesta ('propuesto'), NUNCA conocimiento validado directamente: sigue requiriendo aprobar_conocimiento por un humano."),
			mcp.WithString("concepto", mcp.Required(), mcp.Description("Concepto de negocio que el usuario está enseñando, ej. 'vendedor', 'utilidad'")),
			mcp.WithString("contexto", mcp.Required(), mcp.Description("Ámbito donde aplica esta enseñanza, ej. 'ventas', 'reportes de ventas'. El mismo concepto puede tener fuentes distintas en otros contextos: nunca se extrapola.")),
			mcp.WithString("explicacion", mcp.Required(), mcp.Description("Lo que el usuario explicó, en sus propias palabras")),
			mcp.WithArray("columnas", mcp.Description("Columnas mencionadas por el usuario, formato 'tabla.columna', ej. ['pedidosventa.codUsuario']"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithString("relacion_origen", mcp.Description("Lado origen de una relación mencionada, formato 'tabla.columna'")),
			mcp.WithString("relacion_destino", mcp.Description("Lado destino de una relación mencionada, formato 'tabla.columna'")),
			mcp.WithString("formula", mcp.Description("Fórmula de cálculo si el concepto es derivado, ej. 'precio_venta - costo_producto'")),
			mcp.WithArray("evidencia", mcp.Description("Evidencia adicional (ids de consultas, mensajes, etc.)"), mcp.Items(map[string]any{"type": "string"})),
			mcp.WithString("ensenado_por", mcp.Description("Quién enseñó esto (usuario humano), si se conoce")),
			mcp.WithArray("alias", mcp.Description(
				"Alias legibles enseñados junto con este concepto, ej. [{\"referencia\":\"movimientostock\",\"alias\":\"movimientos de stock\"},{\"referencia\":\"movimientostock.codNotaSalida\",\"alias\":\"id\"}]. "+
					"A diferencia del resto de esta tool, los alias se aplican de inmediato (no requieren aprobar_conocimiento): son solo el nombre con el que hay que hablarle al usuario de esa tabla/columna, nunca el nombre crudo de base de datos. Usa actualizar_alias para corregir uno ya enseñado."),
				mcp.Items(map[string]any{
					"type":     "object",
					"required": []string{"referencia", "alias"},
					"properties": map[string]any{
						"referencia": map[string]any{"type": "string", "description": "'tabla' o 'tabla.columna' exacto del schema"},
						"alias":      map[string]any{"type": "string", "description": "Nombre legible para mostrarle al usuario"},
					},
				})),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Concepto        string   `json:"concepto"`
			Contexto        string   `json:"contexto"`
			Explicacion     string   `json:"explicacion"`
			Columnas        []string `json:"columnas"`
			RelacionOrigen  string   `json:"relacion_origen"`
			RelacionDestino string   `json:"relacion_destino"`
			Formula         string   `json:"formula"`
			Evidencia       []string `json:"evidencia"`
			EnsenadoPor     string   `json:"ensenado_por"`
			Alias           []struct {
				Referencia string `json:"referencia"`
				Alias      string `json:"alias"`
			} `json:"alias"`
		}) (*mcp.CallToolResult, error) {
			if strings.TrimSpace(args.Concepto) == "" || strings.TrimSpace(args.Contexto) == "" || strings.TrimSpace(args.Explicacion) == "" {
				return toolError(fmt.Errorf("concepto, contexto y explicacion son obligatorios"))
			}

			snap, err := deps.Inspector.Current(ctx)
			if err != nil {
				return toolError(err)
			}

			verifColumnas := verificarReferencias(snap, args.Columnas)

			var relacionInfo map[string]any
			relacionVerificada := false
			if args.RelacionOrigen != "" || args.RelacionDestino != "" {
				if args.RelacionOrigen == "" || args.RelacionDestino == "" {
					return toolError(fmt.Errorf("si se indica una relación hay que dar tanto relacion_origen como relacion_destino"))
				}
				origenExiste := columnaExiste(snap, args.RelacionOrigen)
				destinoExiste := columnaExiste(snap, args.RelacionDestino)
				esFK := origenExiste && destinoExiste && esClaveForanea(snap, args.RelacionOrigen, args.RelacionDestino)
				relacionInfo = map[string]any{
					"origen":         args.RelacionOrigen,
					"destino":        args.RelacionDestino,
					"origen_existe":  origenExiste,
					"destino_existe": destinoExiste,
					"es_fk_real":     esFK,
				}
				relacionVerificada = origenExiste && destinoExiste
			}

			algunaColumnaVerificada := false
			for _, c := range verifColumnas {
				if c["existe"].(bool) {
					algunaColumnaVerificada = true
					break
				}
			}

			seDieronReferencias := len(args.Columnas) > 0 || args.RelacionOrigen != ""
			seVerificoAlgo := algunaColumnaVerificada || relacionVerificada

			if seDieronReferencias && !seVerificoAlgo {
				return toJSONResult(map[string]any{
					"creado":                false,
					"motivo":                "Ninguna de las tablas/columnas/relación mencionadas existe en el schema actual. No se creó ninguna propuesta para evitar guardar una estructura inventada. Verifica los nombres (usa buscar_en_base_datos) o llama a refrescar_esquema si el schema cambió recientemente, y vuelve a intentarlo.",
					"verificacion_columnas": verifColumnas,
					"verificacion_relacion": relacionInfo,
				})
			}

			var existente *store.Knowledge
			if found, err := deps.Store.FindValidatedKnowledge(ctx, args.Concepto, args.Contexto); err == nil && len(found) > 0 {
				existente = &found[0]
			}
			version := 1
			var reemplazaAID *int64
			if existente != nil {
				version = existente.Version + 1
				reemplazaAID = &existente.ID
			}

			fuente := map[string]any{
				"columnas": args.Columnas,
				"relacion": relacionInfo,
				"formula":  args.Formula,
			}

			id, err := deps.Store.ProposeKnowledge(ctx, store.Knowledge{
				Subject:      args.Concepto,
				Context:      args.Contexto,
				Claim:        args.Explicacion,
				EvidenceJSON: mustJSON(args.Evidencia),
				SourceJSON:   mustJSON(fuente),
				Version:      version,
				SupersedesID: reemplazaAID,
				TaughtBy:     args.EnsenadoPor,
			})
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "aprender_del_usuario", "propuesta_creada_desde_ensenanza",
				mustJSON(map[string]any{"id": id, "concepto": args.Concepto, "contexto": args.Contexto}), "")

			creado, err := deps.Store.GetKnowledge(ctx, id)
			if err != nil {
				return toolError(err)
			}

			var aliasResultados []map[string]any
			for _, a := range args.Alias {
				res := aplicarAlias(ctx, deps, snap, a.Referencia, a.Alias, args.EnsenadoPor)
				aliasResultados = append(aliasResultados, res)
				recordAudit(ctx, deps, "aprender_del_usuario", "alias_aplicado", mustJSON(res), "")
			}

			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}

			return toJSONResult(map[string]any{
				"creado":                true,
				"propuesta":             conocimientoAJSON(*creado, aliases),
				"verificacion_columnas": verifColumnas,
				"verificacion_relacion": relacionInfo,
				"alias_aplicados":       aliasResultados,
				"nota":                  "Esto es una PROPUESTA pendiente de validación humana. Pide confirmación y usa aprobar_conocimiento(id=" + fmt.Sprint(id) + ") antes de tratarla como un hecho confiable. Los alias, en cambio, ya quedaron aplicados de inmediato.",
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("actualizar_alias",
			mcp.WithDescription(
				"Crea o CORRIGE el alias legible de una tabla o columna real (ej. referencia='movimientostock.codNotaSalida', alias='id'). Se aplica de inmediato, sin aprobación. "+
					"Los alias se resuelven al momento de responder, no se graban dentro del texto del conocimiento: corregir aquí el alias de una tabla/columna actualiza automáticamente cómo se presenta TODO el conocimiento ya enseñado que la referencia, sin necesidad de volver a enseñarlo. "+
					"A partir de esta llamada, el asistente debe usar el alias (no el nombre crudo de base de datos) al hablar con el usuario sobre esa tabla/columna."),
			mcp.WithString("referencia", mcp.Required(), mcp.Description("'tabla' o 'tabla.columna' exacto del schema real")),
			mcp.WithString("alias", mcp.Required(), mcp.Description("Nuevo nombre legible para mostrarle al usuario")),
			mcp.WithString("actualizado_por", mcp.Description("Quién enseñó/corrigió este alias, si se conoce")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Referencia     string `json:"referencia"`
			Alias          string `json:"alias"`
			ActualizadoPor string `json:"actualizado_por"`
		}) (*mcp.CallToolResult, error) {
			snap, err := deps.Inspector.Current(ctx)
			if err != nil {
				return toolError(err)
			}
			res := aplicarAlias(ctx, deps, snap, args.Referencia, args.Alias, args.ActualizadoPor)
			recordAudit(ctx, deps, "actualizar_alias", "alias_aplicado", mustJSON(res), "")
			if aplicado, _ := res["aplicado"].(bool); !aplicado {
				return toJSONResult(res)
			}
			res["nota"] = "El conocimiento ya enseñado que referencia esta tabla/columna se mostrará con el nuevo alias de inmediato, sin volver a enseñarlo."
			return toJSONResult(res)
		}),
	)
}

func tipoAIngles(tipo string) (string, bool) {
	switch tipo {
	case "observacion":
		return "observation", true
	case "hipotesis":
		return "hypothesis", true
	default:
		return "", false
	}
}

// verificarReferencias comprueba, para cada "tabla.columna", si existe
// realmente en el schema descubierto. Nunca se confía en lo que el usuario
// escribe sin contrastarlo: es la aplicación literal de "no inventar
// columnas" del diseño.
func verificarReferencias(snap *schema.Snapshot, refs []string) []map[string]any {
	out := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		out = append(out, map[string]any{
			"referencia": ref,
			"existe":     columnaExiste(snap, ref),
		})
	}
	return out
}

func columnaExiste(snap *schema.Snapshot, ref string) bool {
	tableName, colName, ok := strings.Cut(ref, ".")
	if !ok {
		return false
	}
	t, ok := snap.Tables[tableName]
	if !ok {
		return false
	}
	for _, c := range t.Columns {
		if c.Name == colName {
			return true
		}
	}
	return false
}

// tablaExiste comprueba si una referencia es una tabla completa (sin
// "columna") que existe en el schema real.
func tablaExiste(snap *schema.Snapshot, ref string) bool {
	_, ok := snap.Tables[ref]
	return ok
}

// aplicarAlias verifica cada referencia enseñada ("tabla" o "tabla.columna")
// contra el schema real y, si existe, guarda el alias de inmediato (sin
// aprobación: es solo un nombre de presentación). Nunca crea un alias para
// una tabla/columna inventada, igual que aprender_del_usuario nunca crea
// conocimiento sobre una referencia inventada.
func aplicarAlias(ctx context.Context, deps *Deps, snap *schema.Snapshot, referencia, alias, ensenadoPor string) map[string]any {
	referencia = strings.TrimSpace(referencia)
	alias = strings.TrimSpace(alias)
	if referencia == "" || alias == "" {
		return map[string]any{"referencia": referencia, "aplicado": false, "motivo": "referencia y alias son obligatorios"}
	}

	tableName, colName, esColumna := strings.Cut(referencia, ".")
	var existe bool
	if esColumna {
		existe = columnaExiste(snap, referencia)
	} else {
		tableName = referencia
		colName = ""
		existe = tablaExiste(snap, referencia)
	}
	if !existe {
		return map[string]any{
			"referencia": referencia,
			"aplicado":   false,
			"motivo":     "no existe esa tabla/columna en el schema actual, no se guardó el alias",
		}
	}

	anterior, _ := deps.Store.GetEntityAlias(ctx, tableName, colName)
	saved, err := deps.Store.UpsertEntityAlias(ctx, tableName, colName, alias, ensenadoPor)
	if err != nil {
		return map[string]any{"referencia": referencia, "aplicado": false, "motivo": err.Error()}
	}
	out := map[string]any{"referencia": referencia, "aplicado": true, "alias": saved.Alias}
	if anterior != nil && anterior.Alias != alias {
		out["alias_anterior"] = anterior.Alias
	}
	return out
}

// esClaveForanea comprueba si origen -> destino coincide con una FK real
// descubierta por introspección (hecho), no con una suposición por nombre.
func esClaveForanea(snap *schema.Snapshot, origen, destino string) bool {
	origenTabla, origenCol, ok := strings.Cut(origen, ".")
	if !ok {
		return false
	}
	destinoTabla, destinoCol, ok := strings.Cut(destino, ".")
	if !ok {
		return false
	}
	t, ok := snap.Tables[origenTabla]
	if !ok {
		return false
	}
	for _, fk := range t.ForeignKeys {
		if fk.Table == destinoTabla && fk.Column == destinoCol {
			return true
		}
	}
	for _, c := range t.Columns {
		if c.Name == origenCol && c.ForeignKey != nil && c.ForeignKey.Table == destinoTabla && c.ForeignKey.Column == destinoCol {
			return true
		}
	}
	return false
}
