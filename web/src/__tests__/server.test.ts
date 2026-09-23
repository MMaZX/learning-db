import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { loadConfig } from '../../server/config';
import { getToolPolicy } from '../../server/orchestration/toolPolicy';
import { adaptMcpToOpenAiTools } from '../../server/orchestration/toolSchemaAdapter';
import { buildJarvisSystemPrompt } from '../../server/prompts/jarvis-system';

describe('Server Config', () => {
  const originalEnv = { ...process.env };

  beforeEach(() => {
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('rejects "auto" for OMNIROUTE_STT_MODEL', () => {
    process.env.OMNIROUTE_STT_MODEL = 'auto';
    expect(() => loadConfig()).toThrow('CONFIG_ERROR');
  });

  it('rejects "auto" for OMNIROUTE_TTS_MODEL', () => {
    process.env.OMNIROUTE_TTS_MODEL = 'auto';
    expect(() => loadConfig()).toThrow('CONFIG_ERROR');
  });

  it('loads valid configuration with defaults', () => {
    delete process.env.OMNIROUTE_STT_MODEL;
    delete process.env.OMNIROUTE_TTS_MODEL;
    const config = loadConfig();
    expect(config.bffHost).toBe('127.0.0.1');
    expect(config.bffPort).toBe(4173);
    expect(config.chatMaxToolRounds).toBe(6);
  });
});

describe('Tool Policy Classification', () => {
  it('correctly classifies read-only business tools as AUTO', () => {
    expect(getToolPolicy('recordar_contexto')).toBe('AUTO');
    expect(getToolPolicy('consultar_base_datos')).toBe('AUTO');
    expect(getToolPolicy('obtener_esquema_bd')).toBe('AUTO');
    expect(getToolPolicy('obtener_estadisticas_uso')).toBe('AUTO');
  });

  it('correctly classifies mutating knowledge tools as CONFIRMATION_REQUIRED', () => {
    expect(getToolPolicy('aprender_del_usuario')).toBe('CONFIRMATION_REQUIRED');
    expect(getToolPolicy('refrescar_esquema')).toBe('CONFIRMATION_REQUIRED');
    expect(getToolPolicy('proponer_conocimiento')).toBe('CONFIRMATION_REQUIRED');
  });

  it('classifies administrative and unknown tools as DENIED', () => {
    expect(getToolPolicy('aprobar_conocimiento')).toBe('DENIED');
    expect(getToolPolicy('rechazar_conocimiento')).toBe('DENIED');
    expect(getToolPolicy('actualizar_alias')).toBe('DENIED');
    expect(getToolPolicy('unknown_tool_exploit')).toBe('DENIED');
  });
});

describe('Tool Schema Adapter', () => {
  it('adapts MCP tools into OpenAI function calling format', () => {
    const mcpTools = [
      {
        name: 'consultar_base_datos',
        description: 'Ejecuta una consulta SQL SELECT',
        inputSchema: {
          type: 'object' as const,
          properties: {
            query: { type: 'string', description: 'SQL query' },
          },
          required: ['query'],
        },
      },
    ];

    const openAiTools = adaptMcpToOpenAiTools(mcpTools);
    expect(openAiTools.length).toBe(1);
    expect(openAiTools[0].type).toBe('function');
    expect(openAiTools[0].function.name).toBe('consultar_base_datos');
    expect(openAiTools[0].function.parameters.required).toEqual(['query']);
  });
});

describe('Jarvis System Prompt', () => {
  it('injects protocol treatment and critical operational rules', () => {
    const prompt = buildJarvisSystemPrompt('Señor');
    expect(prompt).toContain('Tratas al usuario siempre como "Señor"');
    expect(prompt).toContain('recordar_contexto');
    expect(prompt).toContain('READ ONLY');
  });
});
