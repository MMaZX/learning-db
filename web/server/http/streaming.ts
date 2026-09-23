import { ServerResponse } from 'node:http';
import { SseEvent, SseEventType } from '../types/protocol.js';

export class SseStreamWriter {
  private res: ServerResponse;
  private conversationId: string;
  private turnId: string;
  private sequence = 0;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private isClosed = false;

  constructor(res: ServerResponse, conversationId: string, turnId: string) {
    this.res = res;
    this.conversationId = conversationId;
    this.turnId = turnId;

    this.res.writeHead(200, {
      'Content-Type': 'text/event-stream; charset=utf-8',
      'Cache-Control': 'no-cache, no-transform',
      'Connection': 'keep-alive',
      'X-Accel-Buffering': 'no',
    });

    // Start 15s keep-alive heartbeat
    this.heartbeatInterval = setInterval(() => {
      if (!this.isClosed) {
        this.res.write(': heartbeat\n\n');
      }
    }, 15000);
  }

  public send<T>(type: SseEventType, data: T): void {
    if (this.isClosed) return;

    this.sequence++;
    const event: SseEvent<T> = {
      type,
      sequence: this.sequence,
      conversationId: this.conversationId,
      turnId: this.turnId,
      data,
    };

    this.res.write(`event: ${type}\n`);
    this.res.write(`data: ${JSON.stringify(event)}\n\n`);
  }

  public close(): void {
    if (this.isClosed) return;
    this.isClosed = true;

    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }

    this.res.end();
  }
}
