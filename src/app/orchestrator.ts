/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import * as path from 'node:path';
import * as fs from 'node:fs';
import { z, type ZodTypeAny } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { createToolContext, type ToolContext, type ProgressReporter } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { ERROR_MESSAGES } from '@/lib/error-messages';
import type {
  ToolOrchestrator,
  OrchestratorConfig,
  ExecuteRequest,
  ExecuteMetadata,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { MCPTool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { logToolExecution, createToolLogEntry } from '@/lib/tool-logger';

// ===== Types =====

/**
 * Get default policy paths
 * Returns all .yaml files in the policies/ directory
 */
function getDefaultPolicyPaths(): string[] {
  const logger = createLogger({ name: 'policy-discovery' });
  try {
    const policiesDir = path.join(process.cwd(), 'policies');

    if (!fs.existsSync(policiesDir)) {
      return [];
    }

    const files = fs.readdirSync(policiesDir);
    return files
      .filter((f) => f.endsWith('.yaml') || f.endsWith('.yml'))
      .sort((a, b) => a.localeCompare(b, undefined, { numeric: true })) // Alphabetical sort with numeric awareness for consistent policy load order
      .map((f) => path.join(policiesDir, f));
  } catch (error) {
    logger.warn(
      { error, cwd: process.cwd() },
      'Failed to read policies directory - using no default policies',
    );
    return [];
  }
}

/**
 * Calculate policy strictness score based on rule priorities
 * Higher score = stricter policy (should override less strict ones)
 */
function calculatePolicyStrictness(policy: Policy): number {
  if (policy.rules.length === 0) return 0;

  // Use max priority as the strictness metric
  // This ensures policies with the highest-priority rules take precedence
  return policy.rules.reduce((max, r) => Math.max(max, r.priority), -Infinity);
}

/**
 * Merge multiple policies into a single unified policy
 * Policies are merged in order of strictness (least strict first)
 * so that stricter policies override less strict ones for rules with the same ID
 */
function mergePolicies(policies: Policy[]): Policy {
  if (policies.length === 0) {
    throw new Error('Cannot merge empty policy list');
  }

  // Sort policies by strictness (ascending) so stricter policies come last and override
  const sortedPolicies = [...policies].sort(
    (a, b) => calculatePolicyStrictness(a) - calculatePolicyStrictness(b),
  );

  if (sortedPolicies.length === 1) {
    const singlePolicy = sortedPolicies[0];
    if (!singlePolicy) {
      throw new Error('Unexpected: sorted policies array is empty after validation');
    }
    return singlePolicy;
  }

  const firstPolicy = sortedPolicies[0];
  if (!firstPolicy) {
    throw new Error('Unexpected: sorted policies array is empty after validation');
  }

  // Start with the first policy as base
  const merged: Policy = {
    version: '2.0',
    metadata: {
      ...firstPolicy.metadata,
      name: 'Merged Policy',
      description: `Combined policy from ${sortedPolicies.length} sources`,
    },
    defaults: {},
    rules: [],
  };

  // Add cache if first policy has it
  if (firstPolicy.cache) {
    merged.cache = firstPolicy.cache;
  }

  // Merge defaults (later policies override earlier ones)
  for (const policy of sortedPolicies) {
    if (policy.defaults) {
      merged.defaults = { ...merged.defaults, ...policy.defaults };
    }
  }

  // Merge rules using a Map to handle duplicates
  // Rules from stricter policies (later in sorted list) override earlier ones
  const rulesMap = new Map<string, (typeof merged.rules)[0]>();

  for (const policy of sortedPolicies) {
    for (const rule of policy.rules) {
      rulesMap.set(rule.id, rule);
    }
  }

  // Convert back to array and sort by priority descending
  merged.rules = Array.from(rulesMap.values()).sort((a, b) => b.priority - a.priority);

  return merged;
}

function childLogger(logger: Logger, bindings: Record<string, unknown>): Logger {
  const candidate = (logger as unknown as { child?: (bindings: Record<string, unknown>) => Logger })
    .child;
  return typeof candidate === 'function' ? candidate.call(logger, bindings) : logger;
}

type ContextFactoryInput<T extends MCPTool<ZodTypeAny, any>> = {
  tool: T;
  request: ExecuteRequest;
  logger: Logger;
};

type ContextFactory<T extends MCPTool<ZodTypeAny, any>> = (
  input: ContextFactoryInput<T>,
) => Promise<ToolContext> | ToolContext;

interface ExecutionEnvironment<T extends MCPTool<ZodTypeAny, any>> {
  policy?: Policy;
  registry: Map<string, T>;
  logger: Logger;
  config: OrchestratorConfig;
  buildContext: ContextFactory<T>;
}

/**
 * Create a tool orchestrator
 */
export function createOrchestrator<T extends MCPTool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
  contextFactory?: ContextFactory<T>;
}): ToolOrchestrator {
  const { registry, config = { chainHintsMode: 'enabled' } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  // Load and merge policies - use defaults if not configured
  let policy: Policy | undefined;
  const policyPaths = config.policyPath ? [config.policyPath] : getDefaultPolicyPaths();

  if (policyPaths.length > 0) {
    // Load all policies
    const loadedPolicies: Policy[] = [];
    for (const policyPath of policyPaths) {
      const policyResult = loadPolicy(policyPath);
      if (policyResult.ok) {
        loadedPolicies.push(policyResult.value);
        logger.debug({ policyPath }, 'Policy loaded successfully');
      } else {
        logger.warn({ policyPath, error: policyResult.error }, 'Failed to load policy');
      }
    }

    // Merge all loaded policies
    if (loadedPolicies.length > 0) {
      policy = mergePolicies(loadedPolicies);
      logger.info(
        {
          policiesLoaded: loadedPolicies.length,
          totalRules: policy.rules.length,
        },
        'Policies merged successfully',
      );
    } else {
      logger.warn(
        {
          policyPathsAttempted: policyPaths.length,
        },
        'All policy files failed to load - orchestrator will run without policy enforcement',
      );
    }
  }

  const buildContext: ContextFactory<T> = async (input) => {
    if (options.contextFactory) {
      return options.contextFactory(input);
    }

    const metadata = input.request.metadata;

    if (options.server) {
      const contextOptions = {
        ...(metadata?.signal && { signal: metadata.signal }),
        ...(metadata?.progress !== undefined && { progress: metadata.progress }),
        ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
      };
      return createToolContext(input.logger, contextOptions);
    }

    return createHostlessToolContext(input.logger, {
      ...(metadata && { metadata }),
    });
  };

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

    return await executeWithOrchestration(tool, request, {
      registry,
      ...(policy && { policy }),
      logger: contextualLogger,
      config,
      buildContext,
    });
  }

  function close(): void {
    // No cleanup needed anymore
  }

  return { execute, close };
}

/**
 * Execute with full orchestration (dependencies, policies)
 */
async function executeWithOrchestration<T extends MCPTool<ZodTypeAny, any>>(
  tool: T,
  request: ExecuteRequest,
  env: ExecutionEnvironment<T>,
): Promise<Result<unknown>> {
  const { params } = request;
  const { policy, logger } = env;

  // Validate parameters using Zod safeParse
  const validation = validateParams(params, tool.schema);
  if (!validation.ok) return validation;
  const validatedParams = validation.value;

  // Apply policies
  if (policy) {
    const policyResults = applyPolicy(policy, {
      tool: tool.name,
      params: validatedParams as Record<string, unknown>,
    });

    const blockers = policyResults
      .filter((r) => r.matched && r.rule.actions.block)
      .map((r) => r.rule.id);

    if (blockers.length > 0) {
      return Failure(ERROR_MESSAGES.POLICY_BLOCKED(blockers));
    }
  }

  const toolContext = await env.buildContext({
    tool,
    request,
    logger,
  });
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

      // Look up chain hints from application-level registry
      const chainHints = env.config.chainHints?.[tool.name];
      if (env.config.chainHintsMode === 'enabled' && chainHints) {
        valueWithMessages = {
          ...valueWithMessages,
          nextSteps: chainHints.success,
        };
      }

      result.value = valueWithMessages;
    } else if (result.guidance) {
      // Add failure hint to error guidance from application-level registry
      const chainHints = env.config.chainHints?.[tool.name];
      if (chainHints) {
        result.guidance.hint = chainHints.failure;
      }
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

export function createHostlessToolContext(
  logger: Logger,
  options: {
    metadata?: ExecuteMetadata;
  },
): ToolContext {
  const progress = coerceProgressReporter(options.metadata?.progress);

  return {
    logger,
    signal: options.metadata?.signal,
    progress,
  };
}

/**
 * Coerce a progress reporter function to the expected interface
 * @public
 */
export function coerceProgressReporter(progress: unknown): ProgressReporter | undefined {
  if (typeof progress !== 'function') {
    return undefined;
  }

  return async (message: string, current?: number, total?: number) => {
    const maybePromise = progress(message, current, total);
    if (maybePromise && typeof (maybePromise as Promise<void>).then === 'function') {
      await maybePromise;
    }
  };
}
