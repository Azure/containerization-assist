/**
 * Core type definitions for the containerization assist system.
 * Provides Result type for error handling and tool system interfaces.
 */

import type { Logger } from 'pino';
import type { ToolContext } from '../mcp/context';
import type { ZodRawShape } from 'zod';
import type { Result } from './core';
import { ToolName } from '@/exports/tools';

// Export enhanced category types
export * from './categories';

// Export consolidated core types
export * from './core';

export type { ToolContext } from '../mcp/context';

/**
 * Tool definition for MCP server operations.
 */
export interface Tool {
  /** Unique tool identifier */
  name: ToolName;
  /** Human-readable tool description */
  description?: string;
  /** JSON schema for parameter validation */
  schema?: Record<string, unknown>;
  /** Zod schema for McpServer compatibility (optional) */
  zodSchema?: ZodRawShape;
  /**
   * Executes the tool with provided parameters.
   * @param params - Tool-specific parameters
   * @param logger - Logger instance for tool execution
   * @param context - Optional ToolContext for AI capabilities and progress reporting
   * @returns Promise resolving to Result with tool output or error
   */
  execute: (
    params: Record<string, unknown>,
    logger: Logger,
    context?: ToolContext,
  ) => Promise<Result<unknown>>;
}

// ===== SESSION =====

/**
 * Represents the state of a tool execution session.
 */
export interface WorkflowState {
  /** Unique session identifier */
  sessionId: string;
  /** Currently executing tool */
  currentStep?: string;
  /** Overall progress (0-100) */
  progress?: number;
  /** Results from completed tools */
  results?: Record<string, unknown>;
  /** Additional metadata */
  metadata?: Record<string, unknown>;
  /** List of completed step names */
  completed_steps?: string[];
  /** Session creation timestamp */
  createdAt: Date;
  /** Last update timestamp */
  updatedAt: Date;
  /** Allow additional properties for extensibility */
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
  topic: string;
  tool: string;
  environment: string;
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
    topic: string;
    knowledgeCount?: number;
    policyCount?: number;
  };
}
