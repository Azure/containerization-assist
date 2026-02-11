/**
 * SDK Executor
 *
 * Provides direct tool execution without MCP orchestration overhead.
 * This is a lightweight execution path for SDK consumers who don't
 * need MCP protocol support, chain hints, or policy enforcement.
 *
 * Design: Minimal wrapper that creates context and calls tool handler.
 */

import type { Logger } from 'pino';
import type { ZodTypeAny } from 'zod';
import { createToolContext, type ContextOptions, type ProgressReporter } from '@/core/context';
import { loadKnowledgeBase } from '@/knowledge/loader';
import { createLogger } from '@/lib/logger';
import { Failure, type Result } from '@/types/core';
import type { Tool } from '@/types/tool';
import type { SDKOptions } from './types.js';

// ===== INTERNAL STATE =====

/**
 * Knowledge loader with closure-based state.
 *
 * Uses a closure instead of 'this' to avoid binding issues
 * when methods are destructured or passed as callbacks.
 */
function createKnowledgeLoader() {
  let loaded = false;

  return {
    /**
     * Ensure knowledge base is loaded (idempotent).
     * Called once per process to load static knowledge packs.
     */
    ensureLoaded(logger: Logger): void {
      if (loaded) return;

      try {
        loadKnowledgeBase();
        loaded = true;
        logger.debug('Knowledge base loaded for SDK');
      } catch (error) {
        // Log but don't fail - tools can work without knowledge
        logger.warn({ error }, 'Failed to load knowledge base');
      }
    },

    /**
     * Check if knowledge is loaded (for assertions).
     */
    isLoaded(): boolean {
      return loaded;
    },

    /**
     * Reset loaded state (for testing only).
     */
    reset(): void {
      loaded = false;
    },
  };
}

const knowledgeLoader = createKnowledgeLoader();

/**
 * Testing utilities exported for test files.
 * @internal
 */
export const _testing = {
  /** Reset knowledge loader state for test isolation */
  resetKnowledgeState: () => knowledgeLoader.reset(),
  /** Check if knowledge is loaded (for assertions) */
  isKnowledgeLoaded: () => knowledgeLoader.isLoaded(),
};

// ===== INTERNAL HELPERS =====

/**
 * Wrap an SDK progress callback as a ProgressReporter.
 *
 * Extracts the progress wrapping logic into a named function for clarity.
 * Returns undefined if no callback is provided.
 *
 * @param onProgress - SDK progress callback or undefined
 * @returns ProgressReporter function or undefined
 */
function wrapProgressCallback(
  onProgress: SDKOptions['onProgress'],
): ProgressReporter | undefined {
  if (!onProgress) return undefined;

  return async (message, progress, total) => {
    onProgress(message, progress, total);
  };
}

/**
 * Create a default logger for SDK usage.
 *
 * Returns a quiet logger that only shows warnings and errors.
 * SDK consumers typically don't want verbose logging.
 */
function createSDKLogger(): Logger {
  return createLogger({
    name: 'containerization-sdk',
    level: 'warn',
  });
}

// ===== EXECUTOR =====

/**
 * Execute a tool directly without MCP orchestration.
 *
 * This function provides the core SDK execution path:
 * 1. Ensures knowledge base is loaded (for tools that need it)
 * 2. Creates a minimal ToolContext
 * 3. Parses and validates input via Zod
 * 4. Calls the tool handler
 * 5. Returns the Result directly
 *
 * @param tool - The tool to execute
 * @param input - Tool input parameters (will be validated)
 * @param options - SDK options for customization
 * @returns Promise resolving to Result<TOutput>
 *
 * @example
 * ```typescript
 * import { executeTool } from 'containerization-assist-mcp/sdk';
 * import analyzeRepoTool from '@/tools/analyze-repo/tool';
 *
 * const result = await executeTool(
 *   analyzeRepoTool,
 *   { repositoryPath: './my-app' },
 *   { onProgress: (msg) => console.log(msg) }
 * );
 *
 * if (result.ok) {
 *   console.log('Modules:', result.value.modules);
 * } else {
 *   console.error('Error:', result.error);
 * }
 * ```
 */
export async function executeTool<TSchema extends ZodTypeAny, TOutput>(
  tool: Tool<TSchema, TOutput>,
  input: unknown,
  options: SDKOptions = {},
): Promise<Result<TOutput>> {
  // Create or use provided logger
  const logger = options.logger ?? createSDKLogger();

  // Ensure knowledge is loaded (idempotent)
  knowledgeLoader.ensureLoaded(logger);

  // Create tool context with optional signal and progress reporter
  // Note: SDK v1 does not support policies - keeps things simple
  // Policy support can be added in a future version if needed
  const progressReporter = wrapProgressCallback(options.onProgress);

  // Build options object explicitly (clearer than conditional spread)
  const ctxOptions: ContextOptions = {};
  if (options.signal !== undefined) ctxOptions.signal = options.signal;
  if (progressReporter !== undefined) ctxOptions.progress = progressReporter;

  const ctx = createToolContext(logger, ctxOptions);

  // Parse and validate input using tool's Zod schema via safeParse
  // This avoids try/catch and is more idiomatic with the Result pattern
  const parseResult = tool.schema.safeParse(input);
  if (!parseResult.success) {
    const issues = parseResult.error.issues
      .map((issue) => {
        const path = issue.path.length > 0 ? `${issue.path.join('.')}: ` : '';
        return `${path}${issue.message}`;
      })
      .join('; ');

    return Failure(`Validation failed: ${issues}`, {
      message: 'Input validation failed',
      hint: `Validation issues: ${issues}`,
      resolution: 'Review the tool schema for expected input format',
    });
  }

  const parsedInput = parseResult.data;

  // Execute the tool handler
  return tool.handler(parsedInput, ctx);
}
