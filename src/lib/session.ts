/**
 * Session Manager Implementation
 *
 * Simplified session management functionality providing:
 * - Session lifecycle management
 * - Simple WorkflowState storage
 * - Thread-safe operations
 */

import { randomUUID } from 'node:crypto';
import type { Logger } from 'pino';
import { Result, Success, Failure, WorkflowState } from '@types';
import { SessionError, ErrorCodes } from './errors';

export interface SessionConfig {
  ttl?: number; // Session TTL in seconds (default: 24 hours)
  maxSessions?: number; // Max concurrent sessions (default: 1000)
  cleanupIntervalMs?: number; // Cleanup interval in ms (default: 5 minutes)
}

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
function createSessionStore(logger: Logger, config: SessionConfig = {}): SessionStore {
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
export async function create(store: SessionStore, sessionId?: string): Promise<WorkflowState> {
  // Check session limit
  if (store.sessions.size >= store.maxSessions) {
    cleanupExpiredSessions(store);
    if (store.sessions.size >= store.maxSessions) {
      throw new SessionError(
        `Maximum sessions (${store.maxSessions}) reached`,
        ErrorCodes.SESSION_LIMIT_EXCEEDED,
        { maxSessions: store.maxSessions, currentCount: store.sessions.size },
      );
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
      sessionKeys: Object.keys(workflowState),
    },
    'Session created',
  );

  return workflowState;
}

/**
 * Get a session by ID
 */
export async function get(store: SessionStore, sessionId: string): Promise<WorkflowState | null> {
  const session = store.sessions.get(sessionId);

  store.logger.info(
    {
      sessionId,
      found: !!session,
      totalSessions: store.sessions.size,
      allSessionIds: Array.from(store.sessions.keys()),
      sessionData: session ? Object.keys(session.workflowState) : null,
    },
    'Session lookup',
  );

  if (!session) {
    return null;
  }

  // Check if expired
  if (Date.now() - session.created_at.getTime() > store.ttl * 1000) {
    store.sessions.delete(sessionId);
    store.logger.debug({ sessionId }, 'Expired session removed');
    return null;
  }

  return session.workflowState;
}

/**
 * Update a session
 */
export async function update(
  store: SessionStore,
  sessionId: string,
  state: Partial<WorkflowState>,
): Promise<void> {
  const session = store.sessions.get(sessionId);
  if (!session) {
    throw new SessionError(`Session ${sessionId} not found`, ErrorCodes.SESSION_NOT_FOUND, {
      sessionId,
    });
  }

  // Update workflow state
  const updatedWorkflowState: WorkflowState = {
    ...session.workflowState,
    ...state,
    metadata: {
      ...(session.workflowState.metadata || {}),
      ...(state.metadata || {}),
    },
    completed_steps: state.completed_steps ?? session.workflowState.completed_steps ?? [],
    updatedAt: new Date(),
  };

  const updatedSession: InternalSession = {
    ...session,
    workflowState: updatedWorkflowState,
    updated_at: new Date(),
  };

  store.sessions.set(sessionId, updatedSession);
  store.logger.info(
    {
      sessionId,
      updatedKeys: Object.keys(updatedWorkflowState),
      hasAnalysisResult: 'analysis_result' in updatedWorkflowState,
      completedSteps: updatedWorkflowState.completed_steps,
      totalSessions: store.sessions.size,
    },
    'Session updated',
  );
}

/**
 * Delete a session
 */
export async function deleteSession(store: SessionStore, sessionId: string): Promise<void> {
  const existed = store.sessions.delete(sessionId);
  if (existed) {
    store.logger.debug({ sessionId }, 'Session deleted');
  }
}

/**
 * List all session IDs
 */
export async function list(store: SessionStore): Promise<string[]> {
  return Array.from(store.sessions.keys());
}

/**
 * Cleanup old sessions
 */
export async function cleanup(store: SessionStore, olderThan: Date): Promise<void> {
  let cleanedCount = 0;
  for (const [id, session] of store.sessions.entries()) {
    if (session.created_at < olderThan) {
      store.sessions.delete(id);
      cleanedCount++;
    }
  }
  store.logger.debug({ cleanedCount }, 'Session cleanup completed');
}

/**
 * Interface compliance methods with Result types
 */

export async function createSession(
  store: SessionStore,
  id: string,
): Promise<Result<WorkflowState>> {
  try {
    const sessionState = await create(store, id);
    return Success(sessionState);
  } catch (error) {
    return Failure(error instanceof Error ? error.message : 'Failed to create session');
  }
}

export async function getSession(store: SessionStore, id: string): Promise<Result<WorkflowState>> {
  try {
    const sessionState = await get(store, id);
    if (!sessionState) {
      return Failure(`Session ${id} not found`);
    }
    return Success(sessionState);
  } catch (error) {
    return Failure(error instanceof Error ? error.message : 'Failed to get session');
  }
}

export async function updateSession(
  store: SessionStore,
  id: string,
  updates: Partial<WorkflowState>,
): Promise<Result<WorkflowState>> {
  try {
    await update(store, id, updates);
    const updatedState = await get(store, id);
    if (!updatedState) {
      return Failure(`Session ${id} not found after update`);
    }
    return Success(updatedState);
  } catch (error) {
    return Failure(error instanceof Error ? error.message : 'Failed to update session');
  }
}

export async function deleteSessionResult(
  store: SessionStore,
  id: string,
): Promise<Result<boolean>> {
  try {
    await deleteSession(store, id);
    return Success(true);
  } catch (error) {
    return Failure(error instanceof Error ? error.message : 'Failed to delete session');
  }
}

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
 * SessionManager interface for backward compatibility
 */
export interface SessionManager {
  create(sessionId?: string): Promise<WorkflowState>;
  get(sessionId: string): Promise<WorkflowState | null>;
  update(sessionId: string, state: Partial<WorkflowState>): Promise<void>;
  delete(sessionId: string): Promise<void>;
  list(): Promise<string[]>;
  cleanup(olderThan: Date): Promise<void>;
  createSession(id: string): Promise<Result<WorkflowState>>;
  getSession(id: string): Promise<Result<WorkflowState>>;
  updateSession(id: string, updates: Partial<WorkflowState>): Promise<Result<WorkflowState>>;
  deleteSession(id: string): Promise<Result<boolean>>;
  close(): void;
}

/**
 * Factory function to create a session manager instance
 * Returns an object with all methods bound to internal store
 */
export function createSessionManager(logger: Logger, config?: SessionConfig): SessionManager {
  const store = createSessionStore(logger, config);

  return {
    create: (sessionId?: string) => create(store, sessionId),
    get: (sessionId: string) => get(store, sessionId),
    update: (sessionId: string, state: Partial<WorkflowState>) => update(store, sessionId, state),
    delete: (sessionId: string) => deleteSession(store, sessionId),
    list: () => list(store),
    cleanup: (olderThan: Date) => cleanup(store, olderThan),
    createSession: (id: string) => createSession(store, id),
    getSession: (id: string) => getSession(store, id),
    updateSession: (id: string, updates: Partial<WorkflowState>) =>
      updateSession(store, id, updates),
    deleteSession: (id: string) => deleteSessionResult(store, id),
    close: () => close(store),
  };
}
