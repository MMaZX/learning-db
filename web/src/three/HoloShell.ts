import * as THREE from 'three';
import dotsVert from '../shaders/shellDots.vert.glsl?raw';
import dotsFrag from '../shaders/shellDots.frag.glsl?raw';
import spikesVert from '../shaders/spikes.vert.glsl?raw';
import spikesFrag from '../shaders/spikes.frag.glsl?raw';
import { AudioFeatures } from '../types/audio';

export const SHELL_RADIUS = 2.2;
const DECAL_COLOR = 0x5ab4ff;
const DECAL_BRIGHT = 0xd6f3ff;
const DEG = Math.PI / 180;

interface DecalMaterial {
  material: THREE.LineBasicMaterial | THREE.MeshBasicMaterial;
  baseOpacity: number;
}

export class HoloShell {
  public readonly group: THREE.Group;
  private dots: THREE.Points;
  private spikes: THREE.LineSegments;
  private decals: THREE.Group;
  private dotsMaterial: THREE.ShaderMaterial;
  private spikesMaterial: THREE.ShaderMaterial;
  private decalMaterials: DecalMaterial[] = [];
  private disposables: { dispose(): void }[] = [];

  constructor(radius = SHELL_RADIUS) {
    this.group = new THREE.Group();

    this.dotsMaterial = new THREE.ShaderMaterial({
      vertexShader: dotsVert,
      fragmentShader: dotsFrag,
      uniforms: {
        uTime: { value: 0 },
        uVolume: { value: 0 },
        uBass: { value: 0 },
        uOnset: { value: 0 },
        uPixelRatio: { value: 1 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.dots = new THREE.Points(this.buildDotLattice(radius), this.dotsMaterial);
    this.group.add(this.dots);

    this.spikesMaterial = new THREE.ShaderMaterial({
      vertexShader: spikesVert,
      fragmentShader: spikesFrag,
      uniforms: {
        uTime: { value: 0 },
        uRadius: { value: radius },
        uVolume: { value: 0 },
        uBass: { value: 0 },
        uTreble: { value: 0 },
        uOnset: { value: 0 },
      },
      transparent: true,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.spikes = new THREE.LineSegments(this.buildSpikes(900), this.spikesMaterial);
    this.spikes.frustumCulled = false;
    this.group.add(this.spikes);

    this.decals = new THREE.Group();
    this.buildDecals(radius);
    this.group.add(this.decals);

    this.disposables.push(this.dots.geometry, this.dotsMaterial, this.spikes.geometry, this.spikesMaterial);
  }

  private buildDotLattice(radius: number): THREE.BufferGeometry {
    const positions: number[] = [];
    const seeds: number[] = [];
    const accents: number[] = [];
    const rows = 64;
    const maxCols = rows * 2;

    for (let r = 1; r < rows; r++) {
      const lat = -Math.PI / 2 + (r / rows) * Math.PI;
      const cols = Math.max(6, Math.round(maxCols * Math.cos(lat)));
      for (let c = 0; c < cols; c++) {
        const lon = (c / cols) * Math.PI * 2;
        positions.push(
          radius * Math.cos(lat) * Math.sin(lon),
          radius * Math.sin(lat),
          radius * Math.cos(lat) * Math.cos(lon)
        );
        seeds.push(Math.random());
        accents.push(Math.random() < 0.025 ? 1 : 0);
      }
    }

    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
    geometry.setAttribute('aSeed', new THREE.Float32BufferAttribute(seeds, 1));
    geometry.setAttribute('aAccent', new THREE.Float32BufferAttribute(accents, 1));
    return geometry;
  }

  private buildSpikes(count: number): THREE.BufferGeometry {
    const positions = new Float32Array(count * 6);
    const ends = new Float32Array(count * 2);
    const seeds = new Float32Array(count * 2);

    for (let i = 0; i < count; i++) {
      const z = Math.random() * 2 - 1;
      const a = Math.random() * Math.PI * 2;
      const s = Math.sqrt(1 - z * z);
      const dir = [s * Math.cos(a), z, s * Math.sin(a)];
      const seed = Math.random();
      for (let e = 0; e < 2; e++) {
        const v = i * 2 + e;
        positions.set(dir, v * 3);
        ends[v] = e;
        seeds[v] = seed;
      }
    }

    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
    geometry.setAttribute('aEnd', new THREE.BufferAttribute(ends, 1));
    geometry.setAttribute('aSeed', new THREE.BufferAttribute(seeds, 1));
    return geometry;
  }

  private lineMaterial(color: number, opacity: number): THREE.LineBasicMaterial {
    const material = new THREE.LineBasicMaterial({
      color,
      transparent: true,
      opacity,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.decalMaterials.push({ material, baseOpacity: opacity });
    this.disposables.push(material);
    return material;
  }

  private arc(radius: number, start = 0, length = Math.PI * 2, color = DECAL_COLOR, opacity = 0.5): THREE.Line {
    const segments = Math.max(12, Math.round((length / (Math.PI * 2)) * 72));
    const points: THREE.Vector3[] = [];
    for (let i = 0; i <= segments; i++) {
      const t = start + (i / segments) * length;
      points.push(new THREE.Vector3(Math.cos(t) * radius, Math.sin(t) * radius, 0));
    }
    const geometry = new THREE.BufferGeometry().setFromPoints(points);
    this.disposables.push(geometry);
    return new THREE.Line(geometry, this.lineMaterial(color, opacity));
  }

  private polyline(points: [number, number][], color = DECAL_COLOR, opacity = 0.55): THREE.Line {
    const geometry = new THREE.BufferGeometry().setFromPoints(points.map(([x, y]) => new THREE.Vector3(x, y, 0)));
    this.disposables.push(geometry);
    return new THREE.Line(geometry, this.lineMaterial(color, opacity));
  }

  private thickArc(radius: number, width: number, start: number, length: number): THREE.Mesh {
    const geometry = new THREE.RingGeometry(radius - width, radius, 48, 1, start, length);
    const material = new THREE.MeshBasicMaterial({
      color: DECAL_BRIGHT,
      transparent: true,
      opacity: 0.75,
      side: THREE.DoubleSide,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
    });
    this.decalMaterials.push({ material, baseOpacity: 0.75 });
    this.disposables.push(geometry, material);
    return new THREE.Mesh(geometry, material);
  }

  // Places a flat 2D decal tangent to the shell so it wraps with the sphere.
  private placeOnShell(object: THREE.Object3D, latDeg: number, lonDeg: number, radius: number): void {
    const lat = latDeg * DEG;
    const lon = lonDeg * DEG;
    const pos = new THREE.Vector3(
      radius * Math.cos(lat) * Math.sin(lon),
      radius * Math.sin(lat),
      radius * Math.cos(lat) * Math.cos(lon)
    );
    object.position.copy(pos);
    object.lookAt(pos.clone().multiplyScalar(2));
    this.decals.add(object);
  }

  private buildDecals(radius: number): void {
    const r = radius * 1.005;

    const linkedRings = new THREE.Group();
    linkedRings.add(this.arc(0.3), this.arc(0.22, 0, Math.PI * 2, DECAL_COLOR, 0.4));
    const offsetRing = this.arc(0.26, 0, Math.PI * 2, DECAL_COLOR, 0.35);
    offsetRing.position.x = 0.36;
    linkedRings.add(offsetRing);
    this.placeOnShell(linkedRings, 40, -34, r);

    const glyph = new THREE.Group();
    glyph.add(
      this.polyline([[-0.12, -0.08], [0.12, -0.08], [0.12, 0.08], [-0.12, 0.08], [-0.12, -0.08]], DECAL_BRIGHT, 0.6),
      this.polyline([[-0.07, -0.03], [0.07, -0.03]], DECAL_BRIGHT, 0.5),
      this.polyline([[-0.07, 0.02], [0.03, 0.02]], DECAL_BRIGHT, 0.5),
      this.arc(0.3, -0.4, 1.6, DECAL_COLOR, 0.45)
    );
    this.placeOnShell(glyph, 38, 40, r);

    const sideBrackets = new THREE.Group();
    sideBrackets.add(this.arc(0.42, -0.7, 1.4, DECAL_COLOR, 0.35), this.arc(0.5, -0.5, 1.0, DECAL_COLOR, 0.25));
    this.placeOnShell(sideBrackets, 8, 72, r);

    const heavyArc = new THREE.Group();
    heavyArc.add(
      this.thickArc(0.55, 0.035, 95 * DEG, 70 * DEG),
      this.arc(0.62, 60 * DEG, 150 * DEG, DECAL_COLOR, 0.35),
      this.arc(0.48, 0, Math.PI * 2, DECAL_COLOR, 0.2)
    );
    this.placeOnShell(heavyArc, -30, -32, r);

    const ringCluster = new THREE.Group();
    const clusterRings: [number, number, number][] = [
      [0, 0, 0.3],
      [0.42, 0.05, 0.26],
      [0.2, -0.28, 0.22],
    ];
    clusterRings.forEach(([x, y, radiusRing]) => {
      const ring = this.arc(radiusRing, 0, Math.PI * 2, DECAL_COLOR, 0.35);
      ring.position.set(x, y, 0);
      ringCluster.add(ring);
    });
    this.placeOnShell(ringCluster, -42, 8, r);

    const stepGlyph = this.polyline(
      [[-0.14, 0], [-0.08, 0], [-0.08, 0.06], [-0.02, 0.06], [-0.02, 0], [0.04, 0], [0.04, 0.06], [0.1, 0.06]],
      DECAL_BRIGHT,
      0.6
    );
    this.placeOnShell(stepGlyph, -52, -38, r);

    const tallArc = this.arc(0.7, 2.3, 1.6, DECAL_COLOR, 0.3);
    this.placeOnShell(tallArc, -4, -74, r);

    const lowerTicks = new THREE.Group();
    for (let i = 0; i < 5; i++) {
      lowerTicks.add(this.polyline([[i * 0.05, 0], [i * 0.05 + 0.03, 0.08]], DECAL_BRIGHT, 0.45));
    }
    this.placeOnShell(lowerTicks, -48, 42, r);
  }

  public setPixelRatio(ratio: number): void {
    this.dotsMaterial.uniforms.uPixelRatio.value = ratio;
  }

  public update(delta: number, elapsed: number, features: AudioFeatures): void {
    this.group.rotation.y += delta * (0.035 + features.smoothedVolume * 0.05);

    const dots = this.dotsMaterial.uniforms;
    dots.uTime.value = elapsed;
    dots.uVolume.value = features.volume;
    dots.uBass.value = features.bass;
    dots.uOnset.value = features.onset;

    const spikes = this.spikesMaterial.uniforms;
    spikes.uTime.value = elapsed;
    spikes.uVolume.value = features.volume;
    spikes.uBass.value = features.bass;
    spikes.uTreble.value = features.treble;
    spikes.uOnset.value = features.onset;

    const scale = 1 + features.bass * 0.06 + features.onset * 0.04;
    this.decals.scale.setScalar(scale);
    const boost = 1 + features.smoothedVolume * 0.8 + features.onset * 0.4;
    for (const { material, baseOpacity } of this.decalMaterials) {
      material.opacity = Math.min(1, baseOpacity * boost);
    }
  }

  public dispose(): void {
    this.disposables.forEach((d) => d.dispose());
    this.disposables = [];
    this.decalMaterials = [];
  }
}
