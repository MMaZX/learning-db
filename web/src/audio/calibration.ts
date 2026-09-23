export class NoiseCalibrator {
  private durationMs: number;
  private startTime = 0;
  private sampleCount = 0;
  private sampleSum = 0;
  private sampleMax = 0;
  private _isCalibrating = false;
  private _isCalibrated = false;
  private _noiseFloor = 0.01;

  constructor(durationMs = 750) {
    this.durationMs = durationMs;
  }

  public start(): void {
    this.sampleCount = 0;
    this.sampleSum = 0;
    this.sampleMax = 0;
    this.startTime = performance.now();
    this._isCalibrating = true;
    this._isCalibrated = false;
  }

  public update(currentRms: number): boolean {
    if (!this._isCalibrating) return this._isCalibrated;

    this.sampleCount++;
    this.sampleSum += currentRms;
    if (currentRms > this.sampleMax) {
      this.sampleMax = currentRms;
    }

    const elapsed = performance.now() - this.startTime;

    if (elapsed >= this.durationMs) {
      this._isCalibrating = false;
      this._isCalibrated = true;
      if (this.sampleCount > 0) {
        const avg = this.sampleSum / this.sampleCount;
        this._noiseFloor = Math.min(0.2, (avg * 0.7) + (this.sampleMax * 0.3) + 0.005);
      }
      return true;
    }

    return false;
  }

  public get isCalibrating(): boolean {
    return this._isCalibrating;
  }

  public get isCalibrated(): boolean {
    return this._isCalibrated;
  }

  public get noiseFloor(): number {
    return this._noiseFloor;
  }

  public reset(): void {
    this.sampleCount = 0;
    this.sampleSum = 0;
    this.sampleMax = 0;
    this._isCalibrating = false;
    this._isCalibrated = false;
    this._noiseFloor = 0.01;
  }
}
