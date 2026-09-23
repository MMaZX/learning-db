import React, { useEffect, useRef, useState } from 'react';
import { AudioAnalyzer } from '../audio/AudioAnalyzer';
import { AudioScene } from '../three/AudioScene';
import { QualityProfile } from '../three/QualityController';

interface AudioVisualizerProps {
  analyzer: AudioAnalyzer;
  /** Microphone live: completes the spectrum boot sequence. */
  micActive?: boolean;
  /** Microphone or server error: switches the spectrum to the alert palette. */
  alert?: boolean;
  onQualityChange?: (profile: QualityProfile, avgFps: number) => void;
}

export const AudioVisualizer: React.FC<AudioVisualizerProps> = ({
  analyzer,
  micActive = false,
  alert = false,
  onQualityChange,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<AudioScene | null>(null);
  const [webGlError, setWebGlError] = useState<string | null>(null);
  const visualStateRef = useRef({ micActive, alert });
  visualStateRef.current = { micActive, alert };

  useEffect(() => {
    if (!containerRef.current) return;

    try {
      const scene = new AudioScene(containerRef.current, analyzer.features);
      sceneRef.current = scene;
      scene.setMicActive(visualStateRef.current.micActive);
      scene.setAlert(visualStateRef.current.alert);
      scene.start();

      // Sync quality updates
      scene.qualityController.onProfileChange((profile) => {
        if (onQualityChange) {
          onQualityChange(profile, scene.qualityController.avgFps);
        }
      });

      // Connect RAF update of analyzer
      let frameId: number;
      const syncAudioLoop = () => {
        analyzer.update();
        if (onQualityChange && sceneRef.current) {
          onQualityChange(
            sceneRef.current.qualityController.profile,
            sceneRef.current.qualityController.avgFps
          );
        }
        frameId = requestAnimationFrame(syncAudioLoop);
      };
      frameId = requestAnimationFrame(syncAudioLoop);

      return () => {
        cancelAnimationFrame(frameId);
        scene.destroy();
        sceneRef.current = null;
      };
    } catch (err: unknown) {
      const error = err as Error;
      setWebGlError(error.message || 'WebGL2 Context could not be initialized');
    }
  }, [analyzer, onQualityChange]);

  useEffect(() => {
    sceneRef.current?.setMicActive(micActive);
  }, [micActive]);

  useEffect(() => {
    sceneRef.current?.setAlert(alert);
  }, [alert]);

  if (webGlError) {
    return (
      <div className="webgl-fallback" role="alert">
        <div className="fallback-badge">HARDWARE FALLBACK</div>
        <h3>Modo visual alternativo 2D</h3>
        <p>{webGlError}</p>
        <div className="fallback-meter">
          <div
            className="fallback-meter-fill"
            style={{ width: `${Math.min(100, analyzer.features.volume * 100)}%` }}
          />
        </div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="visualizer-container"
      aria-label="Organismo visualizador 3D reactivo a voz"
      role="img"
    />
  );
};
