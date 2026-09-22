-- Metadata propia del MCP. Nunca se ejecuta contra la base de datos de
-- producción: vive en SQLite, en MCP_DATA_PATH, separada por completo.

CREATE TABLE IF NOT EXISTS query_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    tool          TEXT NOT NULL,
    sql_text      TEXT NOT NULL,
    agent         TEXT,
    duration_ms   INTEGER NOT NULL,
    row_count     INTEGER NOT NULL DEFAULT 0,
    truncated     INTEGER NOT NULL DEFAULT 0,
    error         TEXT,
    executed_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_query_history_executed_at ON query_history(executed_at);

CREATE TABLE IF NOT EXISTS audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    tool        TEXT NOT NULL,
    agent       TEXT,
    action      TEXT NOT NULL,
    detail_json TEXT,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_log_ts ON audit_log(ts);

-- Observaciones crudas: hechos notados durante una interacción, todavía sin
-- interpretar. kind distingue 'observation' (hecho notado) de 'hypothesis'
-- (posible explicación, explícitamente no un hecho).
CREATE TABLE IF NOT EXISTS observations (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    subject            TEXT NOT NULL,
    kind               TEXT NOT NULL CHECK (kind IN ('observation','hypothesis')),
    statement          TEXT NOT NULL,
    context            TEXT,
    evidence_json      TEXT,
    related_queries_json TEXT,
    agent              TEXT,
    created_at         TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_observations_subject ON observations(subject);

-- Propuestas de conocimiento y conocimiento validado viven en la misma
-- tabla, distinguidos por status, para conservar historial y versionado
-- (sección 14: nunca sobrescribir silenciosamente).
CREATE TABLE IF NOT EXISTS knowledge (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    subject             TEXT NOT NULL,
    claim                TEXT NOT NULL,
    evidence_json        TEXT,
    confidence           REAL,
    status               TEXT NOT NULL CHECK (status IN ('proposed','validated','rejected','deprecated')) DEFAULT 'proposed',
    version              INTEGER NOT NULL DEFAULT 1,
    supersedes_id        INTEGER REFERENCES knowledge(id),
    source_observation_ids_json TEXT,
    created_at           TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    decided_by           TEXT,
    decided_at           TEXT,
    decision_note        TEXT
);

CREATE INDEX IF NOT EXISTS idx_knowledge_subject ON knowledge(subject);
CREATE INDEX IF NOT EXISTS idx_knowledge_status ON knowledge(status);
