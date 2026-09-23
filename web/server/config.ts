export interface ServerConfig {
  bffHost: string;
  bffPort: number;
  omnirouteBaseUrl: string;
  omnirouteApiKey: string;
  voiceExperimentalEnabled: boolean;
  omnirouteSttModel: string;
  omnirouteSttMaxDurationMs: number;
  omnirouteSttMaxBytes: number;
  omnirouteSttAllowedMime: string[];
  omnirouteTtsModel: string;
  omnirouteTtsVoice: string;
  jarvisMcpUrl: string;
  jarvisMcpAuthToken: string;
  chatMaxToolRounds: number;
  chatMaxToolCalls: number;
  chatTurnTimeoutMs: number;
  chatMaxToolResultBytes: number;
  conversationTtlMinutes: number;
  logLevel: string;
}

export function loadConfig(): ServerConfig {
  const env = process.env;

  const omnirouteSttModel = env.OMNIROUTE_STT_MODEL || '';
  const omnirouteTtsModel = env.OMNIROUTE_TTS_MODEL || '';

  if (omnirouteSttModel.toLowerCase() === 'auto') {
    throw new Error('CONFIG_ERROR: "auto" is invalid for OMNIROUTE_STT_MODEL. Speech models must be explicit.');
  }

  if (omnirouteTtsModel.toLowerCase() === 'auto') {
    throw new Error('CONFIG_ERROR: "auto" is invalid for OMNIROUTE_TTS_MODEL. Speech models must be explicit.');
  }

  const allowedMime = (env.OMNIROUTE_STT_ALLOWED_MIME || 'audio/webm;codecs=opus,audio/ogg;codecs=opus,audio/wav')
    .split(',')
    .map((m) => m.trim());

  return {
    bffHost: env.BFF_HOST || '127.0.0.1',
    bffPort: parseInt(env.BFF_PORT || '4173', 10),
    omnirouteBaseUrl: env.OMNIROUTE_OPENAI_BASE_URL || 'http://127.0.0.1:20128/v1',
    omnirouteApiKey: env.OMNIROUTE_API_KEY || '',
    voiceExperimentalEnabled: env.VOICE_EXPERIMENTAL_ENABLED === 'true',
    omnirouteSttModel,
    omnirouteSttMaxDurationMs: parseInt(env.OMNIROUTE_STT_MAX_DURATION_MS || '15000', 10),
    omnirouteSttMaxBytes: parseInt(env.OMNIROUTE_STT_MAX_BYTES || '2097152', 10),
    omnirouteSttAllowedMime: allowedMime,
    omnirouteTtsModel,
    omnirouteTtsVoice: env.OMNIROUTE_TTS_VOICE || 'alloy',
    jarvisMcpUrl: env.JARVIS_MCP_URL || 'http://127.0.0.1:8080/mcp',
    jarvisMcpAuthToken: env.JARVIS_MCP_AUTH_TOKEN || '',
    chatMaxToolRounds: parseInt(env.CHAT_MAX_TOOL_ROUNDS || '6', 10),
    chatMaxToolCalls: parseInt(env.CHAT_MAX_TOOL_CALLS || '12', 10),
    chatTurnTimeoutMs: parseInt(env.CHAT_TURN_TIMEOUT_MS || '90000', 10),
    chatMaxToolResultBytes: parseInt(env.CHAT_MAX_TOOL_RESULT_BYTES || '262144', 10),
    conversationTtlMinutes: parseInt(env.CONVERSATION_TTL_MINUTES || '30', 10),
    logLevel: env.LOG_LEVEL || 'info',
  };
}
