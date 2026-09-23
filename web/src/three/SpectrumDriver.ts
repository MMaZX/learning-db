import { AudioFeatures } from '../types/audio';

/**
 * Visual drive values consumed by every layer of the spectrum. All audio-derived
 * values are bounded to [0, 1] and smoothed in seconds, so neither the
 * sensitivity slider nor the display refresh rate can make the spectrum jump.
 */
export interface SpectrumDrive {
  energy: number;
  bass: number;
  mid: number;
  treble: number;
  pulse: number;
  /** Integrated noise-field phase: advances at a bounded speed, never jumps. */
  flow: number;
  /** Integrated global rotation in radians. */
  spin: number;
  /** Page-load boot stage (sparse white dots), 0 → 1. */
  bootDots: number;
  /** Microphone boot stage (filaments → cyan → lattice/HUD → live), 0 → 1. */
  bootFull: number;
  /** Alert palette mix (magenta), 0 → 1. */
  alert: number;
}

// Attack / release time constants in seconds. The reference ramps between
// idle and active over ~0.3–0.5 s, never frame to frame.
const ENERGY_TAU: [number, number] = [0.12, 0.45];
const BAND_TAU: [number, number] = [0.08, 0.3];
const PULSE_TAU: [number, number] = [0.05, 0.35];
const ALERT_TAU = 0.35;

const BOOT_DOTS_SECONDS = 1.2;
const BOOT_FULL_UP_SECONDS = 3.2;
const BOOT_FULL_DOWN_SECONDS = 1.2;

// Measured in the reference: ~2°/s idle rotation, flow speeds up moderately.
const SPIN_IDLE = 0.03;
const SPIN_ENERGY = 0.035;
const FLOW_IDLE = 0.12;
const FLOW_ENERGY = 0.22;
const REDUCED_MOTION_SCALE = 0.25;

// Long frames (tab switch, GC pause) must not teleport the animation.
const MAX_DT = 0.1;

/** Soft-knee saturation: 0 → 0, 1 → 1, anything above 1 approaches 1 smoothly. */
export function compress(x: number): number {
  const v = Math.max(0, x);
  return Math.min(1, (2 * v) / (1 + v));
}

function smoothTo(current: number, target: number, dt: number, [attack, release]: [number, number]): number {
  const tau = target > current ? attack : release;
  return current + (target - current) * (1 - Math.exp(-dt / tau));
}

function stepTo(current: number, target: number, dt: number, upSeconds: number, downSeconds: number): number {
  if (target > current) return Math.min(target, current + dt / upSeconds);
  return Math.max(target, current - dt / downSeconds);
}

function smoothstep(edge0: number, edge1: number, x: number): number {
  const t = Math.min(1, Math.max(0, (x - edge0) / (edge1 - edge0)));
  return t * t * (3 - 2 * t);
}

export class SpectrumDriver {
  public readonly drive: SpectrumDrive = {
    energy: 0,
    bass: 0,
    mid: 0,
    treble: 0,
    pulse: 0,
    flow: 0,
    spin: 0,
    bootDots: 0,
    bootFull: 0,
    alert: 0,
  };

  private micActive = false;
  private alertTarget = 0;
  private reducedMotion = false;

  public setMicActive(active: boolean): void {
    this.micActive = active;
  }

  public setAlert(active: boolean): void {
    this.alertTarget = active ? 1 : 0;
  }

  public setReducedMotion(reduced: boolean): void {
    this.reducedMotion = reduced;
  }

  public update(deltaSeconds: number, features: AudioFeatures): SpectrumDrive {
    const dt = Math.min(MAX_DT, Math.max(0, deltaSeconds));
    const d = this.drive;

    d.bootDots = stepTo(d.bootDots, 1, dt, BOOT_DOTS_SECONDS, BOOT_DOTS_SECONDS);
    d.bootFull = stepTo(d.bootFull, this.micActive ? 1 : 0, dt, BOOT_FULL_UP_SECONDS, BOOT_FULL_DOWN_SECONDS);
    d.alert = smoothTo(d.alert, this.alertTarget, dt, [ALERT_TAU, ALERT_TAU]);

    // Audio only reaches the visuals once the boot sequence is live.
    const live = smoothstep(0.8, 1, d.bootFull);
    d.energy = smoothTo(d.energy, live * compress(features.volume * 0.85 + features.bass * 0.15), dt, ENERGY_TAU);
    d.bass = smoothTo(d.bass, live * compress(features.bass), dt, BAND_TAU);
    d.mid = smoothTo(d.mid, live * compress(features.mid), dt, BAND_TAU);
    d.treble = smoothTo(d.treble, live * compress(features.treble), dt, BAND_TAU);
    d.pulse = smoothTo(d.pulse, live * compress(features.onset), dt, PULSE_TAU);

    const motion = this.reducedMotion ? REDUCED_MOTION_SCALE : 1;
    d.flow += dt * (FLOW_IDLE + d.energy * FLOW_ENERGY) * motion;
    d.spin += dt * (SPIN_IDLE + d.energy * SPIN_ENERGY) * motion;

    return d;
  }
}
