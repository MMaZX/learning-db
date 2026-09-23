export type SseEventType =
  | 'turn.accepted'
  | 'routing.started'
  | 'assistant.delta'
  | 'tool.requested'
  | 'tool.confirmation.required'
  | 'tool.started'
  | 'tool.completed'
  | 'assistant.completed'
  | 'speech.available'
  | 'speech.unavailable'
  | 'turn.cancelled'
  | 'error'
  | 'done';

export interface SseEvent<T = unknown> {
  type: SseEventType;
  sequence: number;
  conversationId: string;
  turnId: string;
  data: T;
}

export interface TurnRequest {
  text: string;
  treatment?: 'Señor' | 'Señora' | 'Señorita';
}

export interface CapabilitiesResponse {
  chat: boolean;
  mcp: boolean;
  sttExperimental: boolean;
  tts: boolean;
  limits: {
    maxDurationMs: number;
    maxBytes: number;
    allowedMime: string[];
  };
}

export interface McpToolDefinition {
  name: string;
  description?: string;
  inputSchema: {
    type: 'object';
    properties?: Record<string, unknown>;
    required?: string[];
  };
}

export interface McpToolCallResult {
  content: Array<{
    type: 'text' | 'image' | 'resource';
    text?: string;
  }>;
  isError?: boolean;
}
