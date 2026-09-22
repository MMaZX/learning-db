-- Alias legibles para tablas/columnas reales del schema. Se resuelven en
-- tiempo de lectura (nunca se "hornean" dentro del texto de knowledge), así
-- que corregir un alias con actualizar_alias afecta de inmediato a todo el
-- conocimiento ya enseñado que referencia esa tabla/columna, sin necesidad de
-- volver a enseñarlo. column_name es NULL cuando el alias es de la tabla
-- completa (no de una columna concreta).
CREATE TABLE IF NOT EXISTS entity_aliases (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name  TEXT NOT NULL,
    column_name TEXT,
    alias       TEXT NOT NULL,
    taught_by   TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_entity_aliases_ref
    ON entity_aliases(table_name, COALESCE(column_name, ''));
