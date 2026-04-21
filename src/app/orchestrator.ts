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
import { join, resolve } from 'node:path';
import os from 'node:os';
import {
  ENV_VARS,
  POLICY_GLOBAL_APP_NAME,
  POLICY_LEGACY_DIR,
  POLICY_PROJECT_DIR,
  POLICY_SUBDIR,
} from '@/config/constants';

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

export interface DiscoveredPolicy {
  path: string;
  source: 'built-in' | 'global' | 'project' | 'legacy' | 'custom';
}

let deprecationWarned = false;

export function discoverGlobalPolicies(logger: Logger): string[] {
  try {
    const xdgConfigHome = process.env.XDG_CONFIG_HOME || join(os.homedir(), '.config');
    const globalPolicyDir = join(xdgConfigHome, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);

    if (!existsSync(globalPolicyDir)) {
      return [];
    }

    return readdirSync(globalPolicyDir)
      .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
      .map((file) => resolve(join(globalPolicyDir, file)));
  } catch (error) {
    logger.warn({ error }, 'Failed to discover global policies');
    return [];
  }
}

export function discoverProjectPolicies(logger: Logger, workspacePath?: string): string[] {
  try {
    const baseDir = workspacePath || process.cwd();
    const projectPolicyDir = join(baseDir, POLICY_PROJECT_DIR, POLICY_SUBDIR);
    if (!existsSync(projectPolicyDir)) {
      return [];
    }

    return readdirSync(projectPolicyDir)
      .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
      .map((file) => resolve(join(projectPolicyDir, file)));
  } catch (error) {
    logger.warn({ error }, 'Failed to discover project policies');
    return [];
  }
}

export function discoverUserPolicies(logger: Logger, workspacePath?: string): string[] {
  try {
    const baseDir = workspacePath || process.cwd();
    const policiesUserDir = join(baseDir, POLICY_LEGACY_DIR);

    if (!existsSync(policiesUserDir)) {
      return [];
    }

    const files = readdirSync(policiesUserDir)
      .filter((file) => file.endsWith('.rego') && !file.endsWith('_test.rego'))
      .map((file) => resolve(join(policiesUserDir, file)));

    if (files.length > 0) {
      logger.info(
        { policiesUserDir, count: files.length },
        'Discovered user policies from legacy directory',
      );

      if (!deprecationWarned) {
        logger.warn(
          'policies.user/ is deprecated. Move policies to .containerization-assist/policy/ at your project root, or ~/.config/containerization-assist/policy/ for global policies.',
        );
        deprecationWarned = true;
      }
    }

    return files;
  } catch (error) {
    logger.warn({ error }, 'Failed to discover legacy user policies');
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

export interface PolicySearchPath {
  path: string;
  source: DiscoveredPolicy['source'];
  exists: boolean;
}

/**
 * Return all policy search directories with existence status.
 * Unlike discoverPolicies (which scans for .rego files), this returns
 * the directories themselves so users can see where to place policies.
 */
export function getPolicySearchPaths(logger: Logger, workspacePath?: string): PolicySearchPath[] {
  const paths: PolicySearchPath[] = [];

  // Built-in policies directory
  const builtInSearchPaths = resolveModulePaths({
    relativePath: 'policies',
    logger,
    ...(MODULE_URL && { moduleUrl: MODULE_URL }),
  });
  const builtInDir = builtInSearchPaths.find((p) => existsSync(p)) ?? builtInSearchPaths[0];
  if (builtInDir) {
    paths.push({ path: builtInDir, source: 'built-in', exists: existsSync(builtInDir) });
  }

  // Global policies directory
  try {
    const xdgConfigHome = process.env.XDG_CONFIG_HOME || join(os.homedir(), '.config');
    const globalPolicyDir = join(xdgConfigHome, POLICY_GLOBAL_APP_NAME, POLICY_SUBDIR);
    paths.push({ path: globalPolicyDir, source: 'global', exists: existsSync(globalPolicyDir) });
  } catch {
    // os.homedir() can throw on misconfigured systems — skip global path
  }

  // Project policies directory
  const projectBaseDir = workspacePath || process.cwd();
  const projectPolicyDir = join(projectBaseDir, POLICY_PROJECT_DIR, POLICY_SUBDIR);
  paths.push({ path: projectPolicyDir, source: 'project', exists: existsSync(projectPolicyDir) });

  // Legacy policies directory (workspacePath only, no walk-up)
  const legacyBaseDir = workspacePath || process.cwd();
  const legacyDir = join(legacyBaseDir, POLICY_LEGACY_DIR);
  if (existsSync(legacyDir)) {
    paths.push({ path: legacyDir, source: 'legacy', exists: true });
  }

  // Custom policy path (env var)
  const customPath = process.env[ENV_VARS.CUSTOM_POLICY_PATH];
  if (customPath) {
    const resolvedCustom = resolve(customPath);
    paths.push({ path: resolvedCustom, source: 'custom', exists: existsSync(resolvedCustom) });
  }

  return paths;
}
export function discoverPolicies(logger: Logger, workspacePath?: string): DiscoveredPolicy[] {
  const allPolicies: DiscoveredPolicy[] = [];

  // Priority 3 (lowest): Built-in policies
  const builtInPolicies = discoverBuiltInPolicies(logger).map((policyPath) => ({
    path: policyPath,
    source: 'built-in' as const,
  }));
  allPolicies.push(...builtInPolicies);

  const globalPolicies = discoverGlobalPolicies(logger).map((policyPath) => ({
    path: policyPath,
    source: 'global' as const,
  }));
  allPolicies.push(...globalPolicies);

  const projectPolicies = discoverProjectPolicies(logger, workspacePath).map((policyPath) => ({
    path: policyPath,
    source: 'project' as const,
  }));
  allPolicies.push(...projectPolicies);

  const userPolicies = discoverUserPolicies(logger, workspacePath).map((policyPath) => ({
    path: policyPath,
    source: 'legacy' as const,
  }));
  allPolicies.push(...userPolicies);

  const customPath = process.env[ENV_VARS.CUSTOM_POLICY_PATH];
  if (customPath) {
    const customPolicies = discoverCustomPolicies(customPath, logger).map((policyPath) => ({
      path: policyPath,
      source: 'custom' as const,
    }));
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

export function discoverPolicyPaths(logger: Logger, workspacePath?: string): string[] {
  return discoverPolicies(logger, workspacePath).map((policy) => policy.path);
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

  // Policy cache with change detection
  // Re-discovers policy files on each execution, but only reloads if the set of files changed
  // Fingerprint includes file paths + mtime/size so in-place edits are detected
  let policyCache: RegoEvaluator | undefined;
  let cachedPolicyFingerprint: string | undefined;
  let policyLoadPromise: Promise<void> | undefined;
  let policyLoadGeneration = 0;

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

    // Extract workspacePath from params (with repositoryPath fallback for project policy discovery)
    const params = request.params as Record<string, unknown> | undefined;
    const workspacePath = (params?.workspacePath as string) ?? (params?.repositoryPath as string) ?? undefined;

    // Discover policies on every execution and reload if files changed
    const discoveredPolicies = discoverPolicies(logger, workspacePath);
    const policyPaths = discoveredPolicies.map((policy) => policy.path);
    // Build fingerprint from paths + file metadata (mtime/size) so in-place edits are detected
    const fileMeta = policyPaths.map((p) => {
      try {
        const s = statSync(p);
        return `${p}:${s.mtimeMs}:${s.size}`;
      } catch {
        return `${p}:missing`;
      }
    });
    const fingerprint = [workspacePath ?? '', ...fileMeta.sort()].join('\n');
    const policiesChanged = fingerprint !== cachedPolicyFingerprint;

    if (policiesChanged) {
      // Reset cached state so we reload
      if (policyCache) {
        policyCache.close();
        policyCache = undefined;
      }
      policyLoadPromise = undefined;
      cachedPolicyFingerprint = fingerprint;

      if (policyPaths.length === 0) {
        logger.info(
          { cwd: process.cwd() },
          'No policies discovered - tools will run without policy constraints',
        );
      } else {
        logger.info(
          {
            count: policyPaths.length,
            paths: policyPaths,
            sources: discoveredPolicies.map((policy) => policy.source),
          },
          'Policy files changed, reloading...',
        );

        const thisGeneration = ++policyLoadGeneration;

        policyLoadPromise = (async () => {
          try {
            const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);

            // Only write cache if no newer generation has started
            if (thisGeneration !== policyLoadGeneration) {
              logger.info('Policy load superseded by newer generation, discarding result');
              if (policyResult.ok) {
                policyResult.value.close();
              }
              return;
            }

            if (policyResult.ok) {
              policyCache = policyResult.value;
              logger.info(
                { total: policyPaths.length, policyPaths },
                'Policies loaded and merged successfully',
              );
            } else {
              // Reset fingerprint so next execution retries the load
              cachedPolicyFingerprint = undefined;
              logger.error(
                { error: policyResult.error },
                'Failed to load policies - will retry on next tool execution',
              );
            }
          } catch (error) {
            // Reset fingerprint to allow retry on next execution
            cachedPolicyFingerprint = undefined;
            policyLoadPromise = undefined;

            logger.error({ error }, 'Failed to load policies - will retry on next tool execution');
            throw error;
          }
        })();
      }
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
