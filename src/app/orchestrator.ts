/**
 * Tool Orchestrator
 * Simplified tool execution with optional dependency resolution
 */

import { z } from 'zod';
import { type Result, Success, Failure } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
// Removed unused imports
import type {
  ToolOrchestrator,
  OrchestratorConfig,
  ExecuteRequest,
  SessionState,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { Tool } from '@/types';

/**
 * Create a tool orchestrator
 */
export function createOrchestrator(options: {
  registry: Map<string, Tool>;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, config = {} } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });
  const sessions = new Map<string, SessionState>();

  // Load policy if configured
  let policy: Policy | undefined;
  if (config.policyPath) {
    const policyResult = loadPolicy(config.policyPath, config.policyEnvironment);
    if (policyResult.ok) {
      policy = policyResult.value;
    } else {
      logger.warn(`Failed to load policy: ${policyResult.error}`);
    }
  }

  /**
   * Execute a tool with optional orchestration
   */
  async function execute(request: ExecuteRequest): Promise<Result<unknown>> {
    const { toolName, params, sessionId } = request;
    const tool = registry.get(toolName);

    if (!tool) {
      return Failure(`Tool not found: ${toolName}`);
    }

    if (isSimpleTool(tool, policy, sessionId)) {
      logger.debug(`Executing ${toolName} directly (no orchestration needed)`);
      return await executeSimple(tool, params, logger);
    }

    // Complex case: handle dependencies, policies, sessions
    const context: Parameters<typeof executeWithOrchestration>[2] = {
      sessions,
      registry,
      logger,
      config,
    };
    if (policy) context.policy = policy;
    return await executeWithOrchestration(tool, request, context);
  }

  return { execute };
}

/**
 * Check if a tool can be executed simply without orchestration
 */
function isSimpleTool(tool: Tool, policy?: Policy, sessionId?: string): boolean {
  // Needs orchestration if:
  // 1. Has dependencies (if we add this to Tool interface later)
  // 2. Has complex policy rules
  // 3. Part of a session workflow

  if (sessionId) return false; // Session means stateful workflow

  if (policy) {
    // Check if any policy rules apply to this tool
    const hasComplexPolicy = policy.rules?.some(
      (rule) =>
        rule.conditions?.some((c: any) => c.type === 'tool' && c.value === tool.name) &&
        (rule.actions?.block || rule.actions?.require_approval),
    );
    if (hasComplexPolicy) return false;
  }

  return true;
}

/**
 * Execute a simple tool directly
 */
async function executeSimple(
  tool: Tool,
  params: unknown,
  logger: Logger,
): Promise<Result<unknown>> {
  try {
    // Validate parameters if schema exists
    if (tool.zodSchema) {
      const validation = await validateParams(params, tool.zodSchema);
      if (!validation.ok) return validation;
      params = validation.value;
    }

    // Execute tool directly with logger and no MCP context
    const toolLogger = logger.child({ tool: tool.name });
    return await tool.execute(params as Record<string, unknown>, toolLogger, undefined);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Tool execution failed: ${message}`);
  }
}

/**
 * Execute with full orchestration (dependencies, policies, sessions)
 */
async function executeWithOrchestration(
  tool: Tool,
  request: ExecuteRequest,
  context: {
    sessions: Map<string, SessionState>;
    policy?: Policy;
    registry: Map<string, Tool>;
    logger: Logger;
    config: OrchestratorConfig;
  },
): Promise<Result<unknown>> {
  const { params, sessionId } = request;
  const { sessions, policy, logger, config } = context;

  // Get or create session if needed
  let session: SessionState | undefined;
  if (sessionId) {
    session = sessions.get(sessionId);
    if (!session) {
      session = {
        sessionId,
        created: new Date(),
        updated: new Date(),
        completedSteps: [],
        data: {},
      };
      sessions.set(sessionId, session);
    }
  }

  // Validate parameters
  if (tool.zodSchema) {
    const validation = await validateParams(params, tool.zodSchema);
    if (!validation.ok) return validation;
  }

  // Apply policies
  if (policy) {
    const policyResults = applyPolicy(policy, {
      tool: tool.name,
      params: params as Record<string, unknown>,
    });

    const blockers = policyResults
      .filter((r) => r.matched && r.rule.actions.block)
      .map((r) => r.rule.id);

    if (blockers.length > 0) {
      return Failure(`Blocked by policies: ${blockers.join(', ')}`);
    }
  }

  // Execute with retries
  const maxRetries = config.maxRetries || 2;
  const retryDelay = config.retryDelay || 1000;

  let lastError: Error | null = null;
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      const toolLogger = logger.child({ tool: tool.name, attempt });
      const result = await tool.execute(
        params as Record<string, unknown>,
        toolLogger,
        undefined, // No MCP context in orchestrator
      );

      // Update session if successful
      if (result.ok && session) {
        session.completedSteps.push(tool.name);
        session.data[tool.name] = result.value;
        session.updated = new Date();
      }

      return result;
    } catch (error) {
      lastError = error as Error;
      if (attempt < maxRetries - 1) {
        await new Promise((resolve) => setTimeout(resolve, retryDelay));
      }
    }
  }

  return Failure(`Failed after ${maxRetries} attempts: ${lastError?.message}`);
}

/**
 * Validate parameters against schema
 */
async function validateParams(params: unknown, schema: z.ZodSchema): Promise<Result<unknown>> {
  try {
    const validated = await schema.parseAsync(params);
    return Success(validated);
  } catch (error) {
    if (error instanceof z.ZodError) {
      const issues = error.issues.map((i) => `${i.path.join('.')}: ${i.message}`).join(', ');
      return Failure(`Validation failed: ${issues}`);
    }
    return Failure(`Validation error: ${String(error)}`);
  }
}
