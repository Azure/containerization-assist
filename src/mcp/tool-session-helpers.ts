/**
 * Session Helpers Module
 *
 * Provides simplified session management utilities for all tools.
 * Reduced from 437 lines to ~100 lines by removing enterprise-style complexity.
 */

import { randomUUID } from 'node:crypto';
import { Result, Success, Failure, WorkflowState } from '../types';
import type { SessionManager } from '../lib/session';
import type { ToolContext } from '@mcp/context';
import { extractErrorMessage } from '../lib/error-utils';

// Re-export typed utilities for tools to use
export { useSessionSlice, getSessionSlice, updateSessionSlice } from '../lib/session-slice-utils';
export { defineToolIO, type SessionSlice, type ToolIO } from '../lib/session-types';

/**
 * Get session manager from context - returns null if not available
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
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

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
    return Failure(`Failed to get session: ${extractErrorMessage(error)}`);
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
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

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
    return Failure(`Failed to complete step: ${extractErrorMessage(error)}`);
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
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

    const id = sessionId || randomUUID();
    const session = await sessionManager.create(id);
    return Success({ id, state: session });
  } catch (error) {
    return Failure(`Failed to create session: ${extractErrorMessage(error)}`);
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
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

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
    return Failure(`Failed to update session: ${extractErrorMessage(error)}`);
  }
}

/**
 * Ensure session exists - creates if not found, returns existing otherwise
 * Guarantees a WorkflowState exists for the given session ID
 *
 * @param context - Tool context with session manager
 * @param sessionId - Optional session ID (generates random if not provided)
 * @returns Result with session ID and state
 */
export async function ensureSession(
  context?: ToolContext,
  sessionId?: string,
): Promise<Result<{ id: string; state: WorkflowState }>> {
  try {
    const sessionManager = getSessionManager(context);
    if (!sessionManager) {
      return Failure('Session manager not available in context');
    }

    const id = sessionId || randomUUID();

    // Try to get existing session first
    let session = await sessionManager.get(id);

    // Create if doesn't exist
    if (!session) {
      session = await sessionManager.create(id);
    }

    return Success({ id, state: session });
  } catch (error) {
    return Failure(`Failed to ensure session: ${extractErrorMessage(error)}`);
  }
}
