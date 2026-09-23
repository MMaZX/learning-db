import React, { useState } from 'react';
import { PrivacyMode } from '../types/audio';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool';
  content: string;
  timestamp: number;
  toolName?: string;
  isStreaming?: boolean;
  isError?: boolean;
}

interface ConversationPanelProps {
  privacyMode: PrivacyMode;
  onModeSwitchRequest: (mode: PrivacyMode) => void;
  onSendMessage: (text: string) => void;
  messages: ChatMessage[];
  status: 'IDLE' | 'ROUTING' | 'STREAMING' | 'TOOL_RUNNING' | 'SPEAKING' | 'ERROR';
  isCapturingVoice?: boolean;
  onStartPushToTalk?: () => void;
  onStopPushToTalk?: () => void;
  isOpen: boolean;
  onClose: () => void;
}

export const ConversationPanel: React.FC<ConversationPanelProps> = ({
  privacyMode,
  onModeSwitchRequest,
  onSendMessage,
  messages,
  status,
  isCapturingVoice,
  onStartPushToTalk,
  onStopPushToTalk,
  isOpen,
  onClose,
}) => {
  const [inputText, setInputText] = useState('');
  const [treatment, setTreatment] = useState<'Señor' | 'Señora' | 'Señorita'>('Señor');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputText.trim() || status !== 'IDLE') return;
    onSendMessage(inputText.trim());
    setInputText('');
  };

  return (
    <aside
      id="console-drawer"
      className={`console-drawer ${isOpen ? 'is-open' : ''}`}
      aria-hidden={!isOpen}
      inert={!isOpen}
    >
    <section className="conversation-panel" aria-label="Terminal de conversación con Jarvis">
      <div className="panel-header">
        <div className="header-left">
          <span className="hud-label">Consola</span>
          <span className={`status-badge status-${status.toLowerCase()}`}>
            {status}
          </span>
        </div>

        <div className="header-controls">
          <select
            value={treatment}
            onChange={(e) => setTreatment(e.target.value as 'Señor' | 'Señora' | 'Señorita')}
            className="hud-select"
            aria-label="Trato protocolario de Jarvis"
          >
            <option value="Señor">Señor</option>
            <option value="Señora">Señora</option>
            <option value="Señorita">Señorita</option>
          </select>

          <button
            onClick={() => onModeSwitchRequest(privacyMode === 'strict-private' ? 'voice-experimental' : 'strict-private')}
            className={`hud-btn mode-switch-btn ${privacyMode === 'voice-experimental' ? 'mode-experimental' : ''}`}
            title="Cambiar entre modo privado y modo experimental"
          >
            {privacyMode === 'strict-private' ? 'Privado' : 'Voz exp.'}
          </button>

          <button onClick={onClose} className="hud-btn close-btn" aria-label="Cerrar consola">
            ✕
          </button>
        </div>
      </div>

      <div className="messages-stream" role="log" aria-live="polite">
        {messages.length === 0 ? (
          <div className="empty-state">
            <p>Sistemas en línea. Visualizador 3D reactivo al micrófono activo.</p>
            <p className="subtext">Escriba su consulta o active el micrófono para interactuar.</p>
          </div>
        ) : (
          messages.map((msg) => (
            <div
              key={msg.id}
              className={`message-bubble message-${msg.role}${msg.isError ? ' message-error' : ''}`}
              role={msg.isError ? 'alert' : undefined}
            >
              <div className="message-meta">
                <span className="sender-tag">
                  {msg.role === 'user' ? 'USUARIO' : msg.role === 'assistant' ? 'JARVIS' : `TOOL [${msg.toolName}]`}
                </span>
                <span className="timestamp">
                  {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                </span>
              </div>
              <div className="message-body">
                {msg.content}
                {msg.isStreaming && <span className="cursor-blink">▋</span>}
              </div>
            </div>
          ))
        )}
      </div>

      <div className="panel-footer">
        {privacyMode === 'voice-experimental' && (
          <div className="push-to-talk-bar">
            <button
              onMouseDown={onStartPushToTalk}
              onMouseUp={onStopPushToTalk}
              onTouchStart={onStartPushToTalk}
              onTouchEnd={onStopPushToTalk}
              className={`hud-btn ptt-btn ${isCapturingVoice ? 'ptt-capturing' : ''}`}
            >
              {isCapturingVoice ? 'Grabando… suelte para transcribir' : 'Mantenga presionado para hablar'}
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit} className="input-form">
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            placeholder="Escriba su consulta a Jarvis..."
            className="hud-input"
            disabled={status !== 'IDLE'}
          />
          <button
            type="submit"
            className="hud-btn send-btn"
            disabled={!inputText.trim() || status !== 'IDLE'}
          >
            Enviar ↵
          </button>
        </form>
      </div>
    </section>
    </aside>
  );
};
