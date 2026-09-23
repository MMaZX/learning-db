import { AudioFeatures, VisualizerConfig } from '../types/audio';
import { AsymmetricSmoother } from './smoothing';

export class AudioFeatureExtractor {
  private binResolution: number;
  private previousMagnitudes: Float32Array;
  private volumeSmoother: AsymmetricSmoother;
  private lastOnsetTime = 0;
  private onsetRefractoryMs = 90;
  private fluxHistory: number[] = [];
  private maxFluxHistory = 30;

  // Band bin indices calculated dynamically
  private bassRange: [number, number];
  private lowMidRange: [number, number];
  private midRange: [number, number];
  private highMidRange: [number, number];
  private trebleRange: [number, number];

  constructor(sampleRate: number, fftSize: number, config: VisualizerConfig) {
    this.binResolution = sampleRate / fftSize;

    const binCount = fftSize / 2;
    this.previousMagnitudes = new Float32Array(binCount);
    this.volumeSmoother = new AsymmetricSmoother(config.attack, config.release);

    this.bassRange = this.getBinRange(80, 180, binCount);
    this.lowMidRange = this.getBinRange(180, 500, binCount);
    this.midRange = this.getBinRange(500, 2000, binCount);
    this.highMidRange = this.getBinRange(2000, 4000, binCount);
    this.trebleRange = this.getBinRange(4000, 8000, binCount);
  }

  private getBinRange(fMin: number, fMax: number, maxBins: number): [number, number] {
    const minBin = Math.max(0, Math.floor(fMin / this.binResolution));
    const maxBin = Math.min(maxBins - 1, Math.ceil(fMax / this.binResolution));
    return [minBin, Math.max(minBin + 1, maxBin)];
  }

  public extract(
    timeDomainData: Uint8Array,
    frequencyData: Uint8Array,
    noiseFloor: number,
    config: VisualizerConfig,
    targetFeatures: AudioFeatures
  ): void {
    // 1. Calculate Time-Domain RMS Volume
    let sumSquares = 0;
    const len = timeDomainData.length;
    for (let i = 0; i < len; i++) {
      const normalized = (timeDomainData[i] - 128) / 128;
      sumSquares += normalized * normalized;
    }
    const rawRms = Math.sqrt(sumSquares / len);

    // Apply Noise Gate & Sensitivity
    const effectiveRms = Math.max(0, rawRms - noiseFloor);
    const gatedVolume = effectiveRms > config.noiseGate ? effectiveRms * config.sensitivity : 0;
    const clampedVolume = Math.min(1.0, gatedVolume * 2.2);

    targetFeatures.volume = clampedVolume;
    targetFeatures.smoothedVolume = this.volumeSmoother.update(clampedVolume);

    // 2. Extract Frequency Bands & Calculate Spectral Flux
    const binCount = frequencyData.length;
    let flux = 0;

    for (let i = 0; i < binCount; i++) {
      const currentMag = frequencyData[i] / 255;
      const diff = currentMag - this.previousMagnitudes[i];
      if (diff > 0) {
        flux += diff;
      }
      this.previousMagnitudes[i] = currentMag;
    }

    const normalizedFlux = Math.min(1.0, (flux / binCount) * 8.0);
    targetFeatures.spectralFlux = normalizedFlux;

    // Track flux history for adaptive onset threshold
    this.fluxHistory.push(normalizedFlux);
    if (this.fluxHistory.length > this.maxFluxHistory) {
      this.fluxHistory.shift();
    }
    const avgFlux = this.fluxHistory.reduce((a, b) => a + b, 0) / this.fluxHistory.length;
    const onsetThreshold = Math.max(0.12, avgFlux * 1.6);

    // 3. Onset Detection with Refractory Period
    const now = performance.now();
    if (normalizedFlux > onsetThreshold && (now - this.lastOnsetTime) > this.onsetRefractoryMs && clampedVolume > 0.05) {
      targetFeatures.onset = Math.min(1.0, (normalizedFlux - onsetThreshold) * 3.5 + 0.5);
      this.lastOnsetTime = now;
    } else {
      // Exponential decay of onset pulse
      targetFeatures.onset = Math.max(0, targetFeatures.onset * 0.82);
    }

    // 4. Calculate Individual Bands
    targetFeatures.bass = this.calculateBandAverage(frequencyData, this.bassRange) * config.bassResponse;
    targetFeatures.lowMid = this.calculateBandAverage(frequencyData, this.lowMidRange);
    targetFeatures.mid = this.calculateBandAverage(frequencyData, this.midRange) * config.midResponse;
    targetFeatures.highMid = this.calculateBandAverage(frequencyData, this.highMidRange);
    targetFeatures.treble = this.calculateBandAverage(frequencyData, this.trebleRange) * config.highResponse;
  }

  private calculateBandAverage(freqData: Uint8Array, range: [number, number]): number {
    const [start, end] = range;
    let sum = 0;
    const count = end - start;
    if (count <= 0) return 0;

    for (let i = start; i < end; i++) {
      sum += freqData[i];
    }
    return Math.min(1.0, (sum / count) / 255.0);
  }

  public reset(): void {
    this.previousMagnitudes.fill(0);
    this.volumeSmoother.reset();
    this.fluxHistory = [];
    this.lastOnsetTime = 0;
  }
}
