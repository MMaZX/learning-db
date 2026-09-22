-- =============================================================================
-- delete-sql.sql — Prueba de penetración de permisos para el usuario MySQL/
-- MariaDB de solo lectura que usará el MCP db-intelligence.
--
-- OBJETIVO: intentar CADA operación destructiva o de escalado de privilegios
-- que un usuario mal configurado podría ejecutar, y confirmar que TODAS
-- fallan con Access denied (o equivalente). El resultado de correr este
-- script es la evidencia real que respalda "solo lectura" en el README,
-- no una suposición sobre los GRANT que se ejecutaron.
--
-- IMPORTANTE:
--   1. Correr esto conectado DIRECTO con el cliente `mysql`, como el usuario
--      de prueba — nunca a través del MCP (el MCP ya bloquea esto a nivel
--      de aplicación; este script existe para probar la base de datos en sí,
--      capa independiente y más importante que la del código Go).
--   2. Si es posible, ejecutar primero contra una COPIA restaurada de la
--      base de datos, no contra producción. Si algún GRANT quedó mal
--      configurado, este script SÍ puede causar daño real: ese es el punto.
--   3. Cada bloque está etiquetado con el resultado esperado. Un resultado
--      distinto al esperado es una alerta que hay que resolver antes de
--      documentar el usuario como "solo lectura" en cualquier README.
--   4. Reemplaza <TABLA_PRUEBA> por el nombre de una tabla real que exista
--      en la base de datos (cualquiera sirve, incluso una vacía/irrelevante,
--      ya que la prueba es de permisos, no de contenido).
--
-- USO:
--   mysql -h <host> -u <usuario_prueba> -p <database_multi_final> \
--     < delete-sql.sql > resultado.txt 2>&1
--
--   Luego revisa resultado.txt: TODO lo que no sea SELECT/SHOW/EXPLAIN debe
--   aparecer como ERROR. Si ves "Query OK" en cualquier bloque que no sea
--   de la sección 0 (diagnóstico), DETENTE — hay un GRANT mal puesto.
-- =============================================================================

SELECT '=== SECCIÓN 0: DIAGNÓSTICO (debe funcionar, es de solo lectura) ===' AS seccion;

-- Qué privilegios tiene realmente este usuario, según el propio servidor.
-- Esto es lo que respalda con hechos lo que el resto del script confirma
-- por la vía práctica (intentar y fallar).
SELECT CURRENT_USER() AS usuario_actual;
SHOW GRANTS FOR CURRENT_USER();
SELECT * FROM information_schema.USER_PRIVILEGES WHERE GRANTEE LIKE CONCAT('%', SUBSTRING_INDEX(CURRENT_USER(),'@',1), '%');
SELECT * FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE LIKE CONCAT('%', SUBSTRING_INDEX(CURRENT_USER(),'@',1), '%');


SELECT '=== SECCIÓN 1: DML directo — debe fallar TODO ===' AS seccion;

DELETE FROM <TABLA_PRUEBA> WHERE 1=1;
DELETE FROM <TABLA_PRUEBA> LIMIT 1;
UPDATE <TABLA_PRUEBA> SET id = id WHERE 1=1;
INSERT INTO <TABLA_PRUEBA> VALUES ();
REPLACE INTO <TABLA_PRUEBA> VALUES ();
TRUNCATE TABLE <TABLA_PRUEBA>;

-- CTE/subconsulta como vector indirecto de escritura.
INSERT INTO <TABLA_PRUEBA> SELECT * FROM <TABLA_PRUEBA> LIMIT 1;

-- Escritura disfrazada de "solo lectura" vía función con efecto secundario
-- o vía CALL a un procedimiento hipotético con permisos DEFINER elevados.
DO (SELECT 1 FROM <TABLA_PRUEBA> WHERE 1=0);


SELECT '=== SECCIÓN 2: DDL sobre tablas/índices — debe fallar TODO ===' AS seccion;

DROP TABLE <TABLA_PRUEBA>;
DROP TABLE IF EXISTS <TABLA_PRUEBA>;
ALTER TABLE <TABLA_PRUEBA> ADD COLUMN backdoor_test INT;
ALTER TABLE <TABLA_PRUEBA> DROP COLUMN id;
ALTER TABLE <TABLA_PRUEBA> RENAME TO tabla_secuestrada;
RENAME TABLE <TABLA_PRUEBA> TO tabla_secuestrada;
CREATE TABLE tabla_inyectada (id INT);
CREATE TABLE tabla_inyectada AS SELECT * FROM <TABLA_PRUEBA>;
CREATE INDEX idx_inyectado ON <TABLA_PRUEBA> (id);
DROP INDEX PRIMARY ON <TABLA_PRUEBA>;
ALTER TABLE <TABLA_PRUEBA> ENGINE = MEMORY;


SELECT '=== SECCIÓN 3: Base de datos completa — debe fallar TODO ===' AS seccion;

CREATE DATABASE db_inyectada;
DROP DATABASE database_multi_final;
ALTER DATABASE database_multi_final CHARACTER SET utf8mb4;


SELECT '=== SECCIÓN 4: Objetos con lógica ejecutable — debe fallar TODO ===' AS seccion;

CREATE VIEW vista_inyectada AS SELECT * FROM <TABLA_PRUEBA>;
DROP VIEW IF EXISTS vista_inyectada;
CREATE PROCEDURE proc_escalada() BEGIN UPDATE <TABLA_PRUEBA> SET id = id; END;
CREATE FUNCTION func_escalada() RETURNS INT DETERMINISTIC RETURN 1;
CREATE TRIGGER trg_inyectado BEFORE INSERT ON <TABLA_PRUEBA> FOR EACH ROW SET NEW.id = NEW.id;
CREATE EVENT evento_inyectado ON SCHEDULE EVERY 1 MINUTE DO UPDATE <TABLA_PRUEBA> SET id = id;
DROP PROCEDURE IF EXISTS proc_escalada;


SELECT '=== SECCIÓN 5: Escalado de privilegios y gestión de usuarios — debe fallar TODO ===' AS seccion;

CREATE USER 'usuario_inyectado'@'%' IDENTIFIED BY 'password123';
GRANT ALL PRIVILEGES ON *.* TO CURRENT_USER();
GRANT ALL PRIVILEGES ON database_multi_final.* TO CURRENT_USER();
GRANT INSERT, UPDATE, DELETE ON database_multi_final.* TO CURRENT_USER();
GRANT SUPER ON *.* TO CURRENT_USER();
REVOKE SELECT ON database_multi_final.* FROM CURRENT_USER();
SET PASSWORD = PASSWORD('nueva_password');
ALTER USER CURRENT_USER() IDENTIFIED BY 'nueva_password';
DROP USER 'root'@'localhost';
RENAME USER CURRENT_USER() TO 'usuario_disfrazado'@'%';

-- Vector directo: escribir en las tablas del catálogo de privilegios.
INSERT INTO mysql.user (Host, User) VALUES ('%', 'backdoor');
UPDATE mysql.user SET Super_priv = 'Y' WHERE User = CURRENT_USER();
DELETE FROM mysql.db WHERE Db = 'database_multi_final';


SELECT '=== SECCIÓN 6: Administración del servidor — debe fallar TODO ===' AS seccion;

SET GLOBAL read_only = 0;
SET GLOBAL general_log = 'ON';
SET GLOBAL slow_query_log = 'OFF';
SET SESSION sql_log_bin = 0;
FLUSH PRIVILEGES;
FLUSH LOGS;
FLUSH TABLES WITH READ LOCK;
RESET MASTER;
RESET SLAVE;
PURGE BINARY LOGS BEFORE NOW();
CHANGE MASTER TO MASTER_HOST='atacante.example.com';
SHOW PROCESSLIST;

-- NOTA: KILL CONNECTION_ID() NO va aquí. Matar tu PROPIA sesión siempre está
-- permitido en MySQL/MariaDB sin ningún privilegio especial (no es una fuga
-- de permisos) y, como efecto secundario, aborta el resto del script. Por
-- eso vive solo, al final del todo (sección 9.1), en su propia conexión.


SELECT '=== SECCIÓN 7: Locking / disponibilidad — debe fallar o no persistir ===' AS seccion;

LOCK TABLES <TABLA_PRUEBA> WRITE;
UNLOCK TABLES;
SELECT * FROM <TABLA_PRUEBA> FOR UPDATE;


SELECT '=== SECCIÓN 8: Acceso al sistema de archivos del servidor — debe fallar TODO ===' AS seccion;

SELECT 1 INTO OUTFILE '/tmp/exfiltrado.csv';
SELECT LOAD_FILE('/etc/passwd');
LOAD DATA INFILE '/tmp/cualquiera.csv' INTO TABLE <TABLA_PRUEBA>;
CREATE TABLE tabla_desde_archivo (dato TEXT) SELECT LOAD_FILE('/etc/passwd') AS dato;


SELECT '=== SECCIÓN 9: Sentencias apiladas (si el cliente lo permite) — debe fallar TODO lo no-SELECT ===' AS seccion;

SELECT 1; DELETE FROM <TABLA_PRUEBA> WHERE 1=1;


SELECT '=== SECCIÓN 9.1: auto-terminación de sesión (va al final, en su propia conexión) ===' AS seccion;

-- Matar tu propia CONNECTION_ID() está permitido para cualquier usuario en
-- MySQL/MariaDB por diseño: no es un privilegio elevado, es abandonar tu
-- propia sesión. Se incluye solo para dejarlo documentado como "esperado",
-- no como fuga. Debe cortar la conexión (eso es éxito, no fallo del test).
KILL CONNECTION_ID();


SELECT '=== FIN. Revisa resultado.txt: cada bloque de la 1 a la 9 debe mostrar ERROR (salvo 9.1, que corta la conexión por diseño). Solo la 0 y los SELECT/SHOW de diagnóstico deben tener éxito. ===' AS seccion;
