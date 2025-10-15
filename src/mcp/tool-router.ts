/**
 * Tool Router - Intelligent routing with automatic precondition satisfaction
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@/mcp/context';
import { Success, Failure, type Result, type WorkflowState } from '@/types';
import { type Step, getToolEdge, getMissingPreconditions, getExecutionOrder } from './tool-graph';
import { createHostAIAssistant, type HostAIAssistant } from './ai/host-ai-assist';
import type { z } from 'zod';

/**
 * Session manager interface for backwards compatibility with tool-router
 * This will be removed as part of the session removal refactoring
 */
export interface SessionManager {
  get(sessionId: string): Promise<Result<WorkflowState>>;
  create(sessionId?: string): Promise<Result<WorkflowState>>;
  update(sessionId: string, updates: Partial<WorkflowState>): Promise<Result<WorkflowState>>;
}

/**
 * Tool definition for router
 */
export interface RouterTool {
  name: string;
  handler: (params: Record<string, unknown>, context: ToolContext) => Promise<Result<unknown>>;
  schema?: z.ZodObject<z.ZodRawShape> | z.ZodType<unknown>;
}

export interface RouterConfig {
  /** Session manager for state persistence */
  sessionManager: SessionManager;
  /** Logger for router operations */
  logger: Logger;
  /** Map of tool names to Tool instances */
  tools: Map<string, RouterTool>;
  /** AI assistant for parameter suggestions (optional, enabled by default) */
  aiAssistant?: HostAIAssistant;
}

export interface RouteRequest {
  /** Tool to execute */
  toolName: string;
  /** Tool parameters */
  params: Record<string, unknown>;
  /** Bypass precondition checks and re-run */
  force?: boolean;
  /** Session ID for state management */
  sessionId?: string;
  /** Tool context for AI capabilities */
  context?: ToolContext;
}

export interface RouteResult {
  /** Final tool execution result */
  result: Result<unknown>;
  /** Tools executed in order */
  executedTools: string[];
  /** Session state after execution (optional - session functionality is being removed) */
  sessionState?: WorkflowState;
}

/**
 * Tool Router for intelligent precondition management
 */
export class ToolRouter {
  private sessionManager: SessionManager;
  private logger: Logger;
  private tools: Map<string, RouterTool>;
  private aiAssistant: HostAIAssistant;

  constructor(config: RouterConfig) {
    this.sessionManager = config.sessionManager;
    this.logger = config.logger.child({ component: 'tool-router' });
    this.tools = config.tools;
    // Create a default AI assistant with a no-op hostCall if not provided
    this.aiAssistant =
      config.aiAssistant ||
      createHostAIAssistant(
        async (_prompt: string) => Promise.resolve('{}'),
        { enabled: false },
        this.logger,
      );
  }

  /**
   * Route a tool request with automatic precondition satisfaction
   */
  async route(request: RouteRequest): Promise<RouteResult> {
    const { toolName, params, force = false, sessionId, context } = request;

    // Get or create session
    const sessionResult = sessionId
      ? await this.sessionManager.get(sessionId)
      : await this.sessionManager.create();

    if (!sessionResult.ok || !sessionResult.value) {
      return {
        result: Failure('Failed to get or create session'),
        executedTools: [],
      };
    }

    let session = sessionResult.value;
    const executedTools: string[] = [];

    try {
      // Get completed steps from session
      const completedSteps = new Set<Step>((session.completed_steps || []) as Step[]);

      // Check if tool has dependencies
      const edge = getToolEdge(toolName);

      if (!edge) {
        // Tool has no dependencies, execute directly
        this.logger.debug({ toolName }, 'Tool has no dependencies, executing directly');
        const result = await this.executeTool(toolName, params, session, context);
        if (result.ok) {
          executedTools.push(toolName);
        }
        return { result, executedTools, sessionState: session };
      }

      // Skip precondition checks if force flag is set
      if (!force) {
        // Check if effects are already satisfied (idempotency)
        const alreadySatisfied = edge.provides?.every((step) => completedSteps.has(step)) ?? false;

        if (alreadySatisfied) {
          this.logger.info(
            { toolName, provides: edge.provides },
            'Tool effects already satisfied, skipping execution',
          );
          return {
            result: Success({ skipped: true, reason: 'Effects already satisfied' }),
            executedTools: [],
            sessionState: session,
          };
        }

        // Get missing preconditions
        const missingSteps = getMissingPreconditions(toolName, completedSteps);

        if (missingSteps.length > 0) {
          this.logger.info(
            { toolName, missingSteps },
            'Tool has missing preconditions, auto-running corrective tools',
          );

          // Determine execution order for missing preconditions
          const executionOrder = getExecutionOrder(missingSteps, completedSteps);

          // Execute corrective tools
          for (const { tool: correctiveTool, step } of executionOrder) {
            this.logger.debug({ correctiveTool, step }, 'Running corrective tool');

            // Build parameters for corrective tool
            const correctiveParams = this.buildCorrectiveParams(
              toolName,
              correctiveTool,
              step,
              params,
            );

            const result = await this.executeTool(
              correctiveTool,
              correctiveParams,
              session,
              context,
            );

            if (!result.ok) {
              // Update session before returning failure
              await this.sessionManager.update(session.sessionId, {
                completed_steps: Array.from(completedSteps),
                updatedAt: new Date(),
              });

              // Get updated session state
              const failedSessionResult = await this.sessionManager.get(session.sessionId);
              const failedSession =
                failedSessionResult.ok && failedSessionResult.value
                  ? failedSessionResult.value
                  : session;

              return {
                result: Failure(
                  `Failed to satisfy precondition '${step}' with tool '${correctiveTool}': ${result.error}`,
                ),
                executedTools,
                sessionState: failedSession,
              };
            }

            executedTools.push(correctiveTool);

            // Mark step as completed
            completedSteps.add(step);

            // Add provided steps from corrective tool
            const correctiveEdge = getToolEdge(correctiveTool);
            correctiveEdge?.provides?.forEach((s) => completedSteps.add(s));
          }

          // Update session with completed steps from prerequisites
          await this.sessionManager.update(session.sessionId, {
            completed_steps: Array.from(completedSteps),
            updatedAt: new Date(),
          });

          // Reload session to get updated state
          const updatedPrereqSessionResult = await this.sessionManager.get(session.sessionId);
          if (updatedPrereqSessionResult.ok && updatedPrereqSessionResult.value) {
            session = updatedPrereqSessionResult.value;
          }
        }
      }

      // Execute the requested tool
      this.logger.debug({ toolName }, 'Executing requested tool');
      const result = await this.executeTool(toolName, params, session, context);

      if (result.ok) {
        executedTools.push(toolName);

        // Record effects in session
        edge.provides?.forEach((step) => completedSteps.add(step));

        // Update session with completed steps
        await this.sessionManager.update(session.sessionId, {
          completed_steps: Array.from(completedSteps),
          updatedAt: new Date(),
        });

        // Reload session to get updated state
        const updatedSessionResult = await this.sessionManager.get(session.sessionId);
        if (updatedSessionResult.ok && updatedSessionResult.value) {
          session = updatedSessionResult.value;
        } else {
          // If update failed, at least update the local session object
          session.completed_steps = Array.from(completedSteps);
          session.updatedAt = new Date();
        }
      }

      return { result, executedTools, sessionState: session };
    } catch (error) {
      this.logger.error({ error, toolName }, 'Router execution failed');
      return {
        result: Failure(`Router execution failed: ${error}`),
        executedTools,
        sessionState: session,
      };
    }
  }

  /**
   * Build parameters for corrective tool execution
   */
  private buildCorrectiveParams(
    originalTool: string,
    correctiveTool: string,
    step: Step,
    originalParams: Record<string, unknown>,
  ): Record<string, unknown> {
    // Get autofix configuration
    const edge = getToolEdge(originalTool);
    const autofix = edge?.autofix?.[step];

    if (autofix && autofix.tool === correctiveTool) {
      // Use custom parameter builder
      return autofix.buildParams(originalParams);
    }

    // Default parameter mapping
    return {
      path: originalParams.path || '.',
      ...originalParams,
    };
  }

  /**
   * Execute a tool and update session
   */
  private async executeTool(
    toolName: string,
    params: Record<string, unknown>,
    session: WorkflowState,
    context?: ToolContext,
  ): Promise<Result<unknown>> {
    const tool = this.tools.get(toolName);

    if (!tool) {
      return Failure(`Tool not found: ${toolName}`);
    }

    // Add session ID to params if not present
    let enhancedParams = {
      ...params,
      sessionId: session.sessionId,
    };

    // Try to fill missing parameters with AI assistance
    if (this.aiAssistant.isAvailable() && tool.schema) {
      const filledParams = await this.fillMissingParameters(
        toolName,
        enhancedParams,
        tool.schema,
        session,
        context,
      );
      if (filledParams.ok) {
        enhancedParams = {
          ...filledParams.value,
          sessionId: session.sessionId, // Ensure sessionId is preserved
        };
      } else {
        // Log but continue with original params
        this.logger.warn(
          { error: filledParams.error, toolName },
          'Failed to fill missing parameters with AI',
        );
      }
    }

    try {
      // Call tool handler directly - context is required by tools
      if (!context) {
        return Failure(`Tool context is required for tool: ${toolName}`);
      }
      const result = await tool.handler(enhancedParams, context);

      // Update session metadata with tool results
      if (result.ok && session.sessionId) {
        await this.sessionManager.update(session.sessionId, {
          results: {
            ...(session.results || {}),
            [toolName]: result.value,
          },
          currentStep: toolName,
          updatedAt: new Date(),
        });
      }

      return result;
    } catch (error) {
      this.logger.error({ error, toolName }, 'Tool execution failed');
      return Failure(`Tool execution failed: ${error}`);
    }
  }

  /**
   * Get tool dependencies for planning
   */
  getToolDependencies(toolName: string): {
    requires: Step[];
    provides: Step[];
  } {
    const edge = getToolEdge(toolName);
    return {
      requires: edge?.requires || [],
      provides: edge?.provides || [],
    };
  }

  /**
   * Fill missing parameters using AI assistance
   */
  private async fillMissingParameters(
    toolName: string,
    params: Record<string, unknown>,
    schema: z.ZodObject<z.ZodRawShape> | z.ZodType<unknown>,
    session: WorkflowState,
    context?: ToolContext,
  ): Promise<Result<Record<string, unknown>>> {
    try {
      // Identify required vs provided parameters
      const schemaShape = (schema as z.ZodObject<z.ZodRawShape>).shape || {};
      const requiredParams: string[] = [];
      const missingParams: string[] = [];

      // Check each schema field
      for (const [key, fieldSchema] of Object.entries(schemaShape)) {
        if (fieldSchema && typeof fieldSchema === 'object') {
          const zodField = fieldSchema;
          // Check if field is required (not optional)
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

      // If no missing params, return original
      if (missingParams.length === 0) {
        return Success(params);
      }

      this.logger.debug(
        { toolName, missingParams, requiredParams },
        'Attempting to fill missing parameters with AI',
      );

      // Request AI suggestions
      const baseRequest = {
        toolName,
        currentParams: params,
        requiredParams,
        missingParams,
        schema: schemaShape,
      };

      const aiRequest: import('./ai/host-ai-assist').AIParamRequest = session.results
        ? { ...baseRequest, sessionContext: session.results as Record<string, unknown> }
        : baseRequest;

      const aiResult = await this.aiAssistant.suggestParameters(aiRequest, context);

      if (!aiResult.ok) {
        return Failure(aiResult.error);
      }

      // Validate suggestions against schema
      const validationResult = this.aiAssistant.validateSuggestions(
        aiResult.value.suggestions,
        schema,
      );

      if (!validationResult.ok) {
        return Failure(validationResult.error);
      }

      // Merge suggestions with user params (user params take precedence)
      const mergedParams = {
        ...validationResult.value,
        ...params,
      };

      this.logger.info(
        {
          toolName,
          filled: Object.keys(validationResult.value),
          confidence: aiResult.value.confidence,
        },
        'Successfully filled missing parameters with AI',
      );

      return Success(mergedParams);
    } catch (error) {
      this.logger.error({ error, toolName }, 'Error filling missing parameters');
      return Failure(`Failed to fill parameters: ${error}`);
    }
  }

  /**
   * Check if a tool can be executed given current session state
   */
  async canExecute(
    toolName: string,
    sessionId: string,
  ): Promise<{ canExecute: boolean; missingSteps: Step[] }> {
    const sessionResult = await this.sessionManager.get(sessionId);

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
  }

  /**
   * Get execution plan for a tool
   */
  getExecutionPlan(toolName: string, completedSteps: Set<Step> = new Set()): string[] {
    const missingSteps = getMissingPreconditions(toolName, completedSteps);

    if (missingSteps.length === 0) {
      return [toolName];
    }

    const executionOrder = getExecutionOrder(missingSteps, completedSteps);
    const plan = executionOrder.map(({ tool }) => tool);
    plan.push(toolName);

    return plan;
  }
}
