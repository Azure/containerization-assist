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
  metadata?: Record<string, unknown>;
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
}

/**
 * Orchestrator configuration
 */
export interface OrchestratorConfig {
  maxRetries?: number;
  retryDelay?: number;
  sessionTTL?: number;
  policyPath?: string;
  policyEnvironment?: string;
  session?: SessionConfig;
}
