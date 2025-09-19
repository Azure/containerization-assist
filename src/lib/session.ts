/**
 * Session Manager - Workflow state persistence
 *
 * Invariant: Sessions expire after TTL to prevent memory leaks
 * Trade-off: In-memory storage for simplicity over persistence
 * Failure Mode: Session overflow triggers cleanup before rejection
 */

import { randomUUID } from 'node:crypto';
import type { Logger } from 'pino';
import { WorkflowState, Result, Success, Failure } from '@/types';

const DEFAULT_TTL = 86400; // 24 hours in seconds
const DEFAULT_MAX_SESSIONS = 1000;
const DEFAULT_CLEANUP_INTERVAL = 5 * 60 * 1000; // 5 minutes

/**
 * Extended WorkflowState with session tracking
 */
interface Session extends WorkflowState {
  lastAccessedAt: Date;
}

/**
 * Session configuration
 */
export interface SessionConfig {
  ttl?: number; // Time to live in seconds
  maxSessions?: number;
  cleanupIntervalMs?: number;
}

/**
 * Session Manager using class-based approach
 */
export class SessionManager {
  private sessions = new Map<string, Session>();
  private cleanupTimer: NodeJS.Timeout;
  private logger: Logger;
  private ttl: number;
  private maxSessions: number;
  private static instanceCounter = 0;
  private instanceId: number;

  constructor(logger: Logger, config: SessionConfig = {}) {
    this.instanceId = ++SessionManager.instanceCounter;
    this.logger = logger.child({ service: 'session-manager', instanceId: this.instanceId });
    this.ttl = config.ttl ?? DEFAULT_TTL;
    this.maxSessions = config.maxSessions ?? DEFAULT_MAX_SESSIONS;

    // Start automatic cleanup
    const cleanupInterval = config.cleanupIntervalMs ?? DEFAULT_CLEANUP_INTERVAL;
    this.cleanupTimer = setInterval(() => this.performCleanup(), cleanupInterval);
    this.cleanupTimer.unref();

    this.logger.info(
      {
        instanceId: this.instanceId,
        maxSessions: this.maxSessions,
        ttlSeconds: this.ttl,
      },
      'Session manager initialized',
    );
  }

  /**
   * Create a new session
   */
  async create(sessionId?: string): Promise<Result<WorkflowState>> {
    // Check session limit and cleanup if needed
    if (this.sessions.size >= this.maxSessions) {
      this.cleanupExpired();
      if (this.sessions.size >= this.maxSessions) {
        return Failure(`Maximum sessions (${this.maxSessions}) reached`);
      }
    }

    const id = sessionId ?? randomUUID();
    const now = new Date();

    const session: Session = {
      sessionId: id,
      metadata: {},
      completed_steps: [],
      errors: {},
      current_step: null,
      createdAt: now,
      updatedAt: now,
      lastAccessedAt: now,
    };

    this.sessions.set(id, session);
    this.logger.info(
      {
        sessionId: id,
        totalSessions: this.sessions.size,
      },
      'Session created',
    );

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Get a session by ID
   */
  async get(sessionId: string): Promise<Result<WorkflowState | null>> {
    const session = this.sessions.get(sessionId);

    this.logger.info(
      {
        sessionId,
        found: !!session,
        totalSessions: this.sessions.size,
        allSessionIds: Array.from(this.sessions.keys()),
      },
      'Session lookup',
    );

    if (!session) {
      this.logger.warn(
        { sessionId, availableSessions: Array.from(this.sessions.keys()) },
        'Session not found',
      );
      return Success(null);
    }

    // Check if expired based on lastAccessedAt
    const sessionAge = Date.now() - session.lastAccessedAt.getTime();
    const ttlMs = this.ttl * 1000;
    if (sessionAge > ttlMs) {
      this.sessions.delete(sessionId);
      this.logger.warn(
        {
          sessionId,
          sessionAgeMs: sessionAge,
          ttlMs,
          sessionAgeMinutes: Math.round(sessionAge / 60000),
        },
        'Expired session removed',
      );
      return Success(null);
    }

    // Update access time
    session.lastAccessedAt = new Date();

    this.logger.info(
      {
        sessionId,
        sessionKeys: Object.keys(session),
        hasAnalyzeRepoResult: 'analyze-repo-result' in session,
      },
      'Session found and returning',
    );

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Update a session
   */
  async update(sessionId: string, state: Partial<WorkflowState>): Promise<Result<WorkflowState>> {
    const session = this.sessions.get(sessionId);
    if (!session) {
      this.logger.error(
        { sessionId, availableSessions: Array.from(this.sessions.keys()) },
        'Session not found for update',
      );
      return Failure(`Session ${sessionId} not found`);
    }

    this.logger.info(
      {
        sessionId,
        beforeUpdateKeys: Object.keys(session),
        updateDataKeys: Object.keys(state),
        hasAnalyzeRepoResultInUpdate: 'analyze-repo-result' in state,
      },
      'About to update session',
    );

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

    this.logger.info(
      {
        sessionId,
        updatedFields: Object.keys(state),
        afterUpdateKeys: Object.keys(session),
        hasAnalyzeRepoResultAfterUpdate: 'analyze-repo-result' in session,
        analyzeRepoResultData: session['analyze-repo-result']
          ? JSON.stringify(session['analyze-repo-result']).substring(0, 100)
          : null,
      },
      'Session updated successfully',
    );

    // Return without lastAccessedAt to match WorkflowState interface
    const { lastAccessedAt: _lastAccessed, ...workflowState } = session;
    return Success(workflowState);
  }

  /**
   * Delete a session
   */
  async delete(sessionId: string): Promise<Result<void>> {
    const existed = this.sessions.delete(sessionId);
    if (existed) {
      this.logger.debug({ sessionId }, 'Session deleted');
    }
    return Success(undefined);
  }

  /**
   * List all session IDs
   */
  async list(): Promise<Result<string[]>> {
    return Success(Array.from(this.sessions.keys()));
  }

  /**
   * Cleanup sessions older than a specific date
   */
  async cleanup(olderThan: Date): Promise<Result<void>> {
    let cleanedCount = 0;
    for (const [id, session] of this.sessions.entries()) {
      if (session.createdAt < olderThan) {
        this.sessions.delete(id);
        cleanedCount++;
      }
    }
    if (cleanedCount > 0) {
      this.logger.debug({ cleanedCount }, 'Sessions cleaned up');
    }
    return Success(undefined);
  }

  /**
   * Close the session manager and stop cleanup
   */
  close(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer);
    }
    this.logger.info('Session manager closed');
  }

  /**
   * Internal: Cleanup expired sessions based on TTL
   */
  private cleanupExpired(): void {
    const now = Date.now();
    const expired: string[] = [];

    for (const [id, session] of this.sessions.entries()) {
      if (now - session.lastAccessedAt.getTime() > this.ttl * 1000) {
        expired.push(id);
      }
    }

    expired.forEach((id) => this.sessions.delete(id));

    if (expired.length > 0) {
      this.logger.debug({ expiredCount: expired.length }, 'Expired sessions cleaned up');
    }
  }

  /**
   * Internal: Periodic cleanup handler
   */
  private performCleanup(): void {
    try {
      this.cleanupExpired();
    } catch (err) {
      this.logger.warn({ error: err }, 'Session cleanup failed');
    }
  }
}

/**
 * Factory function to create a session manager instance
 * Maintains backward compatibility with existing code
 */
export function createSessionManager(logger: Logger, config?: SessionConfig): SessionManager {
  return new SessionManager(logger, config);
}
