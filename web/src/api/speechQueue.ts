export interface SpeechQueueOptions {
  synthesize: (text: string) => Promise<ArrayBuffer>;
  play: (audio: ArrayBuffer) => Promise<void>;
  onError: (err: unknown, text: string) => void;
}

/**
 * Speaks chunks in order while more are still arriving. Synthesis runs one chunk
 * at a time (the voice server shares a tight GPU with the LLM) and stays ahead of
 * playback; a chunk that fails is skipped instead of silencing the rest.
 */
export class SpeechQueue {
  private synthChain: Promise<unknown> = Promise.resolve();
  private playChain: Promise<void> = Promise.resolve();
  private cancelled = false;
  private pendingCount = 0;

  constructor(private options: SpeechQueueOptions) {}

  public get busy(): boolean {
    return this.pendingCount > 0;
  }

  public enqueue(texts: string[]): void {
    for (const text of texts) {
      if (this.cancelled) return;
      this.pendingCount++;
      const audio = this.synthChain.then(async () => {
        if (this.cancelled) return null;
        try {
          return await this.options.synthesize(text);
        } catch (err) {
          this.options.onError(err, text);
          return null;
        }
      });
      this.synthChain = audio;
      this.playChain = this.playChain.then(async () => {
        try {
          const buffer = await audio;
          if (buffer && !this.cancelled) await this.options.play(buffer);
        } catch (err) {
          this.options.onError(err, text);
        } finally {
          this.pendingCount--;
        }
      });
    }
  }

  /** Resolves once every queued chunk has been played (or skipped). */
  public drain(): Promise<void> {
    return this.playChain;
  }

  public cancel(): void {
    this.cancelled = true;
  }
}
