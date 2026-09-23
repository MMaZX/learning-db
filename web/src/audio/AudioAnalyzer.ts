import { AudioFeatures, DEFAULT_VISUALIZER_CONFIG, MicState, VisualizerConfig } from '../types/audio';
import { AudioSource } from './sources/AudioSource';
import { MicrophoneAudioSource } from './sources/MicrophoneAudioSource';
import { NoiseCalibrator } from './calibration';
import { AudioFeatureExtractor } from './AudioFeatureExtractor';

export class AudioAnalyzer {
  public readonly features: AudioFeatures;
  private audioContext: AudioContext | null = null;
  private analyserNode: AnalyserNode | null = null;
  private activeSource: AudioSource | null = null;
  private calibrator: NoiseCalibrator;
  private extractor: AudioFeatureExtractor | null = null;
  private config: VisualizerConfig;

  // Pre-allocated typed arrays for zero-allocation updates
  private timeDomainBuffer: Uint8Array<ArrayBuffer>;
  private frequencyBuffer: Uint8Array<ArrayBuffer>;

  private _micState: MicState = 'MIC_OFF';
  private onStateChangeCallback: ((state: MicState) => void) | null = null;

  constructor(config: VisualizerConfig = DEFAULT_VISUALIZER_CONFIG) {
    this.config = { ...config };
    this.features = {
      volume: 0,
      smoothedVolume: 0,
      bass: 0,
      lowMid: 0,
      mid: 0,
      highMid: 0,
      treble: 0,
      spectralFlux: 0,
      onset: 0,
    };

    // Pre-allocate buffers for fftSize 2048
    this.timeDomainBuffer = new Uint8Array(new ArrayBuffer(2048));
    this.frequencyBuffer = new Uint8Array(new ArrayBuffer(1024));
    this.calibrator = new NoiseCalibrator(750);
  }

  public get state(): MicState {
    return this._micState;
  }

  public get noiseFloor(): number {
    return this.calibrator.noiseFloor;
  }

  public get isCalibrated(): boolean {
    return this.calibrator.isCalibrated;
  }

  public onStateChange(callback: (state: MicState) => void): void {
    this.onStateChangeCallback = callback;
  }

  private setState(newState: MicState): void {
    if (this._micState !== newState) {
      this._micState = newState;
      if (this.onStateChangeCallback) {
        this.onStateChangeCallback(newState);
      }
    }
  }

  public async startMicrophone(): Promise<void> {
    this.setState('REQUESTING_PERMISSION');

    if (!this.audioContext) {
      const AudioContextClass = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
      this.audioContext = new AudioContextClass();
    }

    if (this.audioContext.state === 'suspended') {
      await this.audioContext.resume();
    }

    if (!this.analyserNode) {
      this.analyserNode = this.audioContext.createAnalyser();
      this.analyserNode.fftSize = 2048;
      this.analyserNode.smoothingTimeConstant = this.config.smoothing;
      this.timeDomainBuffer = new Uint8Array(new ArrayBuffer(this.analyserNode.fftSize));
      this.frequencyBuffer = new Uint8Array(new ArrayBuffer(this.analyserNode.frequencyBinCount));
    }

    this.extractor = new AudioFeatureExtractor(
      this.audioContext.sampleRate,
      this.analyserNode.fftSize,
      this.config
    );

    const micSource = new MicrophoneAudioSource(this.audioContext);
    try {
      await micSource.start();
    } catch (err) {
      this.setState('MIC_ERROR');
      throw err;
    }

    micSource.onEnded(() => {
      this.setState('MIC_ENDED');
    });

    await this.attach(micSource);
    this.calibrator.start();
    this.setState('CALIBRATING');
  }

  public async attach(source: AudioSource): Promise<void> {
    if (this.activeSource && this.activeSource !== source) {
      await this.activeSource.stop();
    }

    this.activeSource = source;
    if (this.activeSource.node && this.analyserNode) {
      this.activeSource.node.connect(this.analyserNode);
    }
  }

  public update(): void {
    if (!this.analyserNode || !this.extractor) return;

    this.analyserNode.getByteTimeDomainData(this.timeDomainBuffer);
    this.analyserNode.getByteFrequencyData(this.frequencyBuffer);

    if (this.calibrator.isCalibrating) {
      // Calculate raw RMS during calibration
      let sumSq = 0;
      for (let i = 0; i < this.timeDomainBuffer.length; i++) {
        const norm = (this.timeDomainBuffer[i] - 128) / 128;
        sumSq += norm * norm;
      }
      const rawRms = Math.sqrt(sumSq / this.timeDomainBuffer.length);
      const isDone = this.calibrator.update(rawRms);
      if (isDone) {
        this.setState('WAITING_SILENCE');
      }
    }

    this.extractor.extract(
      this.timeDomainBuffer,
      this.frequencyBuffer,
      this.calibrator.noiseFloor,
      this.config,
      this.features
    );

    // Update state based on effective audio energy with hysteresis
    if (this.calibrator.isCalibrated && this._micState !== 'MIC_ERROR' && this._micState !== 'MIC_ENDED') {
      if (this.features.volume > (this.config.noiseGate * 2.5)) {
        this.setState('USER_SPEAKING');
      } else if (this.features.volume < this.config.noiseGate) {
        this.setState('WAITING_SILENCE');
      }
    }
  }

  public updateConfig(newConfig: Partial<VisualizerConfig>): void {
    this.config = { ...this.config, ...newConfig };
    if (this.analyserNode && newConfig.smoothing !== undefined) {
      this.analyserNode.smoothingTimeConstant = newConfig.smoothing;
    }
  }

  public async destroy(): Promise<void> {
    if (this.activeSource) {
      await this.activeSource.destroy();
      this.activeSource = null;
    }

    if (this.analyserNode) {
      this.analyserNode.disconnect();
      this.analyserNode = null;
    }

    if (this.audioContext && this.audioContext.state !== 'closed') {
      await this.audioContext.close();
      this.audioContext = null;
    }

    this.extractor = null;
    this.calibrator.reset();
    this.setState('MIC_OFF');
  }
}
