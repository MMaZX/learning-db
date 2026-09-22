package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/fulanito/db-intelligence-mcp/internal/schema"
)

func registerSchemaTools(s *server.MCPServer, deps *Deps) {
	s.AddTool(
		mcp.NewTool("obtener_esquema_bd",
			mcp.WithDescription("Devuelve una visión general del schema: tablas, tipo, comentarios, número de columnas y claves primarias. Por defecto es compacto (sin listar cada columna) para no saturar el contexto en schemas grandes; usa 'filtro' para acotar por substring de nombre de tabla, o 'detallado=true' para incluir columnas completas."),
			mcp.WithString("filtro", mcp.Description("Substring opcional para filtrar tablas por nombre")),
			mcp.WithBoolean("detallado", mcp.Description("Si es true, incluye el listado completo de columnas por tabla (puede ser grande)"), mcp.DefaultBool(false)),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Filtro    string `json:"filtro"`
			Detallado bool   `json:"detallado"`
		}) (*mcp.CallToolResult, error) {
			snap, err := deps.Inspector.Current(ctx)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}

			type columnaConAlias struct {
				schema.Column
				Alias string `json:"alias,omitempty"`
			}

			type tableSummary struct {
				Nombre          string            `json:"nombre"`
				Alias           string            `json:"alias,omitempty"`
				Tipo            string            `json:"tipo"`
				Comentario      string            `json:"comentario,omitempty"`
				NumColumnas     int               `json:"num_columnas"`
				ClavesPrimarias []string          `json:"claves_primarias,omitempty"`
				FilasEstimadas  int64             `json:"filas_estimadas,omitempty"`
				Columnas        []columnaConAlias `json:"columnas,omitempty"`
			}

			names := sortedTableNames(snap)
			filterLower := strings.ToLower(args.Filtro)

			var out []tableSummary
			for _, name := range names {
				if filterLower != "" && !strings.Contains(strings.ToLower(name), filterLower) {
					continue
				}
				t := snap.Tables[name]
				ts := tableSummary{
					Nombre:          t.Name,
					Alias:           resolveAlias(aliases, t.Name),
					Tipo:            t.Type,
					Comentario:      t.Comment,
					NumColumnas:     len(t.Columns),
					FilasEstimadas:  t.RowEstimate,
					ClavesPrimarias: primaryKeyNames(t),
				}
				if args.Detallado {
					for _, c := range t.Columns {
						ts.Columnas = append(ts.Columnas, columnaConAlias{
							Column: c,
							Alias:  resolveAlias(aliases, t.Name+"."+c.Name),
						})
					}
				}
				out = append(out, ts)
			}

			return toJSONResult(map[string]any{
				"base_datos":     snap.Database,
				"actualizado_en": snap.RefreshedAt,
				"total_tablas":   len(snap.Tables),
				"tablas":         out,
				"nota_alias":     "Cuando una tabla/columna tenga 'alias', úsalo al hablar con el usuario en vez del nombre crudo de base de datos.",
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("obtener_esquema_tabla",
			mcp.WithDescription("Devuelve el detalle completo de una tabla: columnas, tipos, nullability, defaults, PK, FK, índices, comentarios y qué otras tablas la referencian."),
			mcp.WithString("tabla", mcp.Required(), mcp.Description("Nombre exacto de la tabla o vista")),
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
				return toolError(fmt.Errorf("la tabla %q no existe (o está excluida) en la base de datos %q", args.Tabla, snap.Database))
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}

			type columnaConAlias struct {
				schema.Column
				Alias string `json:"alias,omitempty"`
			}
			columnas := make([]columnaConAlias, 0, len(t.Columns))
			for _, c := range t.Columns {
				columnas = append(columnas, columnaConAlias{Column: c, Alias: resolveAlias(aliases, t.Name+"."+c.Name)})
			}

			out := map[string]any{
				"nombre":           t.Name,
				"alias":            resolveAlias(aliases, t.Name),
				"tipo":             t.Type,
				"comentario":       t.Comment,
				"columnas":         columnas,
				"indices":          t.Indexes,
				"claves_foraneas":  t.ForeignKeys,
				"referenciada_por": t.ReferencedBy,
				"filas_estimadas":  t.RowEstimate,
				"nota_alias":       "Cuando la tabla o una columna tenga 'alias', úsalo al hablar con el usuario en vez del nombre crudo de base de datos.",
			}
			return toJSONResult(out)
		}),
	)

	s.AddTool(
		mcp.NewTool("buscar_en_base_datos",
			mcp.WithDescription("Busca tablas, columnas o comentarios que coincidan con un término. Útil para explorar el schema sin conocerlo de antemano, ej. buscar todo lo relacionado con 'pagos'."),
			mcp.WithString("consulta", mcp.Required(), mcp.Description("Término de búsqueda, ej. 'pago', 'cliente', 'estado'")),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct {
			Consulta string `json:"consulta"`
		}) (*mcp.CallToolResult, error) {
			snap, err := deps.Inspector.Current(ctx)
			if err != nil {
				return toolError(err)
			}
			aliases, err := aliasMap(ctx, deps)
			if err != nil {
				return toolError(err)
			}

			type columnaMatch struct {
				Nombre string `json:"nombre"`
				Alias  string `json:"alias,omitempty"`
			}
			type match struct {
				Tabla             string         `json:"tabla"`
				AliasTabla        string         `json:"alias_tabla,omitempty"`
				CoincideEnTabla   bool           `json:"coincide_en_nombre_tabla,omitempty"`
				ColumnasCoinciden []columnaMatch `json:"columnas_coinciden,omitempty"`
				Comentario        string         `json:"comentario,omitempty"`
			}

			q := strings.ToLower(args.Consulta)
			var results []match
			for _, name := range sortedTableNames(snap) {
				t := snap.Tables[name]
				m := match{Tabla: name, AliasTabla: resolveAlias(aliases, name)}
				if strings.Contains(strings.ToLower(name), q) {
					m.CoincideEnTabla = true
				}
				if strings.Contains(strings.ToLower(t.Comment), q) {
					m.CoincideEnTabla = true
					m.Comentario = t.Comment
				}
				for _, c := range t.Columns {
					if strings.Contains(strings.ToLower(c.Name), q) || strings.Contains(strings.ToLower(c.Comment), q) {
						m.ColumnasCoinciden = append(m.ColumnasCoinciden, columnaMatch{
							Nombre: c.Name,
							Alias:  resolveAlias(aliases, name+"."+c.Name),
						})
					}
				}
				if m.CoincideEnTabla || len(m.ColumnasCoinciden) > 0 {
					results = append(results, m)
				}
			}

			entities, documents := deps.Knowledge.Search(args.Consulta)

			return toJSONResult(map[string]any{
				"consulta":                args.Consulta,
				"coincidencias_schema":    results,
				"entidades_conocimiento":  entities,
				"documentos_conocimiento": docTitles(documents),
				"nota_alias":              "Cuando una tabla/columna tenga alias, úsalo al hablar con el usuario en vez del nombre crudo de base de datos.",
			})
		}),
	)

	s.AddTool(
		mcp.NewTool("refrescar_esquema",
			mcp.WithDescription("Vuelve a inspeccionar INFORMATION_SCHEMA y refresca la caché interna del schema. Informa qué cambió desde la última inspección (tablas/columnas/índices/FK añadidas, eliminadas o modificadas)."),
		),
		mcp.NewTypedToolHandler(func(ctx context.Context, req mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, error) {
			snap, diff, err := deps.Inspector.RefreshWithDiff(ctx)
			if err != nil {
				return toolError(err)
			}
			recordAudit(ctx, deps, "refrescar_esquema", "schema_refrescado", fmt.Sprintf("tablas=%d cambios=%v", len(snap.Tables), diff.HasChanges()), "")
			return toJSONResult(map[string]any{
				"base_datos":     snap.Database,
				"actualizado_en": snap.RefreshedAt,
				"total_tablas":   len(snap.Tables),
				"hubo_cambios":   diff.HasChanges(),
				"diferencias":    diff,
			})
		}),
	)
}

func sortedTableNames(snap *schema.Snapshot) []string {
	names := make([]string, 0, len(snap.Tables))
	for n := range snap.Tables {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func primaryKeyNames(t *schema.Table) []string {
	var out []string
	for _, c := range t.Columns {
		if c.PrimaryKey {
			out = append(out, c.Name)
		}
	}
	return out
}
