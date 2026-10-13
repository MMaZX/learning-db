-- Migración 0009: agregar 'revoked' al CHECK constraint de status en knowledge.
-- Permite la revocación formal de conocimiento validado erróneo o derogado,
-- manteniendo integridad referencial, índices y triggers FTS5.

CREATE TABLE IF NOT EXISTS knowledge_new (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    subject             TEXT NOT NULL,
    claim               TEXT NOT NULL,
    evidence_json       TEXT,
    confidence          REAL,
    status              TEXT NOT NULL CHECK (status IN ('proposed','validated','rejected','deprecated','revoked')) DEFAULT 'proposed',
    version             INTEGER NOT NULL DEFAULT 1,
    supersedes_id       INTEGER REFERENCES knowledge(id),
    source_observation_ids_json TEXT,
    created_at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    decided_by          TEXT,
    decided_at          TEXT,
    decision_note       TEXT,
    context             TEXT,
    source_json         TEXT,
    taught_by           TEXT,
    topic_key           TEXT
);

INSERT INTO knowledge_new (
    id, subject, claim, evidence_json, confidence, status, version, supersedes_id,
    source_observation_ids_json, created_at, decided_by, decided_at, decision_note,
    context, source_json, taught_by, topic_key
)
SELECT
    id, subject, claim, evidence_json, confidence, status, version, supersedes_id,
    source_observation_ids_json, created_at, decided_by, decided_at, decision_note,
    context, source_json, taught_by, topic_key
FROM knowledge;

DROP TABLE knowledge;

ALTER TABLE knowledge_new RENAME TO knowledge;

CREATE INDEX IF NOT EXISTS idx_knowledge_subject ON knowledge(subject);
CREATE INDEX IF NOT EXISTS idx_knowledge_status ON knowledge(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_topic_key_validated ON knowledge(topic_key) WHERE status = 'validated';

-- Recrear triggers de FTS5 para apuntar a la tabla renovada
DROP TRIGGER IF EXISTS knowledge_ai;
DROP TRIGGER IF EXISTS knowledge_ad;
DROP TRIGGER IF EXISTS knowledge_au;

CREATE TRIGGER IF NOT EXISTS knowledge_ai AFTER INSERT ON knowledge BEGIN
    INSERT INTO knowledge_fts(rowid, subject, claim, context)
    VALUES (new.id, new.subject, new.claim, new.context);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_ad AFTER DELETE ON knowledge BEGIN
    INSERT INTO knowledge_fts(knowledge_fts, rowid, subject, claim, context)
    VALUES ('delete', old.id, old.subject, old.claim, old.context);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_au AFTER UPDATE ON knowledge BEGIN
    INSERT INTO knowledge_fts(knowledge_fts, rowid, subject, claim, context)
    VALUES ('delete', old.id, old.subject, old.claim, old.context);
    INSERT INTO knowledge_fts(rowid, subject, claim, context)
    VALUES (new.id, new.subject, new.claim, new.context);
END;
