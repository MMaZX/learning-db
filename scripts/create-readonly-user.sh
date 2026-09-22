#!/usr/bin/env bash
#
# create-readonly-user.sh — crea el usuario MySQL/MariaDB de solo lectura
# (SELECT + SHOW VIEW, nada más) que usa el MCP db-intelligence (ia-jarvis-gr).
#
# Es el wrapper de scripts/create-readonly-user.sql: sustituye los
# placeholders y lo ejecuta contra el servidor indicado con un usuario
# administrador (uno que SÍ pueda hacer CREATE USER / GRANT).
#
# Uso:
#   scripts/create-readonly-user.sh \
#     --admin-user root \
#     --db-database database_multi_final \
#     --db-username sigea_mcp_auditor \
#     [--host 127.0.0.1] [--port 3307] \
#     [--db-password 'contraseña-fuerte']   # si se omite, se genera una aleatoria
#
# Opciones:
#   --host           Host de MySQL/MariaDB (default: 127.0.0.1)
#   --port           Puerto (default: 3306)
#   --admin-user     Usuario admin para CONECTARSE y crear el usuario (pide password por prompt)
#   --db-database    Base de datos a la que el usuario de solo lectura tendrá acceso (obligatorio)
#   --db-username    Nombre del usuario de solo lectura a crear (obligatorio)
#   --db-password    Contraseña del usuario de solo lectura (si se omite, se genera una)
#   --dry-run        Solo imprime el SQL final, no lo ejecuta
#   -h|--help
#
# Al terminar, imprime las variables DB_* listas para copiar a .env o al
# comando `claude mcp add` / la config del cliente MCP que estés usando.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL_TEMPLATE="$SCRIPT_DIR/create-readonly-user.sql"

HOST="127.0.0.1"
PORT="3306"
ADMIN_USER=""
DB_DATABASE=""
DB_USERNAME=""
DB_PASSWORD=""
DRY_RUN=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        --host) HOST="$2"; shift 2 ;;
        --port) PORT="$2"; shift 2 ;;
        --admin-user) ADMIN_USER="$2"; shift 2 ;;
        --db-database) DB_DATABASE="$2"; shift 2 ;;
        --db-username) DB_USERNAME="$2"; shift 2 ;;
        --db-password) DB_PASSWORD="$2"; shift 2 ;;
        --dry-run) DRY_RUN=1; shift ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "Opción desconocida: $1" >&2
            exit 1
            ;;
    esac
done

if [[ -z "$ADMIN_USER" || -z "$DB_DATABASE" || -z "$DB_USERNAME" ]]; then
    echo "Faltan argumentos obligatorios: --admin-user, --db-database, --db-username" >&2
    echo "Corre con --help para ver el uso." >&2
    exit 1
fi

if [[ -z "$DB_PASSWORD" ]]; then
    DB_PASSWORD="$(LC_ALL=C tr -dc 'A-Za-z0-9_!@#%^&*-' < /dev/urandom | head -c 32)"
    echo "No se pasó --db-password: se generó una aleatoria." >&2
fi

if [[ ! -f "$SQL_TEMPLATE" ]]; then
    echo "No se encontró $SQL_TEMPLATE" >&2
    exit 1
fi

RENDERED_SQL="$(sed \
    -e "s/__DB_USERNAME__/${DB_USERNAME//\//\\/}/g" \
    -e "s/__DB_PASSWORD__/${DB_PASSWORD//\//\\/}/g" \
    -e "s/__DB_DATABASE__/${DB_DATABASE//\//\\/}/g" \
    "$SQL_TEMPLATE")"

if [[ $DRY_RUN -eq 1 ]]; then
    echo "$RENDERED_SQL"
    exit 0
fi

if ! command -v mysql >/dev/null 2>&1; then
    echo "No se encontró el cliente 'mysql' en PATH." >&2
    exit 1
fi

echo "Conectando a ${HOST}:${PORT} como '${ADMIN_USER}' (te pedirá su contraseña)..." >&2
echo "$RENDERED_SQL" | mysql -h "$HOST" -P "$PORT" -u "$ADMIN_USER" -p

cat <<EOF

Usuario de solo lectura creado. Variables listas para copiar:

DB_HOST=$HOST
DB_PORT=$PORT
DB_DATABASE=$DB_DATABASE
DB_USERNAME=$DB_USERNAME
DB_PASSWORD=$DB_PASSWORD

Recomendado: verifica los permisos reales corriendo scripts/delete-sql.sql
conectado como este usuario (ver ese archivo para instrucciones).
EOF
