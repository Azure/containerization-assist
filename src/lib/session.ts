/**
 * Session Manager - Thread-safe workflow state persistence
 *
 * Invariant: Sessions expire after TTL to prevent memory leaks
 * Trade-off: In-memory storage for simplicity over persistence
 * Failure Mode: Session overflow triggers FIFO eviction
 */

import { randomUUID } from 'node:crypto';
import type { Logger } from 'pino';
import { WorkflowState, Result, Success, Failure } from '../types';
import { createSessionError, ErrorCodes } from './errors';

// Session configuration options inline type removed - use direct parameters

const DEFAULT_TTL = 86400; // 24 hours in seconds
const DEFAULT_MAX_SESSIONS = 1000;
const DEFAULT_CLEANUP_INTERVAL = 5 * 60 * 1000; // 5 minutes

// Internal session storage with timestamps
interface InternalSession {
  id: string;
  workflowState: WorkflowState;
  created_at: Date;
  updated_at: Date;
}

// Module state
interface SessionStore {
  sessions: Map<string, InternalSession>;
  cleanupTimer?: NodeJS.Timeout;
  logger: Logger;
  ttl: number;
  maxSessions: number;
}

/**
 * Create session store with configuration
 */
function createSessionStore(
  logger: Logger,
  config: {
    ttl?: number;
    maxSessions?: number;
    cleanupIntervalMs?: number;
  } = {},
): SessionStore {
  const store: SessionStore = {
    sessions: new Map(),
    logger: logger.child({ service: 'session-manager' }),
    ttl: config.ttl ?? DEFAULT_TTL,
    maxSessions: config.maxSessions ?? DEFAULT_MAX_SESSIONS,
  };

  // Start automatic cleanup
  const cleanupInterval = config.cleanupIntervalMs ?? DEFAULT_CLEANUP_INTERVAL;
  store.cleanupTimer = setInterval(() => {
    try {
      cleanupExpiredSessions(store);
    } catch (err) {
      store.logger.warn({ error: err }, 'Session cleanup failed');
    }
  }, cleanupInterval);

  // Don't keep process alive for cleanup
  store.cleanupTimer.unref?.();

  store.logger.info(
    {
      maxSessions: store.maxSessions,
      ttlSeconds: store.ttl,
    },
    'Session manager initialized',
  );

  return store;
}

/**
 * Cleanup expired sessions
 */
function cleanupExpiredSessions(store: SessionStore): void {
  const now = Date.now();
  let expiredCount = 0;

  for (const [id, session] of store.sessions.entries()) {
    if (now - session.created_at.getTime() > store.ttl * 1000) {
      store.sessions.delete(id);
      expiredCount++;
    }
  }

  if (expiredCount > 0) {
    store.logger.debug({ expiredCount }, 'Expired sessions cleaned up');
  }
}

/**
 * Create a new session
 */
export async function create(
  store: SessionStore,
  sessionId?: string,
): Promise<Result<WorkflowState>> {
  // Check session limit
  if (store.sessions.size >= store.maxSessions) {
    cleanupExpiredSessions(store);
    if (store.sessions.size >= store.maxSessions) {
      const error = createSessionError(
        `Maximum sessions (${store.maxSessions}) reached`,
        ErrorCodes.SESSION_LIMIT_EXCEEDED,
        { maxSessions: store.maxSessions, currentCount: store.sessions.size },
      );
      return Failure(`${error.code}: ${error.message}`);
    }
  }

  const id = sessionId ?? randomUUID();
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

  store.sessions.set(id, session);
  store.logger.info(
    {
      sessionId: id,
      totalSessions: store.sessions.size,
    },
    'Session created',
  );

  return Success(workflowState);
}

/**
 * Get a session by ID
 */
export async function get(
  store: SessionStore,
  sessionId: string,
): Promise<Result<WorkflowState | null>> {
  const session = store.sessions.get(sessionId);

  store.logger.debug(
    {
      sessionId,
      found: !!session,
    },
    'Session lookup',
  );

  if (!session) {
    return Success(null);
  }

  // Check if expired
  if (Date.now() - session.created_at.getTime() > store.ttl * 1000) {
    store.sessions.delete(sessionId);
    store.logger.debug({ sessionId }, 'Expired session removed');
    return Success(null);
  }

  return Success(session.workflowState);
}

/**
 * Update a session
 */
export async function update(
  store: SessionStore,
  sessionId: string,
  state: Partial<WorkflowState>,
): Promise<Result<WorkflowState>> {
  const session = store.sessions.get(sessionId);
  if (!session) {
    const error = createSessionError(
      `Session ${sessionId} not found`,
      ErrorCodes.SESSION_NOT_FOUND,
      { sessionId },
    );
    return Failure(`${error.code}: ${error.message}`);
  }

  // Update workflow state - merge state into existing workflowState
  const updatedWorkflowState: WorkflowState = {
    ...session.workflowState,
    ...state,
    metadata: {
      ...(session.workflowState.metadata || {}),
      ...(state.metadata || {}),
    },
    updatedAt: new Date(),
  };

  const updatedSession: InternalSession = {
    ...session,
    workflowState: updatedWorkflowState,
    updated_at: new Date(),
  };

  store.sessions.set(sessionId, updatedSession);
  store.logger.debug(
    {
      sessionId,
      updatedFields: Object.keys(state).length,
      hasAnalysis: 'analysis' in state,
    },
    'Session updated',
  );
  return Success(updatedWorkflowState);
}

/**
 * Delete a session
 */
export async function deleteSession(store: SessionStore, sessionId: string): Promise<Result<void>> {
  const existed = store.sessions.delete(sessionId);
  if (existed) {
    store.logger.debug({ sessionId }, 'Session deleted');
  }
  return Success(undefined);
}

/**
 * List all session IDs
 */
export async function list(store: SessionStore): Promise<Result<string[]>> {
  return Success(Array.from(store.sessions.keys()));
}

/**
 * Cleanup old sessions
 */
export async function cleanup(store: SessionStore, olderThan: Date): Promise<Result<void>> {
  let cleanedCount = 0;
  for (const [id, session] of store.sessions.entries()) {
    if (session.created_at < olderThan) {
      store.sessions.delete(id);
      cleanedCount++;
    }
  }
  store.logger.debug({ cleanedCount }, 'Session cleanup completed');
  return Success(undefined);
}

// Removed duplicate Result-based methods - use main interface methods instead

/**
 * Close the session manager and stop cleanup
 */
export function close(store: SessionStore): void {
  if (store.cleanupTimer) {
    clearInterval(store.cleanupTimer);
    delete store.cleanupTimer;
  }
  store.logger.info('Session manager closed');
}

/**
 * SessionManager interface - unified session management API
 */
export interface SessionManager {
  create(sessionId?: string): Promise<Result<WorkflowState>>;
  get(sessionId: string): Promise<Result<WorkflowState | null>>;
  update(sessionId: string, state: Partial<WorkflowState>): Promise<Result<WorkflowState>>;
  delete(sessionId: string): Promise<Result<void>>;
  list(): Promise<Result<string[]>>;
  cleanup(olderThan: Date): Promise<Result<void>>;
  close(): void;
}

/**
 * Factory function to create a session manager instance
 * Returns an object with all methods bound to internal store
 */
export function createSessionManager(
  logger: Logger,
  config?: {
    ttl?: number;
    maxSessions?: number;
    cleanupIntervalMs?: number;
  },
): SessionManager {
  const store = createSessionStore(logger, config);

  return {
    create: (sessionId?: string) => create(store, sessionId),
    get: (sessionId: string) => get(store, sessionId),
    update: (sessionId: string, state: Partial<WorkflowState>) => update(store, sessionId, state),
    delete: (sessionId: string) => deleteSession(store, sessionId),
    list: () => list(store),
    cleanup: (olderThan: Date) => cleanup(store, olderThan),
    close: () => close(store),
  };
}
