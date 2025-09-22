import { SessionManager } from '@/lib/session-manager';
import type { Middleware } from '@/mcp/router';

/**
 * Lightweight session middleware:
 * - Ensures a sessionId exists (generate if missing)
 * - Touches last-access bookkeeping in SessionManager (optional)
 *
 * Keep it boring: no TTL/persistence logic here  that belongs in SessionManager.
 */
export function createSessionMiddleware(manager = new SessionManager()): Middleware {
  return async (params, logger, _context, next) => {
    // Generate or normalize the session id
    const current = (params?.sessionId as string | undefined) ?? undefined;
    const sessionId = manager.ensureSession(current);

    // Write back so downstream tools see a stable id
    (params).sessionId = sessionId;

    // Very light touch: record last access
    manager.set(sessionId, '__lastAccess', new Date().toISOString());
    logger.debug({ sessionId }, 'session ensured');

    return next(params, logger, _context);
  };
}

// Small helper for one-liner installs
export const sessionMiddleware = createSessionMiddleware();
