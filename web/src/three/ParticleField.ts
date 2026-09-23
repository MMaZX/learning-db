import * as THREE from 'three';
import noiseChunk from '../shaders/noise.glsl?raw';
import particlesVert from '../shaders/particles.vert.glsl?raw';
import particlesFrag from '../shaders/particles.frag.glsl?raw';
import { AudioFeatures } from '../types/audio';

const POINTS_PER_STRAND = 8;
export const CORE_RADIUS = 1.5;

function randomUnitVector(): [number, number, number] {
  const z = Math.random() * 2 - 1;
  const a = Math.random() * Math.PI * 2;
  const s = Math.sqrt(1 - z * z);
  return [s * Math.cos(a), z, s * Math.sin(a)];
}

export class ParticleField {
  public readonly points: THREE.Points;
  private geometry: THREE.BufferGeometry;
  private material: THREE.ShaderMaterial;
  private maxParticles: number;
  private currentCount: number;

  constructor(maxParticles = 24000, initialCount = 16000) {
    this.maxParticles = maxParticles;
    this.currentCount = Math.min(initialCount, maxParticles);

    this.geometry = new THREE.BufferGeometry();

    const positions = new Float32Array(maxParticles * 3);
    const seeds = new Float32Array(maxParticles);
    const sizes = new Float32Array(maxParticles);
    const phases = new Float32Array(maxParticles);
    const layers = new Float32Array(maxParticles);
    const strands = new Float32Array(maxParticles);
    const velocities = new Float32Array(maxParticles * 3);

    // Strands are shuffled across the buffer so that shrinking the draw range
    // (quality LOD) thins every layer evenly instead of dropping one entirely.
    const strandCount = Math.ceil(maxParticles / POINTS_PER_STRAND);
    for (let sIdx = 0; sIdx < strandCount; sIdx++) {
      const roll = Math.random();
      // 0: inner volume, 1: dense outer skin of the core, 2: loose sparks
      const layer = roll < 0.42 ? 0 : roll < 0.92 ? 1 : 2;
      const radius =
        layer === 0
          ? CORE_RADIUS * Math.pow(Math.random(), 0.45) * 0.88
          : layer === 1
            ? CORE_RADIUS * (0.86 + Math.random() * 0.16)
            : CORE_RADIUS * (0.95 + Math.random() * 0.2);
      const [dx, dy, dz] = randomUnitVector();
      const [vx, vy, vz] = randomUnitVector();
      const seed = Math.random();
      const phase = Math.random() * Math.PI * 2;
      const baseSize = layer === 2 ? 0.06 + Math.random() * 0.04 : 0.045 + Math.random() * 0.04;

      for (let k = 0; k < POINTS_PER_STRAND; k++) {
        const i = sIdx * POINTS_PER_STRAND + k;
        if (i >= maxParticles) break;
        const i3 = i * 3;
        positions[i3] = dx * radius;
        positions[i3 + 1] = dy * radius;
        positions[i3 + 2] = dz * radius;
        velocities[i3] = vx * 0.3;
        velocities[i3 + 1] = vy * 0.3;
        velocities[i3 + 2] = vz * 0.3;
        seeds[i] = seed;
        phases[i] = phase;
        layers[i] = layer;
        sizes[i] = baseSize;
        strands[i] = k / (POINTS_PER_STRAND - 1);
      }
    }

    this.geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    this.geometry.setAttribute('aSeed', new THREE.BufferAttribute(seeds, 1));
    this.geometry.setAttribute('aSize', new THREE.BufferAttribute(sizes, 1));
    this.geometry.setAttribute('aPhase', new THREE.BufferAttribute(phases, 1));
    this.geometry.setAttribute('aLayer', new THREE.BufferAttribute(layers, 1));
    this.geometry.setAttribute('aStrand', new THREE.BufferAttribute(strands, 1));
    this.geometry.setAttribute('aVelocity', new THREE.BufferAttribute(velocities, 3));
    this.geometry.setDrawRange(0, this.currentCount);
    this.geometry.boundingSphere = new THREE.Sphere(new THREE.Vector3(), CORE_RADIUS * 3);

    this.material = new THREE.ShaderMaterial({
      vertexShader: noiseChunk + particlesVert,
      fragmentShader: particlesFrag,
      uniforms: {
        uTime: { value: 0 },
        uVolume: { value: 0 },
        uBass: { value: 0 },
        uLowMid: { value: 0 },
        uMid: { value: 0 },
        uHighMid: { value: 0 },
        uTreble: { value: 0 },
        uOnset: { value: 0 },
        uPixelRatio: { value: 1 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      depthTest: true,
    });

    this.points = new THREE.Points(this.geometry, this.material);
  }

  public setCount(count: number): void {
    this.currentCount = Math.max(500, Math.min(count, this.maxParticles));
    this.geometry.setDrawRange(0, this.currentCount);
  }

  public get count(): number {
    return this.currentCount;
  }

  public setPixelRatio(ratio: number): void {
    this.material.uniforms.uPixelRatio.value = ratio;
  }

  public update(elapsed: number, features: AudioFeatures): void {
    const uniforms = this.material.uniforms;
    uniforms.uTime.value = elapsed;
    uniforms.uVolume.value = features.volume;
    uniforms.uBass.value = features.bass;
    uniforms.uLowMid.value = features.lowMid;
    uniforms.uMid.value = features.mid;
    uniforms.uHighMid.value = features.highMid;
    uniforms.uTreble.value = features.treble;
    uniforms.uOnset.value = features.onset;
  }

  public dispose(): void {
    this.geometry.dispose();
    this.material.dispose();
  }
}
