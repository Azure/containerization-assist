/**
 * Session Manager – tiny, typed, TTL-aware
 */
import { createLogger } from '@/lib/logger';
import { randomUUID } from 'node:crypto';

const log = createLogger().child({ module: 'session-manager' });

type SessionId = string;
type Key = string;

interface Entry<T = unknown> {
  value: T;
  expiresAt?: number; // if set, key self-expires
}

type SessionStore = Map<Key, Entry>;
export class SessionManager {
  private sessions = new Map<SessionId, SessionStore>();
  private cleanupTimer?: NodeJS.Timeout;

  constructor(private maxSessions = 100) {
    this.cleanupTimer = setInterval(() => this.cleanup(), 5 * 60 * 1000);
    this.cleanupTimer.unref?.();
  }

  ensureSession(id?: SessionId): SessionId {
    const sid = id ?? randomUUID();
    if (!this.sessions.has(sid)) {
      this.sessions.set(sid, new Map());
      log.debug({ sessionId: sid }, 'Created session');
    }
    return sid;
  }

  get<T>(sessionId: SessionId, key: Key): T | undefined {
    const store = this.sessions.get(sessionId);
    if (!store) return undefined;
    const e = store.get(key);
    if (!e) return undefined;
    if (e.expiresAt && e.expiresAt <= Date.now()) {
      store.delete(key);
      return undefined;
    }
    return e.value as T;
  }

  set<T>(sessionId: SessionId, key: Key, value: T, ttlMs?: number): void {
    const store = this.sessions.get(sessionId) ?? new Map();
    const entry: Entry<T> = ttlMs ? { value, expiresAt: Date.now() + ttlMs } : { value };
    store.set(key, entry);
    this.sessions.set(sessionId, store);
  }

  delete(sessionId: SessionId, key?: Key): void {
    if (!key) {
      this.sessions.delete(sessionId);
      return;
    }
    this.sessions.get(sessionId)?.delete(key);
  }

  has(sessionId: SessionId): boolean {
    return this.sessions.has(sessionId);
  }

  listSessions(): SessionId[] {
    return [...this.sessions.keys()];
  }

  clear(): void {
    this.sessions.clear();
    log.info('Cleared all sessions');
  }

  stop(): void {
    if (this.cleanupTimer) clearInterval(this.cleanupTimer);
  }

  private cleanup(): void {
    // Evict expired keys and keep only newest N sessions
    const now = Date.now();
    for (const store of this.sessions.values()) {
      for (const [k, e] of store) {
        if (e.expiresAt && e.expiresAt <= now) store.delete(k);
      }
    }
    if (this.sessions.size > this.maxSessions) {
      const ids = [...this.sessions.keys()];
      const toRemove = ids.slice(0, this.sessions.size - this.maxSessions);
      toRemove.forEach((id) => this.sessions.delete(id));
      log.debug({ removed: toRemove.length }, 'Trimmed old sessions');
    }
  }
}
