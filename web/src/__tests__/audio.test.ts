import { describe, it, expect, beforeEach } from 'vitest';
import { NoiseCalibrator } from '../audio/calibration';
import { AsymmetricSmoother } from '../audio/smoothing';
import { AudioFeatureExtractor } from '../audio/AudioFeatureExtractor';
import { AudioAnalyzer } from '../audio/AudioAnalyzer';
import { DEFAULT_VISUALIZER_CONFIG, AudioFeatures } from '../types/audio';

describe('NoiseCalibrator', () => {
  it('calculates noise floor after duration', () => {
    const calibrator = new NoiseCalibrator(50);
    calibrator.start();
    expect(calibrator.isCalibrating).toBe(true);
    expect(calibrator.isCalibrated).toBe(false);

    // Simulate samples
    const start = performance.now();
    while (performance.now() - start < 60) {
      calibrator.update(0.02);
    }
    const done = calibrator.update(0.02);

    expect(done).toBe(true);
    expect(calibrator.isCalibrated).toBe(true);
    expect(calibrator.noiseFloor).toBeGreaterThan(0);
  });
});

describe('AsymmetricSmoother', () => {
  it('applies fast attack and slower release', () => {
    const smoother = new AsymmetricSmoother(0.5, 0.1);
    smoother.reset(0);

    // Attack
    const afterAttack = smoother.update(1.0);
    expect(afterAttack).toBe(0.5); // 0 + (1 - 0) * 0.5

    // Release
    const afterRelease = smoother.update(0);
    expect(afterRelease).toBe(0.45); // 0.5 + (0 - 0.5) * 0.1 = 0.45
  });
});

describe('AudioFeatureExtractor', () => {
  let extractor: AudioFeatureExtractor;
  let features: AudioFeatures;

  beforeEach(() => {
    extractor = new AudioFeatureExtractor(44100, 2048, DEFAULT_VISUALIZER_CONFIG);
    features = {
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
  });

  it('mutates the target features object in-place', () => {
    const timeData = new Uint8Array(2048).fill(128); // Pure silence
    const freqData = new Uint8Array(1024).fill(0);

    extractor.extract(timeData, freqData, 0.01, DEFAULT_VISUALIZER_CONFIG, features);

    expect(features.volume).toBe(0);
    expect(features.bass).toBe(0);
    expect(features.mid).toBe(0);
    expect(features.treble).toBe(0);
  });

  it('detects bass energy and spectral flux', () => {
    const timeData = new Uint8Array(2048).fill(200); // Loud signal
    const freqData = new Uint8Array(1024).fill(0);

    // Fill bass bins (bins corresponding to 80-180 Hz with 44100 / 2048 ~ 21.5 Hz per bin)
    // bins 4 to 9
    for (let i = 4; i <= 9; i++) {
      freqData[i] = 220;
    }

    extractor.extract(timeData, freqData, 0.01, DEFAULT_VISUALIZER_CONFIG, features);

    expect(features.volume).toBeGreaterThan(0);
    expect(features.bass).toBeGreaterThan(0);
    expect(features.spectralFlux).toBeGreaterThan(0);
  });
});

describe('AudioAnalyzer', () => {
  it('instantiates with persistent mutable features identity', () => {
    const analyzer = new AudioAnalyzer();
    const initialFeaturesRef = analyzer.features;

    expect(analyzer.state).toBe('MIC_OFF');
    expect(initialFeaturesRef.volume).toBe(0);

    analyzer.update();
    expect(analyzer.features).toBe(initialFeaturesRef); // Same reference in memory!
  });
});
