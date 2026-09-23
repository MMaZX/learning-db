export interface VoiceboxClientConfig {
  baseUrl: string;
  profileId: string;
  language: string;
  engine: string;
  modelSize: string;
  timeoutMs?: number;
}

export class VoiceboxError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = 'VoiceboxError';
  }
}

/**
 * Local Voicebox TTS server (https://github.com/jamiepine/voicebox, tag v0.5.0).
 * Uses POST /generate/stream, which returns the WAV directly without storing
 * it in Voicebox's history.
 */
export class VoiceboxClient {
  private config: Required<VoiceboxClientConfig>;

  constructor(config: VoiceboxClientConfig) {
    this.config = {
      ...config,
      baseUrl: config.baseUrl.replace(/\/+$/, ''),
      timeoutMs: config.timeoutMs || 120000,
    };
  }

  public get isConfigured(): boolean {
    return !!this.config.baseUrl && !!this.config.profileId;
  }

  public async synthesize(text: string): Promise<ArrayBuffer> {
    if (!this.isConfigured) {
      throw new VoiceboxError('VOICEBOX_NOT_CONFIGURED: falta VOICEBOX_PROFILE_ID', 503);
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.config.timeoutMs);

    try {
      const response = await fetch(`${this.config.baseUrl}/generate/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          profile_id: this.config.profileId,
          text,
          language: this.config.language,
          engine: this.config.engine,
          model_size: this.config.modelSize,
        }),
        signal: controller.signal,
      });

      if (!response.ok) {
        const detail = await response.text().catch(() => '');
        throw new VoiceboxError(`VOICEBOX_HTTP_${response.status}: ${detail.slice(0, 300)}`, 502);
      }

      return await response.arrayBuffer();
    } catch (err: unknown) {
      if (err instanceof VoiceboxError) throw err;
      const error = err as Error;
      if (error.name === 'AbortError') {
        throw new VoiceboxError('VOICEBOX_TIMEOUT: la síntesis tardó demasiado', 504);
      }
      throw new VoiceboxError(`VOICEBOX_UNREACHABLE: ${error.message}`, 502);
    } finally {
      clearTimeout(timer);
    }
  }
}
