/**
 * MCP-specific type definitions
 * Minimal types for MCP tool registration and responses
 */

import type { z } from 'zod';
import type { Logger } from 'pino';
import type { ToolContext } from './context/types';

export type TextContent = {
  type: 'text';
  text: string;
};

export type Content = TextContent; // Start with text only, extend as needed

export interface MCPTool<Schema extends z.ZodTypeAny, Out> {
  name: string;
  description?: string;
  inputSchema: Schema;
  handler: (params: z.infer<Schema>, context?: ToolContext) => Promise<MCPResponse<Out>>;
}

export type MCPResponse<T> =
  | { content: Content[]; value?: T }
  | { content: Content[]; error: string };

export interface MCPToolContext {
  logger?: Logger;
  context?: ToolContext;
}
