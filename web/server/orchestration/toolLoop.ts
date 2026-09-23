import { SseStreamWriter } from '../http/streaming.js';
import { McpClient } from '../clients/mcpClient.js';
import { OmniRouteClient, ChatMessageParam } from '../clients/omnirouteClient.js';
import { buildJarvisSystemPrompt } from '../prompts/jarvis-system.js';
import { getToolPolicy } from './toolPolicy.js';
import { adaptMcpToOpenAiTools } from './toolSchemaAdapter.js';
import { ServerConfig } from '../config.js';

export async function runToolLoop(
  userText: string,
  treatment = 'Señor',
  sse: SseStreamWriter,
  mcp: McpClient,
  omni: OmniRouteClient,
  config: ServerConfig
): Promise<void> {
  sse.send('turn.accepted', { userText });
  sse.send('routing.started', { target: 'OmniRoute auto + ia-jarvis-gr MCP' });

  // 1. Get available MCP tools and filter according to policy
  let mcpTools = await mcp.listTools().catch(() => []);
  const allowedMcpTools = mcpTools.filter((t) => getToolPolicy(t.name) !== 'DENIED');
  const openAiTools = adaptMcpToOpenAiTools(allowedMcpTools);

  // 2. Prepare Messages Array
  const messages: ChatMessageParam[] = [
    { role: 'system', content: buildJarvisSystemPrompt(treatment) },
    { role: 'user', content: userText },
  ];

  let rounds = 0;
  let totalToolCalls = 0;
  let finalAnswer = '';

  while (rounds < config.chatMaxToolRounds && totalToolCalls < config.chatMaxToolCalls) {
    rounds++;

    let accumulatedContent = '';
    const accumulatedTools: Record<number, { id: string; name: string; args: string }> = {};

    try {
      for await (const delta of omni.streamChatCompletions(messages, openAiTools)) {
        if (delta.content) {
          accumulatedContent += delta.content;
          finalAnswer += delta.content;
          sse.send('assistant.delta', { delta: delta.content, fullText: accumulatedContent });
        }

        if (delta.tool_calls) {
          for (const tc of delta.tool_calls) {
            const idx = tc.index;
            if (!accumulatedTools[idx]) {
              accumulatedTools[idx] = { id: tc.id || '', name: tc.function?.name || '', args: '' };
            }
            if (tc.id) accumulatedTools[idx].id = tc.id;
            if (tc.function?.name) accumulatedTools[idx].name = tc.function.name;
            if (tc.function?.arguments) accumulatedTools[idx].args += tc.function.arguments;
          }
        }
      }
    } catch (err: unknown) {
      const error = err as Error;
      sse.send('error', { code: 'OMNIROUTE_ERROR', message: error.message });
      break;
    }

    const toolKeys = Object.keys(accumulatedTools);

    // If no tool was requested by model, this is the final answer!
    if (toolKeys.length === 0) {
      break;
    }

    // Execute tool calls
    const assistantToolCalls: Array<{ id: string; type: 'function'; function: { name: string; arguments: string } }> = [];

    for (const k of toolKeys) {
      const toolCall = accumulatedTools[parseInt(k, 10)];
      assistantToolCalls.push({
        id: toolCall.id,
        type: 'function',
        function: {
          name: toolCall.name,
          arguments: toolCall.args,
        },
      });
    }

    // Add assistant turn with tool_calls to history
    messages.push({
      role: 'assistant',
      content: accumulatedContent || null,
      tool_calls: assistantToolCalls,
    });

    for (const toolCall of assistantToolCalls) {
      totalToolCalls++;
      const name = toolCall.function.name;
      const policy = getToolPolicy(name);

      sse.send('tool.requested', { toolName: name, callId: toolCall.id });

      let parsedArgs: Record<string, unknown> = {};
      try {
        parsedArgs = JSON.parse(toolCall.function.arguments || '{}');
      } catch {
        parsedArgs = {};
      }

      if (policy === 'DENIED') {
        const errorMsg = `Error: La herramienta '${name}' tiene denegación de ejecución por seguridad.`;
        sse.send('error', { code: 'MCP_TOOL_DENIED', toolName: name });
        messages.push({ role: 'tool', tool_call_id: toolCall.id, content: errorMsg });
        continue;
      }

      if (policy === 'CONFIRMATION_REQUIRED') {
        sse.send('tool.confirmation.required', {
          toolName: name,
          callId: toolCall.id,
          args: parsedArgs,
        });
      }

      sse.send('tool.started', { toolName: name, callId: toolCall.id });

      let toolResultText = '';
      try {
        const result = await mcp.callTool(name, parsedArgs);
        toolResultText = result.content.map((c) => c.text || '').join('\n');
        sse.send('tool.completed', {
          toolName: name,
          callId: toolCall.id,
          resultSnippet: toolResultText.slice(0, 300),
        });
      } catch (err: unknown) {
        const error = err as Error;
        toolResultText = `Error al ejecutar tool '${name}': ${error.message}`;
        sse.send('error', { code: 'MCP_TOOL_ERROR', toolName: name, message: error.message });
      }

      // Add tool output to history
      messages.push({
        role: 'tool',
        tool_call_id: toolCall.id,
        content: toolResultText,
      });
    }
  }

  sse.send('assistant.completed', { finalAnswer });
  sse.send('done', {});
  sse.close();
}
