-- Índice FTS5 de contenido externo para acelerar el recall sobre knowledge
-- validado. "External content" evita duplicar subject/claim/context:
-- knowledge_fts sólo guarda el índice invertido, content_rowid mapea de
-- vuelta a knowledge.id. unicode61 remove_diacritics 2 hace la búsqueda
-- insensible a tildes ("gestion" encuentra "gestión") sin normalizar en Go.
-- No incluye topic_key: esa columna no existe hasta la migración 0006 y las
-- migraciones se aplican en transacciones separadas por nombre de archivo.
CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
    subject, claim, context,
    content='knowledge', content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

-- Triggers que mantienen knowledge_fts sincronizada con knowledge (patrón
-- estándar de external content). knowledge_ad/knowledge_au usan la fila de
-- comando especial 'delete' de FTS5 para borrar la entrada anterior en vez
-- de intentar insertar una fila normal con el mismo rowid.
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

-- Backfill: reconstruye el índice a partir de las filas ya existentes en
-- knowledge (incluye todos los status; el filtro por status='validated'
-- pasa en la query de lectura, no en el índice).
INSERT INTO knowledge_fts(knowledge_fts) VALUES ('rebuild');
