-- =============================================================================
-- create-readonly-user.sql — crea (o recrea) el usuario MySQL/MariaDB de
-- solo lectura que usa el MCP db-intelligence (ia-jarvis-gr).
--
-- Da exactamente los permisos que el MCP necesita y nada más: SELECT y SHOW
-- VIEW sobre la base indicada. Nada de INSERT/UPDATE/DELETE/DDL/GRANT — la
-- capa de seguridad de MariaDB es la primera línea de defensa, independiente
-- del validador SQL por AST que ya trae el binario Go (internal/security).
--
-- USO:
--   No se ejecuta tal cual: reemplaza los tres placeholders de abajo
--   (o usa scripts/create-readonly-user.sh, que hace el reemplazo por ti).
--
--   mysql -h <host> -P <puerto> -u <admin> -p < create-readonly-user.sql
--
-- Placeholders a reemplazar:
--   __DB_USERNAME__   nombre del usuario de solo lectura (ej. sigea_mcp_auditor)
--   __DB_PASSWORD__   contraseña del usuario (usa una fuerte; no la commitees)
--   __DB_DATABASE__   base de datos a la que tiene acceso (ej. database_multi_final)
--
-- Después de correrlo, verifica los permisos reales con:
--   SHOW GRANTS FOR '__DB_USERNAME__'@'%';
-- y opcionalmente corre scripts/delete-sql.sql conectado como este usuario
-- para confirmar en la práctica que toda operación destructiva falla.
-- =============================================================================

-- Si ya existía con otra contraseña/permisos, lo recrea limpio.
DROP USER IF EXISTS '__DB_USERNAME__'@'%';

CREATE USER '__DB_USERNAME__'@'%' IDENTIFIED BY '__DB_PASSWORD__';

GRANT SELECT, SHOW VIEW
  ON `__DB_DATABASE__`.*
  TO '__DB_USERNAME__'@'%';

-- Sin USAGE global adicional, sin permisos de administración
-- (PROCESS, RELOAD, SUPER, GRANT OPTION, etc. quedan deliberadamente fuera).

FLUSH PRIVILEGES;

SHOW GRANTS FOR '__DB_USERNAME__'@'%';
