import * as THREE from 'three';
import sparksVert from '../shaders/sparks.vert.glsl?raw';
import sparksFrag from '../shaders/sparks.frag.glsl?raw';
import ringVert from '../shaders/orbitalRing.vert.glsl?raw';
import { CORE_RADIUS } from './ParticleField';
import { SpectrumDrive } from './SpectrumDriver';

// Floating motes live between the lattice and ~1.95x the core radius, which
// keeps them inside the frame at the default camera distance.
const SPARK_MIN = CORE_RADIUS * 1.3;
const SPARK_MAX = CORE_RADIUS * 1.95;
const RING_RADIUS = CORE_RADIUS * 1.5;
const RING_DOTS = 180;
const RING_SPIN = 0.05;

/** Cyan and magenta motes drifting around the spectrum. */
export class AmbientSparks {
  public readonly points: THREE.Points;
  private geometry: THREE.BufferGeometry;
  private material: THREE.ShaderMaterial;

  constructor(count = 520) {
    const positions = new Float32Array(count * 3);
    const seeds = new Float32Array(count);
    const warm = new Float32Array(count);

    for (let i = 0; i < count; i++) {
      const z = Math.random() * 2 - 1;
      const a = Math.random() * Math.PI * 2;
      const s = Math.sqrt(1 - z * z);
      const r = SPARK_MIN + (SPARK_MAX - SPARK_MIN) * Math.pow(Math.random(), 1.6);
      positions.set([s * Math.cos(a) * r, z * r, s * Math.sin(a) * r], i * 3);
      seeds[i] = Math.random();
      warm[i] = Math.random() < 0.22 ? 1 : 0;
    }

    this.geometry = new THREE.BufferGeometry();
    this.geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    this.geometry.setAttribute('aSeed', new THREE.BufferAttribute(seeds, 1));
    this.geometry.setAttribute('aWarm', new THREE.BufferAttribute(warm, 1));

    this.material = new THREE.ShaderMaterial({
      vertexShader: sparksVert,
      fragmentShader: sparksFrag,
      uniforms: {
        uTime: { value: 0 },
        uEnergy: { value: 0 },
        uBootDots: { value: 0 },
        uBootFull: { value: 0 },
        uAlert: { value: 0 },
        uPixelRatio: { value: 1 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });

    this.points = new THREE.Points(this.geometry, this.material);
  }

  public setPixelRatio(ratio: number): void {
    this.material.uniforms.uPixelRatio.value = ratio;
  }

  public update(elapsed: number, drive: SpectrumDrive): void {
    const u = this.material.uniforms;
    u.uTime.value = elapsed;
    u.uEnergy.value = drive.energy;
    u.uBootDots.value = drive.bootDots;
    u.uBootFull.value = drive.bootFull;
    u.uAlert.value = drive.alert;
  }

  public dispose(): void {
    this.geometry.dispose();
    this.material.dispose();
  }
}

/** Dotted ring facing the camera; it draws itself in during the page-load boot. */
export class OrbitalRing {
  public readonly points: THREE.Points;
  private geometry: THREE.BufferGeometry;
  private material: THREE.ShaderMaterial;

  constructor(radius = RING_RADIUS, dots = RING_DOTS) {
    const positions = new Float32Array(dots * 3);
    const angles = new Float32Array(dots);
    const accents = new Float32Array(dots);

    for (let i = 0; i < dots; i++) {
      // Start at the top and sweep both ways so the ring closes at the bottom.
      const side = i % 2 === 0 ? 1 : -1;
      const progress = Math.ceil(i / 2) / (dots / 2);
      const theta = Math.PI / 2 + side * progress * Math.PI;
      positions.set([Math.cos(theta) * radius, Math.sin(theta) * radius, 0], i * 3);
      angles[i] = progress;
      accents[i] = Math.random() < 0.08 ? 1 : 0;
    }

    this.geometry = new THREE.BufferGeometry();
    this.geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    this.geometry.setAttribute('aAngle', new THREE.BufferAttribute(angles, 1));
    this.geometry.setAttribute('aAccent', new THREE.BufferAttribute(accents, 1));

    this.material = new THREE.ShaderMaterial({
      vertexShader: ringVert,
      fragmentShader: sparksFrag,
      uniforms: {
        uDraw: { value: 0 },
        uBootFull: { value: 0 },
        uEnergy: { value: 0 },
        uAlert: { value: 0 },
        uPixelRatio: { value: 1 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });

    this.points = new THREE.Points(this.geometry, this.material);
  }

  public setPixelRatio(ratio: number): void {
    this.material.uniforms.uPixelRatio.value = ratio;
  }

  public update(delta: number, drive: SpectrumDrive, reducedMotion: boolean): void {
    if (!reducedMotion) {
      this.points.rotation.z += Math.min(0.1, delta) * RING_SPIN;
    }
    const u = this.material.uniforms;
    u.uDraw.value = drive.bootDots;
    u.uBootFull.value = drive.bootFull;
    u.uEnergy.value = drive.energy;
    u.uAlert.value = drive.alert;
  }

  public dispose(): void {
    this.geometry.dispose();
    this.material.dispose();
  }
}
