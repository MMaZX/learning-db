import { describe, it, expect, afterEach, vi } from 'vitest';
import { SseParser } from '../api/sse';
import { VoiceboxClient } from '../../server/clients/voiceboxClient';

function envelope(type: string, data: unknown): string {
  return `event: ${type}\ndata: ${JSON.stringify({ type, sequence: 1, conversationId: 'c', turnId: 't', data })}\n\n`;
}

describe('SseParser', () => {
  it('unwraps the BFF envelope and returns typed events', () => {
    const parser = new SseParser();
    const events = parser.push(envelope('assistant.delta', { delta: 'Ho', fullText: 'Ho' }));
    expect(events).toEqual([{ type: 'assistant.delta', data: { delta: 'Ho', fullText: 'Ho' } }]);
  });

  it('buffers events split across chunks', () => {
    const parser = new SseParser();
    const raw = envelope('assistant.completed', { finalAnswer: 'Hola, Señor.' });
    const cut = Math.floor(raw.length / 2);

    expect(parser.push(raw.slice(0, cut))).toEqual([]);
    const events = parser.push(raw.slice(cut));
    expect(events[0].data.finalAnswer).toBe('Hola, Señor.');
  });

  it('ignores heartbeats and malformed payloads', () => {
    const parser = new SseParser();
    const events = parser.push(': heartbeat\n\nevent: error\ndata: {not json\n\n' + envelope('done', {}));
    expect(events).toEqual([{ type: 'done', data: {} }]);
  });
});

describe('VoiceboxClient', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const client = new VoiceboxClient({
    baseUrl: 'http://127.0.0.1:17493/',
    profileId: 'profile-1',
    language: 'es',
    engine: 'qwen',
    modelSize: '0.6B',
  });

  it('requests Spanish Qwen3 0.6B from /generate/stream', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Uint8Array([1, 2, 3]), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const wav = await client.synthesize('Hola');

    expect(wav.byteLength).toBe(3);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('http://127.0.0.1:17493/generate/stream');
    expect(JSON.parse(init.body)).toEqual({
      profile_id: 'profile-1',
      text: 'Hola',
      language: 'es',
      engine: 'qwen',
      model_size: '0.6B',
    });
  });

  it('maps upstream failures to a 502 VoiceboxError', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('boom', { status: 500 })));
    await expect(client.synthesize('Hola')).rejects.toMatchObject({ status: 502 });
  });

  it('refuses to synthesize without a profile', async () => {
    const unconfigured = new VoiceboxClient({ baseUrl: 'http://x', profileId: '', language: 'es', engine: 'qwen', modelSize: '0.6B' });
    expect(unconfigured.isConfigured).toBe(false);
    await expect(unconfigured.synthesize('Hola')).rejects.toMatchObject({ status: 503 });
  });
});

describe('splitForSpeech', () => {
  it('keeps the first chunk short and groups later sentences', async () => {
    const { splitForSpeech } = await import('../api/speech');
    const text =
      'Señor, la consulta terminó. Encontré ciento veintitrés registros activos en la tabla de clientes. ' +
      'La mayoría fueron creados este mes. El resto corresponde a cuentas antiguas que siguen vigentes. ¿Desea el detalle?';
    const chunks = splitForSpeech(text);

    expect(chunks.length).toBeGreaterThan(1);
    expect(chunks[0].length).toBeLessThanOrEqual(120);
    expect(chunks.every((c) => c.length <= 220)).toBe(true);
    expect(chunks.join(' ')).toBe(text);
  });

  it('strips markdown and code blocks and splits very long sentences', async () => {
    const { splitForSpeech } = await import('../api/speech');
    expect(splitForSpeech('**Listo**. ```sql\nSELECT 1;\n``` Hecho.')).toEqual(['Listo. Hecho.']);
    const long = Array.from({ length: 80 }, (_, i) => `palabra${i}`).join(' ');
    expect(splitForSpeech(long).every((c) => c.length <= 220)).toBe(true);
    expect(splitForSpeech('   ')).toEqual([]);
  });
});
