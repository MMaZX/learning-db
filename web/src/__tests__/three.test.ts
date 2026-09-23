import { describe, it, expect, beforeEach } from 'vitest';
import { AudioSphere } from '../three/AudioSphere';
import { ParticleField } from '../three/ParticleField';
import { HoloShell } from '../three/HoloShell';
import { QualityController, QUALITY_PROFILES } from '../three/QualityController';
import { AmbientSparks, OrbitalRing } from '../three/Ambience';
import { SpectrumDriver, SpectrumDrive, compress } from '../three/SpectrumDriver';
import { AudioFeatures } from '../types/audio';

function makeFeatures(overrides: Partial<AudioFeatures> = {}): AudioFeatures {
  return {
    volume: 0,
    smoothedVolume: 0,
    bass: 0,
    lowMid: 0,
    mid: 0,
    highMid: 0,
    treble: 0,
    spectralFlux: 0,
    onset: 0,
    ...overrides,
  };
}

function makeDrive(overrides: Partial<SpectrumDrive> = {}): SpectrumDrive {
  return {
    energy: 0.4,
    bass: 0.8,
    mid: 0.6,
    treble: 0.4,
    pulse: 0.9,
    flow: 1.5,
    spin: 0.2,
    bootDots: 1,
    bootFull: 1,
    alert: 0,
    ...overrides,
  };
}

/** Runs the driver for `seconds` at a fixed frame rate with constant features. */
function run(driver: SpectrumDriver, features: AudioFeatures, seconds: number, fps: number): SpectrumDrive {
  const frames = Math.round(seconds * fps);
  for (let i = 0; i < frames; i++) driver.update(1 / fps, features);
  return driver.drive;
}

describe('SpectrumDriver', () => {
  it('keeps every audio-driven value within [0, 1] even with extreme input', () => {
    const driver = new SpectrumDriver();
    driver.setMicActive(true);
    const extreme = makeFeatures({ volume: 50, bass: 50, mid: 50, treble: 50, onset: 50 });
    const drive = run(driver, extreme, 6, 60);

    for (const key of ['energy', 'bass', 'mid', 'treble', 'pulse'] as const) {
      expect(drive[key]).toBeGreaterThan(0.9);
      expect(drive[key]).toBeLessThanOrEqual(1);
    }
  });

  it('ignores audio until the microphone boot sequence completes', () => {
    const driver = new SpectrumDriver();
    const loud = makeFeatures({ volume: 1, bass: 1, onset: 1 });
    const drive = run(driver, loud, 2, 60);

    expect(drive.bootDots).toBe(1);
    expect(drive.bootFull).toBe(0);
    expect(drive.energy).toBe(0);
  });

  it('completes the microphone boot in about 3.2 seconds', () => {
    const driver = new SpectrumDriver();
    driver.setMicActive(true);
    expect(run(driver, makeFeatures(), 2, 60).bootFull).toBeLessThan(1);
    expect(run(driver, makeFeatures(), 1.3, 60).bootFull).toBe(1);
  });

  it('produces the same result at 30, 60 and 144 fps', () => {
    const loud = makeFeatures({ volume: 0.7, bass: 0.5 });
    const results = [30, 60, 144].map((fps) => {
      const driver = new SpectrumDriver();
      driver.setMicActive(true);
      run(driver, makeFeatures(), 4, fps);
      return run(driver, loud, 0.5, fps);
    });

    for (const drive of results.slice(1)) {
      expect(drive.energy).toBeCloseTo(results[0].energy, 2);
      expect(drive.flow).toBeCloseTo(results[0].flow, 2);
      expect(drive.spin).toBeCloseTo(results[0].spin, 2);
    }
  });

  it('never jumps the flow phase when energy changes abruptly', () => {
    const driver = new SpectrumDriver();
    driver.setMicActive(true);
    run(driver, makeFeatures(), 100, 60);

    const before = driver.drive.flow;
    driver.update(1 / 60, makeFeatures({ volume: 1, bass: 1, onset: 1 }));
    const step = driver.drive.flow - before;

    expect(step).toBeGreaterThan(0);
    expect(step).toBeLessThan(0.01);
  });

  it('clamps long frames so the animation cannot teleport', () => {
    const driver = new SpectrumDriver();
    driver.update(30, makeFeatures());
    expect(driver.drive.flow).toBeLessThan(0.02);
  });

  it('fades the alert palette in and out smoothly', () => {
    const driver = new SpectrumDriver();
    driver.setAlert(true);
    driver.update(1 / 60, makeFeatures());
    expect(driver.drive.alert).toBeLessThan(0.1);
    expect(run(driver, makeFeatures(), 1.5, 60).alert).toBeGreaterThan(0.95);
    driver.setAlert(false);
    expect(run(driver, makeFeatures(), 1.5, 60).alert).toBeLessThan(0.05);
  });

  it('compress saturates smoothly towards 1', () => {
    expect(compress(0)).toBe(0);
    expect(compress(1)).toBe(1);
    expect(compress(0.5)).toBeGreaterThan(0.5);
    expect(compress(10)).toBe(1);
    expect(compress(-1)).toBe(0);
  });
});

describe('AudioSphere', () => {
  let sphere: AudioSphere;

  beforeEach(() => {
    sphere = new AudioSphere(1.2, 16);
  });

  it('updates uniforms in-place from the drive values', () => {
    sphere.update(makeDrive());
    const uniforms = (sphere.mesh.material as any).uniforms;

    expect(uniforms.uFlow.value).toBe(1.5);
    expect(uniforms.uBass.value).toBe(0.8);
    expect(uniforms.uPulse.value).toBe(0.9);
  });

  it('disposes geometry and material cleanly', () => {
    expect(() => sphere.dispose()).not.toThrow();
  });
});

describe('ParticleField', () => {
  let particles: ParticleField;

  beforeEach(() => {
    particles = new ParticleField(4000, 2000);
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

    particles.update(makeDrive({ bass: 0.5 }));
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

    expect(() => shell.update(1.0, makeDrive())).not.toThrow();
    expect(() => shell.dispose()).not.toThrow();
  });

  it('hides the lattice, HUD and spikes until the boot sequence reveals them', () => {
    const shell = new HoloShell();
    shell.update(1.0, makeDrive({ bootFull: 0.3 }));
    const [dots, spikes] = shell.group.children as any[];
    expect(dots.material.uniforms.uReveal.value).toBe(0);
    expect(spikes.material.uniforms.uReveal.value).toBe(0);

    shell.update(1.0, makeDrive({ bootFull: 1 }));
    expect(dots.material.uniforms.uReveal.value).toBe(1);
    expect(spikes.material.uniforms.uReveal.value).toBe(1);
    shell.dispose();
  });
});

describe('Ambience', () => {
  it('builds sparks and orbital ring and updates cleanly', () => {
    const sparks = new AmbientSparks(100);
    const ring = new OrbitalRing();
    expect(sparks.points.geometry.getAttribute('position').count).toBe(100);

    ring.update(0.016, makeDrive(), false);
    sparks.update(1.0, makeDrive());
    expect(ring.points.rotation.z).toBeGreaterThan(0);

    ring.update(0.016, makeDrive(), true);
    const z = ring.points.rotation.z;
    ring.update(0.016, makeDrive(), true);
    expect(ring.points.rotation.z).toBe(z);

    expect(() => sparks.dispose()).not.toThrow();
    expect(() => ring.dispose()).not.toThrow();
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
