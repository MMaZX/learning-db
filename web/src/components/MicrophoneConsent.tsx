import React from 'react';
import { MicState, PrivacyMode } from '../types/audio';

export interface MicrophoneConsentProps {
  micState: MicState;
  privacyMode?: PrivacyMode;
  onActivate: () => Promise<void>;
  onDeactivate: () => Promise<void>;
}

export interface PrivacyBadgeProps {
  privacyMode: PrivacyMode;
}

export const PrivacyBadge: React.FC<PrivacyBadgeProps> = ({ privacyMode }) => {
  return (
    <div className="privacy-pill">
      <span className="dot dot-green" />
      <strong>{privacyMode === 'strict-private' ? 'STRICT-PRIVATE' : 'VOICE-EXPERIMENTAL'}</strong>
      <span className="privacy-desc">
        {privacyMode === 'strict-private'
          ? 'Análisis FFT 100% local — 0 bytes de audio salen del navegador'
          : 'Push-to-Talk acotado transitorio'}
      </span>
    </div>
  );
};

export const MicrophoneConsent: React.FC<MicrophoneConsentProps> = ({
  micState,
  onActivate,
  onDeactivate,
}) => {
  const isMicRunning =
    micState === 'CALIBRATING' || micState === 'WAITING_SILENCE' || micState === 'USER_SPEAKING';
  const isRequesting = micState === 'REQUESTING_PERMISSION';
  const isError = micState === 'MIC_ERROR' || micState === 'MIC_ENDED';

  const getStateLabel = () => {
    switch (micState) {
      case 'REQUESTING_PERMISSION':
        return 'Solicitando acceso...';
      case 'CALIBRATING':
        return 'Calibrando (750ms)...';
      case 'WAITING_SILENCE':
        return 'En espera (silencio)';
      case 'USER_SPEAKING':
        return 'Escuchando voz';
      case 'MIC_ERROR':
        return 'Error de micrófono';
      case 'MIC_ENDED':
        return 'Dispositivo desconectado';
      default:
        return 'Micrófono inactivo';
    }
  };

  return (
    <nav className="mic-glass-dock" aria-label="Control de micrófono">
      {!isMicRunning ? (
        <button
          type="button"
          onClick={onActivate}
          className="mic-dock-btn"
          disabled={isRequesting}
          aria-label={isRequesting ? 'Solicitando acceso al micrófono' : 'Activar micrófono'}
        >
          {isRequesting ? (
            <svg
              className="mic-dock-spin"
              width="13"
              height="13"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M21 12a9 9 0 1 1-6.219-8.56" />
            </svg>
          ) : (
            <svg
              width="13"
              height="13"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <path d="M12 2a3 3 0 0 0-3 3v7a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3Z" />
              <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
              <line x1="12" y1="19" x2="12" y2="22" />
            </svg>
          )}
          <span className="mic-dock-btn-label">
            {isRequesting ? 'Solicitando...' : 'Activar micrófono'}
          </span>
        </button>
      ) : (
        <button
          type="button"
          onClick={onDeactivate}
          className="mic-dock-btn is-running"
          aria-label="Detener micrófono"
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <rect x="6" y="6" width="12" height="12" rx="2" />
          </svg>
          <span className="mic-dock-btn-label">Detener micrófono</span>
        </button>
      )}

      <div className="mic-dock-divider" aria-hidden="true" />

      <div className="mic-dock-status" aria-live="polite">
        <span
          className={`mic-dock-led ${
            isMicRunning ? 'is-active' : isError ? 'is-error' : 'is-idle'
          }`}
          aria-hidden="true"
        />
        <span className="mic-dock-text">{getStateLabel()}</span>
      </div>
    </nav>
  );
};

export const MicrophoneControl = MicrophoneConsent;
