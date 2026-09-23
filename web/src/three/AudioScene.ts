import * as THREE from 'three';
import { AudioFeatures } from '../types/audio';
import { AudioSphere } from './AudioSphere';
import { ParticleField } from './ParticleField';
import { HoloShell } from './HoloShell';
import { PostProcessingManager } from './PostProcessing';
import { QualityController, QualityProfile } from './QualityController';
import { SpectrumDriver } from './SpectrumDriver';
import { AmbientSparks, OrbitalRing } from './Ambience';

export class AudioScene {
  private container: HTMLElement;
  private features: AudioFeatures;
  private renderer: THREE.WebGLRenderer;
  private scene: THREE.Scene;
  private camera: THREE.PerspectiveCamera;
  private clock: THREE.Clock;

  private driver: SpectrumDriver;
  // Everything that spins with the spectrum; the orbital ring stays camera-facing.
  private root: THREE.Group;
  private sphere: AudioSphere;
  private particles: ParticleField;
  private shell: HoloShell;
  private sparks: AmbientSparks;
  private ring: OrbitalRing;
  private postProcessing: PostProcessingManager;
  public readonly qualityController: QualityController;

  private animationFrameId: number | null = null;
  private resizeObserver: ResizeObserver;
  private prefersReducedMotion = false;
  private mediaQueryList: MediaQueryList | null = null;
  private isDestroyed = false;

  constructor(container: HTMLElement, features: AudioFeatures) {
    this.container = container;
    this.features = features;

    // 1. Check WebGL2 Context Support
    const testCanvas = document.createElement('canvas');
    const gl = testCanvas.getContext('webgl2');
    if (!gl) {
      throw new Error('WEBGL2_UNAVAILABLE: WebGL2 is required but not supported in this browser');
    }

    const width = container.clientWidth || 800;
    const height = container.clientHeight || 600;

    // 2. Setup Scene, Camera, and Clock
    this.scene = new THREE.Scene();
    this.scene.background = new THREE.Color(0x0a0a0a);

    this.camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 100);
    this.camera.position.set(0, 0, 7.2);
    this.camera.lookAt(0, 0, 0);

    this.clock = new THREE.Clock();
    this.qualityController = new QualityController('HIGH');

    // 3. Renderer configuration
    this.renderer = new THREE.WebGLRenderer({
      powerPreference: 'high-performance',
      antialias: false,
      alpha: true,
    });
    this.renderer.setSize(width, height);
    this.renderer.setPixelRatio(this.qualityController.profile.pixelRatio);
    this.container.appendChild(this.renderer.domElement);

    // 4. Instantiation of 3D entities
    this.driver = new SpectrumDriver();
    this.root = new THREE.Group();
    this.scene.add(this.root);

    this.sphere = new AudioSphere(0.75);
    this.root.add(this.sphere.mesh);

    this.particles = new ParticleField(24000, this.qualityController.profile.particleCount);
    this.root.add(this.points);

    this.shell = new HoloShell();
    this.root.add(this.shell.group);

    this.sparks = new AmbientSparks();
    this.root.add(this.sparks.points);

    this.ring = new OrbitalRing();
    this.scene.add(this.ring.points);
    this.syncPixelRatio(this.qualityController.profile.pixelRatio);

    // 5. Post-Processing Pipeline
    this.postProcessing = new PostProcessingManager(
      this.renderer,
      this.scene,
      this.camera,
      width,
      height
    );

    // 6. Quality Profile Synchronization
    this.qualityController.onProfileChange((profile: QualityProfile) => {
      this.applyQualityProfile(profile);
    });

    // 7. Reduced Motion Observation
    if (typeof window !== 'undefined' && window.matchMedia) {
      this.mediaQueryList = window.matchMedia('(prefers-reduced-motion: reduce)');
      this.prefersReducedMotion = this.mediaQueryList.matches;
      this.driver.setReducedMotion(this.prefersReducedMotion);
      this.mediaQueryList.addEventListener('change', this.handleReducedMotionChange);
    }

    // 8. Container Resize Observer
    this.resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width: w, height: h } = entry.contentRect;
        if (w > 0 && h > 0) {
          this.resize(w, h);
        }
      }
    });
    this.resizeObserver.observe(this.container);
  }

  private get points(): THREE.Points {
    return this.particles.points;
  }

  private handleReducedMotionChange = (e: MediaQueryListEvent) => {
    this.prefersReducedMotion = e.matches;
    this.driver.setReducedMotion(e.matches);
  };

  /** Microphone live: completes the boot sequence (filaments → cyan → HUD → active). */
  public setMicActive(active: boolean): void {
    this.driver.setMicActive(active);
  }

  /** Switches the spectrum to the magenta alert palette. */
  public setAlert(active: boolean): void {
    this.driver.setAlert(active);
  }

  private applyQualityProfile(profile: QualityProfile): void {
    const width = this.container.clientWidth || 800;
    const height = this.container.clientHeight || 600;

    this.renderer.setPixelRatio(profile.pixelRatio);
    this.syncPixelRatio(profile.pixelRatio);
    this.particles.setCount(profile.particleCount);
    this.postProcessing.setQuality(profile.bloomEnabled, profile.bloomHalfRes, width, height);
  }

  private syncPixelRatio(ratio: number): void {
    this.particles.setPixelRatio(ratio);
    this.shell.setPixelRatio(ratio);
    this.sparks.setPixelRatio(ratio);
    this.ring.setPixelRatio(ratio);
  }

  public start(): void {
    if (this.isDestroyed || this.animationFrameId !== null) return;
    this.clock.start();
    this.renderLoop();
  }

  public stop(): void {
    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
  }

  private renderLoop = (): void => {
    if (this.isDestroyed) return;

    const delta = this.clock.getDelta();
    const elapsed = this.clock.getElapsedTime();
    const now = performance.now();

    // 1. Turn raw audio features into bounded, time-smoothed drive values
    const drive = this.driver.update(delta, this.features);
    this.root.rotation.y = drive.spin;

    this.sphere.update(drive);
    this.particles.update(drive);
    this.shell.update(elapsed, drive);
    this.sparks.update(elapsed, drive);
    this.ring.update(delta, drive, this.prefersReducedMotion);

    // 2. Camera drift (disabled if reduced motion)
    if (!this.prefersReducedMotion) {
      this.camera.position.x = Math.sin(elapsed * 0.12) * 0.22;
      this.camera.position.y = Math.cos(elapsed * 0.15) * 0.16;
      this.camera.lookAt(0, 0, 0);
    } else {
      this.camera.position.set(0, 0, 7.2);
      this.camera.lookAt(0, 0, 0);
    }

    // 3. Post-Processing bloom intensity update
    this.postProcessing.setAudioLevel(drive.energy, drive.pulse);

    // 4. Render Scene
    if (this.postProcessing.isBloomEnabled) {
      this.postProcessing.render();
    } else {
      this.renderer.render(this.scene, this.camera);
    }

    // 5. Performance Monitoring
    this.qualityController.update(now);

    this.animationFrameId = requestAnimationFrame(this.renderLoop);
  };

  public resize(width: number, height: number): void {
    if (this.isDestroyed || width <= 0 || height <= 0) return;

    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();

    this.renderer.setSize(width, height);
    this.postProcessing.resize(width, height);
  }

  public destroy(): void {
    if (this.isDestroyed) return;
    this.isDestroyed = true;

    this.stop();
    this.resizeObserver.disconnect();

    if (this.mediaQueryList) {
      this.mediaQueryList.removeEventListener('change', this.handleReducedMotionChange);
    }

    // Dispose entities
    this.sphere.dispose();
    this.particles.dispose();
    this.shell.dispose();
    this.sparks.dispose();
    this.ring.dispose();
    this.postProcessing.dispose();

    // Dispose renderer and force context loss
    this.renderer.dispose();
    this.renderer.forceContextLoss();

    if (this.renderer.domElement.parentElement) {
      this.renderer.domElement.parentElement.removeChild(this.renderer.domElement);
    }
  }
}
