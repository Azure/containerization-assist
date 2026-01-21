/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import { z, type ZodTypeAny } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { getModuleUrl } from '@/lib/module-url';
import { resolveModulePaths } from '@/lib/module-path-resolver';
import { createToolContext, type ToolContext, type ContextOptions } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { ERROR_MESSAGES } from '@/lib/errors';
import {
  type ToolOrchestrator,
  type OrchestratorConfig,
  type ExecuteRequest,
  CHAINHINTSMODE,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { Tool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { logToolExecution, createToolLogEntry } from '@/lib/tool-logger';
import { loadAndMergeRegoPolicies, type RegoEvaluator } from '@/config/policy-rego';
import { readdirSync, existsSync, statSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { ENV_VARS } from '@/config/constants';

// Capture import.meta.url at module scope (only available in ESM builds)
// This will be undefined in CJS builds, which is expected
const MODULE_URL = getModuleUrl();

// ===== Types =====

/**
 * Discover built-in policy files from the policies directory
 * Returns paths to all .rego files (excluding test files)
 *
 * Uses shared module path resolver with symlink support.
 * Works in both ESM (dist/) and CJS (dist-cjs/) builds, and when installed via npm.
 */
export function discoverBuiltInPolicies(logger: Logger): string[] {
  try {
    // Use shared path resolver utility
    const searchPaths = resolveModulePaths({
      relativePath: 'policies',
      logger,
      ...(MODULE_URL && { moduleUrl: MODULE_URL }),
    });

    // Try each search path until we find one that exists
    for (const policiesDir of searchPaths) {
      if (existsSync(policiesDir)) {
        // Find all .rego files except test files
        const files = readdirSync(policiesDir)
          .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
          .map((file) => resolve(join(policiesDir, file)));

        if (files.length > 0) {
          logger.debug({ count: files.length, dir: policiesDir }, 'Discovered built-in policies');
          return files;
        }
      }
    }

    return [];
  } catch (error) {
    logger.warn({ error }, 'Failed to discover built-in policies');
    return [];
  }
}

/**
 * Discover policies in policies.user/ directory (source installation)
 * Returns paths to all .rego files (excluding test files)
 */
export function discoverUserPolicies(logger: Logger): string[] {
  try {
    // Search upward for policies.user directory (similar to built-in search)
    let currentDir = process.cwd();
    let policiesUserDir = join(currentDir, 'policies.user');
    let attempts = 0;
    const maxAttempts = 5;

    while (!existsSync(policiesUserDir) && attempts < maxAttempts) {
      const parentDir = dirname(currentDir);
      if (parentDir === currentDir) break;
      currentDir = parentDir;
      policiesUserDir = join(currentDir, 'policies.user');
      attempts++;
    }

    if (!existsSync(policiesUserDir)) {
      return [];
    }

    const files = readdirSync(policiesUserDir)
      .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
      .map((file) => resolve(join(policiesUserDir, file)));

    if (files.length > 0) {
      logger.info(
        { policiesUserDir, count: files.length },
        'Discovered user policies from policies.user/',
      );
    }

    return files;
  } catch (error) {
    logger.warn({ error }, 'Failed to discover policies.user/ policies');
    return [];
  }
}

/**
 * Discover policies in custom directory (NPM installation)
 * Returns paths to all .rego files (excluding test files)
 */
export function discoverCustomPolicies(customPath: string, logger: Logger): string[] {
  try {
    const resolvedPath = resolve(customPath);

    if (!existsSync(resolvedPath)) {
      logger.warn({ path: resolvedPath }, 'Custom policy path does not exist');
      return [];
    }

    const stats = statSync(resolvedPath);

    // If it's a file, return it directly
    if (stats.isFile()) {
      if (resolvedPath.endsWith('.rego')) {
        return [resolvedPath];
      }
      logger.warn({ path: resolvedPath }, 'Custom policy path is not a .rego file');
      return [];
    }

    // If it's a directory, discover all .rego files
    if (stats.isDirectory()) {
      const files = readdirSync(resolvedPath)
        .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
        .map((file) => resolve(join(resolvedPath, file)));
      return files;
    }

    return [];
  } catch (error) {
    logger.warn({ error, path: customPath }, 'Failed to discover custom policies');
    return [];
  }
}

/**
 * Discover policy files with priority-ordered search paths:
 * 1. Built-in policies/ directory (lowest priority)
 * 2. policies.user/ directory (middle priority - source installation users)
 * 3. Custom directory via CUSTOM_POLICY_PATH (highest priority - NPM users)
 *
 * Returns array of policy paths, with higher priority policies later
 * (later policies override earlier ones during merging)
 */
export function discoverPolicies(logger: Logger): string[] {
  const allPolicies: string[] = [];

  // Priority 3 (lowest): Built-in policies
  const builtInPolicies = discoverBuiltInPolicies(logger);
  allPolicies.push(...builtInPolicies);

  // Priority 2: policies.user/ directory (source installation)
  const userPolicies = discoverUserPolicies(logger);
  allPolicies.push(...userPolicies);

  // Priority 1 (highest): Custom directory via env var
  const customPath = process.env[ENV_VARS.CUSTOM_POLICY_PATH];
  if (customPath) {
    const customPolicies = discoverCustomPolicies(customPath, logger);
    if (customPolicies.length > 0) {
      logger.info(
        { path: customPath, count: customPolicies.length },
        'Discovered custom policies from CUSTOM_POLICY_PATH',
      );
      allPolicies.push(...customPolicies);
    }
  }

  return allPolicies;
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

  logger.debug(
    {
      hasPolicy: !!policy,
      toolName: request.toolName,
    },
    'Creating tool context',
  );

  const contextOptions: ContextOptions = {};
  if (metadata?.signal) contextOptions.signal = metadata.signal;

  if (metadata?.progress !== undefined) {
    const progress = metadata.progress;
    if (
      typeof progress === 'string' ||
      typeof progress === 'number' ||
      progress === null ||
      progress === undefined
    ) {
      contextOptions.progress = progress;
    }
  }

  if (metadata?.sendNotification) contextOptions.sendNotification = metadata.sendNotification;
  if (policy) contextOptions.policy = policy;

  return createToolContext(logger, contextOptions);
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
export function createOrchestrator<T extends Tool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, server, config = { chainHintsMode: CHAINHINTSMODE.ENABLED } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  logger.info('createOrchestrator called - policy loading will happen on first tool execution');

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

    logger.info({ toolName, hasPolicyPromise: !!policyLoadPromise }, 'Orchestrator execute called');

    // Load policies once (with Promise-based guard to prevent race conditions)
    if (!policyLoadPromise) {
      policyLoadPromise = (async () => {
        try {
          // Use new priority-ordered discovery (includes built-in, user, and custom policies)
          const policyPaths = discoverPolicies(logger);

          if (policyPaths.length === 0) {
            logger.warn(
              { cwd: process.cwd() },
              'No policies discovered - tools will run without policy constraints',
            );
            return; // Not an error, just no policies available
          }

          logger.info(
            { count: policyPaths.length, paths: policyPaths },
            'Discovered policies, loading...',
          );

          const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);

          if (policyResult.ok) {
            policyCache = policyResult.value;
            logger.info(
              {
                total: policyPaths.length,
                policyPaths,
              },
              'Policies loaded and merged successfully',
            );
          } else {
            logger.error(
              { error: policyResult.error },
              'Failed to load policies - tools will run without policy constraints',
            );
            // Note: We don't throw here, tools can still run without policies
            // But we log clearly that policies are not available
          }
        } catch (error) {
          // Reset promise to allow retry on next execution
          policyLoadPromise = undefined;

          logger.error({ error }, 'Failed to load policies - will retry on next tool execution');

          // Re-throw to signal failure
          throw error;
        }
      })();
    }

    // Wait for policy loading to complete if in progress
    try {
      await policyLoadPromise;
    } catch {
      // Policy loading errors are logged above, continue without policies
      logger.warn('Policy loading failed, continuing tool execution without policies');
    }

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

      if (env.config.chainHintsMode === CHAINHINTSMODE.ENABLED && tool.chainHints) {
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
