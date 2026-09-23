#!/usr/bin/env bash
#
# jarvis.sh — levanta, detiene y revisa todos los servicios de la UI de Jarvis.
#
#   Voicebox  :17493  voz local en GPU (~/development/voicebox)
#   MCP Go    :8080   db-intelligence en modo HTTP (MySQL read-only)
#   OmniRoute :20128  router LLM OpenAI-compatible
#   BFF Node  :4173   web/server (habla con MCP, OmniRoute y Voicebox)
#   Vite      :5173   frontend React (https://localhost:5173)
#
# Uso:
#   scripts/jarvis.sh up [servicio...]     Arranca los servicios (default: todos). Si el puerto ya
#                                          está ocupado, lo da por activo y no lo toca.
#   scripts/jarvis.sh down [servicio...]   Detiene SOLO lo que arrancó este script.
#   scripts/jarvis.sh restart [servicio...]
#   scripts/jarvis.sh status               Muestra qué puerto responde.
#   scripts/jarvis.sh logs <servicio>      Sigue el log (tail -f) de un servicio.
#   scripts/jarvis.sh -h|--help
#
# Servicios: voicebox mcp omniroute bff vite
#
# Configuración:
#   - El token del MCP sale de JARVIS_MCP_AUTH_TOKEN en web/.env y se pasa al proceso Go como
#     MCP_AUTH_TOKEN junto con MCP_TRANSPORT=http. El .env de la raíz NO se toca: los agentes
#     (Claude Code, OpenCode) siguen usando el mismo binario por stdio vía bin/run-jarvis.sh.
#   - OmniRoute se arranca con OMNIROUTE_CMD de web/.env, o con `omniroute` si está en el PATH.
#   - Logs y PIDs en .run/ (ignorado por git).
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB="$ROOT/web"
RUN_DIR="$ROOT/.run"
LOG_DIR="$RUN_DIR/logs"
VOICEBOX_DIR="${VOICEBOX_DIR:-$HOME/development/voicebox}"
MCP_BIN="$RUN_DIR/db-intelligence-mcp"

ALL_SERVICES=(voicebox mcp omniroute bff vite)
declare -A PORT=([voicebox]=17493 [mcp]=8080 [omniroute]=20128 [bff]=4173 [vite]=5173)
# Voicebox carga el modelo en GPU al arrancar; el resto tarda segundos.
declare -A WAIT_SECS=([voicebox]=90 [mcp]=30 [omniroute]=60 [bff]=20 [vite]=20)

if [[ -t 1 ]]; then
    C_OK=$'\e[32m' C_ERR=$'\e[31m' C_WARN=$'\e[33m' C_DIM=$'\e[2m' C_OFF=$'\e[0m'
else
    C_OK='' C_ERR='' C_WARN='' C_DIM='' C_OFF=''
fi
ok()   { printf '%s✔%s %s\n' "$C_OK" "$C_OFF" "$*"; }
err()  { printf '%s✘%s %s\n' "$C_ERR" "$C_OFF" "$*" >&2; }
warn() { printf '%s!%s %s\n' "$C_WARN" "$C_OFF" "$*"; }
info() { printf '%s·%s %s\n' "$C_DIM" "$C_OFF" "$*"; }

usage() { sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//'; }

# Lee una variable de web/.env sin ejecutar el archivo (no hacemos `source`).
env_get() {
    local key="$1" file="$WEB/.env" line
    [[ -f "$file" ]] || return 0
    line=$(grep -E "^${key}=" "$file" | tail -n1) || return 0
    line="${line#*=}"
    line="${line%\"}"; line="${line#\"}"
    line="${line%\'}"; line="${line#\'}"
    printf '%s' "$line"
}

port_open() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

pid_file() { printf '%s/%s.pid' "$RUN_DIR" "$1"; }

owned_pid() {
    local f; f=$(pid_file "$1")
    [[ -f "$f" ]] || return 1
    local pid; pid=$(<"$f")
    if kill -0 "$pid" 2>/dev/null; then
        printf '%s' "$pid"
    else
        rm -f "$f"
        return 1
    fi
}

# Lanza un comando en su propia sesión (setsid) para poder matar el árbol completo con down.
launch() {
    local name="$1" dir="$2"; shift 2
    local log="$LOG_DIR/$name.log"
    # Solo el nombre: la línea de comando puede llevar secretos (MCP_AUTH_TOKEN).
    printf '\n===== %s  arrancando %s =====\n' "$(date '+%F %T')" "$name" >>"$log"
    (cd "$dir" && exec setsid "$@" >>"$log" 2>&1 </dev/null) &
    echo $! >"$(pid_file "$name")"
}

wait_port() {
    local name="$1" port="${PORT[$1]}" secs="${WAIT_SECS[$1]}" i pid
    for ((i = 0; i < secs; i++)); do
        port_open "$port" && return 0
        pid=$(owned_pid "$name") || return 1   # el proceso murió antes de abrir el puerto
        sleep 1
    done
    return 1
}

start_voicebox() {
    if [[ ! -x "$VOICEBOX_DIR/backend/venv/bin/uvicorn" ]]; then
        warn "voicebox: no encuentro $VOICEBOX_DIR/backend/venv (define VOICEBOX_DIR). Jarvis funcionará sin voz."
        return 2
    fi
    launch voicebox "$VOICEBOX_DIR" backend/venv/bin/uvicorn backend.main:app --host 127.0.0.1 --port "${PORT[voicebox]}"
}

start_mcp() {
    local token; token=$(env_get JARVIS_MCP_AUTH_TOKEN)
    if [[ -z "$token" ]]; then
        err "mcp: falta JARVIS_MCP_AUTH_TOKEN en web/.env (el MCP en HTTP exige token)."
        return 1
    fi
    if [[ ! -f "$ROOT/.env" ]]; then
        err "mcp: falta $ROOT/.env con las credenciales DB_* (copia .env.example)."
        return 1
    fi
    info "mcp: compilando ./cmd/server…"
    if ! (cd "$ROOT" && go build -o "$MCP_BIN" ./cmd/server) >>"$LOG_DIR/mcp.log" 2>&1; then
        err "mcp: falló la compilación (ver $LOG_DIR/mcp.log)."
        return 1
    fi
    # godotenv no pisa variables ya definidas: estas ganan sobre el .env de la raíz.
    launch mcp "$ROOT" env \
        MCP_TRANSPORT=http \
        MCP_HTTP_ADDR="127.0.0.1:${PORT[mcp]}" \
        MCP_AUTH_TOKEN="$token" \
        "$MCP_BIN"
}

start_omniroute() {
    local cmd; cmd=$(env_get OMNIROUTE_CMD)
    if [[ -z "$cmd" ]]; then
        if command -v omniroute >/dev/null 2>&1; then
            cmd="omniroute serve --port ${PORT[omniroute]} --no-open --no-tray --log"
        else
            warn "omniroute: no está instalado. Instálalo o define OMNIROUTE_CMD en web/.env."
            warn "omniroute: sin él Jarvis no puede responder (el chat mostrará el error)."
            return 2
        fi
    fi
    # OMNIROUTE_CMD puede traer argumentos ("npx -y omniroute"), por eso va por bash -c.
    # Corre desde .run/: OmniRoute carga el .env del cwd y el de la raíz trae credenciales DB_*.
    # Solo loopback: por defecto escucha en 0.0.0.0 sin API key y la LAN podría gastar tus proveedores.
    launch omniroute "$RUN_DIR" env OMNIROUTE_SERVER_HOST=127.0.0.1 bash -c "exec $cmd"
}

start_bff() { launch bff "$WEB" npm run --silent server; }

start_vite() { launch vite "$WEB" npm run --silent dev; }

up_one() {
    local name="$1" port="${PORT[$1]}" rc=0
    if port_open "$port"; then
        if owned_pid "$name" >/dev/null; then
            ok "$name ya está activo en :$port"
        else
            ok "$name ya está activo en :$port ${C_DIM}(arrancado fuera de este script)${C_OFF}"
        fi
        return 0
    fi
    mkdir -p "$LOG_DIR"
    "start_$name" || rc=$?
    [[ $rc -eq 0 ]] || return "$rc"
    info "$name: esperando :$port (hasta ${WAIT_SECS[$name]}s)…"
    if wait_port "$name"; then
        ok "$name escuchando en :$port"
    else
        err "$name no abrió :$port. Últimas líneas de $LOG_DIR/$name.log:"
        tail -n 15 "$LOG_DIR/$name.log" >&2 || true
        return 1
    fi
}

down_one() {
    local name="$1" pid i
    if ! pid=$(owned_pid "$name"); then
        if port_open "${PORT[$name]}"; then
            info "$name: activo en :${PORT[$name]} pero no lo arrancó este script, no lo toco."
        else
            info "$name: no estaba corriendo."
        fi
        return 0
    fi
    kill -TERM "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    for ((i = 0; i < 10; i++)); do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.5
    done
    if kill -0 "$pid" 2>/dev/null; then
        kill -KILL "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
    fi
    rm -f "$(pid_file "$name")"
    ok "$name detenido"
}

status() {
    local name mark origin
    printf '%-10s %-7s %-9s %s\n' SERVICIO PUERTO ESTADO ORIGEN
    for name in "${ALL_SERVICES[@]}"; do
        if port_open "${PORT[$name]}"; then
            mark="${C_OK}activo${C_OFF}  "
        else
            mark="${C_ERR}apagado${C_OFF} "
        fi
        if owned_pid "$name" >/dev/null; then origin="jarvis.sh"; else origin="-"; fi
        printf '%-10s %-7s %s  %s\n' "$name" "${PORT[$name]}" "$mark" "$origin"
    done
}

resolve_services() {
    if [[ $# -eq 0 ]]; then
        printf '%s\n' "${ALL_SERVICES[@]}"
        return
    fi
    local s
    for s in "$@"; do
        if [[ -z "${PORT[$s]+x}" ]]; then
            err "servicio desconocido: $s (válidos: ${ALL_SERVICES[*]})"
            exit 1
        fi
        printf '%s\n' "$s"
    done
}

cmd="${1:-up}"
[[ $# -gt 0 ]] && shift

case "$cmd" in
    up)
        list=$(resolve_services "$@")
        mapfile -t targets <<<"$list"
        failed=0
        for s in "${targets[@]}"; do
            rc=0
            up_one "$s" || rc=$?
            [[ $rc -eq 1 ]] && failed=1
        done
        echo
        status
        echo
        if port_open "${PORT[vite]}"; then
            ok "Abre https://localhost:${PORT[vite]}"
        fi
        info "Logs: scripts/jarvis.sh logs <servicio>   ·   Apagar: scripts/jarvis.sh down"
        exit "$failed"
        ;;
    down)
        list=$(resolve_services "$@")
        mapfile -t targets <<<"$list"
        # Orden inverso: primero el frontend, al final lo que los demás consumen.
        for ((i = ${#targets[@]} - 1; i >= 0; i--)); do
            down_one "${targets[$i]}"
        done
        ;;
    restart)
        "$0" down "$@"
        exec "$0" up "$@"
        ;;
    status)
        status
        ;;
    logs)
        [[ $# -eq 1 ]] || { err "uso: scripts/jarvis.sh logs <servicio>"; exit 1; }
        resolve_services "$1" >/dev/null
        exec tail -n 50 -f "$LOG_DIR/$1.log"
        ;;
    -h|--help|help)
        usage
        ;;
    *)
        err "comando desconocido: $cmd"
        usage
        exit 1
        ;;
esac
