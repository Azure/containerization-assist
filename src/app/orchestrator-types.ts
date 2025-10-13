/**
 * Simplified Orchestrator Types
 * Minimal types for tool orchestration without unnecessary complexity
 */

import type { z } from 'zod';
import type { Result } from '@/types/index';
import type { Logger } from 'pino';
import type { SessionConfig } from '@/session/core';

/**
 * Request to execute a tool
 */
export interface ExecuteRequest {
  toolName: string;
  params: unknown;
  sessionId?: string;
  metadata?: ExecuteMetadata;
}

/**
 * Optional execution metadata supplied by transports or callers.
 * Used to pass progress tokens, abort signals, or sampling preferences.
 */
export interface ExecuteMetadata {
  progress?: unknown;
  signal?: AbortSignal;
  maxTokens?: number;
  stopSequences?: string[];
  loggerContext?: Record<string, unknown>;
  sendNotification?: (notification: unknown) => Promise<void>;
}

/**
 * Minimal tool registration
 */
export interface RegisteredTool {
  name: string;
  description: string;
  schema: z.ZodSchema;
  handler: (params: unknown, logger: Logger) => Promise<Result<unknown>>;
  requires?: string[]; // Dependencies if needed
}

/**
 * Session facade for tool handlers
 */
export interface SessionFacade {
  id: string;
  get<T = unknown>(key: string): T | undefined;
  set(key: string, value: unknown): void;
  pushStep(step: string): void;
}

/**
 * Simplified orchestrator interface
 */
export interface ToolOrchestrator {
  execute(request: ExecuteRequest): Promise<Result<unknown>>;
  close(): void;
}

type ChainHintsMode = 'enabled' | 'disabled';

/**
 * Orchestrator configuration
 */
export interface OrchestratorConfig {
  policyPath?: string;
  policyEnvironment?: string;
  session?: SessionConfig;
  chainHintsMode: ChainHintsMode;
}
