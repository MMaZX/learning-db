import * as THREE from 'three';
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';

export class PostProcessingManager {
  private composer: EffectComposer;
  private bloomPass: UnrealBloomPass;
  private baseStrength = 0.95;
  private audioBoost = 0.9;
  private _isBloomEnabled = true;

  constructor(
    renderer: THREE.WebGLRenderer,
    scene: THREE.Scene,
    camera: THREE.PerspectiveCamera,
    width: number,
    height: number
  ) {
    this.composer = new EffectComposer(renderer);

    const renderPass = new RenderPass(scene, camera);
    this.composer.addPass(renderPass);

    this.bloomPass = new UnrealBloomPass(
      new THREE.Vector2(width, height),
      this.baseStrength,
      0.5, // radius
      0.2  // threshold
    );
    this.composer.addPass(this.bloomPass);
  }

  public get isBloomEnabled(): boolean {
    return this._isBloomEnabled;
  }

  public setAudioLevel(smoothedVolume: number, onset: number): void {
    if (!this._isBloomEnabled) return;
    const dynamicBloom = this.baseStrength + (smoothedVolume * this.audioBoost) + (onset * 0.45);
    this.bloomPass.strength = THREE.MathUtils.clamp(dynamicBloom, 0.4, 2.0);
  }

  public setBaseStrength(strength: number): void {
    this.baseStrength = strength;
  }

  public setQuality(enableBloom: boolean, halfResolution = false, width = 800, height = 600): void {
    this._isBloomEnabled = enableBloom;
    this.bloomPass.enabled = enableBloom;
    if (enableBloom && halfResolution) {
      this.bloomPass.resolution.set(Math.floor(width * 0.5), Math.floor(height * 0.5));
    }
  }

  public resize(width: number, height: number): void {
    this.composer.setSize(width, height);
    if (this.bloomPass.resolution) {
      this.bloomPass.resolution.set(width, height);
    }
  }

  public render(): void {
    this.composer.render();
  }

  public dispose(): void {
    this.bloomPass.dispose();
    this.composer.dispose();
  }
}
