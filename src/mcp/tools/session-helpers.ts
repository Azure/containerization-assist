/**
 * Session Helpers Module
 *
 * Provides simplified session management utilities for all tools.
 * Reduced from 437 lines to ~100 lines by removing enterprise-style complexity.
 */

import { randomUUID } from 'node:crypto';
import { Result, Success, Failure, WorkflowState } from '../../domain/types.js';
import type { SessionManager } from '../../lib/session.js';
import type { ToolContext } from '@mcp/context/types';

/**
 * Get session manager from context (no longer creates new instances)
 */
function getSessionManager(context?: ToolContext): SessionManager {
  // Check if context has a shared session manager
  if (context && typeof context === 'object' && 'sessionManager' in context) {
    const manager = context.sessionManager;
    if (manager && typeof manager === 'object') {
      return manager;
    }
  }

  // This is the fix - we should NOT create a new session manager here
  // If no session manager is in context, it's a configuration error
  throw new Error(
    'Session manager not found in context. This is required for session persistence across tools.',
  );
}

/**
 * Get or create session - simplified replacement for resolveSession
 *
 * @param sessionId - Optional session ID (generates random if not provided)
 * @param context - Tool context that may contain session manager
 * @returns Result with session ID and state
 */
export async function getSession(
  sessionId?: string,
  context?: ToolContext,
): Promise<Result<{ id: string; state: WorkflowState; isNew: boolean }>> {
  try {
    const sessionManager = getSessionManager(context);
    const id = sessionId || randomUUID();

    // Try to get existing session
    let session = await sessionManager.get(id);
    let isNew = false;

    // Create if doesn't exist
    if (!session) {
      session = await sessionManager.create(id);
      isNew = true;
    }

    return Success({ id, state: session, isNew });
  } catch (error) {
    return Failure(
      `Failed to get session: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Complete a workflow step - simplified replacement for appendCompletedStep
 *
 * @param sessionId - Session identifier
 * @param stepName - Name of the completed step
 * @param context - Tool context with session manager
 * @returns Result with updated session state
 */
export async function completeStep(
  sessionId: string,
  stepName: string,
  context?: ToolContext,
): Promise<Result<WorkflowState>> {
  try {
    const sessionManager = getSessionManager(context);

    // Get current session
    const currentSession = await sessionManager.get(sessionId);
    if (!currentSession) {
      return Failure(`Session ${sessionId} not found`);
    }

    // Add step to completed_steps array if not already there
    const updatedSteps = [...(currentSession.completed_steps || [])];
    if (!updatedSteps.includes(stepName)) {
      updatedSteps.push(stepName);
    }

    // Update session using our simplified updateSession function
    return updateSession(
      sessionId,
      {
        completed_steps: updatedSteps,
        current_step: stepName,
      },
      context,
    );
  } catch (error) {
    return Failure(
      `Failed to complete step: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Create a new session with optional ID - for explicit creation scenarios
 *
 * @param sessionId - Optional session ID (generates random if not provided)
 * @param context - Tool context with session manager
 * @returns Result with new session ID and state
 */
export async function createSession(
  sessionId?: string,
  context?: ToolContext,
): Promise<Result<{ id: string; state: WorkflowState }>> {
  try {
    const sessionManager = getSessionManager(context);
    const id = sessionId || randomUUID();

    const session = await sessionManager.create(id);
    return Success({ id, state: session });
  } catch (error) {
    return Failure(
      `Failed to create session: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Update session with new data - simplified replacement for updateSessionData
 *
 * @param sessionId - Session identifier
 * @param updates - Partial updates to apply
 * @param context - Tool context with session manager
 * @returns Result with updated session state
 */
export async function updateSession(
  sessionId: string,
  updates: Partial<WorkflowState>,
  context?: ToolContext,
): Promise<Result<WorkflowState>> {
  try {
    const sessionManager = getSessionManager(context);

    // Get current session to merge metadata properly
    const currentSession = await sessionManager.get(sessionId);
    if (!currentSession) {
      return Failure(`Session ${sessionId} not found`);
    }

    // Apply updates with metadata merging
    const mergedUpdates: Partial<WorkflowState> = {
      ...updates,
      metadata: {
        ...currentSession.metadata,
        ...(updates.metadata || {}),
      },
      updatedAt: new Date(),
    };

    await sessionManager.update(sessionId, mergedUpdates);

    // Return updated session
    const updatedSession = await sessionManager.get(sessionId);
    if (!updatedSession) {
      return Failure(`Session ${sessionId} lost after update`);
    }

    return Success(updatedSession);
  } catch (error) {
    return Failure(
      `Failed to update session: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}
