import '@testing-library/jest-dom';
import { vi } from 'vitest';

// Web Audio API mock for test environments
class AudioContextMock {
  state = 'running';
  sampleRate = 44100;
  resume = vi.fn().mockResolvedValue(undefined);
  close = vi.fn().mockResolvedValue(undefined);
  createMediaStreamSource = vi.fn().mockReturnValue({
    connect: vi.fn(),
    disconnect: vi.fn(),
  });
  createAnalyser = vi.fn().mockReturnValue({
    fftSize: 2048,
    frequencyBinCount: 1024,
    smoothingTimeConstant: 0.8,
    getByteFrequencyData: vi.fn(),
    getByteTimeDomainData: vi.fn(),
    connect: vi.fn(),
    disconnect: vi.fn(),
  });
}

(globalThis as unknown as { AudioContext: unknown }).AudioContext = AudioContextMock;
(globalThis as unknown as { webkitAudioContext: unknown }).webkitAudioContext = AudioContextMock;

// ResizeObserver mock
class ResizeObserverMock {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

(globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = ResizeObserverMock;
