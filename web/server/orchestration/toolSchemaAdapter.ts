import { McpToolDefinition } from '../types/protocol.js';

export interface OpenAiFunctionTool {
  type: 'function';
  function: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  };
}

export function adaptMcpToOpenAiTools(mcpTools: McpToolDefinition[]): OpenAiFunctionTool[] {
  return mcpTools.map((tool) => ({
    type: 'function',
    function: {
      name: tool.name,
      description: tool.description || `Herramienta MCP: ${tool.name}`,
      parameters: (tool.inputSchema as Record<string, unknown>) || {
        type: 'object',
        properties: {},
      },
    },
  }));
}
