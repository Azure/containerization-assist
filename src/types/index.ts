/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

export * from './categories';
export * from './core';
export * from './tool';
export * from './topics';

export type { ToolContext } from '../mcp/context';

// ===== AI MESSAGE TYPES =====

/**
 * Content block for AI messages (supports text and other types).
 */
export interface AIContent {
  type: 'text' | string;
  text?: string;
  [key: string]: unknown;
}

/**
 * Individual message in AI conversation.
 */
export interface AIMessage {
  role: 'system' | 'developer' | 'user' | 'assistant';
  content: AIContent[] | string;
}

/**
 * Output contract for structured AI responses.
 */
export interface OutputContract {
  name: string;
  schema?: unknown;
  description?: string;
}
