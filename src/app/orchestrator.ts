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
import { createToolContext, type ToolContext, type ProgressReporter } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { SessionManager } from '@/session/core';
import { ERROR_MESSAGES } from '@/lib/error-messages';
import type {
  ToolOrchestrator,
  OrchestratorConfig,
  ExecuteRequest,
  SessionFacade,
  ExecuteMetadata,
} from './orchestrator-types';
import type { Logger } from 'pino';
import type { MCPTool } from '@/types/tool';
import { createStandardizedToolTracker } from '@/lib/tool-helpers';
import { TOOL_NAME, ToolName } from '@/tools';
import { checkSamplingAvailability, type SamplingCheckResult } from '@/mcp/sampling-check';
import { logToolExecution, type ToolLogEntry } from '@/lib/tool-logger';

// ===== Types =====

function childLogger(logger: Logger, bindings: Record<string, unknown>): Logger {
  const candidate = (logger as unknown as { child?: (bindings: Record<string, unknown>) => Logger })
    .child;
  return typeof candidate === 'function' ? candidate.call(logger, bindings) : logger;
}

type ContextFactoryInput<T extends MCPTool<ZodTypeAny, any>> = {
  tool: T;
  request: ExecuteRequest;
  session: WorkflowState;
  sessionFacade: SessionFacade;
  logger: Logger;
  sessionManager: SessionManager;
};

type ContextFactory<T extends MCPTool<ZodTypeAny, any>> = (
  input: ContextFactoryInput<T>,
) => Promise<ToolContext> | ToolContext;

interface ExecutionEnvironment<T extends MCPTool<ZodTypeAny, any>> {
  sessionManager: SessionManager;
  policy?: Policy;
  registry: Map<string, T>;
  logger: Logger;
  config: OrchestratorConfig;
  buildContext: ContextFactory<T>;
}

/**
 * Create a SessionFacade for tool handlers
 */
function createSessionFacade(session: WorkflowState): SessionFacade {
  return {
    id: session.sessionId,
    get<T = unknown>(key: string): T | undefined {
      return session[key] as T | undefined;
    },
    set(key: string, value: unknown): void {
      session[key] = value;
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
export function createOrchestrator<T extends MCPTool<ZodTypeAny, any>>(options: {
  registry: Map<string, T>;
  server?: Server;
  logger?: Logger;
  config?: OrchestratorConfig;
  sessionManager?: SessionManager;
  contextFactory?: ContextFactory<T>;
}): ToolOrchestrator {
  const { registry, config = { chainHintsMode: 'enabled' } } = options;
  const logger = options.logger || createLogger({ name: 'orchestrator' });

  // Session manager is always enabled for single-session mode
  const sessionManager = options.sessionManager ?? new SessionManager(logger, config.session);
  const ownsSessionManager = !options.sessionManager;

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

  const buildContext: ContextFactory<T> = async (input) => {
    if (options.contextFactory) {
      return options.contextFactory({ ...input, sessionManager });
    }

    const metadata = input.request.metadata;

    if (options.server) {
      const contextOptions = {
        sessionManager,
        session: input.sessionFacade,
        ...(metadata?.signal && { signal: metadata.signal }),
        ...(metadata?.progress !== undefined && { progress: metadata.progress }),
        ...(metadata?.maxTokens !== undefined && { maxTokens: metadata.maxTokens }),
        ...(metadata?.stopSequences && { stopSequences: metadata.stopSequences }),
        ...(metadata?.sendNotification && { sendNotification: metadata.sendNotification }),
      };
      return createToolContext(options.server, input.logger, contextOptions);
    }

    return createHostlessToolContext(input.logger, {
      sessionManager,
      sessionFacade: input.sessionFacade,
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
      ...(request.sessionId ? { sessionId: request.sessionId } : {}),
      ...(request.metadata?.loggerContext ?? {}),
    });

    return await executeWithOrchestration(tool, request, {
      registry,
      sessionManager,
      ...(policy && { policy }),
      logger: contextualLogger,
      config,
      buildContext,
    });
  }

  function close(): void {
    if (!ownsSessionManager) return;
    const closable = sessionManager as SessionManager & { close?: () => void };
    if (typeof closable.close === 'function') {
      closable.close();
    }
  }

  return { execute, close };
}

/**
 * Execute with full orchestration (dependencies, policies, sessions)
 */
async function executeWithOrchestration<T extends MCPTool<ZodTypeAny, any>>(
  tool: T,
  request: ExecuteRequest,
  env: ExecutionEnvironment<T>,
): Promise<Result<unknown>> {
  const { params, sessionId } = request;
  const { sessionManager, policy, logger } = env;

  // Extract sessionId from request or params (tools often pass sessionId in params)
  const paramsSessionId =
    params && typeof params === 'object' && 'sessionId' in params
      ? (params as { sessionId?: string }).sessionId
      : undefined;

  // Always create or get session - generate ID if none provided
  // Prefer request.sessionId, then params.sessionId, then generate new
  const actualSessionId =
    sessionId ||
    paramsSessionId ||
    `session_${Date.now()}_${crypto.randomBytes(9).toString('hex')}`;

  logger.debug(
    {
      toolName: tool.name,
      requestSessionId: sessionId,
      paramsSessionId,
      actualSessionId,
    },
    'Resolved session ID for tool execution',
  );

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

  const sessionFacade = createSessionFacade(session);
  const toolContext = await env.buildContext({
    tool,
    request,
    session,
    sessionFacade,
    logger,
    sessionManager,
  });
  const tracker = createStandardizedToolTracker(tool.name, { sessionId }, logger);

  // Check sampling availability for tools that use AI-driven sampling
  let samplingCheckResult: SamplingCheckResult | undefined;

  //Later on this can be cached in the session to avoid multiple checks
  // Avoiding caching to reduce complexity for now
  if (tool.metadata?.samplingStrategy === 'single') {
    samplingCheckResult = await checkSamplingAvailability(toolContext);
  }
    
  const startTime = Date.now();
  const logEntry: ToolLogEntry = {
    timestamp: new Date().toISOString(),
    toolName: tool.name,
    sessionId: actualSessionId,
    input: validatedParams,
    output: null,
    success: false,
  };

  // Execute tool directly (single attempt)
  try {
    const result = await tool.run(validatedParams, toolContext);
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

    // Update session if successful using SessionManager
    if (result.ok) {
      sessionFacade.pushStep(tool.name);

      // Persist the updated session
      const updateResult = await sessionManager.update(actualSessionId, session);
      if (!updateResult.ok) {
        logger.warn(ERROR_MESSAGES.SESSION_UPDATE_FAILED(updateResult.error));
      }

      // Add sampling warning message if not available (only for sampling-enabled tools)
      let valueWithMessages = result.value;
      if (samplingCheckResult && !samplingCheckResult.available && samplingCheckResult.message) {
        valueWithMessages = {
          ...result.value,
          _samplingWarning: samplingCheckResult.message,
        };
      }

      if (env.config.chainHintsMode === 'enabled') {
        const hint = getChainHint(tool.name as ToolName, result.ok ? 'success' : 'failure');
        if (hint) {
          valueWithMessages = {
            ...valueWithMessages,
            nextSteps: hint,
          };
        }
      }

      result.value = valueWithMessages;
    }
    tracker.complete({
      sessionId,
    });
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

type outputState = 'success' | 'failure';
function getChainHint(toolName: ToolName, outputState: outputState): string {
  const stateToHints = chainHintMap.get(toolName);
  if (!stateToHints) {
    return '';
  }

  const hint = stateToHints[outputState];
  return hint || '';
}

const chainHintMap = new Map<ToolName, { success: string; failure: string }>([
  [
    TOOL_NAME.ANALYZE_REPO,
    {
      success: `Repository analysis completed successfully. Continue by calling the ${TOOL_NAME.GENERATE_DOCKERFILE_PLAN} tool for each identified module.`,
      failure: 'Repository analysis failed. Please check the logs for details.',
    },
  ],
  [
    TOOL_NAME.GENERATE_DOCKERFILE_PLAN,
    {
      success: `Dockerfile plan generated successfully. Continue by calling the ${TOOL_NAME.VALIDATE_DOCKERFILE} tool to ensure best practices are followed.`,
      failure: 'Dockerfile plan generation failed. Please check the logs for details.',
    },
  ],
  [
    TOOL_NAME.VALIDATE_DOCKERFILE,
    {
      success: `Dockerfile validated successfully. Continue by building this Dockerfile. Continue trying to build and fix until it builds successfully. Then call the ${TOOL_NAME.GENERATE_MANIFEST_PLAN} tool to proceed with containerization.`,
      failure:
        'Dockerfile validation failed. Please update the Dockerfile and retry this tool until it passes.',
    },
  ],
  [
    TOOL_NAME.GENERATE_MANIFEST_PLAN,
    {
      success:
        'Manifest plan generated successfully. Continue by generating manifests leveraging plan output.',
      failure: 'Manifest plan generation failed. Please check the logs for details.',
    },
  ],
]);

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
    sessionManager: SessionManager;
    sessionFacade: SessionFacade;
    metadata?: ExecuteMetadata;
  },
): ToolContext {
  const progress = coerceProgressReporter(options.metadata?.progress);

  return {
    sampling: {
      async createMessage(): Promise<never> {
        throw new Error(
          'Sampling is unavailable without an MCP transport. Start or bind to an MCP server to enable sampling.',
        );
      },
    },
    getPrompt: async (name: string): Promise<never> => {
      throw new Error(
        `Prompt '${name}' requested but no MCP transport is bound. Start or bind to an MCP server to access prompts.`,
      );
    },
    logger,
    sessionManager: options.sessionManager,
    session: options.sessionFacade,
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
