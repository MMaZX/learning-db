export interface OllamaClientConfig {
  baseUrl: string;
  /** How long to wait for Ollama to actually release VRAM after an unload request. */
  unloadWaitMs?: number;
}

/**
 * Minimal client for the local Ollama server. Chat goes through OmniRoute; this
 * client only manages which models Ollama keeps resident in GPU memory.
 */
export class OllamaClient {
  private baseUrl: string;
  private unloadWaitMs: number;

  constructor(config: OllamaClientConfig) {
    this.baseUrl = config.baseUrl.replace(/\/+$/, '');
    this.unloadWaitMs = config.unloadWaitMs ?? 5000;
  }

  public async loadedModels(): Promise<string[]> {
    const response = await fetch(`${this.baseUrl}/api/ps`, { signal: AbortSignal.timeout(5000) });
    if (!response.ok) throw new Error(`OLLAMA_PS_HTTP_${response.status}`);
    const data = (await response.json()) as { models?: Array<{ name: string }> };
    return (data.models || []).map((m) => m.name);
  }

  /** Unloads every resident model and waits until Ollama reports none loaded. */
  public async unloadAll(): Promise<void> {
    const models = await this.loadedModels();
    if (models.length === 0) return;

    await Promise.all(
      models.map((model) =>
        fetch(`${this.baseUrl}/api/generate`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ model, keep_alive: 0 }),
          signal: AbortSignal.timeout(10000),
        })
      )
    );

    // keep_alive: 0 schedules the unload; VRAM is only free once /api/ps is empty.
    const deadline = Date.now() + this.unloadWaitMs;
    while (Date.now() < deadline) {
      if ((await this.loadedModels()).length === 0) return;
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    throw new Error(`OLLAMA_UNLOAD_TIMEOUT: los modelos siguen cargados tras ${this.unloadWaitMs}ms`);
  }
}
