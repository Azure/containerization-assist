/**
 * MCP (Model Context Protocol) type definitions for tool framework and registry safety
 *
 * Provides strongly typed interfaces for MCP tools, prompt-backed tools,
 * and tool parameter validation and routing.
 */

import type { z } from 'zod';
import type { Logger } from 'pino';
import type { Result } from './core';
import type { ToolContext } from '../mcp/context';

// ===== CORE MCP TOOL INTERFACES =====

/**
 * Base tool definition interface for MCP compatibility
 */
export interface MCPTool {
  name: string;
  description: string;
  inputSchema: z.ZodSchema<unknown>;
  outputSchema?: z.ZodSchema<unknown>;
}

/**
 * Tool definition with execution handler
 */
export interface ToolDefinition<TIn = unknown, TOut = unknown> extends MCPTool {
  inputSchema: z.ZodSchema<TIn>;
  outputSchema?: z.ZodSchema<TOut>;
  execute: (params: TIn, deps: ToolDependencies, context: ToolContext) => Promise<Result<TOut>>;
}

/**
 * Tool dependencies for dependency injection
 */
export interface ToolDependencies {
  logger: Logger;
  fs?: typeof import('fs');
  docker?: DockerClient;
  k8s?: KubernetesClient;
}

/**
 * Docker client interface (replaces any)
 */
export interface DockerClient {
  buildImage: (options: DockerBuildOptions) => Promise<DockerBuildResult>;
  pushImage: (imageId: string, options?: DockerPushOptions) => Promise<DockerPushResult>;
  tagImage: (imageId: string, tag: string) => Promise<void>;
  scanImage?: (imageId: string) => Promise<DockerScanResult>;
  [key: string]: unknown;
}

/**
 * Kubernetes client interface (replaces any)
 */
export interface KubernetesClient {
  applyManifest: (manifest: string, namespace?: string) => Promise<KubernetesApplyResult>;
  getDeploymentStatus: (name: string, namespace?: string) => Promise<KubernetesDeploymentStatus>;
  createNamespace?: (name: string) => Promise<void>;
  [key: string]: unknown;
}

// ===== DOCKER API TYPES =====

export interface DockerBuildOptions {
  context: string;
  dockerfile?: string;
  tags?: string[];
  buildArgs?: Record<string, string>;
  target?: string;
}

export interface DockerBuildResult {
  imageId: string;
  tags: string[];
  size?: number;
  logs?: string[];
}

export interface DockerPushOptions {
  registry?: string;
  authConfig?: {
    username: string;
    password: string;
    serveraddress?: string;
  };
}

export interface DockerPushResult {
  digest: string;
  repository: string;
  tag: string;
}

export interface DockerScanResult {
  vulnerabilities: Array<{
    id: string;
    severity: 'low' | 'medium' | 'high' | 'critical';
    description: string;
    package?: string;
  }>;
  summary: {
    total: number;
    by_severity: Record<string, number>;
  };
}

// ===== KUBERNETES API TYPES =====

export interface KubernetesApplyResult {
  applied: Array<{
    kind: string;
    name: string;
    namespace?: string;
    action: 'created' | 'configured' | 'unchanged';
  }>;
}

export interface KubernetesDeploymentStatus {
  name: string;
  namespace: string;
  replicas: {
    desired: number;
    ready: number;
    available: number;
  };
  conditions: Array<{
    type: string;
    status: string;
    reason?: string;
    message?: string;
  }>;
}

// ===== PROMPT-BACKED TOOL TYPES =====

/**
 * AI prompt template interface
 */
export interface PromptTemplate {
  id: string;
  template: string;
  system?: string;
  user?: string;
  version?: string;
  variables?: string[];
  metadata?: Record<string, unknown>;
}

/**
 * AI response from prompt execution
 */
export interface AIResponse<T = unknown> {
  content: T;
  metadata?: {
    model?: string;
    tokens?: number;
    latency?: number;
    provenance?: PromptProvenance;
  };
}

/**
 * Prompt execution provenance for debugging
 */
export interface PromptProvenance {
  promptId: string;
  resolvedPrompt: string;
  version: string;
  timestamp: number;
  variables?: Record<string, unknown>;
}

/**
 * Parameter schema for tool validation
 */
export interface ParameterSchema {
  [key: string]: {
    type: 'string' | 'number' | 'boolean' | 'object' | 'array';
    required?: boolean;
    description?: string;
    default?: unknown;
    enum?: unknown[];
  };
}

/**
 * Tool context for MCP operations (augments existing ToolContext)
 */
export interface MCPToolContext extends ToolContext {
  sessionId?: string;
  metadata?: Record<string, unknown>;
}

// ===== RESULT TYPES =====

/**
 * Tool execution result wrapper
 */
export interface ToolResult<T = unknown> {
  success: boolean;
  data?: T;
  error?: string;
  metadata?: {
    executionTime: number;
    toolName: string;
    sessionId?: string;
  };
}

/**
 * Parameter validation result for MCP operations
 */
export interface MCPValidationResult {
  isValid: boolean;
  errors: string[];
  warnings?: string[];
  suggestions?: Record<string, unknown>;
  confidence?: number;
  metadata?: {
    validationTime: number;
    aiEnhanced: boolean;
    rulesApplied: string[];
  };
}

/**
 * Processing state for long-running operations
 */
export interface ProcessingState {
  status: 'pending' | 'running' | 'completed' | 'failed';
  progress?: number;
  message?: string;
  startTime: number;
  endTime?: number;
  results?: unknown;
}

// ===== CONFIGURATION TYPES =====

/**
 * Dynamic configuration object (replaces any in config handling)
 */
export interface ConfigurationObject {
  [key: string]: string | number | boolean | ConfigurationObject | ConfigurationObject[];
}

/**
 * Session data interface (replaces any in session handling)
 */
export interface SessionData {
  state?: Record<string, unknown>;
  completedSteps?: Record<string, boolean>;
  results?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
  createdAt: Date;
  updatedAt: Date;
}

// ===== API INTEGRATION TYPES =====

/**
 * Generic API response wrapper
 */
export interface APIResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
  metadata?: {
    requestId?: string;
    timestamp: number;
    version?: string;
  };
}

/**
 * Error response for API operations
 */
export interface ErrorResponse {
  code: string;
  message: string;
  details?: unknown;
  stack?: string;
}
