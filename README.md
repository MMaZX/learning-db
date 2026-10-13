# db-intelligence-mcp

Servidor MCP en Go para consultar y explorar bases de datos MySQL con validación estricta de solo lectura, caché de esquema y persistencia de contexto en SQLite.

## Requisitos

- Go 1.22+ (para compilar desde código) o Docker.
- Acceso a una base de datos MySQL con permisos de solo lectura (`SELECT`, `SHOW VIEW`).

## Configuración (`.env`)

Copia `.env.example` a `.env` y ajusta las variables de conexión:

```env
# Modo de transporte: 'http' (recomendado) o 'stdio'
MCP_TRANSPORT=http
MCP_HTTP_ADDR=0.0.0.0:8080
MCP_AUTH_TOKEN=tu-token-secreto

# Conexión MySQL
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=mi_base_de_datos
DB_USERNAME=usuario_lectura
DB_PASSWORD=clave_lectura

# Rutas locales
MCP_DATA_PATH=./data
MCP_KNOWLEDGE_PATH=./knowledge
```

## Compilación y ejecución

### Binario local

```bash
# Compilar
go build -o db-intelligence-mcp ./cmd/server

# Ejecutar
./db-intelligence-mcp
```

### Docker / Docker Compose

```bash
docker compose up -d
```

## Conexión con clientes MCP

### Modo HTTP

El servidor expone el endpoint `/mcp`. En clientes como Claude Code, Cursor u OpenCode, configura la conexión con la URL y el token:

- **URL**: `http://localhost:8080/mcp`
- **Header**: `Authorization: Bearer <MCP_AUTH_TOKEN>` o `X-API-Key: <MCP_AUTH_TOKEN>`

### Modo stdio

Para clientes que ejecutan el proceso localmente:

```json
{
  "mcpServers": {
    "db-intelligence": {
      "command": "/ruta/absoluta/a/db-intelligence-mcp",
      "env": {
        "MCP_TRANSPORT": "stdio",
        "DB_HOST": "127.0.0.1",
        "DB_PORT": "3306",
        "DB_DATABASE": "mi_base_de_datos",
        "DB_USERNAME": "usuario_lectura",
        "DB_PASSWORD": "clave_lectura",
        "MCP_DATA_PATH": "/ruta/absoluta/a/data"
      }
    }
  }
}
```

## Herramientas (Tools)

### Esquema
- `obtener_esquema_bd`: Lista tablas, vistas y conteos generales.
- `obtener_esquema_tabla`: Detalle de columnas, tipos, índices y claves foráneas.
- `buscar_en_base_datos`: Busca tablas o columnas por coincidencia de nombre.
- `refrescar_esquema`: Actualiza la caché del esquema en memoria.

### Consultas
- `consultar_base_datos`: Ejecuta consultas SQL. Valida mediante AST que solo contengan sentencias `SELECT`.
- `explicar_consulta`: Ejecuta `EXPLAIN` sobre una consulta para analizar el plan de ejecución.

### Contexto y conocimiento
- `recordar_contexto`: Recupera conocimiento validado relevante para la consulta actual.
- `buscar_conocimiento`: Busca reglas y notas de negocio por texto.
- `obtener_conocimiento_por_id`: Consulta entradas específicas por su identificador.
- `obtener_conocimiento_validado`: Devuelve el conocimiento aprobado para tablas o conceptos.
- `obtener_conocimiento_negocio`: Lee archivos markdown de documentación de negocio.
- `obtener_entidad` y `obtener_relaciones`: Consulta entidades del modelo y sus vínculos.

### Aprendizaje y memoria
- `aprender_del_usuario`: Guarda aclaraciones o reglas explicadas por el usuario.
- `registrar_observacion`: Guarda notas sobre comportamientos o particularidades detectadas.
- `proponer_conocimiento`: Registra propuestas de reglas para posterior revisión.
- `obtener_conocimiento_pendiente`: Lista propuestas sin validar.
- `aprobar_conocimiento` y `rechazar_conocimiento`: Gestiona el estado de las propuestas.
- `actualizar_alias`: Asocia nombres alternativos a tablas o conceptos.

### Telemetría
- `obtener_estadisticas_uso`: Muestra métricas de uso de herramientas y consultas.

## Tests

```bash
go test ./...
```
