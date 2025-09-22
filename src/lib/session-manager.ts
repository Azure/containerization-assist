/**
 * Session Manager
 *
 * Simple centralized session management with Map-based storage
 */

import { createLogger } from '@/lib/logger';
import { randomUUID } from 'node:crypto';

const logger = createLogger().child({ module: 'session-manager' });

/**
 * Session data stored as a simple Map
 */
export class SessionManager {
  private sessions = new Map<string, Map<string, unknown>>();
  private cleanupTimer?: NodeJS.Timeout;

  constructor() {
    // Clean up expired sessions every 5 minutes
    this.cleanupTimer = setInterval(() => this.cleanup(), 5 * 60 * 1000);
    if (this.cleanupTimer.unref) {
      this.cleanupTimer.unref();
    }
  }

  /**
   * Get or create a session
   */
  ensureSession(id?: string): string {
    const sessionId = id || randomUUID();

    if (!this.sessions.has(sessionId)) {
      this.sessions.set(sessionId, new Map());
      logger.debug({ sessionId }, 'Created new session');
    }

    return sessionId;
  }

  /**
   * Get session data
   */
  get<T>(sessionId: string, key: string): T | undefined {
    const session = this.sessions.get(sessionId);
    return session?.get(key) as T | undefined;
  }

  /**
   * Set session data
   */
  set<T>(sessionId: string, key: string, value: T): void {
    const session = this.sessions.get(sessionId);
    if (session) {
      session.set(key, value);
    } else {
      const newSession = new Map();
      newSession.set(key, value);
      this.sessions.set(sessionId, newSession);
    }
  }

  /**
   * Delete session data
   */
  delete(sessionId: string, key?: string): void {
    if (key) {
      this.sessions.get(sessionId)?.delete(key);
    } else {
      this.sessions.delete(sessionId);
    }
  }

  /**
   * Check if session exists
   */
  has(sessionId: string): boolean {
    return this.sessions.has(sessionId);
  }

  /**
   * Get all session IDs
   */
  listSessions(): string[] {
    return Array.from(this.sessions.keys());
  }

  /**
   * Clear all sessions
   */
  clear(): void {
    this.sessions.clear();
    logger.info('Cleared all sessions');
  }

  /**
   * Clean up old sessions (keep only last 100)
   */
  private cleanup(): void {
    if (this.sessions.size > 100) {
      const toKeep = Array.from(this.sessions.keys()).slice(-100);
      const newSessions = new Map<string, Map<string, unknown>>();

      for (const id of toKeep) {
        const session = this.sessions.get(id);
        if (session) {
          newSessions.set(id, session);
        }
      }

      const removed = this.sessions.size - newSessions.size;
      this.sessions = newSessions;

      if (removed > 0) {
        logger.debug({ removed }, 'Cleaned up old sessions');
      }
    }
  }

  /**
   * Stop cleanup timer
   */
  stop(): void {
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer);
    }
  }
}
