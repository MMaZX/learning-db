export class AsymmetricSmoother {
  private currentValue = 0;
  private attackCoeff: number;
  private releaseCoeff: number;

  constructor(attack = 0.35, release = 0.15) {
    this.attackCoeff = attack;
    this.releaseCoeff = release;
  }

  public update(target: number): number {
    if (target > this.currentValue) {
      this.currentValue += (target - this.currentValue) * this.attackCoeff;
    } else {
      this.currentValue += (target - this.currentValue) * this.releaseCoeff;
    }
    return this.currentValue;
  }

  public get value(): number {
    return this.currentValue;
  }

  public reset(initialValue = 0): void {
    this.currentValue = initialValue;
  }

  public setCoefficients(attack: number, release: number): void {
    this.attackCoeff = attack;
    this.releaseCoeff = release;
  }
}
