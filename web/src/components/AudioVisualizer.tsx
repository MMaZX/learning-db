import React, { useEffect, useRef, useState } from 'react';
import { AudioAnalyzer } from '../audio/AudioAnalyzer';
import { AudioScene } from '../three/AudioScene';
import { QualityProfile } from '../three/QualityController';

interface AudioVisualizerProps {
  analyzer: AudioAnalyzer;
  onQualityChange?: (profile: QualityProfile, avgFps: number) => void;
}

export const AudioVisualizer: React.FC<AudioVisualizerProps> = ({
  analyzer,
  onQualityChange,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const sceneRef = useRef<AudioScene | null>(null);
  const [webGlError, setWebGlError] = useState<string | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    try {
      const scene = new AudioScene(containerRef.current, analyzer.features);
      sceneRef.current = scene;
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
