/**
 * Centralized ToolContext Factory
 *
 * Single place for creating ToolContext instances with consistent defaults
 * and configuration. Replaces multiple factory functions with one standard approach.
 */

import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import type { Logger } from 'pino';
import type { ToolContext, ProgressReporter } from './types';
import type { SessionManager } from '@lib/session';
import type { PromptRegistry } from '../../prompts/registry';
import { SimpleToolContext } from './tool-context';
import { extractProgressToken, createProgressReporter } from './progress';

/**
 * Default configuration for all ToolContext instances
 */
const DEFAULT_CONFIG = {
  debug: false,
  defaultTimeout: 30000,
  defaultMaxTokens: 2048,
  defaultStopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
};

/**
 * Common dependencies needed for tool context creation
 */
export interface ToolContextDeps {
  server: Server;
  logger: Logger;
  promptRegistry?: PromptRegistry;
  sessionManager?: SessionManager;
}

/**
 * Options for creating a tool context
 */
export interface CreateContextOptions {
  /** Optional abort signal for cancellation */
  signal?: AbortSignal;
  /** Optional progress reporter or request with progress token */
  progress?: ProgressReporter | unknown;
  /** Enable debug logging */
  debug?: boolean;
  /** Custom timeout for sampling requests */
  timeout?: number;
  /** Custom max tokens for sampling */
  maxTokens?: number;
  /** Custom stop sequences */
  stopSequences?: string[];
}

/**
 * Create a ToolContext with all standard dependencies
 *
 * This is the single factory function that should be used everywhere.
 * It handles progress token extraction, configuration merging, and
 * proper dependency injection.
 *
 * @example
 * ```typescript
 * // In MCP server tool handler
 * const context = createToolContext({
 *   server: this.server.server,
 *   logger: this.deps.logger,
 *   promptRegistry: this.deps.promptRegistry,
 *   sessionManager: this.deps.sessionManager
 * }, {
 *   progress: request // Will extract progress token if present
 * });
 *
 * // In tool implementation
 * const result = await myTool(params, context);
 * ```
 */
export function createToolContext(
  deps: ToolContextDeps,
  options: CreateContextOptions = {},
): ToolContext {
  // Extract progress reporter if needed
  let progressReporter: ProgressReporter | undefined;

  if (options.progress) {
    if (typeof options.progress === 'function') {
      // Already a progress reporter
      progressReporter = options.progress as ProgressReporter;
    } else {
      // Try to extract progress token from request-like object
      const progressToken = extractProgressToken(options.progress);
      if (progressToken) {
        progressReporter = createProgressReporter(deps.server, progressToken, deps.logger);
      }
    }
  }

  // Merge configuration
  const config = {
    ...DEFAULT_CONFIG,
    debug: options.debug ?? DEFAULT_CONFIG.debug,
    defaultTimeout: options.timeout ?? DEFAULT_CONFIG.defaultTimeout,
    defaultMaxTokens: options.maxTokens ?? DEFAULT_CONFIG.defaultMaxTokens,
    defaultStopSequences: options.stopSequences ?? DEFAULT_CONFIG.defaultStopSequences,
  };

  // Create context with all dependencies
  return new SimpleToolContext(
    deps.server,
    deps.logger,
    deps.promptRegistry,
    options.signal,
    progressReporter,
    config,
    deps.sessionManager,
  );
}

/**
 * Helper to create context for MCP tool handlers
 *
 * Specialized version for use in MCP server tool registration.
 * Automatically extracts progress token from MCP request.
 *
 * @example
 * ```typescript
 * server.tool('my-tool', schema, async (params) => {
 *   const context = createMCPToolContext(
 *     server,
 *     params,
 *     logger,
 *     { promptRegistry, sessionManager }
 *   );
 *   return await myTool(params, context);
 * });
 * ```
 */
export function createMCPToolContext(
  server: Server,
  request: any,
  logger: Logger,
  services: {
    promptRegistry?: PromptRegistry;
    sessionManager?: SessionManager;
  },
): ToolContext {
  return createToolContext(
    {
      server,
      logger,
      promptRegistry: services.promptRegistry,
      sessionManager: services.sessionManager,
    } as ToolContextDeps,
    {
      progress: request, // Will auto-extract _progress token
    },
  );
}

/**
 * Create a minimal context for testing
 *
 * @example
 * ```typescript
 * const context = createTestContext(mockServer, testLogger);
 * const result = await myTool(params, context);
 * ```
 */
export function createTestContext(
  server: Server,
  logger: Logger,
  options: Partial<ToolContextDeps & CreateContextOptions> = {},
): ToolContext {
  return createToolContext(
    {
      server,
      logger,
      promptRegistry: options.promptRegistry,
      sessionManager: options.sessionManager,
    } as ToolContextDeps,
    {
      signal: options.signal,
      progress: options.progress,
      debug: options.debug ?? true, // Enable debug by default for tests
    } as CreateContextOptions,
  );
}
