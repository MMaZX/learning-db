import React from 'react';
import { MicState, PrivacyMode } from '../types/audio';

interface MicrophoneConsentProps {
  micState: MicState;
  privacyMode: PrivacyMode;
  onActivate: () => Promise<void>;
  onDeactivate: () => Promise<void>;
}

export const MicrophoneConsent: React.FC<MicrophoneConsentProps> = ({
  micState,
  privacyMode,
  onActivate,
  onDeactivate,
}) => {
  const isMicRunning = micState === 'CALIBRATING' || micState === 'WAITING_SILENCE' || micState === 'USER_SPEAKING';

  const getStateLabel = () => {
    switch (micState) {
      case 'REQUESTING_PERMISSION':
        return 'Solicitando acceso al micrófono...';
      case 'CALIBRATING':
        return 'Calibrando ruido ambiente (750 ms)...';
      case 'WAITING_SILENCE':
        return 'En espera... (Silencio detectado)';
      case 'USER_SPEAKING':
        return 'Escuchando voz (Análisis FFT activo)';
      case 'MIC_ERROR':
        return 'Error de acceso al micrófono';
      case 'MIC_ENDED':
        return 'Dispositivo desconectado';
      default:
        return 'Micrófono inactivo';
    }
  };

  return (
    <div className="mic-consent-container">
      <div className="privacy-pill">
        <span className="dot dot-green" />
        <strong>{privacyMode === 'strict-private' ? 'STRICT-PRIVATE' : 'VOICE-EXPERIMENTAL'}</strong>
        <span className="privacy-desc">
          {privacyMode === 'strict-private'
            ? 'Análisis FFT 100% local — 0 bytes de audio salen del navegador'
            : 'Push-to-Talk acotado transitorio'}
        </span>
      </div>

      <div className="mic-actions">
        {!isMicRunning ? (
          <button
            onClick={onActivate}
            className="hud-btn primary-btn mic-btn"
            disabled={micState === 'REQUESTING_PERMISSION'}
          >
            {micState === 'REQUESTING_PERMISSION' ? '⌛ Solicitando...' : '🎙️ Activar Micrófono'}
          </button>
        ) : (
          <button onClick={onDeactivate} className="hud-btn danger-btn mic-btn">
            ⏹ Detener Micrófono
          </button>
        )}

        <div className="mic-status-indicator" aria-live="polite">
          <span className={`status-led ${isMicRunning ? 'led-active' : 'led-idle'}`} />
          <span className="status-text">{getStateLabel()}</span>
        </div>
      </div>
    </div>
  );
};
