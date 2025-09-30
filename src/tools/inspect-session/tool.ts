/**
 * Session Inspection Tool - Modernized Implementation
 *
 * Provides debugging capabilities for session management.
 * Allows listing all sessions or inspecting specific sessions.
 * Follows the new Tool interface pattern
 */

import type { ToolContext } from '@/mcp/context';
import { Result, Success, Failure } from '@/types';
import { InspectSessionParamsSchema, type InspectSessionResult } from './schema';
import type { SessionManager } from '@/session/core';
import { extractErrorMessage } from '@/lib/error-utils';
import type { Tool } from '@/types/tool';
import type { z } from 'zod';

const DEFAULT_TTL = 86400; // 24 hours in seconds

/**
 * Session inspection implementation
 */
async function run(
  input: z.infer<typeof InspectSessionParamsSchema>,
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
    if (input.sessionId) {
      const sessionResult = await sessionManager.get(input.sessionId);
      if (!sessionResult.ok) {
        return Failure(`Failed to get session: ${sessionResult.error}`);
      }
      const session = sessionResult.value;

      if (!session) {
        return Success({
          sessions: [],
          totalSessions: allSessionIds.length,
          maxSessions: 1, // Single-session mode
          message: `Session ${input.sessionId} not found`,
        });
      }

      const sessionInfo: SessionData = {
        id: input.sessionId,
        createdAt: session.createdAt || new Date(),
        updatedAt: session.updatedAt || new Date(),
        ttlRemaining: calculateTTLRemaining(session.createdAt || new Date()),
        completedSteps: session.completed_steps || [],
        metadata: session.metadata || {},
      };

      if (input.includeSlices) {
        sessionInfo.toolSlices = extractToolSlices(session.metadata || {});
      }

      if (session.errors) {
        sessionInfo.errors = session.errors as Record<string, string>;
      }

      return Success({
        sessions: [sessionInfo],
        totalSessions: allSessionIds.length,
        maxSessions: 1, // Single-session mode
        message:
          input.format === 'json'
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
        const sessionData: SessionData = {
          id,
          createdAt: session.createdAt || new Date(),
          updatedAt: session.updatedAt || new Date(),
          ttlRemaining: calculateTTLRemaining(session.createdAt || new Date()),
          completedSteps: session.completed_steps || [],
          metadata: session.metadata || {},
        };

        if (input.includeSlices) {
          sessionData.toolSlices = extractToolSlices(session.metadata || {});
        }

        if (session.errors) {
          sessionData.errors = session.errors as Record<string, string>;
        }
        sessions.push(sessionData);
      }
    }

    // Sort by most recently updated
    sessions.sort((a, b) => b.updatedAt.getTime() - a.updatedAt.getTime());

    return Success({
      sessions,
      totalSessions: sessions.length,
      maxSessions: 1, // Single-session mode
      message: formatSessionList(sessions, input.format),
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
  completedSteps: string[];
  metadata: Record<string, unknown>;
  errors?: Record<string, string>;
  toolSlices?: Record<string, unknown>;
}

function formatSessionSummary(session: SessionData): string {
  const lines = [
    `Session: ${session.id}`,
    `Created: ${session.createdAt.toISOString()}`,
    `Updated: ${session.updatedAt.toISOString()}`,
    `TTL Remaining: ${Math.floor(session.ttlRemaining / 60)} minutes`,
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
    if (session.completedSteps.length > 0) {
      lines.push(`    Completed: ${session.completedSteps.length} steps`);
    }
    lines.push('');
  }

  return lines.join('\n');
}

/**
 * Inspect session tool conforming to Tool interface
 */
const tool: Tool<typeof InspectSessionParamsSchema, InspectSessionResult> = {
  name: 'inspect-session',
  description: 'Provides debugging capabilities for session management',
  version: '2.0.0',
  schema: InspectSessionParamsSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  },
  run,
};

export default tool;
