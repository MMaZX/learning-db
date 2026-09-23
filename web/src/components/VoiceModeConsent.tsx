import React from 'react';

interface VoiceModeConsentProps {
  isOpen: boolean;
  onAccept: () => void;
  onDecline: () => void;
}

export const VoiceModeConsent: React.FC<VoiceModeConsentProps> = ({
  isOpen,
  onAccept,
  onDecline,
}) => {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="consent-title">
      <div className="hud-panel consent-modal">
        <div className="modal-header">
          <span className="warning-icon">⚠️</span>
          <h3 id="consent-title">MODO CONVERSACIÓN POR VOZ EXPERIMENTAL</h3>
        </div>

        <div className="modal-body">
          <p className="highlight-text">
            <strong>Aviso de Privacidad y Excepción Intencional:</strong>
          </p>
          <p>
            El modo estándar de Jarvis es <strong>Strict-Private</strong> (el audio del micrófono se procesa exclusivamente dentro de tu navegador para alimentar el visualizador 3D y <strong>nunca sale de tu equipo</strong>).
          </p>
          <p>
            Al activar el <strong>Modo Voz Experimental</strong>:
          </p>
          <ul>
            <li>Solo se grabará un fragmento acotado (máximo 15 segundos) al pulsar el botón <em>Push-to-Talk</em>.</li>
            <li>El audio se enviará de forma transitoria en memoria a través del backend BFF hacia el servicio STT verificado de OmniRoute.</li>
            <li>El audio <strong>no se almacena ni se registra en disco ni en base de datos</strong>.</li>
            <li>La transcripción devuelta se mostrará en pantalla para que la revises o edites antes de enviarla al agente.</li>
          </ul>
        </div>

        <div className="modal-actions">
          <button onClick={onDecline} className="hud-btn secondary-btn">
            Cancelar (Permanecer en Strict-Private)
          </button>
          <button onClick={onAccept} className="hud-btn primary-btn warning-accept-btn">
            Entendido, Habilitar Push-to-Talk
          </button>
        </div>
      </div>
    </div>
  );
};
