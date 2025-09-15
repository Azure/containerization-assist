/**
 * Orchestrates tool execution with automatic dependency resolution.
 *
 * Trade-off: Automatic vs manual dependency management - chose automatic for UX
 * Invariant: Tools execute in dependency order unless force=true
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result, type WorkflowState } from '@types';
import type { SessionManager } from '@lib/session';
import {
  type Step,
  type ToolEdge,
  getToolEdge,
  getMissingPreconditions,
  getExecutionOrder,
} from './tool-graph';
import {
  createHostAIAssistant,
  mergeWithSuggestions,
  type HostAIAssistant,
} from './ai/host-ai-assist';
import type { z } from 'zod';

/**
 * Minimal tool interface required for routing and execution
 */
export interface RouterTool {
  name: string;
  handler: (params: Record<string, unknown>, context: ToolContext) => Promise<Result<unknown>>;
  schema?: z.ZodObject<z.ZodRawShape> | z.ZodType<unknown>;
}

export interface RouterConfig {
  sessionManager: SessionManager;
  logger: Logger;
  tools: Map<string, RouterTool>;
  /** Optional AI assistance for missing parameter inference */
  aiAssistant?: HostAIAssistant;
}

export interface RouteRequest {
  toolName: string;
  params: Record<string, unknown>;
  /** Force execution even if effects already satisfied (idempotency override) */
  force?: boolean;
  sessionId?: string;
  context?: ToolContext;
}

export interface WorkflowMetadata {
  nextTool: string;
  description: string;
  params: Record<string, unknown>;
  sessionId: string;
  alternatives?: Array<{
    tool: string;
    description: string;
    params?: Record<string, unknown>;
  }>;
}

export interface WorkflowHint {
  message: string;
  markdown: string;
  ready: boolean;
}

export interface RouteResult {
  result: Result<unknown>;
  executedTools: string[];
  sessionState: WorkflowState;
  workflowMetadata?: WorkflowMetadata;
  workflowHint?: WorkflowHint;
}

/**
 * Router state interface for managing tool execution context
 */
export interface ToolRouterState {
  sessionManager: SessionManager;
  logger: Logger;
  tools: Map<string, RouterTool>;
  aiAssistant: HostAIAssistant;
}

/**
 * Tool router interface for dependency injection compatibility
 */
export interface ToolRouter {
  route(request: RouteRequest): Promise<RouteResult>;
  getToolDependencies(toolName: string): { requires: Step[]; provides: Step[] };
  canExecute(
    toolName: string,
    sessionId: string,
  ): Promise<{ canExecute: boolean; missingSteps: Step[] }>;
  getExecutionPlan(toolName: string, completedSteps?: Set<Step>): string[];
}

/**
 * Extracts session context from WorkflowState for tool parameter resolution.
 *
 * Invariant: Only uses session.results pattern - no legacy field fallbacks for clean architecture
 */
export const extractSessionContext = (session: WorkflowState): Record<string, unknown> => {
  const analysis = (session.results?.['analyze_repo'] as Record<string, unknown>) || {};
  return {
    technology: analysis.technology,
    language: analysis.language,
    framework: analysis.framework,
    runtime: analysis.runtime,
    packageManager: analysis.packageManager,
  };
};

/**
 * Normalizes tool parameters with path aliasing for backward compatibility.
 *
 * Trade-off: Supports legacy parameter names (repoPath, context) to maintain API compatibility
 */
export const normalizeToolParameters = (
  params: Record<string, unknown>,
  session?: WorkflowState,
): Record<string, unknown> => {
  const normalized: Record<string, unknown> = {
    ...params,
    path: params.path || params.repoPath || params.context || '.',
  };

  // Merge session context if available
  if (session) {
    Object.assign(normalized, extractSessionContext(session));
  }

  return normalized;
};

/**
 * Performs atomic session updates with conflict resolution.
 *
 * Postcondition: Session contains merged results from concurrent updates
 * Failure Mode: Returns latest session state if update fails to prevent crashes
 */
export const updateSessionState = async (
  sessionManager: SessionManager,
  logger: Logger,
  sessionId: string,
  updates: Partial<WorkflowState>,
): Promise<WorkflowState> => {
  // Merge results to handle concurrent updates
  if (updates.results) {
    const currentSessionResult = await sessionManager.get(sessionId);
    if (currentSessionResult.ok && currentSessionResult.value?.results) {
      // Merge the results objects to preserve concurrent updates
      updates.results = {
        ...currentSessionResult.value.results,
        ...updates.results,
      };
    }
  }

  const updateData = {
    ...updates,
    updatedAt: new Date(),
  };

  const updateResult = await sessionManager.update(sessionId, updateData);

  if (!updateResult.ok) {
    logger.warn({ sessionId, error: updateResult.error }, 'Session update failed');
    const fallbackResult = await sessionManager.get(sessionId);
    if (fallbackResult.ok && fallbackResult.value) {
      return fallbackResult.value;
    }
    // Minimal session fallback to prevent crashes
    return { sessionId, updatedAt: new Date() } as WorkflowState;
  }

  return updateResult.value;
};

/**
 * Executes tool with automatic dependency resolution.
 *
 * Postcondition: Session contains all completed steps and tool results
 * Failure Mode: Returns partial execution list on failure for debugging
 */
export const routeRequestImpl = async (
  state: ToolRouterState,
  request: RouteRequest,
): Promise<RouteResult> => {
  const { toolName, params, force = false, sessionId, context } = request;

  let session: WorkflowState | null = null;

  if (sessionId) {
    const getResult = await state.sessionManager.get(sessionId);
    if (getResult.ok) {
      session = getResult.value;
    }

    // Create session with explicit ID for workflow continuity
    if (!session) {
      state.logger.debug({ sessionId }, 'Session not found, creating new session with ID');
      const createResult = await state.sessionManager.create(sessionId);
      if (createResult.ok) {
        session = createResult.value;
      }
    }
  } else {
    const createResult = await state.sessionManager.create();
    if (createResult.ok) {
      session = createResult.value;
    }
  }

  if (!session) {
    return {
      result: Failure('Failed to get or create session'),
      executedTools: [],
      sessionState: {} as WorkflowState,
    };
  }

  const executedTools: string[] = [];

  try {
    const completedSteps = new Set<Step>((session.completed_steps || []) as Step[]);

    const edge = getToolEdge(toolName);

    state.logger.debug(
      {
        toolName,
        hasEdge: !!edge,
      },
      'Checking tool dependencies',
    );

    if (!edge) {
      // Direct execution for tools without dependencies
      state.logger.debug({ toolName }, 'Tool has no dependencies, executing directly');
      const result = await executeToolImpl(state, toolName, params, session, context);
      if (result.ok) {
        executedTools.push(toolName);
        // Fetch updated session after tool execution
        const updatedSessionResult = await state.sessionManager.get(session.sessionId);
        if (updatedSessionResult.ok && updatedSessionResult.value) {
          session = updatedSessionResult.value;
        }
      }
      return { result, executedTools, sessionState: session };
    }

    if (!force) {
      // Idempotency check prevents redundant work and maintains workflow efficiency
      const alreadySatisfied = edge.provides?.every((step) => completedSteps.has(step)) ?? false;

      if (alreadySatisfied) {
        state.logger.debug({ toolName }, 'Tool effects already satisfied, skipping execution');
        return {
          result: Success({ skipped: true, reason: 'Effects already satisfied' }),
          executedTools: [],
          sessionState: session,
        };
      }

      const missingSteps = getMissingPreconditions(toolName, completedSteps);

      state.logger.debug(
        {
          toolName,
          missingCount: missingSteps.length,
        },
        'Precondition check',
      );

      if (missingSteps.length > 0) {
        state.logger.debug(
          { toolName, missingCount: missingSteps.length },
          'Tool has missing preconditions, auto-running corrective tools',
        );

        // Execute dependencies in topological order
        const executionOrder = getExecutionOrder(missingSteps, completedSteps);

        for (const { tool: correctiveTool, step } of executionOrder) {
          // Skip if this step is already completed
          if (completedSteps.has(step)) {
            state.logger.debug(
              { correctiveTool, step, completedSteps: Array.from(completedSteps) },
              'Step already completed, skipping',
            );
            continue;
          }

          state.logger.debug(
            { correctiveTool, step, completedSteps: Array.from(completedSteps) },
            'Step not completed, executing tool',
          );

          state.logger.debug({ correctiveTool, step }, 'Running corrective tool');

          const correctiveParams = buildCorrectiveParams(
            toolName,
            correctiveTool,
            step,
            params,
            session,
          );

          state.logger.debug(
            {
              correctiveTool,
              step,
            },
            'Running corrective tool',
          );

          const result = await executeToolImpl(
            state,
            correctiveTool,
            correctiveParams,
            session,
            context,
          );

          if (!result.ok) {
            // Preserve progress on failure
            const failedSession = await updateSessionState(
              state.sessionManager,
              state.logger,
              session.sessionId,
              {
                completed_steps: Array.from(completedSteps),
              },
            );

            return {
              result: Failure(
                `Failed to satisfy precondition '${step}' with tool '${correctiveTool}': ${result.error}`,
              ),
              executedTools,
              sessionState: failedSession || session,
            };
          }

          executedTools.push(correctiveTool);
          completedSteps.add(step);

          const correctiveEdge = getToolEdge(correctiveTool);
          correctiveEdge?.provides?.forEach((s) => completedSteps.add(s));

          // Persist results for downstream tool access
          if (result.ok) {
            const updateData: Partial<WorkflowState> = {
              results: {
                ...session.results,
                [correctiveTool]: result.value,
              },
            };

            const updated = await updateSessionState(
              state.sessionManager,
              state.logger,
              session.sessionId,
              updateData,
            );
            // Update session reference
            session = updated;
          }
        }

        // Single consolidated update after all prerequisites complete
        session = await updateSessionState(state.sessionManager, state.logger, session.sessionId, {
          completed_steps: Array.from(completedSteps),
        });
      }
    }

    state.logger.debug({ toolName }, 'Executing requested tool');
    const result = await executeToolImpl(state, toolName, params, session, context);

    if (result.ok) {
      executedTools.push(toolName);

      edge.provides?.forEach((step) => completedSteps.add(step));

      // Atomic session update
      session = await updateSessionState(state.sessionManager, state.logger, session.sessionId, {
        completed_steps: Array.from(completedSteps),
      });

      // Separate workflow metadata from results
      if (edge.nextSteps && edge.nextSteps.length > 0) {
        const workflowMetadata = buildWorkflowMetadata(edge, session.sessionId, params, result);

        if (workflowMetadata) {
          const workflowHint = buildWorkflowHint(toolName, workflowMetadata);

          return {
            result,
            executedTools,
            sessionState: session,
            workflowMetadata,
            workflowHint,
          };
        }
      }
    }

    return { result, executedTools, sessionState: session };
  } catch (error) {
    state.logger.error({ error, toolName }, 'Router execution failed');
    return {
      result: Failure(`Router execution failed: ${error}`),
      executedTools,
      sessionState: session,
    };
  }
};

/**
 * Constructs parameters for corrective tools using autofix configuration.
 *
 * Invariant: autofix.buildParams always receives normalized parameters
 */
export const buildCorrectiveParams = (
  originalTool: string,
  correctiveTool: string,
  step: Step,
  originalParams: Record<string, unknown>,
  session: WorkflowState,
): Record<string, unknown> => {
  const edge = getToolEdge(originalTool);
  const autofix = edge?.autofix?.[step];

  const normalizedParams = normalizeToolParameters(originalParams, session);

  if (autofix && autofix.tool === correctiveTool) {
    return autofix.buildParams(normalizedParams);
  }

  return normalizedParams;
};

/**
 * Builds workflow metadata from tool edge configuration.
 *
 * Trade-off: Metadata separation prevents pollution of tool results while enabling workflow guidance
 */
export const buildWorkflowMetadata = (
  edge: ToolEdge,
  sessionId: string,
  params: Record<string, unknown>,
  result: Result<unknown>,
): WorkflowMetadata | undefined => {
  if (!edge.nextSteps?.length) return undefined;

  const primary = edge.nextSteps[0];
  if (!primary) return undefined;

  const resultData =
    result.ok && result.value && typeof result.value === 'object'
      ? (result.value as Record<string, unknown>)
      : {};

  const nextParams = primary.buildParams
    ? primary.buildParams({
        ...params,
        sessionId,
        ...resultData,
      })
    : { sessionId };

  return {
    nextTool: primary.tool,
    description: primary.description,
    params: nextParams,
    sessionId,
    alternatives: edge.nextSteps
      .slice(1)
      .map((step) => ({
        tool: step.tool,
        description: step.description,
        params: step.buildParams
          ? step.buildParams({ ...params, sessionId, ...resultData })
          : undefined,
      }))
      .filter((alt) => alt.params !== undefined),
  };
};

/**
 * Generates human-readable workflow hints for CLI/UI display.
 *
 * Design rationale: Separates presentation logic from business logic
 */
export const buildWorkflowHint = (toolName: string, metadata: WorkflowMetadata): WorkflowHint => {
  const message =
    `✅ ${toolName} completed successfully!\n\n` +
    `🔗 IMPORTANT: Use sessionId "${metadata.sessionId}" for next tool call\n\n` +
    `📋 NEXT RECOMMENDED ACTION:\n` +
    `Tool: ${metadata.nextTool}\n` +
    `Purpose: ${metadata.description}\n` +
    `Session: ${metadata.sessionId}`;

  const markdown =
    `### ✅ ${toolName} completed successfully!\n\n` +
    `> **🔗 IMPORTANT:** Pass \`"sessionId": "${metadata.sessionId}"\` to the next tool call\n\n` +
    `#### 📋 Next Recommended Action\n` +
    `- **Tool**: \`${metadata.nextTool}\`\n` +
    `- **Purpose**: ${metadata.description}\n` +
    `- **Session**: \`${metadata.sessionId}\`\n\n` +
    `**Example call:**\n` +
    `\`\`\`json\n{\n  "sessionId": "${metadata.sessionId}",\n  "path": "your/project/path"\n}\n\`\`\``;

  return {
    message,
    markdown,
    ready: true,
  };
};

/**
 * Core tool execution with session persistence.
 *
 * Precondition: Tool must exist in registry and context must be provided
 * Postcondition: Session contains tool results on success
 */
export const executeToolImpl = async (
  state: ToolRouterState,
  toolName: string,
  params: Record<string, unknown>,
  session: WorkflowState,
  context?: ToolContext,
): Promise<Result<unknown>> => {
  const tool = state.tools.get(toolName);

  if (!tool) {
    return Failure(`Tool not found: ${toolName}`);
  }

  // Normalize parameters before any processing
  let enhancedParams = normalizeToolParameters(params, session);
  enhancedParams.sessionId = session.sessionId;

  // Reduce user burden through AI parameter inference
  if (state.aiAssistant.isAvailable() && tool.schema) {
    const filledParams = await fillMissingParameters(
      state,
      toolName,
      enhancedParams,
      tool.schema,
      session,
      context,
    );
    if (filledParams.ok) {
      enhancedParams = {
        ...filledParams.value,
        sessionId: session.sessionId,
      };
    } else {
      state.logger.warn(
        { error: filledParams.error, toolName },
        'Failed to fill missing parameters with AI',
      );
    }
  }

  try {
    // MCP compliance requires context
    if (!context) {
      return Failure(`Tool context is required for tool: ${toolName}`);
    }
    const result = await tool.handler(enhancedParams, context);

    if (result.ok && session.sessionId) {
      const updateData: Partial<WorkflowState> = {
        results: {
          ...session.results,
          [toolName]: result.value,
        },
        currentStep: toolName,
      };

      await updateSessionState(state.sessionManager, state.logger, session.sessionId, updateData);
    }

    return result;
  } catch (error) {
    state.logger.error({ error, toolName }, 'Tool execution failed');
    return Failure(`Tool execution failed: ${error}`);
  }
};

/**
 * Retrieves tool dependency metadata for external planning systems.
 * Used by workflow orchestrators and dependency analyzers.
 */
export const getToolDependencies = (
  toolName: string,
): {
  requires: Step[];
  provides: Step[];
} => {
  const edge = getToolEdge(toolName);
  return {
    requires: edge?.requires || [],
    provides: edge?.provides || [],
  };
};

/**
 * Infers missing tool parameters using AI assistance from session context.
 *
 * Trade-off: Automatic inference reduces user burden but requires validation for safety
 * Invariant: User-provided parameters always override AI suggestions
 * Postcondition: Returns original params if AI unavailable or fails
 */
export const fillMissingParameters = async (
  state: ToolRouterState,
  toolName: string,
  params: Record<string, unknown>,
  schema: z.ZodObject<z.ZodRawShape> | z.ZodType<unknown>,
  session: WorkflowState,
  context?: ToolContext,
): Promise<Result<Record<string, unknown>>> => {
  try {
    const schemaShape = (schema as z.ZodObject<z.ZodRawShape>).shape || {};
    const requiredParams: string[] = [];
    const missingParams: string[] = [];

    for (const [key, fieldSchema] of Object.entries(schemaShape)) {
      if (fieldSchema && typeof fieldSchema === 'object') {
        const zodField = fieldSchema;
        // Check if field is optional using Zod schema
        const isOptional =
          zodField._def?.typeName === 'ZodOptional' ||
          zodField._def?.typeName === 'ZodNullable' ||
          zodField.isOptional?.();

        if (!isOptional) {
          requiredParams.push(key);
          if (!(key in params) || params[key] === undefined || params[key] === null) {
            missingParams.push(key);
          }
        }
      }
    }

    if (missingParams.length === 0) {
      return Success(params);
    }

    state.logger.debug(
      { toolName, missingParams, requiredParams },
      'Attempting to fill missing parameters with AI',
    );

    const aiRequest: import('./ai/host-ai-assist').AIParamRequest = {
      toolName,
      currentParams: params,
      requiredParams,
      missingParams,
      schema: schemaShape,
      ...(session.results && { sessionContext: session.results }),
    };

    const aiResult = await state.aiAssistant.suggestParameters(aiRequest, context);

    if (!aiResult.ok) {
      return Failure(aiResult.error);
    }

    const validationResult = state.aiAssistant.validateSuggestions(
      aiResult.value.suggestions,
      schema,
    );

    if (!validationResult.ok) {
      return Failure(validationResult.error);
    }

    // User parameters override AI for safety
    const mergedParams = mergeWithSuggestions(params, validationResult.value);

    state.logger.debug(
      {
        toolName,
        filledCount: Object.keys(validationResult.value).length,
      },
      'Successfully filled missing parameters with AI',
    );

    return Success(mergedParams);
  } catch (error) {
    state.logger.error({ error, toolName }, 'Error filling missing parameters');
    return Failure(`Failed to fill parameters: ${error}`);
  }
};

/**
 * Validates tool execution prerequisites by checking session state.
 *
 * Precondition: sessionId must reference valid session
 * Postcondition: Returns list of blocking steps if cannot execute
 */
export const canExecuteImpl = async (
  state: ToolRouterState,
  toolName: string,
  sessionId: string,
): Promise<{ canExecute: boolean; missingSteps: Step[] }> => {
  const sessionResult = await state.sessionManager.get(sessionId);

  if (!sessionResult.ok || !sessionResult.value) {
    return { canExecute: false, missingSteps: [] };
  }

  const session = sessionResult.value;
  const completedSteps = new Set<Step>((session.completed_steps || []) as Step[]);

  const missingSteps = getMissingPreconditions(toolName, completedSteps);

  return {
    canExecute: missingSteps.length === 0,
    missingSteps,
  };
};

/**
 * Calculates complete execution plan including all required dependencies.
 * Used for workflow preview and dependency analysis.
 */
export const getExecutionPlan = (
  toolName: string,
  completedSteps: Set<Step> = new Set(),
): string[] => {
  const missingSteps = getMissingPreconditions(toolName, completedSteps);

  if (missingSteps.length === 0) {
    return [toolName];
  }

  const executionOrder = getExecutionOrder(missingSteps, completedSteps);
  const plan = executionOrder.map(({ tool }) => tool);
  plan.push(toolName);

  return plan;
};

/**
 * Creates configured ToolRouter instance with dependency injection.
 *
 * Design pattern: Factory function enables testability and configuration flexibility
 */
export const createToolRouter = (config: RouterConfig): ToolRouter => {
  const state: ToolRouterState = {
    sessionManager: config.sessionManager,
    logger: config.logger.child({ component: 'tool-router' }),
    tools: config.tools,
    aiAssistant: config.aiAssistant || createHostAIAssistant(config.logger, { enabled: true }),
  };

  return {
    route: (request: RouteRequest) => routeRequestImpl(state, request),
    getToolDependencies,
    canExecute: (toolName: string, sessionId: string) => canExecuteImpl(state, toolName, sessionId),
    getExecutionPlan,
  };
};
