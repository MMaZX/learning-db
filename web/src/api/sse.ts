export interface ParsedSseEvent {
  type: string;
  data: Record<string, unknown>;
}

/**
 * Incremental parser for the BFF's `text/event-stream` responses. Feed it the
 * decoded chunks as they arrive; it returns the complete events and keeps any
 * partial event buffered until the next chunk.
 */
export class SseParser {
  private buffer = '';

  public push(chunk: string): ParsedSseEvent[] {
    this.buffer += chunk.replace(/\r\n/g, '\n');
    const events: ParsedSseEvent[] = [];

    let boundary = this.buffer.indexOf('\n\n');
    while (boundary !== -1) {
      const block = this.buffer.slice(0, boundary);
      this.buffer = this.buffer.slice(boundary + 2);
      const event = parseBlock(block);
      if (event) events.push(event);
      boundary = this.buffer.indexOf('\n\n');
    }

    return events;
  }
}

function parseBlock(block: string): ParsedSseEvent | null {
  let type = 'message';
  const dataLines: string[] = [];

  for (const line of block.split('\n')) {
    // Comment lines (": heartbeat") carry no event.
    if (line.startsWith(':')) continue;
    if (line.startsWith('event:')) type = line.slice(6).trim();
    else if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart());
  }

  if (dataLines.length === 0) return null;

  try {
    const payload = JSON.parse(dataLines.join('\n'));
    // The BFF wraps every payload in an envelope: { type, sequence, ..., data }.
    const data = payload && typeof payload === 'object' && 'data' in payload ? payload.data : payload;
    return { type, data: (data ?? {}) as Record<string, unknown> };
  } catch {
    return null;
  }
}
