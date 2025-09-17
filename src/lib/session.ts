/**
 * Session Manager - Simplified workflow state persistence
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
 * Simplified Session Manager using class-based approach
 */
export class SessionManager {
  private sessions = new Map<string, Session>();
  private cleanupTimer: NodeJS.Timeout;
  private logger: Logger;
  private ttl: number;
  private maxSessions: number;

  constructor(logger: Logger, config: SessionConfig = {}) {
    this.logger = logger.child({ service: 'session-manager' });
    this.ttl = config.ttl ?? DEFAULT_TTL;
    this.maxSessions = config.maxSessions ?? DEFAULT_MAX_SESSIONS;

    // Start automatic cleanup
    const cleanupInterval = config.cleanupIntervalMs ?? DEFAULT_CLEANUP_INTERVAL;
    this.cleanupTimer = setInterval(() => this.performCleanup(), cleanupInterval);
    this.cleanupTimer.unref();

    this.logger.info(
      {
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

    this.logger.debug(
      {
        sessionId,
        found: !!session,
      },
      'Session lookup',
    );

    if (!session) {
      return Success(null);
    }

    // Check if expired based on lastAccessedAt
    if (Date.now() - session.lastAccessedAt.getTime() > this.ttl * 1000) {
      this.sessions.delete(sessionId);
      this.logger.debug({ sessionId }, 'Expired session removed');
      return Success(null);
    }

    // Update access time
    session.lastAccessedAt = new Date();

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
