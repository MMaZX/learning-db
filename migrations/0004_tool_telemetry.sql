-- Contador agregado de uso por tool: una fila por tool (no un log que crece
-- sin límite como query_history/audit_log), porque lo único que se necesita
-- para responder "cuántas veces se usa cada herramienta" es el conteo, no el
-- historial de cada llamada individual (eso ya lo cubre audit_log para las
-- tools que auditan). Se alimenta desde un middleware único en server.go que
-- envuelve TODAS las tools, así que nunca depende de que cada handler se
-- acuerde de instrumentarse.
CREATE TABLE IF NOT EXISTS tool_usage (
    tool           TEXT PRIMARY KEY,
    call_count     INTEGER NOT NULL DEFAULT 0,
    error_count    INTEGER NOT NULL DEFAULT 0,
    first_used_at  TEXT,
    last_used_at   TEXT
);
