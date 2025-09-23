/**
 * Session Manager Tests
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { SessionManager } from '@/lib/session-manager';

describe('SessionManager', () => {
  let manager: SessionManager;

  beforeEach(() => {
    manager = new SessionManager();
  });

  afterEach(() => {
    manager.stop();
  });

  describe('ensureSession', () => {
    it('should create a new session when no ID provided', () => {
      const sessionId = manager.ensureSession();
      expect(sessionId).toBeTruthy();
      expect(typeof sessionId).toBe('string');
    });

    it('should return existing session when ID provided', () => {
      const sessionId = manager.ensureSession('test-session-1');
      const sessionId2 = manager.ensureSession('test-session-1');
      expect(sessionId).toBe(sessionId2);
      expect(sessionId).toBe('test-session-1');
    });
  });

  describe('get/set operations', () => {
    it('should store and retrieve data', () => {
      const sessionId = manager.ensureSession();
      manager.set(sessionId, 'key1', 'value1');

      const value = manager.get<string>(sessionId, 'key1');
      expect(value).toBe('value1');
    });

    it('should return undefined for non-existent keys', () => {
      const sessionId = manager.ensureSession();
      const value = manager.get(sessionId, 'non-existent');
      expect(value).toBeUndefined();
    });
  });

  describe('delete operations', () => {
    it('should delete entire session when no key provided', () => {
      const sessionId = manager.ensureSession();
      manager.set(sessionId, 'key1', 'value1');

      manager.delete(sessionId);

      expect(manager.has(sessionId)).toBe(false);
      expect(manager.get(sessionId, 'key1')).toBeUndefined();
    });
  });

  describe('clear operation', () => {
    it('should remove all sessions', () => {
      manager.ensureSession('session-1');
      manager.ensureSession('session-2');

      expect(manager.listSessions()).toHaveLength(2);

      manager.clear();

      expect(manager.listSessions()).toHaveLength(0);
    });
  });
});
