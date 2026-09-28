export interface KokoroRvcClientConfig {
  baseUrl: string;
  timeoutMs?: number;
}

export class KokoroRvcError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = 'KokoroRvcError';
  }
}

/**
 * Local voice service: Kokoro (lightweight Spanish TTS) chained with RVC
 * (timbre conversion to the trained Eugeo voice, model eugeo_v1). Both models
 * together use ~1.2 GB VRAM, so they stay resident alongside the local LLM
 * instead of taking turns for the GPU like Voicebox/Qwen3-TTS did.
 */
export class KokoroRvcClient {
  private config: Required<KokoroRvcClientConfig>;

  constructor(config: KokoroRvcClientConfig) {
    this.config = {
      ...config,
      baseUrl: config.baseUrl.replace(/\/+$/, ''),
      timeoutMs: config.timeoutMs || 60000,
    };
  }

  public get isConfigured(): boolean {
    return !!this.config.baseUrl;
  }

  public async synthesize(text: string): Promise<ArrayBuffer> {
    if (!this.isConfigured) {
      throw new KokoroRvcError('VOICE_NOT_CONFIGURED: falta JARVIS_VOICE_URL', 503);
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.config.timeoutMs);

    try {
      const response = await fetch(`${this.config.baseUrl}/generate/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
        signal: controller.signal,
      });

      if (!response.ok) {
        const detail = await response.text().catch(() => '');
        throw new KokoroRvcError(`VOICE_HTTP_${response.status}: ${detail.slice(0, 300)}`, 502);
      }

      return await response.arrayBuffer();
    } catch (err: unknown) {
      if (err instanceof KokoroRvcError) throw err;
      const error = err as Error;
      if (error.name === 'AbortError') {
        throw new KokoroRvcError('VOICE_TIMEOUT: la síntesis tardó demasiado', 504);
      }
      throw new KokoroRvcError(`VOICE_UNREACHABLE: ${error.message}`, 502);
    } finally {
      clearTimeout(timer);
    }
  }
}
