// Package migrations embebe los scripts SQL de metadata del MCP (SQLite)
// para que el binario compilado no dependa de archivos externos en disco.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
