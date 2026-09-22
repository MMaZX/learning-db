# db-intelligence-mcp

MCP Server en Go que le da a cualquier asistente de IA compatible con MCP
(Claude, Codex, Gemini, OpenCode...) acceso de solo lectura e inteligente a
la base de datos `database_multi_final` (contenedor `manager_mysql`).

No es "un MCP que ejecuta SQL a secas": separa **schema** (hechos),
**documentación de negocio**, **observaciones**, **propuestas** y
**conocimiento validado**, y tiene una regla central: si no existe
conocimiento validado sobre un concepto, el agente debe **preguntarte a ti**
en vez de inventar una relación.

---

## 1. ¿Qué necesito tener corriendo?

Solo el contenedor `manager_mysql` (ya está corriendo), publicando MySQL en
el puerto `127.0.0.1:3307` (o el host/puerto que corresponda si el MCP corre
en otra máquina de la red — ver sección 3).

No hace falta compilar nada tú mismo: ya hay binarios listos en `dist/`.

```
dist/
├── db-intelligence-mcp-linux-amd64       ← para Linux (esta máquina)
└── db-intelligence-mcp-windows-amd64.exe ← para Windows
```

Son binarios estáticos (no dependen de nada más instalado en el sistema:
sin cgo, sin librerías de MySQL/SQLite externas). Copia el que corresponda a
donde lo vayas a usar.

---

## 2. Crear el usuario de solo lectura en MySQL

El MCP necesita un usuario dedicado con **únicamente** `SELECT` y
`SHOW VIEW` sobre la base de datos — nunca uses un usuario con permisos de
escritura, aunque el validador SQL del MCP (capa `internal/security`) ya
bloquee cualquier cosa que no sea `SELECT`. Hay un script que lo crea:

```bash
scripts/create-readonly-user.sh \
  --admin-user root \
  --db-database database_multi_final \
  --db-username sigea_mcp_auditor \
  --host 127.0.0.1 \
  --port 3307
# --db-password es opcional: si lo omites, genera una aleatoria y te la muestra al final
```

Te pide la contraseña del usuario admin (uno que pueda `CREATE USER`/`GRANT`)
y al terminar imprime las variables `DB_*` listas para copiar a `.env` o a
la config del cliente MCP. Para ver el SQL exacto sin ejecutarlo, agregá
`--dry-run`.

Por debajo corre `scripts/create-readonly-user.sql` (podés ejecutarlo a
mano si preferís no usar el wrapper bash — el propio archivo trae las
instrucciones). Después de crearlo, verificá los permisos reales — no solo
lo que crees que otorgaste — corriendo `scripts/delete-sql.sql` conectado
como ese usuario; ese script intenta cada operación destructiva posible y
confirma que todas fallan.

---

## 3. Levantar el servidor MCP (modo HTTP — recomendado)

Desde este cambio, el MCP soporta **`MCP_TRANSPORT=http`**: un único
proceso queda escuchando y **cualquier cantidad de clientes/sesiones se
conectan a la vez** contra ese mismo proceso, compartiendo pool de MySQL y
store de metadata (diseñados para eso). Es el contrato recomendado ahora en
vez de que cada cliente MCP levante su propio proceso por stdio.

Configurá `.env` (copiá `.env.example`) con, como mínimo:

```env
MCP_TRANSPORT=http
MCP_HTTP_ADDR=0.0.0.0:8080   # o 127.0.0.1:8080 si solo se accede desde esta misma máquina
MCP_AUTH_TOKEN=un-token-largo-y-secreto   # obligatorio en modo http

DB_HOST=127.0.0.1
DB_PORT=3307
DB_DATABASE=database_multi_final
DB_USERNAME=sigea_mcp_auditor
DB_PASSWORD=<la que generó/usaste en la sección 2>

MCP_KNOWLEDGE_PATH=/home/fulanito/development/mcp-bd/knowledge
MCP_DATA_PATH=/home/fulanito/development/mcp-bd/data
```

Y arrancá el binario (lee `.env` del directorio desde el que se ejecuta):

```bash
cd /home/fulanito/development/mcp-bd
./dist/db-intelligence-mcp-linux-amd64
```

Vas a ver en el log: `MCP listo, sirviendo por HTTP`. El servidor queda
escuchando en `http://<MCP_HTTP_ADDR>/mcp` — ese es el contrato que copian
todos los clientes en la sección 4: **URL + token**, nada de `command`/`env`
por cliente.

Notas:

- No hay TLS en el binario. Si `MCP_HTTP_ADDR` usa `0.0.0.0` (alcanzable
  desde otras máquinas de la red), restringí el acceso por firewall/VPN a la
  red interna de confianza. Para HTTPS o exposición más amplia, ponelo
  detrás de un proxy (Nginx/Caddy) o un túnel — no lo resuelve el binario.
- Para que quede corriendo persistente (no solo mientras tengas la terminal
  abierta), usá Docker (`MCP_TRANSPORT=http` en `.env`, descomentar el
  mapeo de puerto en `docker-compose.yml` y `docker compose up -d`) o un
  servicio `systemd` con el mismo binario + `.env`.
- El **modo stdio** (un proceso por cliente, sin multi-sesión) sigue
  existiendo como alternativa — ver sección 5 — pero HTTP es el contrato
  recomendado de acá en adelante.

---

## 4. Conectar clientes al nuevo contrato (HTTP + token)

En todos los casos de abajo, el servidor MCP se llama **`ia-jarvis-gr`**
(así aparece, por ejemplo, con el prefijo `mcp__ia-jarvis-gr__*` en Claude
Code) y los tres datos que necesitás son los de la sección 3:

- `MCP_URL` = `http://<MCP_HTTP_ADDR>/mcp` (ej. `http://127.0.0.1:8080/mcp`,
  o `http://<IP-del-servidor>:8080/mcp` si te conectás desde otra máquina)
- `MCP_AUTH_TOKEN` = el token que pusiste en `.env`

### 4.1 Claude Code

```bash
claude mcp add --transport http ia-jarvis-gr http://<HOST>:8080/mcp \
  -H "Authorization: Bearer <MCP_AUTH_TOKEN>" \
  -s local
```

`-s local` lo registra solo para vos en esta máquina. Para compartirlo con
el equipo vía `.mcp.json` versionado, usá `-s project` en su lugar.

Comandos útiles para administrarlo después:

```bash
claude mcp get ia-jarvis-gr
claude mcp remove ia-jarvis-gr -s local
claude mcp list
```

### 4.2 Claude Desktop

Edita el archivo de configuración (Linux/WSL:
`~/.config/Claude/claude_desktop_config.json`; Windows:
`%APPDATA%\Claude\claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ia-jarvis-gr": {
      "type": "http",
      "url": "http://<HOST>:8080/mcp",
      "headers": {
        "Authorization": "Bearer <MCP_AUTH_TOKEN>"
      }
    }
  }
}
```

Reinicia Claude Desktop por completo después de guardar.

### 4.3 Codex CLI

Bloque en `~/.codex/config.toml`. El token va por variable de entorno, no
en el archivo:

```toml
[mcp_servers.ia-jarvis-gr]
url = "http://<HOST>:8080/mcp"
bearer_token_env_var = "IA_JARVIS_GR_TOKEN"
```

Y en el entorno desde donde corras Codex:

```bash
export IA_JARVIS_GR_TOKEN='<MCP_AUTH_TOKEN>'
```

### 4.4 Gemini CLI

`mcpServers` en `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "ia-jarvis-gr": {
      "httpUrl": "http://<HOST>:8080/mcp",
      "headers": {
        "Authorization": "Bearer <MCP_AUTH_TOKEN>"
      }
    }
  }
}
```

### 4.5 OpenCode

Sección `mcp` en `opencode.json` (global en
`~/.config/opencode/opencode.json`, o de proyecto):

```json
{
  "mcp": {
    "ia-jarvis-gr": {
      "type": "remote",
      "url": "http://<HOST>:8080/mcp",
      "enabled": true,
      "headers": {
        "Authorization": "Bearer <MCP_AUTH_TOKEN>"
      }
    }
  }
}
```

Revisá la documentación de cada cliente para la sintaxis exacta si algo no
calza (estos formatos cambian con el tiempo) — el contrato de fondo (URL +
header `Authorization: Bearer`) es siempre el mismo.

Para verificar que está vivo desde cualquiera de estos clientes,
pregúntale directamente en el chat, por ejemplo: *"¿qué tablas hay en la
base de datos?"* — debería usar la tool `obtener_esquema_bd`.

---

## 5. Modo stdio (alternativa sin multi-sesión)

Si preferís que cada cliente levante su **propio proceso** (sin compartir
sesión con nadie más, y sin necesidad de dejar nada escuchando en un
puerto), el binario lo sigue soportando: es el comportamiento por defecto
(`MCP_TRANSPORT=stdio`, o directamente si no seteás esa variable).

Ejemplo con Claude Code:

```bash
claude mcp add ia-jarvis-gr \
  -e DB_HOST=127.0.0.1 \
  -e DB_PORT=3307 \
  -e DB_DATABASE=database_multi_final \
  -e DB_USERNAME=sigea_mcp_auditor \
  -e DB_PASSWORD='<la contraseña real>' \
  -e MCP_KNOWLEDGE_PATH=/home/fulanito/development/mcp-bd/knowledge \
  -e MCP_DATA_PATH=/home/fulanito/development/mcp-bd/data \
  -s local \
  -- /home/fulanito/development/mcp-bd/dist/db-intelligence-mcp-linux-amd64
```

Mismo `env` (7 variables) aplica a Claude Desktop, Codex, Gemini CLI y
OpenCode en su variante `command` + `env`/`environment` (no `url`/`headers`)
— revisá la documentación de cada uno para el nombre exacto del campo.

Nota Windows: si vas a correr el `.exe` **en otra computadora** (no en la
misma máquina que Docker/MySQL), `127.0.0.1` no la va a encontrar — usá el
modo HTTP de la sección 3/4 en su lugar, es justamente para ese caso.

---

## 6. Volver a compilar (si cambias el código)

```bash
cd /home/fulanito/development/mcp-bd

# Linux
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o dist/db-intelligence-mcp-linux-amd64 ./cmd/server

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" \
  -o dist/db-intelligence-mcp-windows-amd64.exe ./cmd/server
```

No hace falta reiniciar nada para que el MCP "aprenda" — eso se hace en
caliente con `aprender_del_usuario` / editando `knowledge/*.md` (ver sección
7). Solo recompilas si cambias el **código Go** (una tool nueva, una regla
de seguridad nueva, etc.). Si estás sirviendo en modo HTTP, recompilar
implica reiniciar el proceso — eso sí corta las sesiones activas en ese
momento (los clientes deberían reconectar solos).

Si prefieres Docker en vez de un binario suelto, sigue existiendo esa opción
(`Dockerfile` / `docker-compose.yml`), pero para uso diario en esta máquina
el binario nativo es más simple.

---

## 7. Cómo "enseñarle" cosas (recordatorio rápido)

- Recuperación al iniciar otra sesión: el servidor indica al cliente MCP que
  llame primero a `recordar_contexto` con la petición completa. Esta tool busca
  de una vez la memoria validada relevante, tolera variaciones de palabras y
  evita volver a descubrir relaciones que ya fueron aprobadas.
- Documentación de negocio a mano: edita `knowledge/business/*.md` y
  `knowledge/index.md`. Se relee solo, sin reiniciar.
- Enseñanza guiada en conversación: cuando el asistente te pregunte de dónde
  sale un dato (porque `obtener_conocimiento_validado` no encontró nada), tú
  le respondes y él llama a `aprender_del_usuario`. Queda como propuesta.
- Tú apruebas con `aprobar_conocimiento` (puedes pedirle al asistente que lo
  haga, dándole el ID de la propuesta y tu nombre) para que quede como
  conocimiento validado y reutilizable en el futuro.
- Alias legibles: cuando enseñas de dónde sale un dato, también puedes
  enseñar cómo se debe *llamar* esa tabla/columna al hablar contigo (ej.
  `movimientostock` → "movimientos de stock", `codNotaSalida` → "id"), pasando
  `alias` a `aprender_del_usuario`. A diferencia del resto de la enseñanza,
  el alias se aplica de inmediato, sin pasar por `aprobar_conocimiento`: es
  solo un nombre de presentación. Para corregir un alias ya enseñado, usa
  `actualizar_alias` — se resuelve en el momento de responder, así que
  corregirlo actualiza al instante cómo se presenta todo el conocimiento que
  ya referencia esa tabla/columna, sin tener que volver a enseñarlo. A partir
  de ahí, el asistente debe usar el alias y no el nombre crudo de base de
  datos al hablarte de esa entidad.

---

## 8. Las 19 tools disponibles

`obtener_esquema_bd`, `obtener_esquema_tabla`, `buscar_en_base_datos`,
`refrescar_esquema`, `consultar_base_datos`, `explicar_consulta`,
`recordar_contexto`, `obtener_conocimiento_negocio`, `buscar_conocimiento`,
`obtener_conocimiento_validado`, `obtener_entidad`, `obtener_relaciones`,
`registrar_observacion`, `proponer_conocimiento`, `aprender_del_usuario`,
`obtener_conocimiento_pendiente`, `aprobar_conocimiento`,
`rechazar_conocimiento`, `actualizar_alias`.

---

## 9. Seguridad (resumen)

- El usuario de solo lectura (creado con la sección 2, ej.
  `sigea_mcp_auditor`) solo tiene `SELECT`/`SHOW VIEW` sobre
  `database_multi_final` — no puede escribir aunque quisiera.
- Aparte de eso, el MCP valida cada SQL por AST (no regex) y rechaza
  cualquier cosa que no sea `SELECT`/`UNION`, sentencias múltiples o
  bloqueos explícitos, antes de que llegue a MySQL.
- Las credenciales de MySQL viven solo en `.env` (que está en `.gitignore`)
  — nunca se le entregan al modelo como texto que pueda repetir.
- En modo HTTP, `MCP_AUTH_TOKEN` es la única barrera de acceso al proceso:
  tratalo como una credencial (no lo commitees, no lo pegues en chats) y
  restringí `MCP_HTTP_ADDR` por red/firewall si no necesitás acceso amplio.

---

## 10. Estructura del proyecto (para referencia)

```
cmd/server/          punto de entrada
internal/config/      carga de variables de entorno
internal/database/    pool MySQL read-only reforzado
internal/schema/      introspección + caché + diff de schema
internal/security/    validador SQL (AST)
internal/query/       query engine (límites, timeout, concurrencia) + EXPLAIN
internal/knowledge/   carga de knowledge/index.md + knowledge/business/*.md
internal/store/       metadata en SQLite (historial, auditoría, observaciones, conocimiento)
internal/mcpserver/   tools MCP + traducción a español (traduccion.go)
migrations/           esquema SQLite embebido
knowledge/            documentación funcional + índice
dist/                 binarios ya compilados (Linux y Windows)
scripts/               create-readonly-user.{sql,sh}, delete-sql.sql (pentest), restart-mcp.sh
```

## 11. Tests

```bash
go test ./...
```
