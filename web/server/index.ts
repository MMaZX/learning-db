import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadConfig } from './config.js';
import { McpClient } from './clients/mcpClient.js';
import { OmniRouteClient } from './clients/omnirouteClient.js';
import { VoiceboxClient, VoiceboxError } from './clients/voiceboxClient.js';
import { SseStreamWriter } from './http/streaming.js';
import { runToolLoop } from './orchestration/toolLoop.js';
import { CapabilitiesResponse } from './types/protocol.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const distDir = path.resolve(__dirname, '../dist');

const config = loadConfig();
const mcpClient = new McpClient({
  url: config.jarvisMcpUrl,
  authToken: config.jarvisMcpAuthToken,
  timeoutMs: 20000,
});

const omniClient = new OmniRouteClient({
  baseUrl: config.omnirouteBaseUrl,
  apiKey: config.omnirouteApiKey,
  timeoutMs: config.chatTurnTimeoutMs,
});

const voiceboxClient = new VoiceboxClient({
  baseUrl: config.voiceboxUrl,
  profileId: config.voiceboxProfileId,
  language: config.voiceboxLanguage,
  engine: config.voiceboxEngine,
  modelSize: config.voiceboxModelSize,
});

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url || '/', `http://${req.headers.host || '127.0.0.1'}`);
  const pathname = url.pathname;

  // CORS / Same-origin check
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization');

  if (req.method === 'OPTIONS') {
    res.writeHead(204);
    res.end();
    return;
  }

  // 1. Capabilities Endpoint
  if (pathname === '/api/capabilities' && req.method === 'GET') {
    const caps: CapabilitiesResponse = {
      chat: true,
      mcp: true,
      sttExperimental: config.voiceExperimentalEnabled,
      tts: voiceboxClient.isConfigured,
      limits: {
        maxDurationMs: config.omnirouteSttMaxDurationMs,
        maxBytes: config.omnirouteSttMaxBytes,
        allowedMime: config.omnirouteSttAllowedMime,
      },
    };
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(caps));
    return;
  }

  // 2. Health Endpoint
  if (pathname === '/api/health' && req.method === 'GET') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', uptime: process.uptime() }));
    return;
  }

  // 3. Turns Execution Endpoint (SSE Stream)
  if (pathname.startsWith('/api/conversations/') && pathname.endsWith('/turns') && req.method === 'POST') {
    const segments = pathname.split('/');
    const conversationId = segments[3] || 'default';
    const turnId = `turn-${Date.now()}`;

    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
      // Guard against oversized bodies (> 1MB)
      if (body.length > 1048576) {
        req.destroy();
      }
    });

    req.on('end', async () => {
      try {
        const parsed = JSON.parse(body || '{}');
        const userText = parsed.text || '';
        const treatment = parsed.treatment || 'Señor';

        if (!userText.trim()) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'El texto del turno no puede estar vacío' }));
          return;
        }

        const sseWriter = new SseStreamWriter(res, conversationId, turnId);
        await runToolLoop(userText, treatment, sseWriter, mcpClient, omniClient, config);
      } catch (err: unknown) {
        const error = err as Error;
        if (!res.headersSent) {
          res.writeHead(500, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: error.message }));
        }
      }
    });
    return;
  }

  // 4. Text-to-speech via local Voicebox (GPU) → audio/wav
  if (pathname === '/api/tts' && req.method === 'POST') {
    let body = '';
    req.on('data', (chunk) => {
      body += chunk;
      if (body.length > 65536) {
        req.destroy();
      }
    });

    req.on('end', async () => {
      try {
        const parsed = JSON.parse(body || '{}');
        const text = typeof parsed.text === 'string' ? parsed.text.trim() : '';
        if (!text) {
          res.writeHead(400, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: 'El texto a sintetizar no puede estar vacío' }));
          return;
        }
        if (text.length > config.ttsMaxChars) {
          res.writeHead(413, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: `El texto supera ${config.ttsMaxChars} caracteres` }));
          return;
        }

        const wav = await voiceboxClient.synthesize(text);
        res.writeHead(200, { 'Content-Type': 'audio/wav', 'Content-Length': wav.byteLength });
        res.end(Buffer.from(wav));
      } catch (err: unknown) {
        const status = err instanceof VoiceboxError ? err.status : 500;
        if (!res.headersSent) {
          res.writeHead(status, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify({ error: (err as Error).message }));
        }
      }
    });
    return;
  }

  // 5. Serve Static Frontend Files from dist/
  if (fs.existsSync(distDir)) {
    let filePath = path.join(distDir, pathname === '/' ? 'index.html' : pathname);
    if (!fs.existsSync(filePath)) {
      filePath = path.join(distDir, 'index.html');
    }

    const ext = path.extname(filePath);
    const mimeTypes: Record<string, string> = {
      '.html': 'text/html',
      '.js': 'application/javascript',
      '.css': 'text/css',
      '.json': 'application/json',
      '.svg': 'image/svg+xml',
      '.png': 'image/png',
      '.ico': 'image/x-icon',
    };

    try {
      const content = fs.readFileSync(filePath);
      res.writeHead(200, { 'Content-Type': mimeTypes[ext] || 'application/octet-stream' });
      res.end(content);
      return;
    } catch {
      // Pass to 404
    }
  }

  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ error: 'Endpoint no encontrado' }));
});

server.listen(config.bffPort, config.bffHost, () => {
  console.log(`[Jarvis BFF] Servidor escuchando en http://${config.bffHost}:${config.bffPort}`);
  console.log(`[Jarvis BFF] Conectando a MCP Go en: ${config.jarvisMcpUrl}`);
  console.log(`[Jarvis BFF] Conectando a OmniRoute en: ${config.omnirouteBaseUrl}`);
  console.log(`[Jarvis BFF] Voz Voicebox en: ${config.voiceboxUrl} (${voiceboxClient.isConfigured ? 'perfil configurado' : 'sin VOICEBOX_PROFILE_ID'})`);
});

process.on('SIGINT', () => {
  server.close(() => process.exit(0));
});

process.on('SIGTERM', () => {
  server.close(() => process.exit(0));
});
