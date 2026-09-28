import { describe, it, expect } from 'vitest';
import { GpuTurn } from '../../server/orchestration/gpuTurn';

function setup(failing?: 'speech' | 'chat') {
  const calls: string[] = [];
  const logs: string[] = [];
  const turn = new GpuTurn(
    {
      releaseSpeech: async () => {
        calls.push('releaseSpeech');
        if (failing === 'speech') throw new Error('voicebox caído');
      },
      releaseChat: async () => {
        calls.push('releaseChat');
        if (failing === 'chat') throw new Error('ollama caído');
      },
    },
    (m) => logs.push(m)
  );
  return { turn, calls, logs };
}

describe('GpuTurn', () => {
  it('releases the other model when the owner changes', async () => {
    const { turn, calls } = setup();
    await turn.forChat();
    await turn.forSpeech();
    await turn.forChat();
    expect(calls).toEqual(['releaseSpeech', 'releaseChat', 'releaseSpeech']);
  });

  it('does nothing while the same owner keeps the GPU', async () => {
    const { turn, calls } = setup();
    await turn.forSpeech();
    await turn.forSpeech();
    await turn.forSpeech();
    expect(calls).toEqual(['releaseChat']);
  });

  it('serializes overlapping requests', async () => {
    const { turn, calls } = setup();
    await Promise.all([turn.forChat(), turn.forSpeech(), turn.forSpeech()]);
    expect(calls).toEqual(['releaseSpeech', 'releaseChat']);
  });

  it('logs a failed release and still hands over the GPU', async () => {
    const { turn, calls, logs } = setup('speech');
    await expect(turn.forChat()).resolves.toBeUndefined();
    await turn.forChat();
    expect(calls).toEqual(['releaseSpeech']);
    expect(logs[0]).toContain('voicebox caído');
  });
});
