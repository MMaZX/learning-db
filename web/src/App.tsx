import { useState, useRef, useEffect, useCallback } from 'react';
import { AudioAnalyzer } from './audio/AudioAnalyzer';
import { AudioVisualizer } from './components/AudioVisualizer';
import { MicrophoneConsent } from './components/MicrophoneConsent';
import { ConversationPanel, ChatMessage } from './components/ConversationPanel';
import { DebugPanel } from './components/DebugPanel';
import { VoiceModeConsent } from './components/VoiceModeConsent';
import { DEFAULT_VISUALIZER_CONFIG, MicState, PrivacyMode, VisualizerConfig } from './types/audio';
import { QualityProfile } from './three/QualityController';
import { SseParser } from './api/sse';
import { splitForSpeech } from './api/speech';
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
  const [serverAlert, setServerAlert] = useState(false);
  const [ttsAvailable, setTtsAvailable] = useState(false);
  const [isJarvisSpeaking, setIsJarvisSpeaking] = useState(false);
  const serverAlertTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const micActive = micState === 'CALIBRATING' || micState === 'WAITING_SILENCE' || micState === 'USER_SPEAKING';
  // Jarvis speaking also brings the spectrum fully online, even with the mic off.
  const spectrumActive = micActive || isJarvisSpeaking;
  const spectrumAlert = micState === 'MIC_ERROR' || micState === 'MIC_ENDED' || serverAlert;

  const flagServerAlert = useCallback(() => {
    setServerAlert(true);
    if (serverAlertTimer.current) clearTimeout(serverAlertTimer.current);
    serverAlertTimer.current = setTimeout(() => setServerAlert(false), 4000);
  }, []);

  useEffect(() => () => {
    if (serverAlertTimer.current) clearTimeout(serverAlertTimer.current);
  }, []);

  useEffect(() => {
    fetch('/api/capabilities')
      .then((r) => (r.ok ? r.json() : null))
      .then((caps) => setTtsAvailable(!!caps?.tts))
      .catch(() => setTtsAvailable(false));
  }, []);

  const speechRunRef = useRef(0);

  const speak = useCallback(
    async (text: string) => {
      const chunks = splitForSpeech(text);
      if (chunks.length === 0) return;

      const run = ++speechRunRef.current;
      const synthesize = async (chunk: string): Promise<ArrayBuffer> => {
        const response = await fetch('/api/tts', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ text: chunk }),
        });
        if (!response.ok) throw new Error(`TTS HTTP ${response.status}`);
        return response.arrayBuffer();
      };

      setConversationStatus('SPEAKING');
      setIsJarvisSpeaking(true);
      try {
        // Synthesize the next chunk while the current one is playing.
        let pending = synthesize(chunks[0]);
        for (let i = 0; i < chunks.length; i++) {
          const audio = await pending;
          if (speechRunRef.current !== run) return;
          if (i + 1 < chunks.length) pending = synthesize(chunks[i + 1]);
          await analyzer.playSpeech(audio);
          if (speechRunRef.current !== run) return;
        }
      } catch (err) {
        console.error('Error synthesizing speech:', err);
        flagServerAlert();
      } finally {
        if (speechRunRef.current === run) {
          setIsJarvisSpeaking(false);
          setConversationStatus('IDLE');
        }
      }
    },
    [analyzer, flagServerAlert]
  );

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

    // A new question interrupts whatever Jarvis is still saying.
    speechRunRef.current++;
    analyzer.stopSpeech();
    setIsJarvisSpeaking(false);
    let spokenAnswer = '';

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
        const parser = new SseParser();
        let accumulated = '';

        if (reader) {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            for (const event of parser.push(decoder.decode(value, { stream: true }))) {
              if (event.type === 'assistant.delta' && typeof event.data.fullText === 'string') {
                accumulated = event.data.fullText;
              } else if (event.type === 'assistant.completed' && typeof event.data.finalAnswer === 'string') {
                accumulated = event.data.finalAnswer;
                spokenAnswer = event.data.finalAnswer;
              } else if (event.type === 'error') {
                flagServerAlert();
              }
            }
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
        if (response && !response.ok) flagServerAlert();
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
      flagServerAlert();
      setConversationStatus('ERROR');
    } finally {
      setConversationStatus('IDLE');
    }

    if (ttsAvailable && spokenAnswer.trim()) {
      await speak(spokenAnswer);
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
          micActive={spectrumActive}
          alert={spectrumAlert}
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
