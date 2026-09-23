import React, { useEffect, useState } from 'react';
import { AudioAnalyzer } from '../audio/AudioAnalyzer';
import { AudioFeatures, VisualizerConfig } from '../types/audio';
import { QualityProfile } from '../three/QualityController';

interface DebugPanelProps {
  analyzer: AudioAnalyzer;
  config: VisualizerConfig;
  qualityProfile: QualityProfile | null;
  avgFps: number;
  onConfigChange: (newConfig: Partial<VisualizerConfig>) => void;
}

export const DebugPanel: React.FC<DebugPanelProps> = ({
  analyzer,
  config,
  qualityProfile,
  avgFps,
  onConfigChange,
}) => {
  const [metrics, setMetrics] = useState<AudioFeatures>({ ...analyzer.features });
  const [isOpen, setIsOpen] = useState(false);

  // Throttle updates to ~10 Hz to prevent React re-renders from killing performance
  useEffect(() => {
    const interval = setInterval(() => {
      setMetrics({ ...analyzer.features });
    }, 100);

    return () => clearInterval(interval);
  }, [analyzer]);

  if (!isOpen) {
    return (
      <button
        onClick={() => setIsOpen(true)}
        className="hud-btn debug-toggle-btn"
        aria-label="Abrir panel de diagnóstico"
      >
        ⚙ Telemetry & Tuning
      </button>
    );
  }

  return (
    <aside className="hud-panel debug-panel" aria-label="Panel de depuración y rendimiento">
      <div className="debug-header">
        <h3>JARVIS TELEMETRY</h3>
        <button
          onClick={() => setIsOpen(false)}
          className="hud-btn close-btn"
          aria-label="Cerrar panel de depuración"
        >
          ✕
        </button>
      </div>

      <div className="debug-section">
        <h4>ENGINE PERFORMANCE</h4>
        <div className="metric-row">
          <span>FPS:</span>
          <strong>{Math.round(avgFps)} FPS</strong>
        </div>
        <div className="metric-row">
          <span>QUALITY TIER:</span>
          <span className={`badge badge-${qualityProfile?.tier.toLowerCase()}`}>
            {qualityProfile?.tier || 'HIGH'}
          </span>
        </div>
        <div className="metric-row">
          <span>PARTICLES:</span>
          <strong>{qualityProfile?.particleCount || 0}</strong>
        </div>
        <div className="metric-row">
          <span>BLOOM:</span>
          <strong>{qualityProfile?.bloomEnabled ? (qualityProfile.bloomHalfRes ? 'Half-Res' : 'Full') : 'OFF'}</strong>
        </div>
      </div>

      <div className="debug-section">
        <h4>SPECTRAL FEATURES (LOCAL FFT)</h4>
        <div className="meter-row">
          <span>Volume</span>
          <progress value={metrics.volume} max={1} />
          <span>{(metrics.volume * 100).toFixed(0)}%</span>
        </div>
        <div className="meter-row">
          <span>Bass</span>
          <progress value={metrics.bass} max={1} />
          <span>{(metrics.bass * 100).toFixed(0)}%</span>
        </div>
        <div className="meter-row">
          <span>Mid</span>
          <progress value={metrics.mid} max={1} />
          <span>{(metrics.mid * 100).toFixed(0)}%</span>
        </div>
        <div className="meter-row">
          <span>Treble</span>
          <progress value={metrics.treble} max={1} />
          <span>{(metrics.treble * 100).toFixed(0)}%</span>
        </div>
        <div className="meter-row">
          <span>Onset (Pulse)</span>
          <progress value={metrics.onset} max={1} />
          <span>{(metrics.onset * 100).toFixed(0)}%</span>
        </div>
        <div className="meter-row">
          <span>Noise Floor</span>
          <progress value={analyzer.noiseFloor * 5} max={1} />
          <span>{(analyzer.noiseFloor * 100).toFixed(1)}%</span>
        </div>
      </div>

      <div className="debug-section">
        <h4>REAL-TIME TUNING</h4>
        <label className="slider-row">
          <span>Sensitivity ({config.sensitivity.toFixed(1)}x)</span>
          <input
            type="range"
            min="0.5"
            max="3.0"
            step="0.1"
            value={config.sensitivity}
            onChange={(e) => onConfigChange({ sensitivity: parseFloat(e.target.value) })}
          />
        </label>
        <label className="slider-row">
          <span>Noise Gate ({(config.noiseGate * 100).toFixed(0)}%)</span>
          <input
            type="range"
            min="0.0"
            max="0.1"
            step="0.005"
            value={config.noiseGate}
            onChange={(e) => onConfigChange({ noiseGate: parseFloat(e.target.value) })}
          />
        </label>
        <label className="slider-row">
          <span>Bloom Strength ({config.bloomStrength.toFixed(2)})</span>
          <input
            type="range"
            min="0.2"
            max="2.0"
            step="0.05"
            value={config.bloomStrength}
            onChange={(e) => onConfigChange({ bloomStrength: parseFloat(e.target.value) })}
          />
        </label>
        <label className="slider-row">
          <span>Smoothing ({config.smoothing.toFixed(2)})</span>
          <input
            type="range"
            min="0.2"
            max="0.95"
            step="0.05"
            value={config.smoothing}
            onChange={(e) => onConfigChange({ smoothing: parseFloat(e.target.value) })}
          />
        </label>
      </div>
    </aside>
  );
};
