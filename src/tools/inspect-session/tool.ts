/**
 * Inspect Session Tool
 *
 * Provides debugging capabilities for session management.
 * Allows listing all sessions or inspecting specific sessions.
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Result, Success, Failure, WorkflowState } from '@types';
import { InspectSessionParams, InspectSessionResult, InspectSessionParamsSchema } from './schema';

const DEFAULT_TTL = 86400; // 24 hours in seconds

// Extended session interface with additional debugging properties
interface SessionDetails extends Omit<WorkflowState, 'sessionId'> {
  id?: string;
  sessionId?: string;
  lastAccessedAt?: Date;
  ttlRemaining?: number;
  currentStep?: string | null;
  completedSteps?: string[];
  errors?: Record<string, unknown>;
}

export interface InspectSessionDeps {
  logger: Logger;
}

/**
 * Calculate remaining TTL in seconds
 */
function calculateTTLRemaining(createdAt: Date): number {
  const now = Date.now();
  const elapsed = Math.floor((now - createdAt.getTime()) / 1000);
  return Math.max(0, DEFAULT_TTL - elapsed);
}

/**
 * Extract tool slices from metadata - simplified
 */
function extractToolSlices(metadata: Record<string, unknown>): Record<string, unknown> {
  // Return all metadata as-is for now - let consumers filter
  return metadata;
}

/**
 * Format session summary for display - simplified
 */
function formatSessionSummary(session: SessionDetails): string {
  return JSON.stringify(
    {
      id: session.id,
      created:
        session.createdAt instanceof Date
          ? session.createdAt.toISOString()
          : typeof session.createdAt === 'string'
            ? session.createdAt
            : 'unknown',
      updated:
        session.updatedAt instanceof Date
          ? session.updatedAt.toISOString()
          : typeof session.updatedAt === 'string'
            ? session.updatedAt
            : 'unknown',
      ttl: session.ttlRemaining ? `${Math.floor(session.ttlRemaining / 60)}m` : 'unknown',
      step: session.currentStep || 'none',
      completed: session.completedSteps,
      errors: session.errors ? Object.keys(session.errors) : [],
    },
    null,
    2,
  );
}

/**
 * Format session list for display - simplified
 */
function formatSessionList(sessions: SessionDetails[]): string {
  if (sessions.length === 0) {
    return 'No active sessions';
  }

  return JSON.stringify(
    sessions.map((s) => ({
      id: s.id,
      ttl: s.ttlRemaining ? `${Math.floor(s.ttlRemaining / 60)}m` : 'unknown',
      step: s.currentStep,
      completed: s.completedSteps ? s.completedSteps.length : 0,
    })),
    null,
    2,
  );
}

/**
 * Create inspect session tool with explicit dependencies
 */
export function createInspectSessionTool(deps: InspectSessionDeps) {
  return async (
    params: InspectSessionParams,
    context: ToolContext,
  ): Promise<Result<InspectSessionResult>> => {
    const start = Date.now();
    const { logger } = deps;

    try {
      // Get session manager from context
      if (!context.sessionManager) {
        return Failure('Session manager not available in context');
      }

      const sessionManager = context.sessionManager;

      // Get all session IDs
      const listResult = await sessionManager.list();
      if (!listResult.ok) {
        return Failure(`Failed to list sessions: ${listResult.error}`);
      }
      const allSessionIds = listResult.value;

      // If specific session requested
      if (params.sessionId) {
        const sessionResult = await sessionManager.get(params.sessionId);
        if (!sessionResult.ok) {
          return Failure(`Failed to get session: ${sessionResult.error}`);
        }
        const session = sessionResult.value;

        if (!session) {
          return Success({
            sessions: [],
            totalSessions: allSessionIds.length,
            maxSessions: 1000,
            message: `Session ${params.sessionId} not found`,
          });
        }

        const sessionInfo = {
          id: params.sessionId,
          createdAt: session.createdAt || new Date(),
          updatedAt: session.updatedAt || new Date(),
          ttlRemaining: calculateTTLRemaining(session.createdAt || new Date()),
          completedSteps: session.completed_steps || [],
          currentStep: (session.current_step as string) || null,
          metadata: session.metadata || {},
          toolSlices: params.includeSlices ? extractToolSlices(session.metadata || {}) : undefined,
          errors: session.errors ? (session.errors as Record<string, string>) : undefined,
        };

        const duration = Date.now() - start;
        logger.info(
          { sessionId: params.sessionId, duration, tool: 'inspect-session' },
          'Tool execution complete',
        );

        return Success({
          sessions: [sessionInfo],
          totalSessions: allSessionIds.length,
          maxSessions: 1000,
          message:
            params.format === 'json'
              ? JSON.stringify(sessionInfo, null, 2)
              : formatSessionSummary(sessionInfo),
        });
      }

      // List all sessions
      const sessions = [];
      for (const id of allSessionIds) {
        const sessionResult = await sessionManager.get(id);
        if (sessionResult.ok && sessionResult.value) {
          const session = sessionResult.value;
          sessions.push({
            id,
            createdAt: session.createdAt || new Date(),
            updatedAt: session.updatedAt || new Date(),
            ttlRemaining: calculateTTLRemaining(session.createdAt || new Date()),
            completedSteps: session.completed_steps || [],
            currentStep: (session.current_step as string) || null,
            metadata: session.metadata || {},
            toolSlices: params.includeSlices
              ? extractToolSlices(session.metadata || {})
              : undefined,
            errors: session.errors ? (session.errors as Record<string, string>) : undefined,
          });
        }
      }

      // Sort by most recently updated
      sessions.sort((a, b) => b.updatedAt.getTime() - a.updatedAt.getTime());

      const duration = Date.now() - start;
      logger.info(
        { sessionCount: sessions.length, duration, tool: 'inspect-session' },
        'Tool execution complete',
      );

      return Success({
        sessions,
        totalSessions: sessions.length,
        maxSessions: 1000,
        message:
          params.format === 'json'
            ? JSON.stringify(sessions, null, 2)
            : formatSessionList(sessions),
      });
    } catch (error) {
      const duration = Date.now() - start;
      const message = error instanceof Error ? error.message : String(error);
      logger.error({ error: message, duration, tool: 'inspect-session' }, 'Tool execution failed');
      return Failure(`Failed to inspect session: ${message}`);
    }
  };
}

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'inspect-session',
  description: 'Inspect and debug session data for troubleshooting',
  inputSchema: InspectSessionParamsSchema,
  execute: createInspectSessionTool,
};
