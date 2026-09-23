/**
 * Splits an answer into speakable chunks at sentence boundaries. Voicebox on
 * the local GPU synthesizes ~2.4x slower than real time, so speaking chunk by
 * chunk lets the first sentence play while the rest is still being generated.
 */
export function splitForSpeech(text: string, maxChars = 220): string[] {
  const clean = text
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/[*_`#>|]/g, '')
    .replace(/\s+/g, ' ')
    .trim();
  if (!clean) return [];

  // Opening ¿/¡ belong to the sentence that follows, so they are not terminators.
  const sentences = clean.match(/[^.!?;:]+[.!?;:]+["')\]]*|[^.!?;:]+$/g) ?? [clean];
  const chunks: string[] = [];
  let current = '';

  for (const raw of sentences) {
    const sentence = raw.trim();
    if (!sentence) continue;

    if (sentence.length > maxChars) {
      if (current) chunks.push(current);
      current = '';
      chunks.push(...splitLong(sentence, maxChars));
      continue;
    }

    // Keep the first chunk short so the voice starts as soon as possible.
    const limit = chunks.length === 0 ? Math.min(maxChars, 120) : maxChars;
    if (current && current.length + 1 + sentence.length > limit) {
      chunks.push(current);
      current = sentence;
    } else {
      current = current ? `${current} ${sentence}` : sentence;
    }
  }

  if (current) chunks.push(current);
  return chunks;
}

function splitLong(sentence: string, maxChars: number): string[] {
  const parts: string[] = [];
  let current = '';
  for (const word of sentence.split(' ')) {
    if (current && current.length + 1 + word.length > maxChars) {
      parts.push(current);
      current = word;
    } else {
      current = current ? `${current} ${word}` : word;
    }
  }
  if (current) parts.push(current);
  return parts;
}
