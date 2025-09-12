/**
 * Mutex-protected Session Manager
 *
 * Thread-safe session management with mutex protection for atomic updates
 */

import { randomUUID } from 'node:crypto';
import type { Logger } from 'pino';
import { createKeyedMutex, type KeyedMutexInstance } from './mutex';
import { Result, Success, Failure, WorkflowState } from '../types';
import { config } from '../config';

export interface SessionConfig {
  ttl?: number; // Session TTL in seconds (default: 24 hours)
  maxSessions?: number; // Max concurrent sessions (default: 1000)
  cleanupIntervalMs?: number; // Cleanup interval in ms (default: 5 minutes)
}

const DEFAULT_TTL = 86400; // 24 hours in seconds
const DEFAULT_MAX_SESSIONS = 1000;
const DEFAULT_CLEANUP_INTERVAL = 5 * 60 * 1000; // 5 minutes

interface InternalSession {
  id: string;
  workflowState: WorkflowState;
  created_at: Date;
  updated_at: Date;
}

/**
 * Mutex-protected session manager
 */
export class MutexSessionManager {
  private readonly sessions = new Map<string, InternalSession>();
  private readonly mutex: KeyedMutexInstance;
  private readonly logger: Logger;
  private readonly ttl: number;
  private readonly maxSessions: number;
  private cleanupTimer?: NodeJS.Timeout;

  constructor(logger: Logger, sessionConfig: SessionConfig = {}) {
    this.logger = logger.child({ service: 'mutex-session-manager' });
    this.ttl = sessionConfig.ttl ?? DEFAULT_TTL;
    this.maxSessions = sessionConfig.maxSessions ?? DEFAULT_MAX_SESSIONS;

    this.mutex = createKeyedMutex({
      defaultTimeout: config.mutex.defaultTimeout,
      monitoringEnabled: config.mutex.monitoringEnabled,
    });

    // Start automatic cleanup
    const cleanupInterval = sessionConfig.cleanupIntervalMs ?? DEFAULT_CLEANUP_INTERVAL;
    this.cleanupTimer = setInterval(() => {
      this.cleanupExpiredSessions().catch((err) => {
        this.logger.warn({ error: err }, 'Session cleanup failed');
      });
    }, cleanupInterval);

    // Don't keep process alive for cleanup
    this.cleanupTimer.unref?.();

    this.logger.info(
      {
        maxSessions: this.maxSessions,
        ttlSeconds: this.ttl,
      },
      'Mutex session manager initialized',
    );
  }

  /**
   * Create a new session with mutex protection
   */
  async create(sessionId?: string): Promise<Result<WorkflowState>> {
    const id = sessionId ?? randomUUID();
    const lockKey = `session:create:${id}`;

    return this.mutex.withLock(lockKey, async () => {
      // Check if session already exists
      if (this.sessions.has(id)) {
        return Failure(`Session ${id} already exists`);
      }

      // Check session limit
      if (this.sessions.size >= this.maxSessions) {
        await this.cleanupExpiredSessions();
        if (this.sessions.size >= this.maxSessions) {
          return Failure(
            `Maximum sessions (${this.maxSessions}) reached. Current: ${this.sessions.size}`,
          );
        }
      }

      const now = new Date();
      const workflowState: WorkflowState = {
        sessionId: id,
        metadata: {},
        completed_steps: [],
        errors: {},
        current_step: null,
        createdAt: now,
        updatedAt: now,
      };

      const session: InternalSession = {
        id,
        workflowState,
        created_at: now,
        updated_at: now,
      };

      this.sessions.set(id, session);

      this.logger.info(
        {
          sessionId: id,
          totalSessions: this.sessions.size,
        },
        'Session created',
      );

      return Success(workflowState);
    });
  }

  /**
   * Get a session by ID (no mutex needed for reads)
   */
  async get(sessionId: string): Promise<Result<WorkflowState | null>> {
    const session = this.sessions.get(sessionId);

    if (!session) {
      this.logger.debug({ sessionId }, 'Session not found');
      return Success(null);
    }

    // Check if expired
    const now = Date.now();
    if (now - session.created_at.getTime() > this.ttl * 1000) {
      this.logger.debug({ sessionId }, 'Session expired');
      this.sessions.delete(sessionId);
      return Success(null);
    }

    return Success(session.workflowState);
  }

  /**
   * Update a session with mutex protection
   */
  async update(sessionId: string, updates: Partial<WorkflowState>): Promise<Result<WorkflowState>> {
    const lockKey = `session:update:${sessionId}`;

    return this.mutex.withLock(lockKey, async () => {
      const session = this.sessions.get(sessionId);

      if (!session) {
        return Failure(`Session ${sessionId} not found`);
      }

      // Check if expired
      const now = Date.now();
      if (now - session.created_at.getTime() > this.ttl * 1000) {
        this.sessions.delete(sessionId);
        return Failure(`Session ${sessionId} has expired`);
      }

      // Merge updates
      const updatedState: WorkflowState = {
        ...session.workflowState,
        ...updates,
        sessionId, // Preserve session ID
        updatedAt: new Date(),
      };

      // Deep merge for nested objects
      if (updates.metadata) {
        updatedState.metadata = {
          ...session.workflowState.metadata,
          ...updates.metadata,
        };
      }

      if (updates.errors) {
        updatedState.errors = {
          ...(session.workflowState.errors || {}),
          ...updates.errors,
        };
      }

      if (updates.completed_steps) {
        updatedState.completed_steps = [
          ...new Set([
            ...(session.workflowState.completed_steps || []),
            ...updates.completed_steps,
          ]),
        ];
      }

      // Update session
      session.workflowState = updatedState;
      session.updated_at = new Date();

      this.logger.debug(
        {
          sessionId,
          updatedFields: Object.keys(updates),
        },
        'Session updated',
      );

      return Success(updatedState);
    });
  }

  /**
   * Delete a session with mutex protection
   */
  async delete(sessionId: string): Promise<Result<boolean>> {
    const lockKey = `session:delete:${sessionId}`;

    return this.mutex.withLock(lockKey, async () => {
      const existed = this.sessions.delete(sessionId);

      if (existed) {
        this.logger.info({ sessionId }, 'Session deleted');
      } else {
        this.logger.debug({ sessionId }, 'Session not found for deletion');
      }

      return Success(existed);
    });
  }

  /**
   * List all active sessions (no mutex needed for reads)
   */
  async list(): Promise<Result<string[]>> {
    const sessionIds = Array.from(this.sessions.keys());
    return Success(sessionIds);
  }

  /**
   * Clear all sessions with mutex protection
   */
  async clear(): Promise<Result<number>> {
    const lockKey = 'session:clear:all';

    return this.mutex.withLock(lockKey, async () => {
      const count = this.sessions.size;
      this.sessions.clear();

      this.logger.info({ clearedCount: count }, 'All sessions cleared');
      return Success(count);
    });
  }

  /**
   * Cleanup expired sessions with mutex protection
   */
  private async cleanupExpiredSessions(): Promise<void> {
    const lockKey = 'session:cleanup';

    await this.mutex.withLock(lockKey, async () => {
      const now = Date.now();
      let expiredCount = 0;

      for (const [id, session] of this.sessions.entries()) {
        if (now - session.created_at.getTime() > this.ttl * 1000) {
          this.sessions.delete(id);
          expiredCount++;
        }
      }

      if (expiredCount > 0) {
        this.logger.debug({ expiredCount }, 'Expired sessions cleaned up');
      }
    });
  }

  /**
   * Get session statistics
   */
  getStats(): {
    totalSessions: number;
    maxSessions: number;
    ttlSeconds: number;
    mutexStatus: Map<string, any>;
  } {
    return {
      totalSessions: this.sessions.size,
      maxSessions: this.maxSessions,
      ttlSeconds: this.ttl,
      mutexStatus: this.mutex.getStatus(),
    };
  }

  /**
   * Cleanup resources
   */
  destroy(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer);
      delete this.cleanupTimer;
    }
    this.sessions.clear();
    this.mutex.releaseAll();
    this.logger.info('Session manager destroyed');
  }
}

/**
 * Create a mutex-protected session manager instance
 */
export function createMutexSessionManager(
  logger: Logger,
  config?: SessionConfig,
): MutexSessionManager {
  return new MutexSessionManager(logger, config);
}
