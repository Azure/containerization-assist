/**
 * Minimal Tool Router
 * - Validates inputs using optional zod schema
 * - Resolves dependencies via tool-graph helpers
 * - Executes tools in order and returns the final Result
 *
 * Keep this boring. Compose AI assistance / workflow hints outside.
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@/mcp/context';
import { Failure, type Result } from '@/types';
import type { SessionManager } from '@/lib/session';
import type { z } from 'zod';
import { ToolName } from '@/exports/tools';
import { type Step, getToolEdge, getMissingPreconditions, getExecutionOrder } from './tool-graph';

export interface RouterTool {
  name: string;
  handler: (params: Record<string, unknown>, context: ToolContext) => Promise<Result<unknown>>;
  schema?: z.ZodTypeAny;
}

export interface RouteRequest {
  toolName: ToolName;
  params: Record<string, unknown>;
  force?: boolean;
  sessionId?: string | undefined;
  context?: ToolContext;
}

export interface ToolRouterConfig {
  sessionManager: SessionManager;
  logger: Logger;
  tools: Map<ToolName, RouterTool>;
}

export interface ToolRouter {
  route(req: RouteRequest): Promise<Result<unknown>>;
  getToolDependencies(toolName: ToolName): { requires: Step[]; provides: Step[] };
  canExecute(
    toolName: ToolName,
    sessionId: string,
  ): Promise<{ canExecute: boolean; missingSteps: Step[] }>;
  getExecutionPlan(toolName: ToolName, completedSteps?: Set<Step>): string[];
}

export function createToolRouter(config: ToolRouterConfig): ToolRouter {
  const { tools, sessionManager } = config;

  const getDeps = (toolName: ToolName): { requires: Step[]; provides: Step[] } => {
    const tool = tools.get(toolName);
    if (!tool) throw new Error(`Unknown tool: ${toolName}`);

    // Use tool-graph for dependency inference
    const edge = getToolEdge(toolName);
    return {
      requires: edge?.requires ?? [],
      provides: edge?.provides ?? [],
    };
  };

  const canExecute = async (
    toolName: ToolName,
    sessionId: string,
  ): Promise<{ canExecute: boolean; missingSteps: Step[] }> => {
    const sessionResult = await sessionManager.get(sessionId);
    const session = sessionResult.ok ? sessionResult.value : null;

    // Valid steps for filtering
    const validSteps = new Set<Step>([
      'analyzed_repo',
      'resolved_base_images',
      'dockerfile_generated',
      'built_image',
      'scanned_image',
      'pushed_image',
      'k8s_prepared',
      'manifests_generated',
      'helm_charts_generated',
      'aca_manifests_generated',
      'deployed',
    ]);

    // Use single array format for completed steps
    const completed = new Set<Step>();
    if (session && Array.isArray(session.completed_steps)) {
      session.completed_steps.forEach((step) => {
        if (validSteps.has(step as Step)) {
          completed.add(step as Step);
        }
      });
    }

    const missingSteps = getMissingPreconditions(toolName, completed);
    return {
      canExecute: missingSteps.length === 0,
      missingSteps,
    };
  };

  const getExecutionPlan = (
    toolName: ToolName,
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
   * Core routing implementation
   * - Validates parameters
   * - Checks dependencies
   * - Executes tools in order
   * - Updates session with results
   */
  const route = async (req: RouteRequest): Promise<Result<unknown>> => {
    const { toolName, params = {}, sessionId, context, force } = req;

    const tool = tools.get(toolName);
    if (!tool) {
      return Failure(`Tool not found: ${toolName}`);
    }

    // Enhanced schema validation with detailed error information
    if (tool.schema) {
      try {
        tool.schema.parse(params);
      } catch (err: any) {
        const logger = config.logger;
        logger.debug({ err, toolName, params }, 'Parameter validation failed');

        return Failure(
          `Parameter validation failed for ${toolName}: ${err?.message ?? String(err)}`,
        );
      }
    }

    // Get or create session
    let session: { sessionId: string; [key: string]: unknown } | null = null;
    if (sessionId) {
      const sessionResult = await sessionManager.get(sessionId);
      session = sessionResult.ok ? sessionResult.value : null;
    }

    if (!session) {
      const newSessionResult = await sessionManager.create(JSON.stringify({}));
      if (!newSessionResult.ok) {
        return Failure(`Failed to create session: ${newSessionResult.error}`);
      }
      session = newSessionResult.value;
    }

    // Check dependencies unless forced
    if (!force) {
      const { canExecute: canExec, missingSteps } = await canExecute(toolName, session.sessionId);
      if (!canExec) {
        // Try to execute missing dependencies
        const executionOrder = getExecutionOrder(missingSteps, new Set());

        for (const { tool: depTool } of executionOrder) {
          const depToolInstance = tools.get(depTool);
          if (!depToolInstance) {
            return Failure(`Dependency tool not found: ${depTool}`);
          }

          const depParams = { ...params, sessionId: session.sessionId };
          const depResult = await depToolInstance.handler(
            depParams,
            context || ({} as ToolContext),
          );

          if (!depResult.ok) {
            return Failure(`Dependency ${depTool} failed: ${depResult.error}`);
          }

          // Update session with dependency results
          const edge = getToolEdge(depTool);
          if (edge?.provides) {
            const completed = new Set<Step>();
            if (Array.isArray(session.completed_steps)) {
              session.completed_steps.forEach((s) => completed.add(s as Step));
            }
            edge.provides.forEach((step) => completed.add(step));

            await sessionManager.update(session.sessionId, {
              completed_steps: Array.from(completed),
              [`${depTool}Result`]: depResult.value,
            });
          }
        }
      }
    }

    // Execute the main tool
    const toolParams = { ...params, sessionId: session.sessionId };
    const result = await tool.handler(toolParams, context || ({} as ToolContext));

    if (result.ok) {
      // Update session with tool results
      const edge = getToolEdge(toolName);
      if (edge?.provides) {
        const completed = new Set<Step>();
        if (Array.isArray(session.completed_steps)) {
          session.completed_steps.forEach((s) => completed.add(s as Step));
        }
        edge.provides.forEach((step) => completed.add(step));

        // Convert tool name from kebab-case to camelCase for result key
        const camelCaseName = toolName.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
        await sessionManager.update(session.sessionId, {
          completed_steps: Array.from(completed),
          current_step: toolName,
          [`${camelCaseName}Result`]: result.value,
        });
      }
    }

    return result;
  };

  return {
    route,
    getToolDependencies: getDeps,
    canExecute,
    getExecutionPlan,
  };
}
