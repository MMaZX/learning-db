package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fulanito/db-intelligence-mcp/internal/knowledge"
	"github.com/fulanito/db-intelligence-mcp/internal/store"
)

func docTitles(docs []knowledge.Document) []map[string]string {
	out := make([]map[string]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, map[string]string{"path": d.Path, "title": d.Title})
	}
	return out
}

// formatRowsTSV serializa filas de resultados SQL como TSV: cabecera de
// columnas una sola vez, en vez del JSON de []map[string]any que repite cada
// nombre de columna en cada fila. Es la estructura tabular donde más pesa
// el formato, así que se trata aparte del resto de tools (que se quedan en
// JSON compacto vía toJSONResult).
func formatRowsTSV(columns []string, rows []map[string]any) string {
	var b strings.Builder
	b.WriteString(strings.Join(columns, "\t"))
	b.WriteByte('\n')
	for _, row := range rows {
		for i, col := range columns {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(formatCellTSV(row[col]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// formatCellTSV formatea un valor de celda para TSV, sustituyendo tabs y
// saltos de línea embebidos (rompen el formato) por espacios, y marcando
// NULL explícitamente para no confundirlo con una cadena vacía.
func formatCellTSV(v any) string {
	if v == nil {
		return "NULL"
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprint(v)
	}
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// aliasMap construye el lookup "tabla" -> alias y "tabla.columna" -> alias
// a partir de lo enseñado con aprender_del_usuario/actualizar_alias. Se
// recalcula en cada llamada de tool (barato: son pocas filas) para que un
// alias corregido se refleje de inmediato en toda respuesta posterior, sin
// caché que pueda quedar desincronizada.
func aliasMap(ctx context.Context, deps *Deps) (map[string]string, error) {
	aliases, err := deps.Store.AllEntityAliases(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(aliases))
	for _, a := range aliases {
		key := a.TableName
		if a.ColumnName != "" {
			key = a.TableName + "." + a.ColumnName
		}
		out[key] = a.Alias
	}
	return out, nil
}

// resolveAlias busca el alias de una tabla ("tabla") o columna
// ("tabla.columna") en el mapa devuelto por aliasMap.
func resolveAlias(aliases map[string]string, ref string) string {
	return aliases[ref]
}

// recordAudit es un helper de conveniencia que ignora errores de auditoría
// (nunca debe romper la respuesta al agente si sqlite falla momentáneamente),
// pero los registra vía logger si se provee.
func recordAudit(ctx context.Context, deps *Deps, tool, action, detail, errMsg string) {
	entry := store.AuditEntry{Tool: tool, Action: action, Detail: detail, Error: errMsg}
	if err := deps.Store.RecordAudit(ctx, entry); err != nil && deps.Logger != nil {
		deps.Logger.Warn("no se pudo registrar auditoría", "error", err)
	}
}
