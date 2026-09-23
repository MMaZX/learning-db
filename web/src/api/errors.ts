/**
 * Turns BFF failures into a line Jarvis can say in the chat, so a turn that
 * fails upstream no longer ends in silence.
 */

export interface ServerErrorEvent {
  code?: unknown;
  message?: unknown;
  toolName?: unknown;
}

// Node's fetch reports an unreachable host as "fetch failed"; its cause is not
// forwarded by the BFF, so this is the only signal that the service is down.
const UNREACHABLE = /fetch failed|ECONNREFUSED|ENOTFOUND|EHOSTUNREACH|socket hang up/i;
const TIMEOUT = /abort|timed? ?out/i;

export function describeServerError(event: ServerErrorEvent): string {
  const code = typeof event.code === 'string' ? event.code : '';
  const message = typeof event.message === 'string' ? event.message : '';
  const tool = typeof event.toolName === 'string' ? event.toolName : 'desconocida';

  switch (code) {
    case 'OMNIROUTE_ERROR':
      if (UNREACHABLE.test(message)) {
        return 'No puedo conectar con OmniRoute (puerto 20128). Arránquelo con scripts/jarvis.sh up omniroute.';
      }
      if (TIMEOUT.test(message)) {
        return 'OmniRoute tardó demasiado en responder y cancelé la consulta.';
      }
      if (message.startsWith('OMNIROUTE_UPSTREAM')) {
        return `Ningún proveedor de IA en OmniRoute pudo responder. Conecte uno en http://localhost:20128. Detalle: ${message.slice(0, 300)}`;
      }
      return `OmniRoute rechazó la consulta: ${message || 'sin detalle'}.`;
    case 'MCP_TOOL_ERROR':
      if (UNREACHABLE.test(message)) {
        return `No puedo conectar con el MCP de base de datos (puerto 8080) para usar «${tool}». Arránquelo con scripts/jarvis.sh up mcp.`;
      }
      return `La herramienta «${tool}» falló: ${message || 'sin detalle'}.`;
    case 'MCP_TOOL_DENIED':
      return `La herramienta «${tool}» está bloqueada por seguridad.`;
    default:
      return message ? `Error del servidor: ${message}.` : 'El servidor devolvió un error sin detalle.';
  }
}

export function describeHttpFailure(status: number | null, body: string): string {
  if (status === null) {
    return 'No puedo conectar con mi servidor (BFF, puerto 4173). Arránquelo con scripts/jarvis.sh up bff.';
  }
  // Vite answers 500/502/504 with an empty body when its /api proxy cannot reach the BFF.
  if (status >= 500 && !body.trim()) {
    return `Mi servidor (BFF, puerto 4173) no responde (HTTP ${status}). Arránquelo con scripts/jarvis.sh up bff.`;
  }
  let detail = body.trim();
  try {
    const parsed = JSON.parse(body) as { error?: unknown };
    if (typeof parsed.error === 'string') detail = parsed.error;
  } catch {
    // Not JSON: keep the raw body.
  }
  return `El servidor respondió HTTP ${status}${detail ? `: ${detail.slice(0, 200)}` : ''}.`;
}
