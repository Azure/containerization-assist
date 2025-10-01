/**
 * Session Manager - Single-session workflow state persistence
 *
 * Simplified for single-user operation with one active session.
 * Trade-off: In-memory storage for simplicity over persistence
 *
 * CANONICAL METADATA STRUCTURE:
 * ==================================
 * The ONLY authoritative location for session data:
 *
 * - session.metadata.results: Record<toolName, toolOutput>
 *   SINGLE SOURCE OF TRUTH for tool results (e.g., { 'analyze-repo': {...}, 'build-image': {...} })
 *   Managed by orchestrator via SessionFacade.storeResult/getResult
 *   NEVER use session.results (top-level) - this field is deprecated and removed
 *
 * - session.metadata[key]: Arbitrary workflow metadata (timestamps, flags, custom data)
 *   Managed by tools via SessionFacade.get/set
 *
 * - session.completed_steps: Array of completed tool names
 *
 * MIGRATION NOTES:
 * - Legacy writes to session.results (top-level) have been removed
 * - All tool result reads MUST use SessionFacade.getResult() which reads from metadata.results
 * - Do NOT add fallback logic to read from old locations
 */

import { randomUUID } from 'node:crypto';
import type { Logger } from 'pino';
import { WorkflowState, Result, Success, Failure } from '@/types';

/**
 * Extended WorkflowState with session tracking
 */
interface Session extends WorkflowState {
  lastAccessedAt: Date;
}

/**
 * Session configuration
 * Empty for single-session mode but kept for API compatibility
 */
// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface SessionConfig {
  // Simplified config - no TTL or max sessions needed for single-session mode
}

/**
 * Session Manager - Single-session mode
 */
export class SessionManager {
  private currentSession: Session | null = null;
  private logger: Logger;

  constructor(logger: Logger, _config: SessionConfig = {}) {
    this.logger = logger.child({ service: 'session-manager' });
    this.logger.info('Session manager initialized (single-session mode)');
  }

  /**
   * Create a new session (replaces any existing session)
   * Initializes canonical structure with metadata.results
   */
  async create(sessionId?: string): Promise<Result<WorkflowState>> {
    const id = sessionId ?? randomUUID();
    const now = new Date();

    const session: Session = {
      sessionId: id,
      metadata: {
        results: {}, // Initialize canonical results location
      },
      completed_steps: [],
      errors: {},
      createdAt: now,
      updatedAt: now,
      lastAccessedAt: now,
    };

    this.currentSession = session;
    this.logger.info({ sessionId: id }, 'Session created with canonical structure');

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Get the current session by ID
   */
  async get(sessionId: string): Promise<Result<WorkflowState | null>> {
    const session = this.currentSession;

    this.logger.debug(
      {
        sessionId,
        found: !!session && session.sessionId === sessionId,
      },
      'Session lookup',
    );

    if (!session || session.sessionId !== sessionId) {
      return Success(null);
    }

    // Update access time
    session.lastAccessedAt = new Date();

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Update the current session
   */
  async update(sessionId: string, state: Partial<WorkflowState>): Promise<Result<WorkflowState>> {
    const session = this.currentSession;
    if (!session || session.sessionId !== sessionId) {
      return Failure(`Session ${sessionId} not found`);
    }

    // Update session with merged metadata
    const now = new Date();
    Object.assign(session, state, {
      metadata: {
        ...(session.metadata || {}),
        ...(state.metadata || {}),
      },
      updatedAt: now,
      lastAccessedAt: now,
    });

    this.logger.debug(
      {
        sessionId,
        updatedFields: Object.keys(state).length,
      },
      'Session updated',
    );

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Delete the current session
   */
  async delete(sessionId: string): Promise<Result<void>> {
    if (this.currentSession && this.currentSession.sessionId === sessionId) {
      this.currentSession = null;
      this.logger.debug({ sessionId }, 'Session deleted');
    }
    return Success(undefined);
  }

  /**
   * List current session ID
   */
  async list(): Promise<Result<string[]>> {
    return Success(this.currentSession ? [this.currentSession.sessionId] : []);
  }

  /**
   * Cleanup the current session if older than specified date
   */
  async cleanup(olderThan: Date): Promise<Result<void>> {
    if (this.currentSession && this.currentSession.createdAt < olderThan) {
      this.currentSession = null;
      this.logger.debug('Session cleaned up');
    }
    return Success(undefined);
  }

  /**
   * Close the session manager
   */
  close(): void {
    this.currentSession = null;
    this.logger.info('Session manager closed');
  }
}

/**
 * Factory function to create a session manager instance
 * Maintains backward compatibility with existing code
 */
export function createSessionManager(logger: Logger, config?: SessionConfig): SessionManager {
  return new SessionManager(logger, config);
}
