/**
 * Tool Orchestrator
 * Tool execution with optional dependency resolution
 */

import { z, type ZodTypeAny } from 'zod';
import * as crypto from 'crypto';
import { type Result, Success, Failure, WorkflowState } from '@/types/index';
import { createLogger } from '@/lib/logger';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
import { createToolContext } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { SessionManager } from '@/session/core';
import { ERROR_MESSAGES } from '@/lib/error-messages';
import type {
  ToolOrchestrator,
  OrchestratorConfig,
  ExecuteRequest,
  SessionFacade,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { Tool } from '@/types/tool';

/**
 * Create a SessionFacade for tool handlers
 */
function createSessionFacade(session: WorkflowState): SessionFacade {
  return {
    id: session.sessionId,
    get<T = unknown>(key: string): T | undefined {
      return session.metadata?.[key] as T | undefined;
    },
    set(key: string, value: unknown): void {
      if (!session.metadata) {
        session.metadata = {};
      }
      session.metadata[key] = value;
      session.updatedAt = new Date();
    },
    pushStep(step: string): void {
      if (!session.completed_steps) {
        session.completed_steps = [];
      }
      if (!session.completed_steps.includes(step)) {
        session.completed_steps.push(step);
        session.updatedAt = new Date();
      }
    },
  };
}

/**
 * Create a tool orchestrator
 */
export function createOrchestrator<T extends Tool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
}): ToolOrchestrator {
  const { registry, server, config = {} } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });
  const sessionManager = new SessionManager(logger, config.session);

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
   * Execute a tool with orchestration
   */
  async function execute(request: ExecuteRequest): Promise<Result<unknown>> {
    const { toolName } = request;
    const tool = registry.get(toolName);

    if (!tool) {
      return Failure(ERROR_MESSAGES.TOOL_NOT_FOUND(toolName));
    }

    // Always use orchestration path to provide consistent session handling
    const context: Parameters<typeof executeWithOrchestration>[2] = {
      sessionManager,
      registry,
      logger,
      config,
      server,
    };
    if (policy) context.policy = policy;
    return await executeWithOrchestration(tool, request, context);
  }

  return { execute };
}

/**
 * Execute with full orchestration (dependencies, policies, sessions)
 */
async function executeWithOrchestration<T extends Tool<ZodTypeAny, any>>(
  tool: T,
  request: ExecuteRequest,
  context: {
    sessionManager: SessionManager;
    policy?: Policy;
    registry: Map<string, T>;
    logger: Logger;
    config: OrchestratorConfig;
    server: Server;
  },
): Promise<Result<unknown>> {
  const { params, sessionId } = request;
  const { sessionManager, policy, logger, config, server } = context;

  // Always create or get session - generate ID if none provided
  const actualSessionId =
    sessionId || `session_${Date.now()}_${crypto.randomBytes(9).toString('hex')}`;

  // Get or create session using SessionManager
  const sessionResult = await sessionManager.get(actualSessionId);
  if (!sessionResult.ok) {
    return Failure(ERROR_MESSAGES.SESSION_GET_FAILED(sessionResult.error));
  }

  let session = sessionResult.value;
  if (!session) {
    // Create new session if it doesn't exist
    const createResult = await sessionManager.create(actualSessionId);
    if (!createResult.ok) {
      return Failure(ERROR_MESSAGES.SESSION_CREATE_FAILED(createResult.error));
    }
    session = createResult.value;
  }

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

  // Execute with retries
  const maxRetries = config.maxRetries || 2;
  const retryDelay = config.retryDelay || 1000;

  let lastError: Error | null = null;
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      const toolLogger = logger.child({ tool: tool.name, attempt });

      // Create session facade for tool handler
      const sessionFacade = createSessionFacade(session);

      // Create a proper context with MCP server and session
      const context = createToolContext(server, toolLogger, {
        session: sessionFacade,
      });
      const result = await tool.run(validatedParams, context);

      // Update session if successful using SessionManager
      if (result.ok) {
        const updatedSession: Partial<WorkflowState> = {
          completed_steps: [...(session.completed_steps || []), tool.name],
          metadata: {
            ...(session.metadata || {}),
            [tool.name]: result.value,
          },
        };

        const updateResult = await sessionManager.update(actualSessionId, updatedSession);
        if (!updateResult.ok) {
          logger.warn(ERROR_MESSAGES.SESSION_UPDATE_FAILED(updateResult.error));
        }
      }

      return result;
    } catch (error) {
      lastError = error as Error;
      if (attempt < maxRetries - 1) {
        await new Promise((resolve) => setTimeout(resolve, retryDelay));
      }
    }
  }

  return Failure(ERROR_MESSAGES.RETRY_EXHAUSTED(maxRetries, lastError?.message || 'Unknown error'));
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
