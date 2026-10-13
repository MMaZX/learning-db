# Feature: Identidad por usuario, preferencias en servidor y entrega de la skill
Branch: feat/usuarios-y-preferencias (crear desde feat/ciclo-vida-y-memoria; la migración 0009 ya existe ahí)
Delivery Strategy: ask-on-risk
Test Runner: go test ./...
Implementación: delegada a otro agente, tarea por tarea. Leer este documento hasta `## Log` antes de editar.

## Specs

### S1
Identidad por usuario con un token personal por usuario (decisión del usuario: opción A). Tabla `users` con el token guardado como hash (nunca en claro). Cada persona configura su token en el conector; el servidor resuelve quién llama en cada request. Hoy hay un único token estático compartido (`cfg.MCPAuthToken`, `internal/mcpserver/auth.go`, `cmd/server/main.go:116`).

### S2
Atribución confiable: `taught_by`, `decided_by` y los demás autores salen del usuario autenticado en el servidor, no de texto que escribe el modelo. Hoy salen de los parámetros `ensenado_por`, `aprobado_por` y `revocado_por` (`internal/mcpserver/tools_learning.go`).

### S3
Bug de visibilidad, a reproducir primero: "cuando la persona 3 por ejemplo le enseña algo y la persona 2 lo quiere leer al parecer no trae el conocimiento de la persona 3 sino que solo le lista el conocimiento que le enseño la persona 2 unicamente". Hallazgo ya verificado en el código: `recall.go` filtra solo por `status = 'validated'`, no por autor. Hipótesis sin confirmar: lo enseñado queda como `propuesto` y espera `aprobar_conocimiento`, por eso otro usuario no lo ve.

### S4
Preferencias en el servidor: "el preferences.json, no persiste en memoria osea le mcp a cada usuario nuevo no persiste la memoria". Las preferencias pasan a una tabla nueva ligada al usuario registrado, válida en todas las sesiones.

### S5
Tool de arranque de una sola ejecución: "una tool que se llame recordando_configuracion o alguna herramienta que se ejecuta 1 sola vez". Lee las preferencias del usuario desde el servidor y las devuelve al modelo junto con las instrucciones de comportamiento.

### S6
Migración inicial: "ahora que ya las preferences.json ya no existiran sino que sera un dato guardado en el sistema, debe reconocer por cuenta o usuario, hay que revisar una manera de que pueda entender las preferencias correctamente y migrar la información completa". La tool debe ser idempotente y no perder información.

### S7
Tool de entrega de la skill: "una tool tambien de descargar_skill o algo por el estilo que le devuelva la skill actualizada, osea podemos agregar una tool update_skill que le diga al modelo esta es la skill que debes actualizar en el proyecto o en el chat de la conversacion". Debe funcionar desde un chat móvil sin terminal: "necesito que si o si pueda leer esa skill antes de continuar".

### S8
Documentación y ayuda al día: README, descripciones de las tools nuevas y el prompt `internal/mcpserver/prompts/jarvis.md`.

## Assumptions (marcadas como supuestos, validar con el usuario si chocan con algo)
- A1: el token estático actual sigue funcionando como credencial de administrador y de transición, hasta que todos tengan token propio.
- A2: la alta de usuarios y tokens se hace por línea de comandos del servidor (no por una tool MCP); el token se muestra una sola vez.
- A3: `migrar_preferencias` recibe el contenido crudo de `preferences.json` y lo guarda completo como JSON, sin interpretar su esquema.
- A4: el parámetro `ensenado_por` y similares siguen aceptándose por compatibilidad, pero el valor del servidor prevalece.
- A5: el token puede llegar por header (`Authorization: Bearer` o `X-API-Key`). Si el conector móvil no permite headers personalizados, hay que evaluar el token en la URL; el implementador debe reportarlo como hueco al padre, no decidirlo.

## Tasks
- [x] T1: (S1, S4) [delegated] Migración `0010` con tablas `users` (id, nombre, hash de token, activo, created_at) y `user_preferences` (user_id, JSON completo, migrated_at, updated_at). Riesgo: item 1. Commit: 0f12307.
- [x] T2: (S1) [delegated] Middleware que resuelve el usuario por token y lo pone en el contexto; conserva el token estático según A1; comando CLI para crear usuarios según A2. Riesgo: item 2. Commit: ba8292b.
- [x] T3: (S2) [delegated] Autor desde el usuario autenticado en `aprender_del_usuario`, `proponer_conocimiento`, `aprobar_conocimiento`, `rechazar_conocimiento`, `revocar_conocimiento` y alias. Riesgo: item 3. Commit: ebf71ac.
- [x] T4: (S3) [delegated] Reproducir el bug con un test que falle (usuario 3 enseña, usuario 2 recuerda). Confirmar o descartar la hipótesis. No decidir la política de visibilidad: devolver el hueco al padre. Commit: 660e983.
- [ ] T5: (S4, S5) [delegated] Tool `recordando_configuracion`: devuelve las preferencias del usuario autenticado y, si no tiene, indica que falta migrar.
- [ ] T6: (S6) [delegated] Tool `migrar_preferencias` según A3, idempotente.
- [ ] T7: (S7) [delegated] Tool `update_skill` que devuelve el contenido de `jarvis.md` (embebido) con un identificador de versión.
- [ ] T8: (S8) [delegated] Actualizar `jarvis.md`, README y descripciones; el prompt debe mandar llamar `recordando_configuracion` una vez y respetar `update_skill`.

## Log
- L1 (2026-10-13, usuario, pedido original): "ok un detalle que no estoy auditando en este proyecto es los usuarios que interactuan con el sistema y aqui hay un pequeño bug actualmente. y te expico cual es: por ejemplo 3 personas usan el mcp, y cada uno le brinda el conocimiento  para que aprenda, sin embargo cuando la persona 3 por ejemplo le enseña algo y la persona 2 lo quiere leer al parecer no trae el conocimiento de la persona 3 sino que solo le lista el conocimiento que le enseño la persona 2 unicamente. y no se como se está actualmente autorizando ello. otro detalle es que el preferences.json, no persiste en memoria osea le mcp a cada usuario nuevo no persiste la memoria, asi que necesito que el mcp tenga una tool que se llame recordando_configuracion o alguna herramienta que se ejecuta 1 sola vez, actualiza la skill de jarvis,  y lee e preferences.json que no vive en local sino que vive en el servidor osea en una tabla nueva vamos a tener a los usuarios registrados en todas las sesiones. adicional a ello hay que hacer un plan de migracion la primera vez. por ejemplo ahora que ya las preferences.json ya no existiran sino que sera un dato guardado en el sistema, debe reconocer por cuenta o usuario, hay que revisar una manera de que pueda entender las preferencias ocrrectamente y migrar la información completa y una tool tambien de descargar_skill o algo por el estilo que le devuelva la skill actualizada, osea podemos agregar una tool update_skill que le diga al modelo esta es la skill que debes actualizar en el proyecto o en el chat de la conversacion que estemos porque claro al ser un conector que se puede usar desde un mobile donde no tengo una terminal sino un chat de chatgpt o claude necesito que si o si pueda leer esa skill antes de continuar y darme mucha información y sepa como comportarse almenos en esta conversación."
- L2 (2026-10-13, usuario): eligió la opción A, un token personal por usuario.
- L3 (2026-10-13, padre): verificado en el código: un único token estático (`auth.go`), autores escritos por el modelo (`tools_learning.go:232,335`), recall filtra solo por `status = 'validated'` (`recall.go:226,261`).
- L4 (2026-10-13): T1 completada (0f12307): Migración 0010_users_and_preferences.sql con tablas users y user_preferences. TDD RED/GREEN. Revisión Codex detectó índices redundantes en columnas UNIQUE; Claude eliminó los índices duplicados. Verificado con go test ./...
- L5 (2026-10-13): T2 completada (ba8292b): WithUserAuth resuelve usuarios por token personal o token estático admin en contexto. CLI user create y user list implementados. Revisión Codex detectó necesidad de despachar CLI sin requerir MySQL, permitir arranque HTTP sin token estático si hay usuarios y distinguir Role; Claude corrigió estos puntos. Verificado con go test ./...
- L6 (2026-10-13): T3 completada (ebf71ac): Atribución confiable de autor desde usuario en contexto para aprender_del_usuario, aprobar_conocimiento, rechazar_conocimiento, revocar_conocimiento y alias. Revisión Codex observó que proponer_conocimiento no debía setear taught_by (es propuesta de evidencia); Claude ajustó proponer_conocimiento preservando la primacía del usuario autenticado en las herramientas de decisión y enseñanza. Verificado con go test ./...
- L7 (2026-10-13): T4 completada (660e983): Hipótesis CONFIRMADA. En visibility_bug_test.go se reprodujo que lo enseñado por persona 3 queda en status='proposed' y recordar_contexto filtra estrictamente por status='validated'; persona 2 sólo ve el conocimiento previamente validado. Una vez aprobado, es visible para todos. Revisión Codex verificó el contrato de conocimiento validado y Claude estructuró el test en dos sub-tests que prueban ambas facetas determinísticamente. Hueco devuelto al usuario para decidir política de visibilidad.
