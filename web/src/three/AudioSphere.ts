import * as THREE from 'three';
import noiseChunk from '../shaders/noise.glsl?raw';
import sphereVert from '../shaders/sphere.vert.glsl?raw';
import sphereFrag from '../shaders/sphere.frag.glsl?raw';
import { SpectrumDrive } from './SpectrumDriver';

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
        uFlow: { value: 0 },
        uEnergy: { value: 0 },
        uBass: { value: 0 },
        uMid: { value: 0 },
        uPulse: { value: 0 },
        uBootFull: { value: 0 },
        uAlert: { value: 0 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      depthTest: false,
    });

    this.mesh = new THREE.Mesh(this.geometry, this.material);
    this.mesh.renderOrder = -1;
  }

  public update(drive: SpectrumDrive): void {
    const uniforms = this.material.uniforms;
    uniforms.uFlow.value = drive.flow;
    uniforms.uEnergy.value = drive.energy;
    uniforms.uBass.value = drive.bass;
    uniforms.uMid.value = drive.mid;
    uniforms.uPulse.value = drive.pulse;
    uniforms.uBootFull.value = drive.bootFull;
    uniforms.uAlert.value = drive.alert;
  }

  public dispose(): void {
    this.geometry.dispose();
    this.material.dispose();
  }
}
