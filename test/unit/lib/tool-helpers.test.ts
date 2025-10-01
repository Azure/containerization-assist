/**
 * Unit tests for tool-helpers module
 * Tests the storeToolResults and updateSessionResults functions
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import { storeToolResults, updateSessionResults } from '@/lib/tool-helpers';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import type { SessionManager } from '@/session/core';
import type { Result, WorkflowState } from '@/types';

describe('tool-helpers', () => {
  describe('storeToolResults', () => {
    let mockSessionManager: SessionManager;
    let mockContext: ToolContext;
    const logger = createLogger({ name: 'test-tool-helpers' });

    beforeEach(() => {
      // Reset mock session manager before each test
      mockSessionManager = {
        get: async () => ({
          ok: true,
          value: {
            sessionId: 'test-session',
            metadata: { results: {} },
            completed_steps: [],
            errors: {},
            createdAt: new Date(),
            updatedAt: new Date(),
          },
        }),
        update: async () => ({ ok: true, value: {} as WorkflowState }),
        create: async () => ({ ok: true, value: {} as WorkflowState }),
        delete: async () => ({ ok: true, value: undefined }),
        list: async () => ({ ok: true, value: [] }),
        cleanup: async () => ({ ok: true, value: undefined }),
        close: () => {},
      } as unknown as SessionManager;

      mockContext = {
        logger,
        sessionManager: mockSessionManager,
      } as ToolContext;
    });

    it('should successfully store tool results when session exists', async () => {
      const sessionId = 'test-session';
      const toolName = 'test-tool';
      const results = { foo: 'bar', count: 42 };
      const metadata = { timestamp: Date.now() };

      const result = await storeToolResults(mockContext, sessionId, toolName, results, metadata);

      expect(result.ok).toBe(true);
      expect(result.value).toBeUndefined();
    });

    it('should return success when sessionId is undefined', async () => {
      const result = await storeToolResults(mockContext, undefined, 'test-tool', { foo: 'bar' });

      expect(result.ok).toBe(true);
      expect(result.value).toBeUndefined();
    });

    it('should return success when sessionManager is not available', async () => {
      const contextWithoutSessionManager = {
        logger,
        sessionManager: undefined,
      } as ToolContext;

      const result = await storeToolResults(
        contextWithoutSessionManager,
        'test-session',
        'test-tool',
        { foo: 'bar' },
      );

      expect(result.ok).toBe(true);
      expect(result.value).toBeUndefined();
    });

    it('should return failure when session lookup fails', async () => {
      mockSessionManager.get = async () => ({
        ok: false,
        error: 'Database connection failed',
      });

      const result = await storeToolResults(mockContext, 'test-session', 'test-tool', {
        foo: 'bar',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Session lookup failed');
      expect(result.error).toContain('Database connection failed');
    });

    it('should return failure when session does not exist', async () => {
      mockSessionManager.get = async () => ({
        ok: true,
        value: null,
      });

      const result = await storeToolResults(mockContext, 'missing-session', 'test-tool', {
        foo: 'bar',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Session missing-session does not exist');
      expect(result.error).toContain('Sessions must be created before tool execution');
    });

    it('should return failure when session update fails', async () => {
      mockSessionManager.update = async () => ({
        ok: false,
        error: 'Write conflict: session was modified by another process',
      });

      const result = await storeToolResults(mockContext, 'test-session', 'test-tool', {
        foo: 'bar',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Session update failed');
      expect(result.error).toContain('Write conflict');
    });

    it('should return failure when session manager throws exception', async () => {
      mockSessionManager.update = async () => {
        throw new Error('Network timeout after 30 seconds');
      };

      const result = await storeToolResults(mockContext, 'test-session', 'test-tool', {
        foo: 'bar',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toContain('Failed to store results');
      expect(result.error).toContain('Network timeout');
    });

    it('should merge results with existing session results', async () => {
      const existingResults = {
        'previous-tool': { data: 'existing' },
      };

      mockSessionManager.get = async () => ({
        ok: true,
        value: {
          sessionId: 'test-session',
          metadata: { results: existingResults },
          completed_steps: [],
          errors: {},
          createdAt: new Date(),
          updatedAt: new Date(),
        },
      });

      let capturedUpdatePayload: any = null;
      mockSessionManager.update = async (_sessionId, payload) => {
        capturedUpdatePayload = payload;
        return { ok: true, value: {} as WorkflowState };
      };

      const newResults = { newData: 'test' };
      await storeToolResults(mockContext, 'test-session', 'current-tool', newResults);

      expect(capturedUpdatePayload.metadata.results).toEqual({
        'previous-tool': { data: 'existing' },
        'current-tool': { newData: 'test' },
      });
    });

    it('should include metadata in update payload when provided', async () => {
      let capturedUpdatePayload: any = null;
      mockSessionManager.update = async (_sessionId, payload) => {
        capturedUpdatePayload = payload;
        return { ok: true, value: {} as WorkflowState };
      };

      const metadata = { customField: 'value', timestamp: 12345 };
      await storeToolResults(mockContext, 'test-session', 'test-tool', { foo: 'bar' }, metadata);

      expect(capturedUpdatePayload.metadata.customField).toBe('value');
      expect(capturedUpdatePayload.metadata.timestamp).toBe(12345);
      expect(capturedUpdatePayload.metadata.results).toEqual({
        'test-tool': { foo: 'bar' },
      });
    });

    it('should handle empty results object', async () => {
      const result = await storeToolResults(mockContext, 'test-session', 'test-tool', {});

      expect(result.ok).toBe(true);
    });

    it('should handle non-Error exceptions', async () => {
      mockSessionManager.update = async () => {
        throw 'String error message';
      };

      const result = await storeToolResults(mockContext, 'test-session', 'test-tool', {
        foo: 'bar',
      });

      expect(result.ok).toBe(false);
      expect(result.error).toContain('String error message');
    });

    it('should preserve existing metadata when adding new results', async () => {
      const existingMetadata = {
        appName: 'my-app',
        analyzedPath: '/path/to/app',
        results: {
          'analyze-repo': { language: 'typescript' },
        },
      };

      mockSessionManager.get = async () => ({
        ok: true,
        value: {
          sessionId: 'test-session',
          metadata: existingMetadata,
          completed_steps: ['analyze-repo'],
          errors: {},
          createdAt: new Date(),
          updatedAt: new Date(),
        },
      });

      let capturedUpdatePayload: any = null;
      mockSessionManager.update = async (_sessionId, payload) => {
        capturedUpdatePayload = payload;
        return { ok: true, value: {} as WorkflowState };
      };

      await storeToolResults(mockContext, 'test-session', 'build-image', { imageId: 'sha256:abc' });

      // Should merge new results without losing existing ones
      expect(capturedUpdatePayload.metadata.results).toEqual({
        'analyze-repo': { language: 'typescript' },
        'build-image': { imageId: 'sha256:abc' },
      });

      // Should preserve other metadata fields
      expect(capturedUpdatePayload.metadata.appName).toBe('my-app');
      expect(capturedUpdatePayload.metadata.analyzedPath).toBe('/path/to/app');
    });
  });

  describe('updateSessionResults (canonical helper)', () => {
    it('should write results to canonical location metadata.results', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: {},
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      const results = { language: 'TypeScript', framework: 'Express' };
      updateSessionResults(session, 'analyze-repo', results);

      expect(session.metadata?.results).toBeDefined();
      const storedResults = session.metadata.results as Record<string, unknown>;
      expect(storedResults['analyze-repo']).toEqual(results);
    });

    it('should initialize metadata.results if not present', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      updateSessionResults(session, 'test-tool', { data: 'value' });

      expect(session.metadata).toBeDefined();
      expect(session.metadata?.results).toBeDefined();
      expect(typeof session.metadata?.results).toBe('object');
    });

    it('should update timestamp when storing results', () => {
      const oldDate = new Date('2023-01-01');
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: { results: {} },
        createdAt: oldDate,
        updatedAt: oldDate,
      };

      updateSessionResults(session, 'test-tool', { foo: 'bar' });

      expect(session.updatedAt.getTime()).toBeGreaterThan(oldDate.getTime());
    });

    it('should preserve existing results when adding new ones', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: {
          results: {
            'tool-a': { data: 'a' },
            'tool-b': { data: 'b' },
          },
        },
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      updateSessionResults(session, 'tool-c', { data: 'c' });

      const results = session.metadata.results as Record<string, unknown>;
      expect(results['tool-a']).toEqual({ data: 'a' });
      expect(results['tool-b']).toEqual({ data: 'b' });
      expect(results['tool-c']).toEqual({ data: 'c' });
    });

    it('should throw error when session is null', () => {
      const session = null as unknown as WorkflowState;

      expect(() => {
        updateSessionResults(session, 'test-tool', { data: 'value' });
      }).toThrow(/Cannot update session results: session is null/);
    });

    it('should throw error when session is undefined', () => {
      const session = undefined as unknown as WorkflowState;

      expect(() => {
        updateSessionResults(session, 'test-tool', { data: 'value' });
      }).toThrow(/Cannot update session results: session is undefined/);
    });

    it('should throw error when sessionId is missing', () => {
      const session = {
        sessionId: '',
        metadata: {},
        createdAt: new Date(),
        updatedAt: new Date(),
      } as WorkflowState;

      expect(() => {
        updateSessionResults(session, 'test-tool', { data: 'value' });
      }).toThrow(/Cannot update session results: session\.sessionId is missing/);
    });

    it('should throw error when toolName is empty', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: {},
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      expect(() => {
        updateSessionResults(session, '', { data: 'value' });
      }).toThrow(/Cannot update session results: toolName is invalid/);
    });

    it('should throw error when toolName is not a string', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: {},
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      expect(() => {
        updateSessionResults(session, null as unknown as string, { data: 'value' });
      }).toThrow(/Cannot update session results: toolName is invalid/);
    });

    it('should handle null/undefined results gracefully', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: { results: {} },
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      updateSessionResults(session, 'test-tool', null);
      const results = session.metadata.results as Record<string, unknown>;
      expect(results['test-tool']).toBeNull();

      updateSessionResults(session, 'test-tool-2', undefined);
      expect(results['test-tool-2']).toBeUndefined();
    });

    it('should overwrite existing tool results', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: {
          results: {
            'test-tool': { version: 1, data: 'old' },
          },
        },
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      updateSessionResults(session, 'test-tool', { version: 2, data: 'new' });

      const results = session.metadata.results as Record<string, unknown>;
      expect(results['test-tool']).toEqual({ version: 2, data: 'new' });
    });

    it('should handle complex nested result objects', () => {
      const session: WorkflowState = {
        sessionId: 'test-session',
        metadata: { results: {} },
        createdAt: new Date(),
        updatedAt: new Date(),
      };

      const complexResult = {
        analysis: {
          language: 'Python',
          dependencies: ['fastapi', 'uvicorn'],
          config: {
            port: 8000,
            workers: 4,
          },
        },
        metadata: {
          timestamp: Date.now(),
          version: '1.0.0',
        },
      };

      updateSessionResults(session, 'analyze-repo', complexResult);

      const results = session.metadata.results as Record<string, unknown>;
      expect(results['analyze-repo']).toEqual(complexResult);
    });
  });
});
