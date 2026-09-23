# Proceso: plan Jarvis + OmniRoute + UI de voz

## Objetivo

Generar, revisar y validar en una sola ejecución un plan de implementación brownfield para integrar:

- la especificación visual y de audio de `PROMPT-UI.md`;
- el MCP HTTP existente `ia-jarvis-gr`;
- OmniRoute mediante su API compatible con OpenAI y `model: auto`;
- conversación por texto/voz y TTS real analizable por Web Audio;
- una aplicación React + TypeScript + Vite + Three.js segura.

## Fases

1. Leer `PROMPT-UI.md` literalmente en tiempo de ejecución.
2. Auditar infraestructura reutilizable y trazar rutas reales de ejecución.
3. Escribir `PLAN-OMNIROUTE-JARVIS-UI.md` sin implementar código de producto.
4. Releer el plan y compararlo de forma adversarial con la especificación.
5. Corregir el plan y ejecutar verificaciones deterministas de sus secciones obligatorias.

## Entregable

`PLAN-OMNIROUTE-JARVIS-UI.md`

No se modifica el servidor MCP ni se crea la UI durante esta ejecución.
