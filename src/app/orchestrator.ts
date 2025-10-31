/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import { z, type ZodTypeAny } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { createToolContext, type ToolContext } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { ERROR_MESSAGES } from '@/lib/errors';
import type { ToolOrchestrator, OrchestratorConfig, ExecuteRequest } from './orchestrator-types';
import type { Logger } from 'pino';
import type { Tool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { logToolExecution, createToolLogEntry } from '@/lib/tool-logger';
import { loadAndMergeRegoPolicies, type RegoEvaluator } from '@/config/policy-rego';
import { readdirSync, existsSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';

// ===== Types =====

/**
 * Discover built-in policy files from the policies directory
 * Returns paths to all .rego files (excluding test files)
 *
 * Searches upward from process.cwd() to find the policies directory.
 * Works in both ESM (dist/) and CJS (dist-cjs/) builds.
 */
function discoverBuiltInPolicies(logger: Logger): string[] {
  try {
    // Start from current working directory and search upward
    let currentDir = process.cwd();
    let policiesDir = join(currentDir, 'policies');
    let attempts = 0;
    const maxAttempts = 5;

    // Search upward for policies directory
    while (!existsSync(policiesDir) && attempts < maxAttempts) {
      const parentDir = dirname(currentDir);
      if (parentDir === currentDir) {
        // Reached filesystem root
        break;
      }
      currentDir = parentDir;
      policiesDir = join(currentDir, 'policies');
      attempts++;
    }

    if (!existsSync(policiesDir)) {
      logger.warn({ cwd: process.cwd(), attempts }, 'Built-in policies directory not found');
      return [];
    }

    // Find all .rego files except test files
    const files = readdirSync(policiesDir)
      .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
      .map((file) => resolve(join(policiesDir, file)));

    logger.info({ policiesDir, count: files.length }, 'Discovered built-in policies');
    return files;
  } catch (error) {
    logger.warn({ error }, 'Failed to discover built-in policies');
    return [];
  }
}

/**
 * Create a child logger with additional bindings
 * Assumes Pino logger (fail fast if not)
 */
function childLogger(logger: Logger, bindings: Record<string, unknown>): Logger {
  return logger.child(bindings);
}

/**
 * Create a ToolContext for the given request
 * Delegates to the canonical createToolContext from @mcp/context
 */
function createContextForTool(
  request: ExecuteRequest,
  logger: Logger,
  policy?: RegoEvaluator,
): ToolContext {
  const metadata = request.metadata;

  return createToolContext(logger, {
    ...(metadata?.signal && { signal: metadata.signal }),
    ...(metadata?.progress !== undefined && { progress: metadata.progress }),
    ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
    ...(policy && { policy }),
  });
}

interface ExecutionEnvironment<T extends Tool<ZodTypeAny, any>> {
  registry: Map<string, T>;
  logger: Logger;
  config: OrchestratorConfig;
  server?: Server;
}

/**
 * Create a tool orchestrator
 */
export function createOrchestrator<
  T extends Tool<ZodTypeAny, any> = Tool<ZodTypeAny, any>,
>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, server, config = { chainHintsMode: 'enabled' } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  // Cache the loaded policy to avoid reloading on every execution
  let policyCache: RegoEvaluator | undefined;
  let policyLoadPromise: Promise<void> | undefined;

  async function execute(request: ExecuteRequest): Promise<Result<unknown>> {
    const { toolName } = request;
    const tool = registry.get(toolName);

    if (!tool) {
      return Failure(ERROR_MESSAGES.TOOL_NOT_FOUND(toolName));
    }

    const contextualLogger = childLogger(logger, {
      tool: tool.name,
      ...(request.metadata?.loggerContext ?? {}),
    });

    // Load policies once (with Promise-based guard to prevent race conditions)
    if (!policyLoadPromise) {
      policyLoadPromise = (async () => {
        // Always load built-in policies
        const builtInPolicies = discoverBuiltInPolicies(logger);

        // Optionally add user-provided custom policy
        const policyPaths = config.policyPath
          ? [...builtInPolicies, config.policyPath]
          : builtInPolicies;

        if (policyPaths.length === 0) {
          logger.warn('No policies found (built-in or custom)');
          return;
        }

        const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
        if (policyResult.ok) {
          policyCache = policyResult.value;
          logger.info(
            {
              builtIn: builtInPolicies.length,
              custom: config.policyPath ? 1 : 0,
              total: policyPaths.length,
            },
            'Policies loaded for orchestrator',
          );
        } else {
          logger.warn(
            { error: policyResult.error },
            'Failed to load policies, continuing without them',
          );
        }
      })();
    }

    // Wait for policy loading to complete if in progress
    await policyLoadPromise;

    return await executeWithOrchestration(
      tool,
      request,
      {
        registry,
        logger: contextualLogger,
        config,
        ...(server && { server }),
      },
      policyCache,
    );
  }

  function close(): void {
    // Cleanup policy resources if loaded
    if (policyCache) {
      policyCache.close();
    }
  }

  return { execute, close };
}

/**
 * Execute with full orchestration (dependencies, policies)
 */
async function executeWithOrchestration<T extends Tool<ZodTypeAny, any>>(
  tool: T,
  request: ExecuteRequest,
  env: ExecutionEnvironment<T>,
  policy?: RegoEvaluator,
): Promise<Result<unknown>> {
  const { params } = request;
  const { logger } = env;

  // Validate parameters using Zod safeParse
  const validation = validateParams(params, tool.schema);
  if (!validation.ok) return validation;
  const validatedParams = validation.value;

  const toolContext = createContextForTool(request, logger, policy);
  const tracker = createStandardizedToolTracker(tool.name, {}, logger);

  const startTime = Date.now();
  const logEntry = createToolLogEntry(tool.name, validatedParams);

  // Execute tool directly (single attempt)
  try {
    const result = await tool.handler(validatedParams, toolContext);
    const durationMs = Date.now() - startTime;

    logEntry.output = result.ok ? result.value : { error: result.error };
    logEntry.success = result.ok;
    logEntry.durationMs = durationMs;
    if (!result.ok) {
      logEntry.error = result.error;
      if (result.guidance) {
        logEntry.errorGuidance = result.guidance;
      }
    }

    await logToolExecution(logEntry, logger);

    // Add metadata to successful results
    if (result.ok) {
      let valueWithMessages = result.value;

      if (env.config.chainHintsMode === 'enabled' && tool.chainHints) {
        valueWithMessages = {
          ...valueWithMessages,
          nextSteps: tool.chainHints.success,
        };
      }

      result.value = valueWithMessages;
    } else if (result.guidance && tool.chainHints) {
      // Add failure hint to error guidance
      result.guidance.hint = tool.chainHints.failure;
    }
    tracker.complete({});
    return result;
  } catch (error) {
    const durationMs = Date.now() - startTime;
    const errorMessage = (error as Error).message || 'Unknown error';

    logEntry.output = { error: errorMessage };
    logEntry.success = false;
    logEntry.durationMs = durationMs;
    logEntry.error = errorMessage;

    await logToolExecution(logEntry, logger);

    logger.error({ error: errorMessage }, 'Tool execution failed');
    tracker.fail(error as Error);
    return Failure(errorMessage);
  }
}

/**
 * Validate parameters against schema using safeParse
 */
function validateParams<T extends z.ZodSchema>(params: unknown, schema: T): Result<z.infer<T>> {
  const parsed = schema.safeParse(params);
  if (!parsed.success) {
    const issues = parsed.error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
    return Failure(ERROR_MESSAGES.VALIDATION_FAILED(issues));
  }
  return Success(parsed.data);
}
