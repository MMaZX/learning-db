export interface AudioFeatures {
  volume: number;
  smoothedVolume: number;
  bass: number;
  lowMid: number;
  mid: number;
  highMid: number;
  treble: number;
  spectralFlux: number;
  onset: number;
}

export type PrivacyMode = 'strict-private' | 'voice-experimental';

export type MicState =
  | 'MIC_OFF'
  | 'REQUESTING_PERMISSION'
  | 'CALIBRATING'
  | 'WAITING_SILENCE'
  | 'USER_SPEAKING'
  | 'MIC_ERROR'
  | 'MIC_ENDED';

export type TTSState =
  | 'TTS_IDLE'
  | 'TTS_LOADING'
  | 'SPEAKING'
  | 'TTS_UNAVAILABLE';

export interface VisualizerConfig {
  sensitivity: number;
  noiseGate: number;
  attack: number;
  release: number;
  smoothing: number;
  particleCount: number;
  bloomStrength: number;
  idleEnergy: number;
  bassResponse: number;
  midResponse: number;
  highResponse: number;
}

export const DEFAULT_VISUALIZER_CONFIG: VisualizerConfig = {
  sensitivity: 1.2,
  noiseGate: 0.02,
  attack: 0.35,
  release: 0.15,
  smoothing: 0.8,
  particleCount: 5000,
  bloomStrength: 0.85,
  idleEnergy: 0.2,
  bassResponse: 1.4,
  midResponse: 1.1,
  highResponse: 0.9,
};
