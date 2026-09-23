import { describe, it, expect, beforeEach } from 'vitest';
import { AudioSphere } from '../three/AudioSphere';
import { ParticleField } from '../three/ParticleField';
import { HoloShell } from '../three/HoloShell';
import { QualityController, QUALITY_PROFILES } from '../three/QualityController';
import { AudioFeatures } from '../types/audio';

describe('AudioSphere', () => {
  let sphere: AudioSphere;
  let features: AudioFeatures;

  beforeEach(() => {
    sphere = new AudioSphere(1.2, 16);
    features = {
      volume: 0.5,
      smoothedVolume: 0.45,
      bass: 0.8,
      lowMid: 0.3,
      mid: 0.6,
      highMid: 0.2,
      treble: 0.4,
      spectralFlux: 0.25,
      onset: 0.9,
    };
  });

  it('updates uniforms in-place from mutable features', () => {
    sphere.update(1.5, features);
    const uniforms = (sphere.mesh.material as any).uniforms;

    expect(uniforms.uTime.value).toBe(1.5);
    expect(uniforms.uBass.value).toBe(0.8);
    expect(uniforms.uOnset.value).toBe(0.9);
  });

  it('disposes geometry and material cleanly', () => {
    expect(() => sphere.dispose()).not.toThrow();
  });
});

describe('ParticleField', () => {
  let particles: ParticleField;
  let features: AudioFeatures;

  beforeEach(() => {
    particles = new ParticleField(4000, 2000);
    features = {
      volume: 0.3,
      smoothedVolume: 0.25,
      bass: 0.5,
      lowMid: 0.2,
      mid: 0.4,
      highMid: 0.1,
      treble: 0.3,
      spectralFlux: 0.1,
      onset: 0.0,
    };
  });

  it('initializes with correct attributes and draw range', () => {
    const geo = particles.points.geometry;
    expect(geo.getAttribute('position')).toBeDefined();
    expect(geo.getAttribute('aLayer')).toBeDefined();
    expect(geo.getAttribute('aVelocity')).toBeDefined();
    expect(geo.drawRange.count).toBe(2000);
  });

  it('scales LOD dynamically using setDrawRange without reallocating buffers', () => {
    particles.setCount(3500);
    expect(particles.points.geometry.drawRange.count).toBe(3500);
    expect(particles.count).toBe(3500);

    particles.update(0.016, features);
    const uniforms = (particles.points.material as any).uniforms;
    expect(uniforms.uBass.value).toBe(0.5);
  });

  it('disposes without errors', () => {
    expect(() => particles.dispose()).not.toThrow();
  });
});

describe('HoloShell', () => {
  it('builds dot lattice, spikes and decals, and updates cleanly', () => {
    const shell = new HoloShell();
    expect(shell.group.children.length).toBe(3);

    const features: AudioFeatures = {
      volume: 0.4,
      smoothedVolume: 0.3,
      bass: 0.5,
      lowMid: 0,
      mid: 0,
      highMid: 0,
      treble: 0.2,
      spectralFlux: 0,
      onset: 0.6,
    };

    expect(() => shell.update(0.016, 1.0, features)).not.toThrow();
    expect(() => shell.dispose()).not.toThrow();
  });
});

describe('QualityController', () => {
  it('initializes with default tier and profiles', () => {
    const controller = new QualityController('HIGH');
    expect(controller.profile.tier).toBe('HIGH');
    expect(controller.profile.particleCount).toBe(QUALITY_PROFILES.HIGH.particleCount);
  });

  it('degrades to MEDIUM and LOW when sustained FPS is low', () => {
    const controller = new QualityController('HIGH');
    let profileChanged = false;
    controller.onProfileChange((_p) => {
      profileChanged = true;
    });

    // Simulate 35 FPS for 1.8 seconds (frame interval ~28ms)
    let time = 1000;
    for (let i = 0; i < 70; i++) {
      time += 28.5; // ~35 FPS
      controller.update(time);
    }

    expect(controller.profile.tier).toBe('MEDIUM');
    expect(profileChanged).toBe(true);
  });
});
