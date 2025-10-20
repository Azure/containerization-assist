/**
 * Orchestrator Types
 * Types for tool orchestration
 */

import type { Result } from '@/types/index';
import type { ChainHintsRegistry } from './chain-hints';

/**
 * Request to execute a tool
 */
export interface ExecuteRequest {
  toolName: string;
  params: unknown;
  metadata?: ExecuteMetadata;
}

/**
 * Optional execution metadata supplied by transports or callers.
 * Used to pass progress tokens and abort signals.
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
 * Orchestrator interface
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
  chainHintsMode: ChainHintsMode;
  chainHints?: ChainHintsRegistry;
  /** Reverse mapping from alias to original tool name (alias -> original) */
  aliasToOriginalMap?: Record<string, string>;
}
