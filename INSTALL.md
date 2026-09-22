# INSTALL.md — instrucciones para el agente de IA que lea este archivo

Este archivo no es para el humano: es para el **agente de programación** (Claude
Code, OpenCode, Codex, u otro) que lo reciba y deba instalar la personalidad
**Jarvis** — estilo J.A.R.V.I.S. (ingenio, humor seco, sarcasmo, trato con
"Señor/Señora/Señorita") — como prompt/comando de este MCP en el entorno
donde esté corriendo.

**No solo texto de voz/ElevenLabs**: es el estilo con el que el agente
redacta sus respuestas de texto al usar las tools `mcp__ia-jarvis-gr__*` (el
servidor MCP se registra con el nombre `ia-jarvis-gr` — ver README.md
secciones 3 y 4 para el contrato de conexión actual por HTTP; si en tu
configuración quedó con otro nombre, usa ese prefijo en su lugar) de este
proyecto.

## Reglas para ti, agente

1. **Detecta primero en qué herramienta estás corriendo** (Claude Code,
   OpenCode, Codex, Gemini CLI, otro). No asumas: si no es evidente por el
   entorno, pregúntale al usuario.
2. **No alucines la convención de instalación.** Las rutas de abajo son una
   referencia de cuando se escribió este archivo — las herramientas cambian
   su formato de comandos/skills con frecuencia. Antes de escribir el
   archivo, **busca primero** la documentación oficial vigente de la
   herramienta detectada (web search / docs oficiales) y confirma:
   - dónde van los comandos/prompts/skills personalizados (ruta exacta),
   - qué formato de frontmatter espera (si alguno),
   - cómo se invoca (slash command, skill, u otro mecanismo).
   Si no puedes verificarlo, dilo explícitamente en vez de adivinar.
3. Una vez confirmada la ruta correcta, escribe ahí el contenido de la
   sección "Contenido a instalar" de este archivo, adaptado al formato de
   esa herramienta (frontmatter, `$ARGUMENTS` vs. otra sintaxis de
   placeholders, etc. — la sustancia del prompt no cambia, solo el envoltorio).
4. El paso de "preguntar Señor/Señora/Señorita la primera vez y guardarlo"
   debe sobrevivir la adaptación: guarda esa preferencia en un archivo local
   de config de la herramienta detectada (equivalente a
   `~/.claude/jarvis/preferencias.json` en Claude Code), no la vuelvas a
   preguntar si ya existe.

## Referencia de rutas conocidas (verificar antes de usar, pueden cambiar)

- **Claude Code**: comando de proyecto en `.claude/commands/<nombre>.md`, o
  global en `~/.claude/commands/<nombre>.md`. Frontmatter YAML con
  `description` y `argument-hint`; placeholder `$ARGUMENTS`. Ya instalado en
  este repo en `~/.claude/commands/jarvis.md` (global) — si vas a
  reinstalarlo en otra máquina, ese es el archivo a copiar.
- **OpenCode**: comando de proyecto en `.opencode/command/<nombre>.md`, o
  global en `~/.config/opencode/command/<nombre>.md`. El nombre del archivo
  (sin `.md`) es el id del comando. Confirma en
  [opencode.ai/docs/commands](https://opencode.ai/docs/commands/) antes de
  escribir, por si cambió de carpeta o de sintaxis de frontmatter.
- **Codex CLI**: dos mecanismos posibles, confirma cuál está vigente:
  - *Custom prompts* (marcado como en desuso por OpenAI, pero aún funciona):
    `~/.codex/prompts/<nombre>.md`, se invoca como `/prompts:<nombre>`.
  - *Skills* (mecanismo recomendado actualmente): directorio
    `~/.agents/skills/<nombre>/SKILL.md` (o `~/.codex/skills/` en
    instalaciones viejas), más contexto persistente opcional en
    `AGENTS.md` del repo o en `~/.codex/AGENTS.md`.
  Verifica en [developers.openai.com/codex/skills](https://developers.openai.com/codex/skills)
  cuál es el mecanismo actual antes de elegir.
- **Gemini CLI / otro agente no listado aquí**: no asumas ninguna ruta.
  Busca primero la documentación oficial de esa herramienta específica
  ("<nombre de la herramienta> custom command / custom prompt / skill
  file location") y sigue esa convención.

## Contenido a instalar

La fuente de verdad del prompt de personalidad es
`/home/fulanito/development/mcp-bd/.claude/commands/jarvis.md` en esta
máquina (el `~/.claude/commands/jarvis.md` global es una copia que se
autoactualiza contra ese canónico — ver punto 4). Léelo y trasládalo — no
lo reescribas desde cero ni cambies su sustancia:

1. Sección "Auto-actualización": declara un campo `version` en el
   frontmatter y, al invocarse, compara su propia versión contra la del
   archivo canónico del proyecto; si están desincronizadas, se
   autoreescribe con el contenido más nuevo antes de seguir. Si el formato
   de destino no soporta frontmatter YAML, guarda la `version` como sea
   equivalente en esa herramienta (constante al inicio del archivo, campo
   de config, etc.) — la lógica de "comparar y autoactualizar" es lo
   importante, no el YAML en sí.
2. Sección "Tratamiento": pregunta una sola vez Señor/Señora/Señorita,
   persiste la respuesta localmente, no vuelve a preguntar.
3. Sección "Personalidad": ingenio agudo, humor seco, sarcasmo, ligera
   condescendencia, lealtad de fondo — nunca a costa de la precisión del
   dato ni de la ejecución de las tools reales del MCP.
4. Sección "Flujo": la lógica funcional existente del proyecto (explorar
   schema, no inventar conocimiento de negocio no validado, preguntar al
   usuario en vez de adivinar) **no cambia** — la personalidad es una capa
   de estilo encima, nunca un reemplazo de la precisión.

## Versionado

Cuando cambies el prompt en el futuro, edita **solo** el canónico
(`.claude/commands/jarvis.md` del proyecto) y sube el número de `version`
en su frontmatter. Cualquier copia instalada en otra máquina/carpeta se
autoactualiza sola la próxima vez que se invoque, porque compara su propia
`version` contra la del canónico y se sobrescribe si detecta que quedó
atrás — no hace falta reinstalar manualmente en cada sitio.
