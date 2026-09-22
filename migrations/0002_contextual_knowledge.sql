-- Añade soporte para conocimiento CONTEXTUAL: un mismo concepto de negocio
-- (ej. "vendedor") puede tener una fuente distinta según el contexto (ej.
-- "ventas" vs "compras"). Nunca se extrapola un concepto de un contexto a
-- otro sin evidencia propia.
--
-- source_json guarda de forma estructurada lo que el usuario enseñó y lo que
-- el MCP verificó contra el schema real: tablas/columnas mencionadas,
-- relación origen->destino, si esa relación coincide con una FK real, y una
-- fórmula opcional (para conceptos calculados como "utilidad").
ALTER TABLE knowledge ADD COLUMN context TEXT;
ALTER TABLE knowledge ADD COLUMN source_json TEXT;
ALTER TABLE knowledge ADD COLUMN taught_by TEXT;

CREATE INDEX IF NOT EXISTS idx_knowledge_context ON knowledge(context);
