/**
 * Turn-taking for a small GPU (RTX 3050, 4 GB): the local LLM (~2.9 GB) and the
 * Voicebox TTS model (~3.1 GB peak) do not fit together, so each one takes the GPU
 * in turn. Chat and speech already run one after the other (the frontend speaks
 * only after the answer is complete), which is what makes this workable.
 *
 * Measured cost per turn: ~3 s to reload the LLM and ~5 s to reload the TTS model.
 */

export type GpuOwner = 'chat' | 'speech';

export interface GpuReleasers {
  /** Frees the TTS model so the LLM can load. */
  releaseSpeech: () => Promise<void>;
  /** Frees the LLM so the TTS model can load. */
  releaseChat: () => Promise<void>;
}

export class GpuTurn {
  private owner: GpuOwner | null = null;
  private queue: Promise<void> = Promise.resolve();

  constructor(
    private releasers: GpuReleasers,
    private log: (message: string) => void = (m) => console.warn(m)
  ) {}

  public forChat(): Promise<void> {
    return this.acquire('chat');
  }

  public forSpeech(): Promise<void> {
    return this.acquire('speech');
  }

  // Serialized so two overlapping requests never release each other's model mid-load.
  private acquire(next: GpuOwner): Promise<void> {
    const run = this.queue.then(async () => {
      if (this.owner === next) return;
      try {
        await (next === 'chat' ? this.releasers.releaseSpeech() : this.releasers.releaseChat());
      } catch (err: unknown) {
        // Not fatal: the request still runs, only slower if both models end up sharing the GPU.
        this.log(`[Jarvis BFF] Turno de GPU (${next}): ${(err as Error).message}`);
      }
      this.owner = next;
    });
    this.queue = run.catch(() => undefined);
    return run;
  }
}
