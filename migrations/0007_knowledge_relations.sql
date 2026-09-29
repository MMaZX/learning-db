-- Relaciones detectadas entre una propuesta de conocimiento (from_id) y filas
-- existentes (to_id): mismo topic_key, posible conflicto por similitud FTS,
-- candidata a reemplazo o coincidencia con conocimiento ya rechazado.
--
-- Solo se persiste la relación (from, to, tipo, score). El estado, la razón y
-- quién decidió sobre la fila relacionada se calculan al leer (JOIN con
-- knowledge), así nunca quedan desactualizados si esa fila se aprueba, se
-- rechaza o se depreca después. No modifica la tabla knowledge.
CREATE TABLE IF NOT EXISTS knowledge_relations (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    from_id    INTEGER NOT NULL REFERENCES knowledge(id),
    to_id      INTEGER NOT NULL REFERENCES knowledge(id),
    relation   TEXT NOT NULL CHECK (relation IN ('same_topic','possible_conflict','supersedes_candidate','previously_rejected')),
    score      REAL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (from_id, to_id, relation)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_relations_from ON knowledge_relations(from_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_relations_to ON knowledge_relations(to_id);
