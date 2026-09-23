import { AudioSource } from './AudioSource';

export class MicrophoneAudioSource implements AudioSource {
  private audioContext: AudioContext | null = null;
  private mediaStream: MediaStream | null = null;
  private sourceNode: MediaStreamAudioSourceNode | null = null;
  private endedCallback: (() => void) | null = null;
  private _isConnected = false;

  constructor(audioContext: AudioContext) {
    this.audioContext = audioContext;
  }

  public get node(): AudioNode | null {
    return this.sourceNode;
  }

  public get isConnected(): boolean {
    return this._isConnected;
  }

  public async start(): Promise<void> {
    if (this._isConnected) return;

    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
      throw new Error('MIC_UNAVAILABLE: MediaDevices API is not supported in this environment');
    }

    if (this.audioContext && this.audioContext.state === 'suspended') {
      await this.audioContext.resume();
    }

    try {
      this.mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      });
    } catch (err: unknown) {
      const error = err as Error;
      if (error.name === 'NotAllowedError' || error.name === 'PermissionDeniedError') {
        throw new Error('MIC_PERMISSION_DENIED: Permission was denied by user');
      }
      throw new Error(`MIC_ERROR: ${error.message || 'Could not acquire microphone'}`);
    }

    if (!this.audioContext) {
      throw new Error('AUDIO_CONTEXT_SUSPENDED: AudioContext is not initialized');
    }

    this.sourceNode = this.audioContext.createMediaStreamSource(this.mediaStream);

    // Track disconnection
    const audioTrack = this.mediaStream.getAudioTracks()[0];
    if (audioTrack) {
      audioTrack.onended = () => {
        this.handleTrackEnded();
      };
    }

    this._isConnected = true;
  }

  public onEnded(callback: () => void): void {
    this.endedCallback = callback;
  }

  private handleTrackEnded(): void {
    this._isConnected = false;
    if (this.endedCallback) {
      this.endedCallback();
    }
  }

  public async stop(): Promise<void> {
    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach((track) => track.stop());
      this.mediaStream = null;
    }
    if (this.sourceNode) {
      this.sourceNode.disconnect();
      this.sourceNode = null;
    }
    this._isConnected = false;
  }

  public async destroy(): Promise<void> {
    await this.stop();
    this.audioContext = null;
    this.endedCallback = null;
  }
}
