package mcpserver

import (
	"encoding/json"
	"strings"

	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

// Este archivo concentra la traducción entre el modelo interno (store,
// schema, query — en inglés, por convención Go) y la superficie expuesta a
// los asistentes MCP (en español). Es la única capa que habla el protocolo
// MCP, así que es la única que debe conocer este mapeo.

var estadoConocimientoAEspanol = map[string]string{
	store.KnowledgeStatusProposed:   "propuesto",
	store.KnowledgeStatusValidated:  "validado",
	store.KnowledgeStatusRejected:   "rechazado",
	store.KnowledgeStatusDeprecated: "obsoleto",
}

// conocimientoAJSON traduce el modelo interno a la superficie MCP. aliases
// es el lookup "tabla"/"tabla.columna" -> alias (ver aliasMap en util.go):
// cuando una columna mencionada en la fuente tiene alias enseñado, se anota
// en "fuente_alias" para que el asistente hable de ella por su alias, nunca
// por el nombre crudo de base de datos.
func conocimientoAJSON(k store.Knowledge, aliases map[string]string) map[string]any {
	out := map[string]any{
		"id":         k.ID,
		"concepto":   k.Subject,
		"afirmacion": k.Claim,
		"estado":     estadoLegible(k.Status),
		"version":    k.Version,
		"creado_en":  k.CreatedAt,
	}
	if k.Context != "" {
		out["contexto"] = k.Context
	}
	if k.Confidence != nil {
		out["confianza"] = *k.Confidence
	}
	if k.SupersedesID != nil {
		out["reemplaza_a_id"] = *k.SupersedesID
	}
	if v := parseJSONField(k.EvidenceJSON); v != nil {
		out["evidencia"] = v
	}
	if v := parseJSONField(k.SourceJSON); v != nil {
		out["fuente"] = v
		if fuenteAlias := aliasesDeFuente(v, aliases); len(fuenteAlias) > 0 {
			out["fuente_alias"] = fuenteAlias
			out["nota_alias"] = "Usa estos alias al hablar con el usuario de esta fuente; no menciones el nombre crudo de tabla/columna salvo que sea necesario técnicamente (ej. para escribir SQL)."
		}
	}
	if v := parseJSONField(k.SourceObservationIDsJSON); v != nil {
		out["observaciones_origen_ids"] = v
	}
	if k.TaughtBy != "" {
		out["ensenado_por"] = k.TaughtBy
	}
	if k.DecidedBy != "" {
		out["decidido_por"] = k.DecidedBy
	}
	if k.DecidedAt != nil {
		out["decidido_en"] = *k.DecidedAt
	}
	if k.DecisionNote != "" {
		out["nota_decision"] = k.DecisionNote
	}
	return out
}

func conocimientosAJSON(ks []store.Knowledge, aliases map[string]string) []map[string]any {
	out := make([]map[string]any, 0, len(ks))
	for _, k := range ks {
		out = append(out, conocimientoAJSON(k, aliases))
	}
	return out
}

// aliasesDeFuente recorre el campo "fuente" (columnas, relacion.origen,
// relacion.destino) y devuelve el alias de cada referencia "tabla.columna"
// o "tabla" que lo tenga enseñado.
func aliasesDeFuente(fuente any, aliases map[string]string) map[string]string {
	if len(aliases) == 0 {
		return nil
	}
	m, ok := fuente.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]string{}
	addRef := func(ref string) {
		if ref == "" {
			return
		}
		if alias := resolveAlias(aliases, ref); alias != "" {
			out[ref] = alias
		}
		if tabla, _, found := strings.Cut(ref, "."); found {
			if alias := resolveAlias(aliases, tabla); alias != "" {
				out[tabla] = alias
			}
		}
	}
	if cols, ok := m["columnas"].([]any); ok {
		for _, c := range cols {
			if s, ok := c.(string); ok {
				addRef(s)
			}
		}
	}
	if rel, ok := m["relacion"].(map[string]any); ok {
		if s, ok := rel["origen"].(string); ok {
			addRef(s)
		}
		if s, ok := rel["destino"].(string); ok {
			addRef(s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func estadoLegible(s string) string {
	if v, ok := estadoConocimientoAEspanol[s]; ok {
		return v
	}
	return s
}

var tipoObservacionAEspanol = map[string]string{
	"observation": "observacion",
	"hypothesis":  "hipotesis",
}

func observacionAJSON(o store.Observation) map[string]any {
	tipo := o.Kind
	if v, ok := tipoObservacionAEspanol[tipo]; ok {
		tipo = v
	}
	out := map[string]any{
		"id":        o.ID,
		"tema":      o.Subject,
		"tipo":      tipo,
		"enunciado": o.Statement,
		"creado_en": o.CreatedAt,
	}
	if o.Context != "" {
		out["contexto"] = o.Context
	}
	if o.Agent != "" {
		out["agente"] = o.Agent
	}
	if v := parseJSONField(o.EvidenceJSON); v != nil {
		out["evidencia"] = v
	}
	if v := parseJSONField(o.RelatedQueriesJSON); v != nil {
		out["consultas_relacionadas"] = v
	}
	return out
}

func observacionesAJSON(os []store.Observation) []map[string]any {
	out := make([]map[string]any, 0, len(os))
	for _, o := range os {
		out = append(out, observacionAJSON(o))
	}
	return out
}

func toolUsageAJSON(us []store.ToolUsage) []map[string]any {
	out := make([]map[string]any, 0, len(us))
	for _, u := range us {
		m := map[string]any{
			"tool":        u.Tool,
			"veces_usada": u.CallCount,
			"veces_fallo": u.ErrorCount,
		}
		if u.FirstUsedAt != nil {
			m["primer_uso"] = *u.FirstUsedAt
		}
		if u.LastUsedAt != nil {
			m["ultimo_uso"] = *u.LastUsedAt
		}
		out = append(out, m)
	}
	return out
}

func parseJSONField(raw string) any {
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}
