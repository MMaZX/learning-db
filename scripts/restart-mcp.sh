#!/usr/bin/env bash
#
# restart-mcp.sh — mata/reinicia procesos de servidores MCP colgados o duplicados
# de agentes de programación (Claude Code, OpenCode, Aider, Agy, Cursor, etc.)
# sin necesidad de reiniciar la computadora.
#
# Cuando un agente crashea o queda "colgado", sus procesos MCP hijos a veces
# sobreviven como huérfanos (reparented a init) o quedan duplicados. Este
# script los detecta y los termina; la mayoría de los agentes vuelven a
# lanzar el MCP automáticamente en la próxima llamada a herramienta / sesión.
#
# Uso:
#   scripts/restart-mcp.sh --list                 Solo muestra lo que encontró (no mata nada)
#   scripts/restart-mcp.sh --kill-orphans         (default) Mata solo MCP cuyo agente padre ya murió
#   scripts/restart-mcp.sh --kill-all             Mata TODOS los procesos MCP detectados
#   scripts/restart-mcp.sh --agent claude         Restringe a un agente (claude|opencode|aider|agy|cursor|...)
#   scripts/restart-mcp.sh --dry-run              Combina con cualquier modo: muestra qué mataría, sin matar
#   scripts/restart-mcp.sh --force                Permite matar aunque el proceso cuelgue del propio terminal actual
#   scripts/restart-mcp.sh -h|--help
#
set -euo pipefail

# --- Patrones de procesos "host" de agentes de programación ---------------
AGENT_PATTERNS=(
    "claude"
    "opencode-cli|ai\\.opencode\\.desktop"
    "aider"
    "agy"
    "cursor"
    "continue"
    "windsurf"
    "cline"
    "zed"
)

# --- Patrones de procesos de servidores MCP --------------------------------
MCP_PATTERNS=(
    "mcp-server"
    "mcp_server"
    " mcp\\b"
    "mcp@"
    "playwright-mcp"
    "playwright/mcp"
    "codegraph"
    "gopls"
    "dart_mcp_server"
    "db-intelligence-mcp"
    "jarvis"
)

MODE="kill-orphans"
DRY_RUN=0
FORCE=0
AGENT_FILTER=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --list) MODE="list" ;;
        --kill-orphans) MODE="kill-orphans" ;;
        --kill-all) MODE="kill-all" ;;
        --agent) AGENT_FILTER="${2:-}"; shift ;;
        --dry-run) DRY_RUN=1 ;;
        --force) FORCE=1 ;;
        -h|--help)
            grep '^#' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "Opción desconocida: $1" >&2
            exit 1
            ;;
    esac
    shift
done

MCP_REGEX=$(IFS='|'; echo "${MCP_PATTERNS[*]}")
AGENT_REGEX=$(IFS='|'; echo "${AGENT_PATTERNS[*]}")

# PID/PPID de este propio script y de la shell que lo invoca, para no auto-matarnos.
SELF_PID=$$
SELF_TREE=("$SELF_PID" "$PPID")

is_alive() {
    kill -0 "$1" 2>/dev/null
}

# Sube la cadena de ppid hasta encontrar un proceso "host" de agente vivo,
# o hasta llegar a pid 1 (huérfano real).
find_agent_ancestor() {
    local pid="$1" ppid args
    local depth=0
    while [[ "$pid" -gt 1 && $depth -lt 25 ]]; do
        ppid=$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ')
        args=$(ps -o args= -p "$pid" 2>/dev/null)
        [[ -z "${ppid:-}" ]] && break
        if [[ "$args" =~ $AGENT_REGEX ]]; then
            echo "$pid"
            return 0
        fi
        pid="$ppid"
        depth=$((depth + 1))
    done
    return 1
}

declare -a ORPHAN_PIDS=()
declare -a ALL_PIDS=()
declare -A AGENT_OF_PID=()

while read -r pid comm args; do
    [[ -z "$pid" ]] && continue
    # nunca tocar este mismo script/shell
    for s in "${SELF_TREE[@]}"; do
        [[ "$pid" == "$s" ]] && continue 2
    done

    ancestor_pid=""
    if ancestor_pid=$(find_agent_ancestor "$pid"); then
        ancestor_comm=$(ps -o comm= -p "$ancestor_pid" 2>/dev/null || echo "?")
        AGENT_OF_PID["$pid"]="$ancestor_comm(pid $ancestor_pid)"
    else
        AGENT_OF_PID["$pid"]="ORPHAN"
        ORPHAN_PIDS+=("$pid")
    fi
    ALL_PIDS+=("$pid")
done < <(ps -eo pid=,comm=,args= | grep -Ei "$MCP_REGEX" | grep -v -E 'restart-mcp\.sh|grep')

if [[ -n "$AGENT_FILTER" ]]; then
    declare -a FILTERED=()
    for pid in "${ALL_PIDS[@]}"; do
        if [[ "${AGENT_OF_PID[$pid]}" =~ $AGENT_FILTER ]] || { [[ "$AGENT_FILTER" == "orphan" ]] && [[ "${AGENT_OF_PID[$pid]}" == "ORPHAN" ]]; }; then
            FILTERED+=("$pid")
        fi
    done
    ALL_PIDS=("${FILTERED[@]}")
fi

if [[ ${#ALL_PIDS[@]} -eq 0 ]]; then
    echo "No se encontraron procesos MCP que coincidan."
    exit 0
fi

printf "%-8s %-28s %-40s %s\n" "PID" "AGENTE" "COMANDO" "RSS(MB)"
for pid in "${ALL_PIDS[@]}"; do
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ') || continue
    args=$(ps -o args= -p "$pid" 2>/dev/null)
    rss_mb=$(( ${rss:-0} / 1024 ))
    printf "%-8s %-28s %-40s %s\n" "$pid" "${AGENT_OF_PID[$pid]}" "${args:0:40}" "$rss_mb"
done

case "$MODE" in
    list)
        exit 0
        ;;
    kill-orphans)
        TARGETS=("${ORPHAN_PIDS[@]}")
        if [[ -n "$AGENT_FILTER" ]]; then
            declare -a T2=()
            for pid in "${TARGETS[@]}"; do
                for f in "${ALL_PIDS[@]}"; do [[ "$pid" == "$f" ]] && T2+=("$pid"); done
            done
            TARGETS=("${T2[@]}")
        fi
        ;;
    kill-all)
        TARGETS=("${ALL_PIDS[@]}")
        ;;
esac

if [[ ${#TARGETS[@]} -eq 0 ]]; then
    echo
    echo "Nada que matar en modo '$MODE'."
    exit 0
fi

echo
echo "Modo: $MODE  ->  ${#TARGETS[@]} proceso(s) objetivo."

for pid in "${TARGETS[@]}"; do
    is_alive "$pid" || continue
    if [[ $DRY_RUN -eq 1 ]]; then
        echo "[dry-run] mataría PID $pid ($(ps -o args= -p "$pid" 2>/dev/null | cut -c1-60))"
        continue
    fi
    pgid=$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ')
    echo "-> SIGTERM a PID $pid (grupo $pgid)"
    if [[ -n "$pgid" ]]; then
        kill -TERM "-$pgid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    else
        kill -TERM "$pid" 2>/dev/null || true
    fi
done

if [[ $DRY_RUN -eq 0 ]]; then
    sleep 2
    for pid in "${TARGETS[@]}"; do
        if is_alive "$pid"; then
            pgid=$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ')
            echo "-> SIGKILL a PID $pid (no respondió a SIGTERM)"
            if [[ -n "$pgid" ]]; then
                kill -KILL "-$pgid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
            else
                kill -KILL "$pid" 2>/dev/null || true
            fi
        fi
    done
    echo
    echo "Listo. Los agentes que sigan activos deberían relanzar sus MCP en la próxima llamada a herramienta."
    echo "Si un agente ya no responde a sus herramientas MCP tras esto, reinicia esa sesión puntual (no hace falta reiniciar la PC)."
fi
