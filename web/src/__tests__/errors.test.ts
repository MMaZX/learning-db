import { describe, expect, it } from 'vitest';
import { describeHttpFailure, describeServerError } from '../api/errors';

describe('describeServerError', () => {
  it('reports OmniRoute as down when fetch fails', () => {
    const text = describeServerError({ code: 'OMNIROUTE_ERROR', message: 'fetch failed' });
    expect(text).toContain('No puedo conectar con OmniRoute');
    expect(text).toContain('scripts/jarvis.sh up omniroute');
  });

  it('reports an OmniRoute timeout', () => {
    const text = describeServerError({ code: 'OMNIROUTE_ERROR', message: 'This operation was aborted' });
    expect(text).toContain('tardó demasiado');
  });

  it('passes through other OmniRoute errors', () => {
    const text = describeServerError({ code: 'OMNIROUTE_ERROR', message: 'HTTP 401 invalid api key' });
    expect(text).toBe('OmniRoute rechazó la consulta: HTTP 401 invalid api key.');
  });

  it('names the tool when the MCP is unreachable', () => {
    const text = describeServerError({ code: 'MCP_TOOL_ERROR', toolName: 'consultar_base_datos', message: 'fetch failed' });
    expect(text).toContain('«consultar_base_datos»');
    expect(text).toContain('scripts/jarvis.sh up mcp');
  });

  it('explains denied tools', () => {
    expect(describeServerError({ code: 'MCP_TOOL_DENIED', toolName: 'borrar' })).toContain('bloqueada');
  });

  it('ignores non-string fields', () => {
    expect(describeServerError({ code: 42, message: {} })).toBe('El servidor devolvió un error sin detalle.');
  });
});

describe('describeHttpFailure', () => {
  it('reports the BFF as down when the request never completed', () => {
    expect(describeHttpFailure(null, '')).toContain('scripts/jarvis.sh up bff');
  });

  it('treats an empty 5xx from the Vite proxy as BFF down', () => {
    expect(describeHttpFailure(502, '')).toContain('no responde (HTTP 502)');
  });

  it('uses the JSON error field when present', () => {
    expect(describeHttpFailure(400, '{"error":"El texto del turno no puede estar vacío"}')).toBe(
      'El servidor respondió HTTP 400: El texto del turno no puede estar vacío.'
    );
  });
});
