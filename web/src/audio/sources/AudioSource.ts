export interface AudioSource {
  readonly node: AudioNode | null;
  readonly isConnected: boolean;
  start(): Promise<void>;
  stop(): Promise<void>;
  destroy(): Promise<void>;
  onEnded?(callback: () => void): void;
}
