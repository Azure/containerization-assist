/**
 * Session Module - Consolidated session management
 *
 * Provides session management capabilities with different implementations:
 * - core: Main SessionManager class
 * - manager: Alternative session manager with stats
 * - slices: Session slice utilities
 * - types: Session type definitions
 */

// Main session manager (most commonly used)
export { SessionManager, type SessionConfig } from './core';

// Alternative enhanced manager with stats
export { EnhancedSessionManager } from './manager';
