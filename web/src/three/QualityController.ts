export type QualityTier = 'HIGH' | 'MEDIUM' | 'LOW';

export interface QualityProfile {
  tier: QualityTier;
  particleCount: number;
  bloomEnabled: boolean;
  bloomHalfRes: boolean;
  pixelRatio: number;
}

export const QUALITY_PROFILES: Record<QualityTier, QualityProfile> = {
  HIGH: {
    tier: 'HIGH',
    particleCount: 20000,
    bloomEnabled: true,
    bloomHalfRes: false,
    pixelRatio: typeof window !== 'undefined' ? Math.min(window.devicePixelRatio || 1, 2) : 1,
  },
  MEDIUM: {
    tier: 'MEDIUM',
    particleCount: 12000,
    bloomEnabled: true,
    bloomHalfRes: true,
    pixelRatio: 1.0,
  },
  LOW: {
    tier: 'LOW',
    particleCount: 6000,
    bloomEnabled: false,
    bloomHalfRes: false,
    pixelRatio: 1.0,
  },
};

export class QualityController {
  private currentTier: QualityTier = 'HIGH';
  private frameTimes: number[] = [];
  private maxFrameTimes = 60;
  private lastTime = 0;
  private lowFpsStartTime = 0;
  private highFpsStartTime = 0;
  private onProfileChangeCallback: ((profile: QualityProfile) => void) | null = null;

  public fps = 60;
  public avgFps = 60;

  constructor(initialTier: QualityTier = 'HIGH') {
    this.currentTier = initialTier;
  }

  public onProfileChange(callback: (profile: QualityProfile) => void): void {
    this.onProfileChangeCallback = callback;
  }

  public get profile(): QualityProfile {
    return QUALITY_PROFILES[this.currentTier];
  }

  public setTier(tier: QualityTier): void {
    if (this.currentTier !== tier) {
      this.currentTier = tier;
      if (this.onProfileChangeCallback) {
        this.onProfileChangeCallback(this.profile);
      }
    }
  }

  public update(now: number): void {
    if (this.lastTime === 0) {
      this.lastTime = now;
      return;
    }

    const delta = now - this.lastTime;
    this.lastTime = now;
    if (delta <= 0) return;

    this.fps = 1000 / delta;
    this.frameTimes.push(this.fps);
    if (this.frameTimes.length > this.maxFrameTimes) {
      this.frameTimes.shift();
    }

    this.avgFps = this.frameTimes.reduce((a, b) => a + b, 0) / this.frameTimes.length;

    // Quality degradation / upgrade with hysteresis
    if (this.avgFps < 48) {
      this.highFpsStartTime = 0;
      if (this.lowFpsStartTime === 0) {
        this.lowFpsStartTime = now;
      } else if (now - this.lowFpsStartTime > 1500) {
        this.degradeQuality();
        this.lowFpsStartTime = 0;
      }
    } else if (this.avgFps > 58) {
      this.lowFpsStartTime = 0;
      if (this.highFpsStartTime === 0) {
        this.highFpsStartTime = now;
      } else if (now - this.highFpsStartTime > 4000) {
        this.upgradeQuality();
        this.highFpsStartTime = 0;
      }
    } else {
      this.lowFpsStartTime = 0;
      this.highFpsStartTime = 0;
    }
  }

  private degradeQuality(): void {
    if (this.currentTier === 'HIGH') {
      this.setTier('MEDIUM');
    } else if (this.currentTier === 'MEDIUM') {
      this.setTier('LOW');
    }
  }

  private upgradeQuality(): void {
    if (this.currentTier === 'LOW') {
      this.setTier('MEDIUM');
    } else if (this.currentTier === 'MEDIUM') {
      this.setTier('HIGH');
    }
  }
}
