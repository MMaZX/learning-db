/**
 * Turns the model's markdown into text a TTS engine can read, and cuts a streamed
 * answer into speakable chunks as soon as each sentence is complete, so Jarvis
 * starts talking while the model is still writing.
 */

const MONTHS = [
  'enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio',
  'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre',
];

// Non-breaking and typographic dashes show up in model output (e.g. 2026‑09‑22).
const DASH = '[-‐‑‒–—]';
const DATE = `(\\d{4})${DASH}(\\d{2})${DASH}(\\d{2})`;
const DATE_TIME_RE = new RegExp(`${DATE}(?:\\s*T\\s*|\\s+)(\\d{2}):(\\d{2})(?::\\d{2}(?:\\.\\d+)?)?Z?`, 'g');
const DATE_RE = new RegExp(DATE, 'g');

function spokenDate(y: string, m: string, d: string): string {
  const month = MONTHS[parseInt(m, 10) - 1];
  return month ? `${parseInt(d, 10)} de ${month} de ${y}` : `${d} ${m} ${y}`;
}

/** Converts one line (or sentence) of markdown into plain, readable Spanish text. */
export function markdownToSpeech(line: string): string {
  let text = line;

  // Structural lines that carry nothing to read.
  if (/^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)*\|?\s*$/.test(text)) return '';
  if (/^\s*([-*_]\s*){3,}$/.test(text)) return '';

  text = text
    .replace(/^\s*#{1,6}\s+/, '')
    .replace(/^\s*>+\s?/, '')
    .replace(/^\s*(?:[-*+•]|\d+[.)])\s+/, '');

  if (/^\s*\|/.test(text)) {
    text = text
      .split('|')
      .map((cell) => cell.trim())
      .filter(Boolean)
      .join(', ');
  }

  text = text
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/https?:\/\/\S+/g, 'un enlace')
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*|__|~~/g, '')
    .replace(/\*/g, '')
    .replace(/(?<=[\p{L}\p{N}])_(?=[\p{L}\p{N}])/gu, ' ')
    .replace(/_/g, '')
    .replace(DATE_TIME_RE, (_m, y, mo, d, hh, mm) => `${spokenDate(y, mo, d)} a las ${hh}:${mm}`)
    .replace(DATE_RE, (_m, y, mo, d) => spokenDate(y, mo, d))
    .replace(/([\p{L}\p{N}.+-]+)@([\p{L}\p{N}-]+)\.([\p{L}.]+)/gu, (_m, user, host, tld) =>
      `${user} arroba ${host} punto ${tld.split('.').join(' punto ')}`)
    .replace(/(?<=\p{L})\.(?=\p{L})/gu, ' punto ')
    .replace(/\s*(?:→|⇒|->|=>)\s*/g, ' a ')
    .replace(/\s+\/\s+/g, ' o ')
    .replace(new RegExp(`\\s+${DASH}\\s+`, 'g'), ', ')
    .replace(new RegExp(DASH, 'g'), '-')
    .replace(/[“”«»"]/g, '')
    .replace(/[^\p{L}\p{N}\s.,;:!?¿¡()%$€+'-]/gu, ' ')
    .replace(/\s+([.,;:!?])/g, '$1')
    .replace(/\s+/g, ' ')
    .trim();

  return /[\p{L}\p{N}]/u.test(text) ? text : '';
}

function splitLong(text: string, maxChars: number): string[] {
  const parts: string[] = [];
  let rest = text;
  while (rest.length > maxChars) {
    const comma = rest.lastIndexOf(', ', maxChars);
    let cut = comma > maxChars / 3 ? comma + 1 : rest.lastIndexOf(' ', maxChars);
    if (cut <= 0) cut = maxChars;
    parts.push(rest.slice(0, cut).trim());
    rest = rest.slice(cut).trim();
  }
  if (rest) parts.push(rest);
  return parts;
}

export interface SpeechChunkerOptions {
  /** Longest chunk sent to the TTS (longer audio needs more GPU memory in RVC). */
  maxChars?: number;
  /** Short pieces (like "Contexto: Ventas") wait to be merged with what follows. */
  minChars?: number;
}

/**
 * Accumulates streamed text and returns chunks ready to speak. Only complete
 * sentences are released: a sentence ends at a line break or at . ! ? ; followed
 * by whitespace, so "15:03", "54.165" or "gmail.com" are never cut.
 */
export class SpeechChunker {
  private buffer = '';
  private pending = '';
  private inCodeBlock = false;
  private emittedAny = false;
  private maxChars: number;
  private minChars: number;

  constructor(options: SpeechChunkerOptions = {}) {
    this.maxChars = options.maxChars ?? 160;
    this.minChars = options.minChars ?? 40;
  }

  public push(delta: string): string[] {
    this.buffer += delta;
    const out: string[] = [];

    let newline = this.buffer.indexOf('\n');
    while (newline !== -1) {
      this.takeLine(this.buffer.slice(0, newline), out);
      this.buffer = this.buffer.slice(newline + 1);
      newline = this.buffer.indexOf('\n');
    }

    // Speak finished sentences of the line still being written, except inside code
    // blocks, tables, or a line that could still turn into a code fence.
    const tail = this.buffer.trimStart();
    if (!this.inCodeBlock && !tail.startsWith('|') && !tail.startsWith('`')) {
      const boundary = this.lastSentenceEnd(this.buffer);
      if (boundary > 0) {
        this.addUnit(this.buffer.slice(0, boundary), out);
        this.buffer = this.buffer.slice(boundary);
      }
    }
    return out;
  }

  /** Releases everything left once the answer is complete. */
  public flush(): string[] {
    const out: string[] = [];
    if (this.buffer) this.takeLine(this.buffer, out);
    this.buffer = '';
    if (this.pending) out.push(...splitLong(this.pending, this.maxChars));
    this.pending = '';
    return out;
  }

  private lastSentenceEnd(text: string): number {
    let end = -1;
    for (const match of text.matchAll(/[.!?;](?=\s)/g)) end = (match.index ?? 0) + 1;
    return end;
  }

  private takeLine(line: string, out: string[]): void {
    if (/^\s*(```|~~~)/.test(line)) {
      this.inCodeBlock = !this.inCodeBlock;
      return;
    }
    if (this.inCodeBlock) return;
    // A line break ends the sentence even without punctuation (titles, list items).
    this.addUnit(line, out);
  }

  private addUnit(raw: string, out: string[]): void {
    const text = markdownToSpeech(raw);
    if (text) {
      const joiner = this.pending && !/[.!?;:,]$/.test(this.pending) ? '. ' : ' ';
      this.pending = this.pending ? `${this.pending}${joiner}${text}` : text;
    }
    // The first chunk goes out right away so the voice starts as soon as possible.
    if (this.pending && (!this.emittedAny || this.pending.length >= this.minChars)) {
      out.push(...splitLong(this.pending, this.maxChars));
      this.pending = '';
      this.emittedAny = true;
    }
  }
}
