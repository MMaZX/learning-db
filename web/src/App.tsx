import { useState, useRef, useEffect, useCallback } from 'react';
import { AudioAnalyzer } from './audio/AudioAnalyzer';
import { AudioVisualizer } from './components/AudioVisualizer';
import { MicrophoneConsent } from './components/MicrophoneConsent';
import { ConversationPanel, ChatMessage } from './components/ConversationPanel';
import { DebugPanel } from './components/DebugPanel';
import { VoiceModeConsent } from './components/VoiceModeConsent';
import { DEFAULT_VISUALIZER_CONFIG, MicState, PrivacyMode, VisualizerConfig } from './types/audio';
import { QualityProfile } from './three/QualityController';
import './styles.css';

export function App() {
  const analyzerRef = useRef<AudioAnalyzer | null>(null);
  if (!analyzerRef.current) {
    analyzerRef.current = new AudioAnalyzer();
  }
  const analyzer = analyzerRef.current;

  const [micState, setMicState] = useState<MicState>('MIC_OFF');
  const [privacyMode, setPrivacyMode] = useState<PrivacyMode>('strict-private');
  const [isConsentModalOpen, setIsConsentModalOpen] = useState(false);
  const [qualityProfile, setQualityProfile] = useState<QualityProfile | null>(null);
  const [avgFps, setAvgFps] = useState(60);
  const [config, setConfig] = useState<VisualizerConfig>(DEFAULT_VISUALIZER_CONFIG);

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [conversationStatus, setConversationStatus] = useState<
    'IDLE' | 'ROUTING' | 'STREAMING' | 'TOOL_RUNNING' | 'SPEAKING' | 'ERROR'
  >('IDLE');
  const [isCapturingVoice, setIsCapturingVoice] = useState(false);
  const [isConsoleOpen, setIsConsoleOpen] = useState(false);

  useEffect(() => {
    analyzer.onStateChange((newState) => {
      setMicState(newState);
    });

    return () => {
      analyzer.destroy();
    };
  }, [analyzer]);

  const handleActivateMic = async () => {
    try {
      await analyzer.startMicrophone();
    } catch (err) {
      console.error('Error activating microphone:', err);
    }
  };

  const handleDeactivateMic = async () => {
    await analyzer.destroy();
    setMicState('MIC_OFF');
  };

  const handleModeSwitchRequest = (mode: PrivacyMode) => {
    if (mode === 'voice-experimental') {
      setIsConsentModalOpen(true);
    } else {
      setPrivacyMode('strict-private');
    }
  };

  const handleAcceptVoiceConsent = () => {
    setPrivacyMode('voice-experimental');
    setIsConsentModalOpen(false);
  };

  const handleDeclineVoiceConsent = () => {
    setPrivacyMode('strict-private');
    setIsConsentModalOpen(false);
  };

  const handleConfigChange = (newConfig: Partial<VisualizerConfig>) => {
    setConfig((prev) => {
      const updated = { ...prev, ...newConfig };
      analyzer.updateConfig(updated);
      return updated;
    });
  };

  const handleQualityChange = useCallback((profile: QualityProfile, fps: number) => {
    setQualityProfile(profile);
    setAvgFps(fps);
  }, []);

  const handleSendMessage = async (text: string) => {
    const userMsg: ChatMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: text,
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMsg]);
    setConversationStatus('ROUTING');

    // Attempt to call BFF /api/conversations/turns if running, otherwise provide local confirmation
    try {
      setConversationStatus('STREAMING');
      const assistantId = `asst-${Date.now()}`;
      
      const response = await fetch('/api/conversations/current/turns', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
      }).catch(() => null);

      if (response && response.ok) {
        // Stream SSE
        const reader = response.body?.getReader();
        const decoder = new TextDecoder();
        let accumulated = '';

        if (reader) {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            accumulated += decoder.decode(value);
            setMessages((prev) => {
              const withoutCurrent = prev.filter((m) => m.id !== assistantId);
              return [
                ...withoutCurrent,
                {
                  id: assistantId,
                  role: 'assistant',
                  content: accumulated,
                  timestamp: Date.now(),
                  isStreaming: true,
                },
              ];
            });
          }
        }
      } else {
        // Local interactive feedback when BFF is in setup
        const reply: ChatMessage = {
          id: assistantId,
          role: 'assistant',
          content: `Entendido, Señor. He recibido su instrucción: "${text}". El organismo visual y los analizadores de voz operan en modo ${privacyMode}.`,
          timestamp: Date.now(),
          isStreaming: false,
        };
        setMessages((prev) => [...prev, reply]);
      }
    } catch {
      setConversationStatus('ERROR');
    } finally {
      setConversationStatus('IDLE');
    }
  };

  const handleStartPushToTalk = () => {
    if (privacyMode !== 'voice-experimental') return;
    setIsCapturingVoice(true);
  };

  const handleStopPushToTalk = () => {
    if (!isCapturingVoice) return;
    setIsCapturingVoice(false);
  };

  return (
    <main className="app-layout">
      <div className="stage">
        <AudioVisualizer
          analyzer={analyzer}
          onQualityChange={handleQualityChange}
        />

        <header className="hud-header">
          <div className="brand-title">
            <h1>JARVIS</h1>
            <span className="version-tag">v3.8.40</span>
          </div>

          <div className="header-actions">
            <MicrophoneConsent
              micState={micState}
              privacyMode={privacyMode}
              onActivate={handleActivateMic}
              onDeactivate={handleDeactivateMic}
            />
            <button
              className="hud-btn"
              onClick={() => setIsConsoleOpen((open) => !open)}
              aria-expanded={isConsoleOpen}
              aria-controls="console-drawer"
            >
              {isConsoleOpen ? 'Ocultar consola' : 'Consola'}
            </button>
          </div>
        </header>

        <div className="hud-body">
          <DebugPanel
            analyzer={analyzer}
            config={config}
            qualityProfile={qualityProfile}
            avgFps={avgFps}
            onConfigChange={handleConfigChange}
          />
        </div>
      </div>

      <ConversationPanel
        isOpen={isConsoleOpen}
        onClose={() => setIsConsoleOpen(false)}
        privacyMode={privacyMode}
        onModeSwitchRequest={handleModeSwitchRequest}
        onSendMessage={handleSendMessage}
        messages={messages}
        status={conversationStatus}
        isCapturingVoice={isCapturingVoice}
        onStartPushToTalk={handleStartPushToTalk}
        onStopPushToTalk={handleStopPushToTalk}
      />

      {/* Experimental Voice Mode Consent Modal */}
      <VoiceModeConsent
        isOpen={isConsentModalOpen}
        onAccept={handleAcceptVoiceConsent}
        onDecline={handleDeclineVoiceConsent}
      />
    </main>
  );
}
export default App;
