/**
 * Orchestrates tool execution with automatic dependency resolution.
 *
 * Trade-off: Automatic vs manual dependency management - chose automatic for UX
 * Invariant: Tools execute in dependency order unless force=true
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result, type WorkflowState } from '../types';
import type { SessionManager } from '../lib/session';
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
 * Central orchestrator managing tool dependencies and workflow state.
 *
 * Invariant: Session state must be consistent across tool executions
 * Trade-off: Stateful sessions vs stateless - chose stateful for workflow continuity
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
    this.aiAssistant = config.aiAssistant || createHostAIAssistant(this.logger, { enabled: true });
  }

  /**
   * Extracts session context from WorkflowState using ONLY session.results pattern
   * No fallback to legacy fields - clean implementation
   */
  private extractSessionContext(session: WorkflowState): Record<string, unknown> {
    const analysis = (session.results?.['analyze-repo'] as Record<string, unknown>) || {};
    return {
      technology: analysis.technology,
      language: analysis.language,
      framework: analysis.framework,
      runtime: analysis.runtime,
      packageManager: analysis.packageManager,
    };
  }

  /**
   * Single source of truth for tool parameters
   * Only 'path' parameter is used - no repoPath, no context fallbacks
   */
  private normalizeToolParameters(
    params: Record<string, unknown>,
    session?: WorkflowState,
  ): Record<string, unknown> {
    const normalized: Record<string, unknown> = {
      ...params,
      path: params.path || '.', // Only check 'path', default to '.'
    };

    // Merge session context if available
    if (session) {
      Object.assign(normalized, this.extractSessionContext(session));
    }

    return normalized;
  }

  /**
   * Consolidated session update helper that ensures atomic updates with automatic timestamp.
   * Returns the updated session or fetches latest on null return.
   * Handles concurrent updates by merging results properly.
   */
  private async updateSessionState(
    sessionId: string,
    updates: Partial<WorkflowState>,
  ): Promise<WorkflowState> {
    // If we're updating results, fetch current session to merge properly
    if (updates.results) {
      const currentSession = await this.sessionManager.get(sessionId);
      if (currentSession?.results) {
        // Merge the results objects to preserve concurrent updates
        updates.results = {
          ...currentSession.results,
          ...updates.results,
        };
      }
    }

    const updateData = {
      ...updates,
      updatedAt: new Date(),
    };

    const updated = await this.sessionManager.update(sessionId, updateData);

    if (!updated) {
      this.logger.warn({ sessionId }, 'Session update returned null, fetching latest');
      const fallback = await this.sessionManager.get(sessionId);
      if (fallback) {
        return fallback;
      }
      // Return minimal session if all else fails to prevent crashes
      return { sessionId, updatedAt: new Date() } as WorkflowState;
    }

    return updated;
  }

  /**
   * Executes tool with automatic dependency resolution.
   *
   * Postcondition: Session contains all completed steps and tool results
   * Failure Mode: Returns partial execution list on failure for debugging
   */
  async route(request: RouteRequest): Promise<RouteResult> {
    const { toolName, params, force = false, sessionId, context } = request;

    let session: WorkflowState | null = null;

    if (sessionId) {
      session = await this.sessionManager.get(sessionId);

      // Session creation with explicit ID ensures workflow continuity
      if (!session) {
        this.logger.debug({ sessionId }, 'Session not found, creating new session with ID');
        session = await this.sessionManager.create(sessionId);
      }
    } else {
      session = await this.sessionManager.create();
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

      this.logger.debug(
        {
          toolName,
          hasEdge: !!edge,
        },
        'Checking tool dependencies',
      );

      if (!edge) {
        // Tools without dependencies bypass orchestration layer
        this.logger.debug({ toolName }, 'Tool has no dependencies, executing directly');
        const result = await this.executeTool(toolName, params, session, context);
        if (result.ok) {
          executedTools.push(toolName);
          // Fetch updated session after tool execution
          const updatedSession = await this.sessionManager.get(session.sessionId);
          if (updatedSession) {
            session = updatedSession;
          }
        }
        return { result, executedTools, sessionState: session };
      }

      // Force flag bypasses idempotency check for re-execution scenarios
      if (!force) {
        // Idempotency: Skip execution if all effects already present
        // Prevents redundant work and maintains workflow efficiency
        const alreadySatisfied = edge.provides?.every((step) => completedSteps.has(step)) ?? false;

        if (alreadySatisfied) {
          this.logger.debug({ toolName }, 'Tool effects already satisfied, skipping execution');
          return {
            result: Success({ skipped: true, reason: 'Effects already satisfied' }),
            executedTools: [],
            sessionState: session,
          };
        }

        const missingSteps = getMissingPreconditions(toolName, completedSteps);

        this.logger.debug(
          {
            toolName,
            missingCount: missingSteps.length,
          },
          'Precondition check',
        );

        if (missingSteps.length > 0) {
          this.logger.debug(
            { toolName, missingCount: missingSteps.length },
            'Tool has missing preconditions, auto-running corrective tools',
          );

          // Topological sort ensures dependencies execute in correct order
          const executionOrder = getExecutionOrder(missingSteps, completedSteps);

          for (const { tool: correctiveTool, step } of executionOrder) {
            this.logger.debug({ correctiveTool, step }, 'Running corrective tool');

            const correctiveParams = this.buildCorrectiveParams(
              toolName,
              correctiveTool,
              step,
              params,
              session,
            );

            this.logger.debug(
              {
                correctiveTool,
                step,
              },
              'Running corrective tool',
            );

            const result = await this.executeTool(
              correctiveTool,
              correctiveParams,
              session,
              context,
            );

            if (!result.ok) {
              // Partial state update preserves progress on failure
              const failedSession = await this.updateSessionState(session.sessionId, {
                completed_steps: Array.from(completedSteps),
              });

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

            // Result persistence enables downstream tools to access upstream outputs
            if (result.ok) {
              const updateData: Partial<WorkflowState> = {
                results: {
                  ...session.results,
                  [correctiveTool]: result.value,
                },
              };

              const updated = await this.updateSessionState(session.sessionId, updateData);
              // Update local session reference with latest state
              session = updated;
            }
          }

          // Single consolidated update after all prerequisites complete
          session = await this.updateSessionState(session.sessionId, {
            completed_steps: Array.from(completedSteps),
          });
        }
      }

      this.logger.debug({ toolName }, 'Executing requested tool');
      const result = await this.executeTool(toolName, params, session, context);

      if (result.ok) {
        executedTools.push(toolName);

        edge.provides?.forEach((step) => completedSteps.add(step));

        // Single atomic update with automatic timestamp
        session = await this.updateSessionState(session.sessionId, {
          completed_steps: Array.from(completedSteps),
        });

        // Clean separation: workflow metadata never pollutes results
        if (edge.nextSteps && edge.nextSteps.length > 0) {
          const workflowMetadata = this.buildWorkflowMetadata(
            edge,
            session.sessionId,
            params,
            result,
          );

          if (workflowMetadata) {
            const workflowHint = this.buildWorkflowHint(toolName, workflowMetadata);

            return {
              result, // Result stays completely clean
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
      this.logger.error({ error, toolName }, 'Router execution failed');
      return {
        result: Failure(`Router execution failed: ${error}`),
        executedTools,
        sessionState: session,
      };
    }
  }

  /**
   * Constructs parameters for dependency tools using autofix configuration.
   * Clean implementation with no backwards compatibility.
   */
  private buildCorrectiveParams(
    originalTool: string,
    correctiveTool: string,
    step: Step,
    originalParams: Record<string, unknown>,
    session: WorkflowState,
  ): Record<string, unknown> {
    const edge = getToolEdge(originalTool);
    const autofix = edge?.autofix?.[step];

    const normalizedParams = this.normalizeToolParameters(originalParams, session);

    if (autofix && autofix.tool === correctiveTool) {
      return autofix.buildParams(normalizedParams);
    }

    return normalizedParams;
  }

  /**
   * Builds workflow metadata from edge configuration.
   * Separates workflow control data from tool results.
   */
  private buildWorkflowMetadata(
    edge: ToolEdge,
    sessionId: string,
    params: Record<string, unknown>,
    result: Result<unknown>,
  ): WorkflowMetadata | undefined {
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
  }

  /**
   * Generates human-readable workflow hint.
   * Used for CLI/UI display without polluting result data.
   */
  private buildWorkflowHint(toolName: string, metadata: WorkflowMetadata): WorkflowHint {
    const message =
      `✅ ${toolName} completed successfully!\n\n` +
      `📋 NEXT RECOMMENDED ACTION:\n` +
      `Tool: ${metadata.nextTool}\n` +
      `Purpose: ${metadata.description}\n` +
      `Session: ${metadata.sessionId}`;

    const markdown =
      `### ✅ ${toolName} completed successfully!\n\n` +
      `#### 📋 Next Recommended Action\n` +
      `- **Tool**: \`${metadata.nextTool}\`\n` +
      `- **Purpose**: ${metadata.description}\n` +
      `- **Session**: \`${metadata.sessionId}\`\n\n` +
      `To continue: \`${metadata.nextTool} --session ${metadata.sessionId}\``;

    return {
      message,
      markdown,
      ready: true,
    };
  }

  /**
   * Core tool execution with session persistence.
   *
   * Precondition: Tool must exist in registry and context must be provided
   * Postcondition: Session contains tool results on success
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

    // Normalize parameters before any processing
    let enhancedParams = this.normalizeToolParameters(params, session);
    enhancedParams.sessionId = session.sessionId;

    // AI parameter inference reduces user burden for complex workflows
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
          sessionId: session.sessionId,
        };
      } else {
        this.logger.warn(
          { error: filledParams.error, toolName },
          'Failed to fill missing parameters with AI',
        );
      }
    }

    try {
      // Context requirement enforced at runtime for MCP compliance
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

        await this.updateSessionState(session.sessionId, updateData);
      }

      return result;
    } catch (error) {
      this.logger.error({ error, toolName }, 'Tool execution failed');
      return Failure(`Tool execution failed: ${error}`);
    }
  }

  /**
   * Exposes dependency metadata for external planning tools
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
   * Leverages AI to infer missing required parameters from context.
   *
   * Trade-off: Automatic inference vs explicit user input - validate all suggestions
   * Postcondition: Returns original params if AI unavailable or fails
   */
  private async fillMissingParameters(
    toolName: string,
    params: Record<string, unknown>,
    schema: z.ZodObject<z.ZodRawShape> | z.ZodType<unknown>,
    session: WorkflowState,
    context?: ToolContext,
  ): Promise<Result<Record<string, unknown>>> {
    try {
      const schemaShape = (schema as z.ZodObject<z.ZodRawShape>).shape || {};
      const requiredParams: string[] = [];
      const missingParams: string[] = [];

      for (const [key, fieldSchema] of Object.entries(schemaShape)) {
        if (fieldSchema && typeof fieldSchema === 'object') {
          const zodField = fieldSchema;
          // Zod schema introspection to identify required fields
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

      this.logger.debug(
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

      const aiResult = await this.aiAssistant.suggestParameters(aiRequest, context);

      if (!aiResult.ok) {
        return Failure(aiResult.error);
      }

      const validationResult = this.aiAssistant.validateSuggestions(
        aiResult.value.suggestions,
        schema,
      );

      if (!validationResult.ok) {
        return Failure(validationResult.error);
      }

      // User parameters always override AI suggestions for safety
      const mergedParams = mergeWithSuggestions(params, validationResult.value);

      this.logger.debug(
        {
          toolName,
          filledCount: Object.keys(validationResult.value).length,
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
   * Pre-execution check for tool readiness.
   *
   * Postcondition: Returns list of blocking steps if cannot execute
   */
  async canExecute(
    toolName: string,
    sessionId: string,
  ): Promise<{ canExecute: boolean; missingSteps: Step[] }> {
    const session = await this.sessionManager.get(sessionId);

    if (!session) {
      return { canExecute: false, missingSteps: [] };
    }

    const completedSteps = new Set<Step>((session.completed_steps || []) as Step[]);

    const missingSteps = getMissingPreconditions(toolName, completedSteps);

    return {
      canExecute: missingSteps.length === 0,
      missingSteps,
    };
  }

  /**
   * Generates complete execution sequence including dependencies
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
