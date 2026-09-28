import { describe, it, expect, afterEach, vi } from 'vitest';
import { SseParser } from '../api/sse';
import { KokoroRvcClient } from '../../server/clients/kokoroRvcClient';
import { markdownToSpeech, SpeechChunker } from '../api/speechText';
import { SpeechQueue } from '../api/speechQueue';

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

describe('KokoroRvcClient', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  const client = new KokoroRvcClient({ baseUrl: 'http://127.0.0.1:17494/' });

  it('requests the Eugeo voice from /generate/stream', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Uint8Array([1, 2, 3]), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    const wav = await client.synthesize('Hola');

    expect(wav.byteLength).toBe(3);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('http://127.0.0.1:17494/generate/stream');
    expect(JSON.parse(init.body)).toEqual({ text: 'Hola' });
  });

  it('maps upstream failures to a 502 KokoroRvcError', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('boom', { status: 500 })));
    await expect(client.synthesize('Hola')).rejects.toMatchObject({ status: 502 });
  });

  it('refuses to synthesize without a base URL', async () => {
    const unconfigured = new KokoroRvcClient({ baseUrl: '' });
    expect(unconfigured.isConfigured).toBe(false);
    await expect(unconfigured.synthesize('Hola')).rejects.toMatchObject({ status: 503 });
  });
});

describe('markdownToSpeech', () => {
  it('reads bold labels, italics and ISO dates naturally', () => {
    expect(markdownToSpeech('**Concepto:** *Presentación de unidades vendidas / ventas netas*  ')).toBe(
      'Concepto: Presentación de unidades vendidas o ventas netas'
    );
    expect(markdownToSpeech('**Fecha de creación:** 2026‑09‑22 T15:03:54.165Z  ')).toBe(
      'Fecha de creación: 22 de septiembre de 2026 a las 15:03'
    );
  });

  it('spells out emails, dotted identifiers and arrows', () => {
    expect(markdownToSpeech('**Decidido por:** setup200yan@gmail.com')).toBe(
      'Decidido por: setup200yan arroba gmail punto com'
    );
    expect(markdownToSpeech('(detallefactura_venta.codAlmacen → almacen.nombre)')).toBe(
      '(detallefactura venta punto codAlmacen a almacen punto nombre)'
    );
  });

  it('drops markdown structure and unreadable symbols', () => {
    expect(markdownToSpeech('## Resumen')).toBe('Resumen');
    expect(markdownToSpeech('- `obtener_esquema_tabla` sirve ✅')).toBe('obtener esquema tabla sirve');
    expect(markdownToSpeech('| Almacén | Total |')).toBe('Almacén, Total');
    expect(markdownToSpeech('|---|---:|')).toBe('');
    expect(markdownToSpeech('Ver [la guía](https://x.io/doc)')).toBe('Ver la guía');
    expect(markdownToSpeech('***')).toBe('');
  });
});

describe('SpeechChunker', () => {
  const answer = [
    'Habiendo consultado la memoria de JARVIS, el último conocimiento validado que se ha registrado es:',
    '',
    '**Concepto:** *Presentación de unidades vendidas / ventas netas*  ',
    '**Contexto:** *Ventas*  ',
    '**Fecha de creación:** 2026‑09‑22 T15:03:54.165Z  ',
    '**Decidido por:** setup200yan@gmail.com  ',
    '',
    'El usuario solicitó que la respuesta siempre se arme como una tabla desglosada por almacén. Esto permite ver la totalización por almacén.',
    '',
    '```sql',
    'SELECT * FROM almacen;',
    '```',
    'Listo.',
  ].join('\n');

  function streamed(text: string, step: number): string[] {
    const chunker = new SpeechChunker();
    const out: string[] = [];
    for (let i = 0; i < text.length; i += step) out.push(...chunker.push(text.slice(i, i + step)));
    return [...out, ...chunker.flush()];
  }

  it('says the same words however the text is streamed', () => {
    const whole = streamed(answer, answer.length).join(' ');
    expect(streamed(answer, 1).join(' ')).toBe(whole);
    expect(streamed(answer, 7).join(' ')).toBe(whole);
  });

  it('never loses content, never reads code and keeps chunks short', () => {
    const chunks = streamed(answer, 3);
    const spoken = chunks.join(' ');
    expect(spoken).toContain('Concepto: Presentación de unidades vendidas o ventas netas');
    expect(spoken).toContain('22 de septiembre de 2026 a las 15:03');
    expect(spoken).toContain('setup200yan arroba gmail punto com');
    expect(spoken).toContain('Esto permite ver la totalización por almacén.');
    expect(spoken).toContain('Listo.');
    expect(spoken).not.toMatch(/SELECT|\*|`/);
    expect(chunks.every((c) => c.length <= 160)).toBe(true);
  });

  it('releases the first sentence before the answer is complete', () => {
    const chunker = new SpeechChunker();
    expect(chunker.push('Claro, Señor. Estoy revis')).toEqual(['Claro, Señor.']);
    expect(chunker.push('ando la base')).toEqual([]);
    expect(chunker.flush()).toEqual(['Estoy revisando la base']);
  });

  it('does not cut times, decimals or domains', () => {
    const chunker = new SpeechChunker();
    expect(chunker.push('A las 15:03:54.165 en gmail.com se')).toEqual([]);
  });
});

describe('SpeechQueue', () => {
  it('plays in order and skips a chunk that fails to synthesize', async () => {
    const played: string[] = [];
    const errors: string[] = [];
    const queue = new SpeechQueue({
      synthesize: async (t) => {
        if (t === 'mala') throw new Error('TTS HTTP 502');
        await new Promise((r) => setTimeout(r, t.length));
        return new TextEncoder().encode(t).buffer as ArrayBuffer;
      },
      play: async (a) => {
        played.push(new TextDecoder().decode(a));
      },
      onError: (_e, t) => errors.push(t),
    });
    queue.enqueue(['uno largo', 'mala']);
    queue.enqueue(['dos']);
    expect(queue.busy).toBe(true);
    await queue.drain();
    expect(played).toEqual(['uno largo', 'dos']);
    expect(errors).toEqual(['mala']);
    expect(queue.busy).toBe(false);
  });

  it('stops after cancel', async () => {
    const played: string[] = [];
    const queue = new SpeechQueue({
      synthesize: async (t) => new TextEncoder().encode(t).buffer as ArrayBuffer,
      play: async (a) => {
        played.push(new TextDecoder().decode(a));
      },
      onError: () => {},
    });
    queue.enqueue(['a', 'b']);
    queue.cancel();
    queue.enqueue(['c']);
    await queue.drain();
    expect(played).toEqual([]);
  });
});
