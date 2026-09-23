import * as THREE from 'three';
import noiseChunk from '../shaders/noise.glsl?raw';
import sphereVert from '../shaders/sphere.vert.glsl?raw';
import sphereFrag from '../shaders/sphere.frag.glsl?raw';
import { AudioFeatures } from '../types/audio';

export class AudioSphere {
  public readonly mesh: THREE.Mesh;
  private geometry: THREE.IcosahedronGeometry;
  private material: THREE.ShaderMaterial;

  constructor(radius = 1.0, detail = 12) {
    this.geometry = new THREE.IcosahedronGeometry(radius, detail);
    this.material = new THREE.ShaderMaterial({
      vertexShader: noiseChunk + sphereVert,
      fragmentShader: sphereFrag,
      uniforms: {
        uTime: { value: 0 },
        uVolume: { value: 0 },
        uBass: { value: 0 },
        uLowMid: { value: 0 },
        uMid: { value: 0 },
        uHighMid: { value: 0 },
        uTreble: { value: 0 },
        uSpectralFlux: { value: 0 },
        uOnset: { value: 0 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      depthTest: false,
    });

    this.mesh = new THREE.Mesh(this.geometry, this.material);
    this.mesh.renderOrder = -1;
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
    uniforms.uSpectralFlux.value = features.spectralFlux;
    uniforms.uOnset.value = features.onset;
  }

  public dispose(): void {
    this.geometry.dispose();
    this.material.dispose();
  }
}
