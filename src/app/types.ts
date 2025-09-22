/**
 * Application Kernel Types
 * Type definitions for the unified execution kernel
 */

import type { z } from 'zod';
import type { Result } from '@/types/index';

// ============================================================================
// Core Types
// ============================================================================

/**
 * Request to execute a tool
 */
export interface ExecuteRequest {
  toolName: string;
  params: unknown;
  sessionId?: string;
  force?: boolean;
  metadata?: Record<string, unknown>;
}

/**
 * Tool execution plan
 */
export interface ExecutionPlan {
  steps: string[];
  dependencies: Map<string, string[]>;
  completed: Set<string>;
  remaining: string[];
}

/**
 * Tool registration information
 */
export interface RegisteredTool {
  name: string;
  description: string;
  schema: z.ZodSchema;
  handler: ToolHandler;
  provides?: string[];
  requires?: string[];
  category?: string;
  version?: string;
  requiresOrchestration?: boolean; // Tool explicitly needs complex orchestration
}

/**
 * Tool handler function
 */
export type ToolHandler = (params: unknown, context: ToolContext) => Promise<Result<unknown>>;

/**
 * Context provided to tool handlers
 */
export interface ToolContext {
  sessionId?: string;
  session?: SessionState;
  logger: Logger;
  progress: ProgressReporter;
  telemetry?: TelemetrySystem;
  ai?: AIClient;
  knowledge?: KnowledgeBase;
  policies?: PolicyEngine;
}

// ============================================================================
// Session Types
// ============================================================================

/**
 * Session state with tool-specific slices
 */
export interface SessionState {
  sessionId: string;
  created: Date;
  updated: Date;
  completed_steps: string[];
  data: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}

/**
 * Session manager interface
 */
export interface SessionManager {
  get(sessionId: string): Promise<Result<SessionState>>;
  create(): Promise<Result<SessionState>>;
  update(sessionId: string, updates: Partial<SessionState>): Promise<Result<void>>;
  delete(sessionId: string): Promise<Result<void>>;
  list(): Promise<Result<string[]>>;
}

// ============================================================================
// Logging & Progress Types
// ============================================================================

/**
 * Logger interface
 */
export interface Logger {
  error(message: string, ...args: unknown[]): void;
  warn(message: string, ...args: unknown[]): void;
  info(message: string, ...args: unknown[]): void;
  debug(message: string, ...args: unknown[]): void;
  trace(message: string, ...args: unknown[]): void;
}

/**
 * Progress reporter interface
 */
export interface ProgressReporter {
  start(message: string): void;
  update(message: string, percentage?: number): void;
  complete(message: string): void;
  fail(message: string): void;
}

// ============================================================================
// AI & Knowledge Types
// ============================================================================

/**
 * AI client interface for prompt execution
 */
export interface AIClient {
  sample(options: {
    prompt: string;
    temperature?: number;
    format?: 'json' | 'text';
    maxTokens?: number;
  }): Promise<string>;
}

/**
 * Knowledge base interface
 */
export interface KnowledgeBase {
  getRelevant(toolName: string): Promise<Record<string, unknown>>;
  search(query: string): Promise<Array<{ id: string; content: unknown; score: number }>>;
}

/**
 * Policy engine interface
 */
export interface PolicyEngine {
  getApplicable(toolName: string): Promise<Array<{ id: string; rules: unknown[] }>>;
  evaluate(input: unknown): Promise<Array<{ rule: string; matched: boolean; actions: unknown }>>;
}

// ============================================================================
// Telemetry Types
// ============================================================================

/**
 * Telemetry system interface
 */
export interface TelemetrySystem {
  track(event: TelemetryEvent): void;
  getMetrics(): Map<string, AggregatedMetric>;
  getHealth(): HealthStatus;
}

/**
 * Telemetry event
 */
export interface TelemetryEvent {
  type: string;
  toolName?: string;
  timestamp: number;
  duration?: number;
  error?: string;
  metadata?: Record<string, unknown>;
}

/**
 * Aggregated metric
 */
export interface AggregatedMetric {
  name: string;
  value: number;
  count: number;
  min: number;
  max: number;
  avg: number;
}

/**
 * Health status
 */
export interface HealthStatus {
  status: 'healthy' | 'degraded' | 'critical';
  issues?: string[];
  metrics?: Record<string, number>;
}

// ============================================================================
// Kernel Interface
// ============================================================================

/**
 * Main kernel interface - the unified execution path
 */
export interface Kernel {
  // Core execution
  execute(request: ExecuteRequest): Promise<Result<unknown>>;

  // Planning & validation
  getPlan(toolName: string, sessionId?: string): Promise<string[]>;
  canExecute(
    toolName: string,
    sessionId?: string,
  ): Promise<{
    canExecute: boolean;
    missing: string[];
    completed: string[];
  }>;

  // Registry access
  tools(): Map<string, RegisteredTool>;
  getTool(name: string): RegisteredTool | undefined;

  // Session management
  getSession(sessionId: string): Promise<Result<SessionState>>;
  createSession(): Promise<Result<SessionState>>;

  // Health & metrics
  getHealth(): HealthStatus;
  getMetrics(): Map<string, AggregatedMetric>;
}

// ============================================================================
// Configuration Types
// ============================================================================

/**
 * Kernel configuration
 */
export interface KernelConfig {
  // Core settings
  maxRetries?: number;
  retryDelay?: number;
  timeout?: number;

  // Session settings
  sessionStore?: 'memory' | 'file' | 'redis';
  sessionTTL?: number;
  sessionPath?: string;

  // Telemetry settings
  telemetryEnabled?: boolean;
  telemetryFlushInterval?: number;
  telemetryBufferSize?: number;

  // Policy settings
  policyPath?: string;
  policyEnvironment?: string;
  policyEnforcement?: 'strict' | 'lenient' | 'advisory';

  // AI settings
  aiProvider?: 'anthropic' | 'openai' | 'local';
  aiModel?: string;
  aiApiKey?: string;

  // Knowledge settings
  knowledgePath?: string;
  knowledgeIndexPath?: string;
}

/**
 * Kernel factory options
 */
export interface KernelFactoryOptions {
  config: KernelConfig;
  tools?: Map<string, RegisteredTool>;
  sessionManager?: SessionManager;
  telemetry?: TelemetrySystem;
  logger?: Logger;
}

// ============================================================================
// Export Types
// ============================================================================

export type { Result };
