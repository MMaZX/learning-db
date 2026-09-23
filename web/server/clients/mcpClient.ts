import { McpToolCallResult, McpToolDefinition } from '../types/protocol.js';

export interface McpClientConfig {
  url: string;
  authToken: string;
  timeoutMs?: number;
}

export class McpClient {
  private url: string;
  private authToken: string;
  private timeoutMs: number;
  private sessionId: string | null = null;
  private cachedTools: McpToolDefinition[] | null = null;
  private cacheExpiresAt = 0;
  private requestId = 0;

  constructor(config: McpClientConfig) {
    this.url = config.url;
    this.authToken = config.authToken;
    this.timeoutMs = config.timeoutMs || 15000;
  }

  private nextId(): number {
    this.requestId++;
    return this.requestId;
  }

  public async initialize(): Promise<void> {
    const payload = {
      jsonrpc: '2.0',
      id: this.nextId(),
      method: 'initialize',
      params: {
        protocolVersion: '2024-11-05',
        capabilities: {},
        clientInfo: {
          name: 'jarvis-bff-client',
          version: '1.0.0',
        },
      },
    };

    const res = await this.postJsonRpc(payload);
    if (res.error) {
      throw new Error(`MCP_INITIALIZE_FAILED: ${res.error.message || 'Unknown error'}`);
    }
  }

  public async listTools(forceRefresh = false): Promise<McpToolDefinition[]> {
    const now = Date.now();
    if (!forceRefresh && this.cachedTools && now < this.cacheExpiresAt) {
      return this.cachedTools;
    }

    const payload = {
      jsonrpc: '2.0',
      id: this.nextId(),
      method: 'tools/list',
      params: {},
    };

    const res = await this.postJsonRpc(payload);
    if (res.error) {
      throw new Error(`MCP_TOOLS_LIST_FAILED: ${res.error.message}`);
    }

    const tools: McpToolDefinition[] = (res.result?.tools || []).map((t: any) => ({
      name: t.name,
      description: t.description,
      inputSchema: t.inputSchema || { type: 'object', properties: {} },
    }));

    this.cachedTools = tools;
    this.cacheExpiresAt = now + (5 * 60 * 1000); // 5 minutes cache
    return tools;
  }

  public async callTool(name: string, args: Record<string, unknown>): Promise<McpToolCallResult> {
    const payload = {
      jsonrpc: '2.0',
      id: this.nextId(),
      method: 'tools/call',
      params: {
        name,
        arguments: args,
      },
    };

    const res = await this.postJsonRpc(payload);
    if (res.error) {
      return {
        content: [{ type: 'text', text: `Error de ejecución MCP: ${res.error.message}` }],
        isError: true,
      };
    }

    return res.result as McpToolCallResult;
  }

  private async postJsonRpc(body: unknown): Promise<any> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Accept': 'application/json, text/event-stream',
    };

    if (this.authToken) {
      headers['Authorization'] = `Bearer ${this.authToken}`;
    }

    if (this.sessionId) {
      headers['Mcp-Session-Id'] = this.sessionId;
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      const response = await fetch(this.url, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: controller.signal,
      });

      const sessionHeader = response.headers.get('mcp-session-id');
      if (sessionHeader) {
        this.sessionId = sessionHeader;
      }

      if (!response.ok) {
        if (response.status === 401) {
          throw new Error('MCP_AUTH: Token de autenticación de MCP inválido o no autorizado');
        }
        throw new Error(`MCP_HTTP_ERROR: HTTP ${response.status} en ${this.url}`);
      }

      return await response.json();
    } catch (err: unknown) {
      const error = err as Error;
      if (error.name === 'AbortError') {
        throw new Error(`MCP_TIMEOUT: La llamada a ${this.url} excedió ${this.timeoutMs}ms`);
      }
      throw error;
    } finally {
      clearTimeout(timer);
    }
  }
}
