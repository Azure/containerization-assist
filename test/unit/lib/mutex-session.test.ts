import { MutexSessionManager } from '../../../src/lib/mutex-session';
import pino from 'pino';

// Mock config
jest.mock('../../../src/config', () => ({
  config: {
    mutex: {
      defaultTimeout: 30000,
      monitoringEnabled: true
    }
  }
}));

describe('MutexSessionManager', () => {
  let manager: MutexSessionManager;
  let logger: pino.Logger;

  beforeEach(() => {
    logger = pino({ level: 'silent' });
    manager = new MutexSessionManager(logger, {
      ttl: 60, // 60 seconds for tests
      maxSessions: 5,
      cleanupIntervalMs: 10000
    });
  });

  afterEach(() => {
    manager.destroy();
  });

  describe('create', () => {
    test('should create a new session', async () => {
      const result = await manager.create();
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.sessionId).toBeDefined();
        expect(result.value.metadata).toEqual({});
        expect(result.value.completed_steps).toEqual([]);
      }
    });

    test('should create session with specific ID', async () => {
      const sessionId = 'test-session-123';
      const result = await manager.create(sessionId);
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.sessionId).toBe(sessionId);
      }
    });

    test('should prevent duplicate session IDs', async () => {
      const sessionId = 'duplicate-test';
      
      const result1 = await manager.create(sessionId);
      expect(result1.ok).toBe(true);
      
      const result2 = await manager.create(sessionId);
      expect(result2.ok).toBe(false);
      if (!result2.ok) {
        expect(result2.error).toContain('already exists');
      }
    });

    test('should enforce max sessions limit', async () => {
      // Create max sessions
      for (let i = 0; i < 5; i++) {
        const result = await manager.create(`session-${i}`);
        expect(result.ok).toBe(true);
      }
      
      // Try to create one more
      const result = await manager.create('overflow');
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Maximum sessions');
      }
    });
  });

  describe('get', () => {
    test('should retrieve existing session', async () => {
      const createResult = await manager.create('get-test');
      expect(createResult.ok).toBe(true);
      
      const getResult = await manager.get('get-test');
      expect(getResult.ok).toBe(true);
      if (getResult.ok && getResult.value) {
        expect(getResult.value.sessionId).toBe('get-test');
      }
    });

    test('should return null for non-existent session', async () => {
      const result = await manager.get('non-existent');
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBeNull();
      }
    });
  });

  describe('update', () => {
    test('should update existing session', async () => {
      const sessionId = 'update-test';
      await manager.create(sessionId);
      
      const updateResult = await manager.update(sessionId, {
        current_step: 'build',
        metadata: { buildId: '123' }
      });
      
      expect(updateResult.ok).toBe(true);
      if (updateResult.ok) {
        expect(updateResult.value.current_step).toBe('build');
        expect(updateResult.value.metadata.buildId).toBe('123');
      }
    });

    test('should merge nested objects', async () => {
      const sessionId = 'merge-test';
      await manager.create(sessionId);
      
      // First update
      await manager.update(sessionId, {
        metadata: { key1: 'value1' },
        errors: { step1: 'error1' }
      });
      
      // Second update - should merge
      const result = await manager.update(sessionId, {
        metadata: { key2: 'value2' },
        errors: { step2: 'error2' }
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.metadata).toEqual({
          key1: 'value1',
          key2: 'value2'
        });
        expect(result.value.errors).toEqual({
          step1: 'error1',
          step2: 'error2'
        });
      }
    });

    test('should merge completed_steps without duplicates', async () => {
      const sessionId = 'steps-test';
      await manager.create(sessionId);
      
      await manager.update(sessionId, {
        completed_steps: ['step1', 'step2']
      });
      
      const result = await manager.update(sessionId, {
        completed_steps: ['step2', 'step3']
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.completed_steps).toEqual(['step1', 'step2', 'step3']);
      }
    });

    test('should fail for non-existent session', async () => {
      const result = await manager.update('non-existent', {
        current_step: 'test'
      });
      
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
      }
    });

    test('should handle concurrent updates', async () => {
      const sessionId = 'concurrent-update';
      await manager.create(sessionId);
      
      // Simulate concurrent updates
      const updates = await Promise.all([
        manager.update(sessionId, { metadata: { update1: 'value1' } }),
        manager.update(sessionId, { metadata: { update2: 'value2' } }),
        manager.update(sessionId, { metadata: { update3: 'value3' } })
      ]);
      
      // All should succeed
      expect(updates.every(r => r.ok)).toBe(true);
      
      // Final state should have all updates
      const final = await manager.get(sessionId);
      expect(final.ok).toBe(true);
      if (final.ok && final.value) {
        expect(final.value.metadata).toHaveProperty('update1');
        expect(final.value.metadata).toHaveProperty('update2');
        expect(final.value.metadata).toHaveProperty('update3');
      }
    });
  });

  describe('delete', () => {
    test('should delete existing session', async () => {
      const sessionId = 'delete-test';
      await manager.create(sessionId);
      
      const deleteResult = await manager.delete(sessionId);
      expect(deleteResult.ok).toBe(true);
      if (deleteResult.ok) {
        expect(deleteResult.value).toBe(true);
      }
      
      // Verify deleted
      const getResult = await manager.get(sessionId);
      expect(getResult.ok).toBe(true);
      if (getResult.ok) {
        expect(getResult.value).toBeNull();
      }
    });

    test('should return false for non-existent session', async () => {
      const result = await manager.delete('non-existent');
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe(false);
      }
    });
  });

  describe('list', () => {
    test('should list all session IDs', async () => {
      await manager.create('list-1');
      await manager.create('list-2');
      await manager.create('list-3');
      
      const result = await manager.list();
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveLength(3);
        expect(result.value).toContain('list-1');
        expect(result.value).toContain('list-2');
        expect(result.value).toContain('list-3');
      }
    });
  });

  describe('clear', () => {
    test('should clear all sessions', async () => {
      await manager.create('clear-1');
      await manager.create('clear-2');
      
      const clearResult = await manager.clear();
      
      expect(clearResult.ok).toBe(true);
      if (clearResult.ok) {
        expect(clearResult.value).toBe(2);
      }
      
      const listResult = await manager.list();
      expect(listResult.ok).toBe(true);
      if (listResult.ok) {
        expect(listResult.value).toHaveLength(0);
      }
    });
  });

  describe('getStats', () => {
    test('should return statistics', async () => {
      await manager.create('stats-1');
      await manager.create('stats-2');
      
      const stats = manager.getStats();
      
      expect(stats.totalSessions).toBe(2);
      expect(stats.maxSessions).toBe(5);
      expect(stats.ttlSeconds).toBe(60);
      expect(stats.mutexStatus).toBeDefined();
    });
  });

  describe('expiration', () => {
    test('should not return expired sessions', async () => {
      // Create manager with very short TTL
      const shortManager = new MutexSessionManager(logger, {
        ttl: 0.1, // 100ms
        maxSessions: 10
      });
      
      try {
        const sessionId = 'expire-test';
        await shortManager.create(sessionId);
        
        // Wait for expiration
        await new Promise(resolve => setTimeout(resolve, 150));
        
        const result = await shortManager.get(sessionId);
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBeNull();
        }
      } finally {
        shortManager.destroy();
      }
    });
  });
});