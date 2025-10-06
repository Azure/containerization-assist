/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

// Export enhanced category types
export * from './categories';

// Export consolidated core types (includes Result type)
export * from './core';

// Export the new unified Tool interface
export * from './tool';

// Export topic types and constants
export * from './topics';

export type { ToolContext } from '../mcp/context';

// Import Result and Topic for local use in this file
import type { Result } from './core';
import type { Topic } from './topics';

// ===== SESSION =====

/**
 * Represents the state of a tool execution session.
 */
export interface WorkflowState {
  /** Unique session identifier */
  sessionId: string;
  /** List of completed step names */
  completed_steps?: string[];
  /** Session creation timestamp */
  createdAt: Date;
  /** Last update timestamp */
  updatedAt: Date;
  /** Allow additional properties for workflow flags and computed values */
  [key: string]: unknown;
}

// ===== AI SERVICE TYPES =====

export interface AIService {
  isAvailable(): boolean;
  generateResponse(prompt: string, context?: Record<string, unknown>): Promise<Result<string>>;
  analyzeCode(code: string, language: string): Promise<Result<unknown>>;
  enhanceDockerfile(
    dockerfile: string,
    requirements?: Record<string, unknown>,
  ): Promise<Result<string>>;
  validateParameters?(params: Record<string, unknown>): Promise<Result<unknown>>;
  analyzeResults?(results: unknown): Promise<Result<unknown>>;
}

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
 * Collection of messages for AI conversation.
 */
export interface AIMessages {
  messages: AIMessage[];
}

/**
 * Output contract for structured AI responses.
 */
export interface OutputContract {
  name: string;
  schema?: unknown;
  description?: string;
}

/**
 * Parameters for building AI prompt messages.
 */
export interface BuildPromptParams {
  basePrompt: string;
  topic: Topic;
  tool: string;
  environment: string;
  language?: string;
  framework?: string;
  contract?: OutputContract;
  knowledgeBudget?: number;
}

/**
 * Envelope containing structured prompt with metadata.
 */
export interface PromptEnvelope {
  system?: string;
  developer?: string;
  user: string;
  metadata?: {
    tool: string;
    environment: string;
    topic: Topic;
    knowledgeCount?: number;
    policyCount?: number;
  };
}
