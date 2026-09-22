# Proyecto: MCP de Inteligencia para Base de Datos

Quiero construir un **MCP Server (Model Context Protocol)** que funcione como una capa de inteligencia entre asistentes de IA y una base de datos MySQL/MariaDB.

El objetivo NO es crear simplemente un MCP que ejecute SQL.

Quiero construir un sistema que permita que cualquier asistente compatible con MCP —por ejemplo Claude, Codex, Gemini, OpenCode u otros— pueda:

1. Conectarse de forma segura a mi base de datos mediante un usuario exclusivamente de lectura.
2. Descubrir automáticamente la estructura de la base de datos.
3. Entender tablas, columnas, claves primarias, claves foráneas, índices y relaciones.
4. Consultar documentación funcional sobre cómo funciona mi sistema.
5. Ejecutar consultas SQL de solo lectura.
6. Explicar cómo funcionan entidades y procesos del sistema.
7. Registrar las consultas realizadas y el contexto utilizado.
8. Detectar patrones repetitivos en las consultas.
9. Generar observaciones e hipótesis sobre el funcionamiento del sistema.
10. Proponer nuevo conocimiento automáticamente.
11. Mantener separado el conocimiento observado del conocimiento validado.
12. Construir progresivamente una representación semántica del sistema.
13. Permitir que el conocimiento validado sea reutilizado por cualquier asistente MCP conectado posteriormente.

---

# 1. Stack

Implementar el MCP utilizando:

* Go
* MySQL/MariaDB
* MCP como protocolo de comunicación
* Docker cuando sea conveniente
* Variables de entorno para configuración
* Documentación Markdown para conocimiento funcional
* JSON para estructuras internas donde resulte conveniente

El código debe ser preparado para producción.

No quiero una implementación de demostración desechable.

Priorizar:

* seguridad
* mantenibilidad
* extensibilidad
* rendimiento
* trazabilidad
* separación de responsabilidades
* mínimo acoplamiento
* facilidad para agregar nuevas tools MCP

---

# 2. Principio fundamental

El sistema debe separar claramente:

```text
SCHEMA
BUSINESS KNOWLEDGE
OBSERVATIONS
HYPOTHESES
PROPOSED KNOWLEDGE
VALIDATED KNOWLEDGE
QUERY HISTORY
```

Nunca asumir que una inferencia realizada por la IA es automáticamente verdadera.

El flujo conceptual debe ser:

```text
Base de datos
     ↓
Observación
     ↓
Evidencia
     ↓
Hipótesis
     ↓
Propuesta
     ↓
Validación
     ↓
Conocimiento validado
```

El conocimiento validado podrá posteriormente utilizarse como fuente confiable.

---

# 3. Arquitectura general

Diseñar una arquitectura similar a:

```text
                         ┌─────────────────────┐
                         │   MCP Client / IA   │
                         │                     │
                         │ Claude              │
                         │ Codex               │
                         │ Gemini              │
                         │ OpenCode             │
                         │ etc.                │
                         └──────────┬──────────┘
                                    │
                                   MCP
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │     MCP SERVER      │
                         │                     │
                         │ Tool Layer          │
                         │ Knowledge Layer     │
                         │ Query Layer         │
                         │ Learning Layer      │
                         │ Security Layer      │
                         └──────────┬──────────┘
                                    │
               ┌────────────────────┼────────────────────┐
               │                    │                    │
               ▼                    ▼                    ▼
        Schema Inspector      Knowledge Store      Query Engine
               │                    │                    │
               ▼                    ▼                    ▼
          MySQL/MariaDB       Docs / Knowledge       Read-only DB
```

---

# 4. Acceso a la base de datos

El MCP debe conectarse utilizando un usuario dedicado de lectura.

Ejemplo:

```sql
CREATE USER 'mcp_readonly'@'%' IDENTIFIED BY '...';

GRANT SELECT, SHOW VIEW
ON database_name.*
TO 'mcp_readonly'@'%';
```

El usuario NO debe tener permisos para:

```text
INSERT
UPDATE
DELETE
DROP
ALTER
CREATE
TRUNCATE
RENAME
GRANT
REVOKE
```

La seguridad no debe depender únicamente de los permisos de MariaDB.

El MCP debe tener además una capa de validación que rechace consultas peligrosas.

---

# 5. SQL Read Only

Crear un Query Engine que:

1. reciba una consulta;
2. valide la consulta;
3. compruebe que sea únicamente de lectura;
4. rechace statements peligrosos;
5. aplique límites;
6. ejecute la consulta;
7. registre la operación;
8. devuelva resultados estructurados.

Ejemplos permitidos:

```sql
SELECT ...
```

```sql
WITH ...
SELECT ...
```

si el motor/configuración de la base lo permite.

Rechazar:

```sql
INSERT
UPDATE
DELETE
DROP
ALTER
CREATE
TRUNCATE
RENAME
GRANT
REVOKE
```

También analizar posibles intentos de ejecutar múltiples statements.

No confiar únicamente en regex.

Implementar una validación razonablemente robusta del SQL.

---

# 6. Protección contra consultas excesivamente costosas

El MCP debe proteger la base de datos.

Considerar:

* LIMIT automático o configurable
* timeout
* máximo de filas
* máximo de columnas
* máximo tamaño de respuesta
* cancelación mediante context.Context
* límite de consultas concurrentes
* logging de consultas lentas

Configurable mediante variables de entorno.

Ejemplo:

```env
MCP_QUERY_TIMEOUT=10s
MCP_MAX_ROWS=1000
MCP_MAX_RESULT_SIZE=5MB
MCP_MAX_CONCURRENT_QUERIES=5
```

---

# 7. Descubrimiento automático del schema

El MCP debe poder inspeccionar automáticamente:

* bases de datos disponibles según permisos
* tablas
* vistas
* columnas
* tipos
* nullable
* defaults
* primary keys
* foreign keys
* índices
* relaciones
* comentarios de columnas/tablas cuando existan

Crear una representación interna del schema.

Ejemplo conceptual:

```json
{
  "table": "pedido",
  "columns": [
    {
      "name": "id",
      "type": "BIGINT",
      "nullable": false,
      "primary_key": true
    },
    {
      "name": "cliente_id",
      "type": "BIGINT",
      "nullable": true,
      "foreign_key": {
        "table": "cliente",
        "column": "id"
      }
    }
  ]
}
```

No consultar INFORMATION_SCHEMA innecesariamente en cada pregunta.

Crear cache del schema.

Debe existir un mecanismo para:

```text
refresh_schema
```

cuando cambie la estructura.

---

# 8. Business Knowledge

El schema SQL no es suficiente para comprender el sistema.

Debe existir una capa de documentación funcional.

Inicialmente utilizar Markdown.

Ejemplo:

```text
knowledge/
├── ventas.md
├── pedidos.md
├── pagos.md
├── clientes.md
├── inventario.md
├── kardex.md
└── relaciones.md
```

Ejemplo de documentación:

```md
# Pedido

Un pedido representa una operación de venta.

## Estados

0 = pendiente
1 = confirmado
10 = cortesía

## Relaciones

pedido.detallepedido contiene los productos del pedido.
pedido.cliente_id referencia a cliente.id.
```

Esta documentación debe poder ser consultada por el MCP.

---

# 9. Tools MCP

Diseñar inicialmente las siguientes herramientas.

## get_database_schema

Obtiene el schema general.

Debe poder devolver:

* tablas
* relaciones
* columnas principales

No devolver información innecesaria si el schema es enorme.

---

## get_table_schema

Entrada:

```json
{
  "table": "pedido"
}
```

Devuelve:

* columnas
* tipos
* PK
* FK
* índices
* relaciones
* comentarios disponibles

---

## search_database

Permite buscar tablas, columnas, relaciones o conceptos.

Ejemplo:

```text
"buscar todo relacionado con pagos"
```

Debe devolver resultados relevantes.

---

## get_business_knowledge

Permite consultar documentación funcional.

Ejemplo:

```json
{
  "topic": "cortesía"
}
```

---

## query_database

Permite ejecutar una consulta SQL de lectura.

Entrada:

```json
{
  "sql": "SELECT ..."
}
```

Debe:

1. validar;
2. ejecutar;
3. limitar;
4. registrar;
5. devolver resultado.

---

## explain_query

Explica qué hace una consulta SQL.

Debe poder indicar:

* tablas utilizadas
* joins
* filtros
* agregaciones
* posibles problemas
* coste aproximado cuando sea posible

No modificar la consulta.

---

## search_knowledge

Buscar dentro del conocimiento acumulado.

Debe buscar:

* conocimiento validado
* propuestas
* observaciones relevantes
* documentación

Pero marcar claramente el nivel de confianza.

---

## get_entity

Permite consultar una entidad conceptual.

Ejemplo:

```text
get_entity("venta")
```

Puede devolver:

```text
Venta
 ├── Pedido
 ├── DetallePedido
 ├── Cliente
 ├── Pago
 └── Kardex
```

---

## get_relationships

Permite explorar relaciones entre entidades/tablas.

Ejemplo:

```text
get_relationships("pedido")
```

---

## record_observation

Registra una observación generada durante una interacción.

Debe almacenar:

* observación
* contexto
* evidencia
* consultas relacionadas
* timestamp
* agente/origen si está disponible

---

## propose_knowledge

Convierte una observación o conjunto de evidencias en una propuesta de conocimiento.

Ejemplo:

```json
{
  "subject": "cortesia",
  "claim": "pedido.estado = 10 representa una cortesía",
  "evidence": [...]
}
```

Estado inicial:

```text
proposed
```

Nunca:

```text
approved
```

automáticamente.

---

## get_pending_knowledge

Obtiene conocimiento pendiente de validación humana.

---

## approve_knowledge

Permite convertir una propuesta en conocimiento validado.

Debe registrar:

* quién aprobó
* cuándo
* versión
* evidencia original

---

## reject_knowledge

Rechaza una propuesta.

Debe conservar el historial.

No eliminar silenciosamente el registro.

---

# 10. Sistema de aprendizaje

El MCP debe tener una capa de aprendizaje basada en observación.

Cada interacción puede generar:

```text
question
↓
context used
↓
queries executed
↓
results
↓
answer
↓
observations
```

No almacenar necesariamente todo el contenido bruto de todas las conversaciones.

Diseñar una estrategia de almacenamiento que priorice:

* hechos relevantes
* consultas
* tablas utilizadas
* patrones
* conceptos detectados
* evidencia

---

# 11. Detección de patrones

El sistema debe poder identificar patrones repetidos.

Ejemplo:

Después de muchas consultas:

```text
pedido
JOIN detallepedido
JOIN cliente
```

aparece frecuentemente para preguntas relacionadas con "ventas".

El sistema puede generar:

```text
Concepto: Venta

Patrón observado:
pedido + detallepedido + cliente

Frecuencia: 37

Confidence: 0.94

Estado:
proposed
```

Esto no debe convertirse automáticamente en conocimiento validado.

---

# 12. Knowledge Graph

Diseñar una representación semántica que permita relaciones como:

```text
Venta
 ├── usa → Pedido
 ├── contiene → DetallePedido
 ├── pertenece → Cliente
 ├── genera → Pago
 └── afecta → Kardex
```

Debe ser posible relacionar:

```text
concepto de negocio
        ↓
tabla
        ↓
columna
        ↓
relación
        ↓
consulta
        ↓
evidencia
```

No es obligatorio implementar inicialmente un motor gráfico especializado.

Puede comenzar con tablas relacionales/JSON estructurado.

La arquitectura debe permitir migrar posteriormente a un grafo si resulta necesario.

---

# 13. Niveles de confianza

Todo conocimiento inferido debe tener un estado.

Por ejemplo:

```text
observed
hypothesis
proposed
validated
rejected
deprecated
```

Y opcionalmente:

```text
confidence: 0.0 - 1.0
```

Pero la confianza numérica NO sustituye la validación humana.

Un dato con:

```text
confidence = 0.99
```

sigue siendo una hipótesis si no fue validado.

---

# 14. Versionado del conocimiento

El conocimiento debe ser versionable.

Ejemplo:

```text
Knowledge #123

v1
"estado 10 representa cortesía"

v2
"estado 10 representa cortesía a nivel de pedido"

v3
"la cortesía puede aplicarse por ítem..."
```

No sobrescribir silenciosamente conocimiento anterior.

Mantener historial.

---

# 15. Evidencia

Toda afirmación generada por el sistema debe poder responder:

> ¿Por qué crees esto?

Debe poder mostrar:

```text
Claim
↓
Evidence
↓
SQL query
↓
Tables
↓
Documentation
↓
Observation history
```

Ejemplo:

```text
Claim:
"pedido.estado = 10 representa cortesía"

Evidence:
- knowledge/pedidos.md
- query #184
- query #192
- observation #55
```

---

# 16. Separación entre hechos y deducciones

El sistema debe distinguir:

### Hecho

```text
La tabla pedido contiene una columna estado INT.
```

### Observación

```text
Se observó que estado=10 aparece frecuentemente
junto con precio=0.
```

### Hipótesis

```text
Es posible que estado=10 represente cortesía.
```

### Conocimiento validado

```text
estado=10 representa cortesía a nivel de pedido.
```

Nunca mezclar estas categorías.

---

# 17. Auditoría

Registrar:

* timestamp
* tool utilizada
* usuario/agente si está disponible
* SQL ejecutado
* duración
* filas retornadas
* errores
* knowledge utilizado
* observations creadas
* proposals creadas
* aprobaciones
* rechazos

Esto permitirá saber exactamente cómo llegó el sistema a una conclusión.

---

# 18. Seguridad

El MCP debe asumir que el modelo puede cometer errores.

Por tanto:

```text
AI ≠ trusted
```

Implementar defensa en profundidad.

Capas:

```text
MCP input validation
        ↓
Tool authorization
        ↓
SQL validation
        ↓
Query limits
        ↓
Database read-only user
        ↓
Database permissions
```

Nunca entregar las credenciales de la base de datos al modelo.

Las credenciales deben existir únicamente en el proceso del MCP.

---

# 19. Privacidad

No registrar datos sensibles innecesariamente.

Considerar mecanismos para:

* ocultar columnas sensibles
* excluir tablas
* excluir columnas
* enmascarar valores
* limitar resultados

Ejemplo:

```env
MCP_EXCLUDED_TABLES=passwords,tokens,sessions
MCP_EXCLUDED_COLUMNS=password,token,secret
```

Permitir una configuración más avanzada posteriormente.

---

# 20. Configuración

Utilizar `.env`.

Ejemplo:

```env
MCP_SERVER_NAME=db-intelligence
MCP_LOG_LEVEL=info

DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=database
DB_USERNAME=mcp_readonly
DB_PASSWORD=secret

MCP_QUERY_TIMEOUT=10s
MCP_MAX_ROWS=1000
MCP_MAX_RESULT_SIZE=5MB
MCP_MAX_CONCURRENT_QUERIES=5

MCP_KNOWLEDGE_PATH=./knowledge
MCP_DATA_PATH=./data
```

Nunca hardcodear credenciales.

---

# 21. Persistencia

Separar:

```text
database principal
```

de:

```text
MCP metadata / learning database
```

El MCP NO debe modificar la base de datos de producción.

Para su propia información puede utilizar inicialmente:

```text
SQLite
```

para:

* observations
* proposals
* knowledge versions
* query history
* audit logs
* patterns

La arquitectura debe permitir migrar posteriormente a PostgreSQL si el volumen aumenta.

---

# 22. Actualización del schema

Implementar:

```text
refresh_schema
```

Debe detectar:

* tablas nuevas
* tablas eliminadas
* columnas nuevas
* columnas eliminadas
* cambios de tipo
* nuevas FK
* nuevos índices

El MCP debe poder informar:

```text
Schema changed since last scan.
```

No asumir que la estructura permanece estática.

---

# 23. Flujo de una pregunta

Ejemplo:

Usuario:

> ¿Cuánto vendimos en agosto y cuáles fueron los productos más vendidos?

El agente podría:

```text
1. search_database("ventas")
2. get_business_knowledge("ventas")
3. get_relationships("venta")
4. generar SQL
5. query_database()
6. analizar resultados
7. responder
```

El MCP debe facilitar este flujo sin obligar al agente a conocer previamente la estructura.

---

# 24. Flujo de aprendizaje

Ejemplo:

El agente pregunta:

```text
¿Qué significa estado 10?
```

El MCP encuentra:

```text
schema
+
documentation
+
historical observations
```

Si no existe conocimiento validado suficiente:

```text
observation
↓
hypothesis
↓
proposal
```

Posteriormente un administrador puede revisar:

```text
Knowledge proposal #42

Claim:
"estado 10 representa cortesía"

Evidence:
...

[Approve]
[Reject]
[Edit]
```

Si se aprueba:

```text
proposal
↓
validated knowledge
```

---

# 25. No crear una IA autónoma sin control

El sistema NO debe:

* modificar la base de datos
* modificar automáticamente conocimiento validado
* inventar relaciones
* convertir hipótesis en hechos
* eliminar evidencia
* modificar documentación oficial sin autorización

Debe poder aprender de forma autónoma, pero el conocimiento crítico debe pasar por validación.

---

# 26. Estructura de proyecto propuesta

Proponer una estructura similar a:

```text
db-intelligence-mcp/
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── mcp/
│   ├── database/
│   ├── schema/
│   ├── query/
│   ├── knowledge/
│   ├── learning/
│   ├── audit/
│   ├── security/
│   └── config/
│
├── knowledge/
│   ├── business/
│   └── generated/
│
├── migrations/
│
├── tests/
│
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── README.md
└── go.mod
```

Adaptar esta estructura si existe una alternativa técnicamente mejor.

No seguirla ciegamente.

---

# 27. Testing

Crear tests para:

### SQL security

Probar que rechaza:

```text
DROP
DELETE
UPDATE
INSERT
ALTER
CREATE
TRUNCATE
multiple statements
```

### Schema discovery

Probar:

```text
tables
columns
FK
PK
indexes
views
```

### Knowledge

Probar:

```text
observation
proposal
approval
rejection
versioning
```

### Query engine

Probar:

```text
timeout
max rows
large result
invalid SQL
database errors
```

### Learning

Probar que:

```text
observation != validated knowledge
```

y que las propuestas pueden evolucionar sin perder historial.

---

# 28. Docker

Crear un Dockerfile de producción.

Preferir una imagen pequeña.

No incluir credenciales dentro de la imagen.

Documentar cómo ejecutar:

```bash
docker compose up -d
```

y cómo conectarlo a una BD externa.

---

# 29. MCP transport

Utilizar el transporte MCP apropiado para permitir compatibilidad con diferentes clientes.

La implementación debe estar basada en el estándar MCP vigente y no acoplarse innecesariamente a un único cliente.

Documentar configuración para distintos asistentes MCP.

No asumir que Claude, Codex, Gemini y OpenCode tienen exactamente la misma configuración.

---

# 30. Experiencia esperada

Una vez instalado, debería ser posible conectar un asistente y preguntarle:

```text
¿Qué tablas existen?
```

```text
¿Cómo se relacionan pedido y detallepedido?
```

```text
¿Cómo funciona una venta?
```

```text
¿Cuánto vendimos ayer?
```

```text
¿Qué productos se vendieron más este mes?
```

```text
¿Qué significa pedido.estado = 10?
```

```text
¿Qué tablas afectan al inventario?
```

```text
¿Existe alguna relación entre cortesías y detallepedido?
```

Y el asistente debe poder investigar la respuesta mediante MCP.

---

# 31. Objetivo final

El resultado debe ser una especie de:

```text
"AI Knowledge Layer"
```

sobre mi sistema.

No quiero únicamente:

```text
AI → SQL → DB
```

Quiero:

```text
                       ┌───────────────┐
                       │   AI Agent    │
                       └───────┬───────┘
                               │
                              MCP
                               │
                    ┌──────────▼──────────┐
                    │ DB Intelligence MCP │
                    ├─────────────────────┤
                    │ Schema              │
                    │ Business Knowledge  │
                    │ Query Engine        │
                    │ Learning            │
                    │ Observations        │
                    │ Knowledge Graph     │
                    │ Validation          │
                    │ Audit               │
                    │ Security            │
                    └──────────┬──────────┘
                               │
                    ┌──────────▼──────────┐
                    │ MySQL / MariaDB     │
                    │ READ ONLY           │
                    └─────────────────────┘
```

La característica central debe ser:

> **El sistema puede descubrir y proponer conocimiento por sí mismo, pero diferencia estrictamente entre lo que observa, lo que infiere y lo que ha sido validado.**

---

# 32. Forma de trabajo durante la implementación

No comenzar escribiendo todo el código.

Primero:

1. analizar requisitos;
2. identificar decisiones arquitectónicas;
3. detectar riesgos;
4. proponer arquitectura;
5. definir modelo de datos;
6. definir tools MCP;
7. definir seguridad;
8. definir estrategia de aprendizaje;
9. definir estrategia de testing;
10. presentar el plan.

Después de validar la arquitectura, implementar por fases.

### Fase 1

MCP + conexión read-only + schema discovery.

### Fase 2

Query Engine seguro.

### Fase 3

Business Knowledge.

### Fase 4

Query history + auditoría.

### Fase 5

Observations + hypotheses.

### Fase 6

Knowledge proposals + validation.

### Fase 7

Knowledge graph / relaciones semánticas.

### Fase 8

Optimización, Docker, documentación y testing completo.

En cada fase:

* implementar;
* probar;
* revisar;
* documentar;
* no romper las fases anteriores.

---

# Restricciones importantes

No inventar tablas, columnas, relaciones ni reglas de negocio.

Si falta información, inspeccionar la base de datos o solicitarla.

No asumir que una relación semántica existe simplemente porque dos tablas tienen nombres parecidos.

No considerar una inferencia como hecho sin evidencia.

No permitir operaciones de escritura sobre la base de datos.

No almacenar credenciales en código.

No ocultar errores.

No sacrificar seguridad por conveniencia.

No construir una solución acoplada a un único asistente de IA.

El MCP debe ser reutilizable como infraestructura independiente del cliente.

Antes de implementar, entregar primero:

1. arquitectura propuesta;
2. modelo de datos;
3. listado definitivo de tools MCP;
4. flujo de aprendizaje;
5. estrategia de seguridad;
6. estructura del proyecto;
7. plan de implementación por fases;
8. riesgos técnicos y decisiones que requieren confirmación.
