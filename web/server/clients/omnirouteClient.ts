export interface ChatMessageParam {
  role: 'system' | 'user' | 'assistant' | 'tool';
  content?: string | null;
  tool_calls?: Array<{
    id: string;
    type: 'function';
    function: {
      name: string;
      arguments: string;
    };
  }>;
  tool_call_id?: string;
}

export interface StreamDelta {
  content?: string;
  tool_calls?: Array<{
    index: number;
    id?: string;
    type?: 'function';
    function?: {
      name?: string;
      arguments?: string;
    };
  }>;
}

export interface OmniRouteClientConfig {
  baseUrl: string;
  apiKey: string;
  timeoutMs?: number;
}

export class OmniRouteClient {
  private baseUrl: string;
  private apiKey: string;
  private timeoutMs: number;

  constructor(config: OmniRouteClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/+$/, '');
    this.apiKey = config.apiKey;
    this.timeoutMs = config.timeoutMs || 90000;
  }

  public async *streamChatCompletions(
    messages: ChatMessageParam[],
    tools?: Array<{ type: 'function'; function: { name: string; description: string; parameters: Record<string, unknown> } }>
  ): AsyncGenerator<StreamDelta, void, unknown> {
    const endpoint = `${this.baseUrl}/chat/completions`;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    const body: Record<string, unknown> = {
      model: 'auto', // Decision no negociable #3: siempre model "auto" para chat
      messages,
      stream: true,
    };

    if (tools && tools.length > 0) {
      body.tools = tools;
      body.tool_choice = 'auto';
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Accept': 'text/event-stream',
    };

    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }

    try {
      const response = await fetch(endpoint, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      if (!response.ok) {
        if (response.status === 401) {
          throw new Error('OMNIROUTE_AUTH: Clave de API de OmniRoute inválida o expirada');
        }
        if (response.status === 429) {
          throw new Error('OMNIROUTE_RATE_LIMITED: Cuota excedida o límite de velocidad alcanzado');
        }
        if (response.status >= 500) {
          throw new Error(`OMNIROUTE_EXHAUSTED: Proveedores upstream no disponibles (HTTP ${response.status})`);
        }
        throw new Error(`OMNIROUTE_ERROR: HTTP ${response.status} en ${endpoint}`);
      }

      if (!response.body) {
        throw new Error('OMNIROUTE_STREAM_BROKEN: Cuerpo de respuesta vacío en streaming');
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed || trimmed.startsWith(':')) continue; // comments/heartbeats

          if (trimmed.startsWith('data: ')) {
            const dataStr = trimmed.slice(6);
            if (dataStr === '[DONE]') {
              return;
            }

            try {
              const parsed = JSON.parse(dataStr);
              const choice = parsed.choices?.[0];
              if (choice?.delta) {
                yield choice.delta as StreamDelta;
              }
            } catch {
              // Ignore partial JSON parse errors until full chunk arrives
            }
          }
        }
      }
    } catch (err: unknown) {
      const error = err as Error;
      if (error.name === 'AbortError') {
        throw new Error(`OMNIROUTE_TIMEOUT: La llamada a OmniRoute excedió ${this.timeoutMs}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timer);
    }
  }
}
