/**
 * Session Inspection Tool
 *
 * Provides debugging capabilities for session management.
 * Allows listing all sessions or inspecting specific sessions.
 */

import type { ToolContext } from '@/mcp/context';
import { Result, Success, Failure } from '@/types';
import { InspectSessionParams, InspectSessionResult } from './schema';
import type { SessionManager } from '@/lib/session';
import { extractErrorMessage } from '@/lib/error-utils';

const DEFAULT_TTL = 86400; // 24 hours in seconds

export async function inspectSession(
  params: InspectSessionParams,
  context?: ToolContext,
): Promise<Result<InspectSessionResult>> {
  try {
    // Get session manager from context
    const sessionManager = getSessionManager(context);
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

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
          maxSessions: 1000, // Default from session manager
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

      return Success({
        sessions: [sessionInfo as any],
        totalSessions: allSessionIds.length,
        maxSessions: 1000,
        message:
          params.format === 'json'
            ? JSON.stringify(sessionInfo, null, 2)
            : formatSessionSummary(sessionInfo as any),
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
          toolSlices: params.includeSlices ? extractToolSlices(session.metadata || {}) : undefined,
          errors: session.errors ? (session.errors as Record<string, string>) : undefined,
        });
      }
    }

    // Sort by most recently updated
    sessions.sort((a, b) => b.updatedAt.getTime() - a.updatedAt.getTime());

    return Success({
      sessions: sessions as any,
      totalSessions: sessions.length,
      maxSessions: 1000,
      message: formatSessionList(sessions as any, params.format),
    });
  } catch (error) {
    return Failure(`Failed to inspect session: ${extractErrorMessage(error)}`);
  }
}

/**
 * Get session manager from context
 */
function getSessionManager(context?: ToolContext): SessionManager | null {
  if (context && typeof context === 'object' && 'sessionManager' in context) {
    const manager = context.sessionManager;
    if (manager && typeof manager === 'object') {
      return manager;
    }
  }
  return null;
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
 * Extract tool slices from metadata
 */
function extractToolSlices(metadata: Record<string, unknown>): Record<string, unknown> {
  const slices: Record<string, unknown> = {};

  if (metadata.session && typeof metadata.session === 'object') {
    const sessionData = metadata.session as Record<string, unknown>;
    if (sessionData.toolSlices && typeof sessionData.toolSlices === 'object') {
      return sessionData.toolSlices as Record<string, unknown>;
    }
  }

  // Look for tool-specific keys in metadata
  for (const [key, value] of Object.entries(metadata)) {
    if (key.includes('_result') || key.includes('_input') || key.includes('_state')) {
      slices[key] = value;
    }
  }

  return slices;
}

/**
 * Format session summary for display
 */
interface SessionData {
  id: string;
  createdAt: Date;
  updatedAt: Date;
  ttlRemaining: number;
  currentStep?: string;
  completedSteps: string[];
  errors?: Record<string, unknown>;
  toolSlices?: Record<string, unknown>;
}

function formatSessionSummary(session: SessionData): string {
  const lines = [
    `Session: ${session.id}`,
    `Created: ${session.createdAt.toISOString()}`,
    `Updated: ${session.updatedAt.toISOString()}`,
    `TTL Remaining: ${Math.floor(session.ttlRemaining / 60)} minutes`,
    `Current Step: ${session.currentStep || 'none'}`,
    `Completed Steps: ${session.completedSteps.length > 0 ? session.completedSteps.join(', ') : 'none'}`,
  ];

  if (session.errors && Object.keys(session.errors).length > 0) {
    lines.push(`Errors: ${Object.keys(session.errors).join(', ')}`);
  }

  if (session.toolSlices && Object.keys(session.toolSlices).length > 0) {
    lines.push(`Tool Slices: ${Object.keys(session.toolSlices).join(', ')}`);
  }

  return lines.join('\n');
}

/**
 * Format session list for display
 */
function formatSessionList(sessions: SessionData[], format: string): string {
  if (format === 'json') {
    return JSON.stringify(sessions, null, 2);
  }

  if (sessions.length === 0) {
    return 'No active sessions';
  }

  const lines = [`Found ${sessions.length} active session(s):\n`];

  for (const session of sessions) {
    lines.push(`  • ${session.id}`);
    lines.push(`    TTL: ${Math.floor(session.ttlRemaining / 60)}m remaining`);
    if (session.currentStep) {
      lines.push(`    Current: ${session.currentStep}`);
    }
    if (session.completedSteps.length > 0) {
      lines.push(`    Completed: ${session.completedSteps.length} steps`);
    }
    lines.push('');
  }

  return lines.join('\n');
}
