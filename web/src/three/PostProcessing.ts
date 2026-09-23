import * as THREE from 'three';
import { EffectComposer } from 'three/examples/jsm/postprocessing/EffectComposer.js';
import { RenderPass } from 'three/examples/jsm/postprocessing/RenderPass.js';
import { UnrealBloomPass } from 'three/examples/jsm/postprocessing/UnrealBloomPass.js';
import { ShaderPass } from 'three/examples/jsm/postprocessing/ShaderPass.js';

// Radial RGB split, stronger towards the edges, as in the reference footage.
const ChromaticAberrationShader = {
  uniforms: {
    tDiffuse: { value: null },
    uAmount: { value: 0.004 },
  },
  vertexShader: /* glsl */ `
    varying vec2 vUv;
    void main() {
      vUv = uv;
      gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
    }
  `,
  fragmentShader: /* glsl */ `
    uniform sampler2D tDiffuse;
    uniform float uAmount;
    varying vec2 vUv;
    void main() {
      vec2 offset = (vUv - 0.5) * uAmount;
      vec4 base = texture2D(tDiffuse, vUv);
      float r = texture2D(tDiffuse, vUv + offset).r;
      float b = texture2D(tDiffuse, vUv - offset).b;
      gl_FragColor = vec4(r, base.g, b, base.a);
    }
  `,
};

export class PostProcessingManager {
  private composer: EffectComposer;
  private bloomPass: UnrealBloomPass;
  private chromaPass: ShaderPass;
  private baseStrength = 0.6;
  private audioBoost = 0.35;
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

    this.chromaPass = new ShaderPass(ChromaticAberrationShader);
    this.composer.addPass(this.chromaPass);
  }

  public get isBloomEnabled(): boolean {
    return this._isBloomEnabled;
  }

  public setAudioLevel(energy: number, pulse: number): void {
    if (!this._isBloomEnabled) return;
    const dynamicBloom = this.baseStrength + (energy * this.audioBoost) + (pulse * 0.15);
    this.bloomPass.strength = THREE.MathUtils.clamp(dynamicBloom, 0.4, 2.0);
    this.chromaPass.uniforms.uAmount.value = 0.004 + energy * 0.005 + pulse * 0.003;
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
    this.chromaPass.dispose();
    this.composer.dispose();
  }
}
