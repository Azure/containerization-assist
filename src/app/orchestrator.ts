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
import { type ToolOrchestrator, type OrchestratorConfig, type ExecuteRequest, CHAINHINTSMODE } from './orchestrator-types';
import type { Logger } from 'pino';
import type { Tool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { logToolExecution, createToolLogEntry } from '@/lib/tool-logger';
import { loadAndMergeRegoPolicies, type RegoEvaluator } from '@/config/policy-rego';
import { readdirSync, existsSync, statSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { ENV_VARS } from '@/config/constants';

// ===== Types =====

/**
 * Discover built-in policy files from the policies directory
 * Returns paths to all .rego files (excluding test files)
 *
 * Searches relative to the module's installation location first,
 * then falls back to searching upward from process.cwd().
 * Works in both ESM (dist/) and CJS (dist-cjs/) builds, and when installed via npm.
 */
export function discoverBuiltInPolicies(logger: Logger): string[] {
  try {
    const searchPaths: string[] = [];

    // 1. First, try relative to the installed module location
    // This ensures policies are found when the package is installed via npm

    // Strategy: Try CJS __dirname first, then fall back to ESM import.meta.url
    // We use try-catch for both because:
    //   - __dirname may throw ReferenceError in strict ESM modules
    //   - import.meta.url may not be available in CJS or Jest

    let modulePathResolved = false;

    // Try CJS approach first (most common in Node.js)
    try {
      // Using Function constructor to bypass TypeScript's static analysis
      // This is safe: the string is a compile-time constant with no user input
      const dirName = new Function('try { return __dirname; } catch { return undefined; }')();
      if (typeof dirName === 'string') {
        const moduleRelativePath = resolve(dirName, '../../../policies');
        searchPaths.push(moduleRelativePath);
        modulePathResolved = true;
      }
    } catch (error) {
      logger.debug({ error }, 'Failed to resolve module path from __dirname');
    }

    // If CJS failed, try ESM approach
    if (!modulePathResolved) {
      try {
        // Using Function constructor to access import.meta.url dynamically
        // This bypasses static analysis issues in Jest and CJS builds
        // The returned value will be undefined in non-ESM environments
        const getMetaUrl = new Function('try { return import.meta.url; } catch { return undefined; }');
        const metaUrl = getMetaUrl();
        if (typeof metaUrl === 'string') {
          const __filename = fileURLToPath(metaUrl);
          const __dirname = dirname(__filename);
          const moduleRelativePath = resolve(__dirname, '../../../policies');
          searchPaths.push(moduleRelativePath);
        }
      } catch (error) {
        logger.debug({ error }, 'Failed to resolve module path from import.meta.url');
      }
    }

    // 2. Then search upward from current working directory (for development)
    let currentDir = process.cwd();
    searchPaths.push(join(currentDir, 'policies'));

    let attempts = 0;
    const maxAttempts = 5;
    while (attempts < maxAttempts) {
      const parentDir = dirname(currentDir);
      if (parentDir === currentDir) {
        // Reached filesystem root
        break;
      }
      currentDir = parentDir;
      searchPaths.push(join(currentDir, 'policies'));
      attempts++;
    }

    // Try each search path until we find one that exists
    for (const policiesDir of searchPaths) {
      if (existsSync(policiesDir)) {
        // Find all .rego files except test files
        const files = readdirSync(policiesDir)
          .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
          .map((file) => resolve(join(policiesDir, file)));

        if (files.length > 0) {
          logger.info({ policiesDir, count: files.length, searchPaths }, 'Discovered built-in policies');
          return files;
        }
      }
    }

    logger.warn({ searchPaths, cwd: process.cwd() }, 'Built-in policies directory not found in any search path');
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
      logger.info({ policiesUserDir, count: files.length }, 'Discovered user policies from policies.user/');
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
export function createOrchestrator<T extends Tool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, server, config = { chainHintsMode: CHAINHINTSMODE.ENABLED } } = options;
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
        // Use new priority-ordered discovery (includes built-in, user, and custom policies)
        const policyPaths = discoverPolicies(logger);

        if (policyPaths.length === 0) {
          logger.warn('No policies discovered');
          return;
        }

        const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
        if (policyResult.ok) {
          policyCache = policyResult.value;
          logger.info(
            {
              total: policyPaths.length,
            },
            'Policies loaded for orchestrator',
          );
        } else {
          logger.warn({ error: policyResult.error }, 'Failed to load policies, continuing without them');
        }
      })();
    }

    // Wait for policy loading to complete if in progress
    await policyLoadPromise;

    return await executeWithOrchestration(tool, request, {
      registry,
      logger: contextualLogger,
      config,
      ...(server && { server }),
    }, policyCache);
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
