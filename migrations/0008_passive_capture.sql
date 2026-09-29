-- Captura pasiva de consultas exitosas: un registro por conjunto de tablas
-- (table_set = nombres ordenados separados por coma, sin SQL ni literales).
-- Sirve para deduplicar y limitar la tasa de observaciones generadas
-- automáticamente: cada consulta suma hits/last_seen, pero solo se inserta una
-- observación la primera vez que se ve el conjunto (o pasada la ventana desde
-- last_observed). last_observed queda NULL si el tope horario impidió crearla.
CREATE TABLE IF NOT EXISTS passive_capture (
    table_set     TEXT PRIMARY KEY,
    first_seen    TEXT NOT NULL,
    last_seen     TEXT NOT NULL,
    last_observed TEXT,
    hits          INTEGER NOT NULL DEFAULT 1
);

-- Para contar rápido las observaciones pasivas de la última hora (tope global).
CREATE INDEX IF NOT EXISTS idx_observations_agent_created ON observations(agent, created_at);
