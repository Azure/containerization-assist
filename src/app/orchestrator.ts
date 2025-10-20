/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import * as path from 'node:path';
import * as fs from 'node:fs';
import { z, type ZodTypeAny } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadAndMergePolicies } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { createToolContext, type ToolContext } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { ERROR_MESSAGES } from '@/lib/errors';
import type { ToolOrchestrator, OrchestratorConfig, ExecuteRequest } from './orchestrator-types';
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
function createContextForTool(request: ExecuteRequest, logger: Logger): ToolContext {
  const metadata = request.metadata;

  return createToolContext(logger, {
    ...(metadata?.signal && { signal: metadata.signal }),
    ...(metadata?.progress !== undefined && { progress: metadata.progress }),
    ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
  });
}

interface ExecutionEnvironment<T extends MCPTool<ZodTypeAny, any>> {
  policy?: Policy;
  registry: Map<string, T>;
  logger: Logger;
  config: OrchestratorConfig;
  server?: Server;
}

/**
 * Create a tool orchestrator
 */
export function createOrchestrator<T extends MCPTool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, server, config = { chainHintsMode: 'enabled' } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  // Load and merge policies - use defaults if not configured
  let policy: Policy | undefined;
  const policyPaths = config.policyPath ? [config.policyPath] : getDefaultPolicyPaths();

  if (policyPaths.length > 0) {
    const policyResult = loadAndMergePolicies(policyPaths);
    if (policyResult.ok) {
      policy = policyResult.value;
      logger.info(
        {
          policiesLoaded: policyPaths.length,
          totalRules: policy.rules.length,
        },
        'Policies loaded and merged successfully',
      );
    } else {
      logger.warn(
        {
          policyPathsAttempted: policyPaths.length,
          error: policyResult.error,
        },
        'Failed to load policies - orchestrator will run without policy enforcement',
      );
    }
  }

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
      ...(server && { server }),
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

  const toolContext = createContextForTool(request, logger);
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
