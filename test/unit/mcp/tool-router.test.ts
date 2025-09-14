/**
 * Tool Router Integration Tests
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';

// IMPORTANT: Unmock the session module for these tests - we need a real session manager
jest.unmock('@lib/session');
jest.unmock('../../../src/lib/session');

import { ToolRouter } from '../../../src/mcp/tool-router';
import { createSessionManager } from '../../../src/lib/session';
import { createLogger } from '../../../src/lib/logger';
import { Success, Failure } from '../../../src/types';
import type { ToolContext } from '../../../src/mcp/context';
import { createHostAIAssistant } from '../../../src/mcp/ai/host-ai-assist';
import { z } from 'zod';

describe('ToolRouter', () => {
  let router: ToolRouter;
  let mockTools: Map<string, any>;
  let sessionManager: ReturnType<typeof createSessionManager>;
  let logger: any;

  beforeEach(() => {
    // Create test logger
    logger = createLogger({ name: 'test', level: 'error' });

    // Create session manager
    sessionManager = createSessionManager(logger, {
      ttl: 60,
      maxSessions: 10,
    });

    // Create mock tools
    mockTools = new Map();

    // Mock analyze-repo tool
    mockTools.set('analyze-repo', {
      name: 'analyze-repo',
      schema: z.object({
        path: z.string(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          framework: 'nodejs',
          language: 'typescript',
          path: params.path,
        });
      }),
    });

    // Mock resolve-base-images tool
    mockTools.set('resolve-base-images', {
      name: 'resolve-base-images',
      schema: z.object({
        framework: z.string(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          baseImage: 'node:18-alpine',
          framework: params.framework,
        });
      }),
    });

    // Mock generate-dockerfile tool
    mockTools.set('generate-dockerfile', {
      name: 'generate-dockerfile',
      schema: z.object({
        path: z.string(),
        framework: z.string().optional(),
        baseImage: z.string().optional(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          dockerfilePath: '/tmp/Dockerfile',
          content: 'FROM node:18-alpine',
        });
      }),
    });

    // Mock build-image tool
    mockTools.set('build-image', {
      name: 'build-image',
      schema: z.object({
        path: z.string(),
        imageId: z.string(),
        dockerfilePath: z.string().optional(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          imageId: params.imageId,
          digest: 'sha256:abc123',
        });
      }),
    });

    // Mock deploy tool
    mockTools.set('deploy', {
      name: 'deploy',
      schema: z.object({
        imageId: z.string(),
        namespace: z.string().optional(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          deployment: 'app-deployment',
          namespace: params.namespace || 'default',
        });
      }),
    });

    // Mock prepare-cluster tool
    mockTools.set('prepare-cluster', {
      name: 'prepare-cluster',
      schema: z.object({
        namespace: z.string().optional(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          namespace: params.namespace || 'default',
          ready: true,
        });
      }),
    });

    // Mock generate-k8s-manifests tool
    mockTools.set('generate-k8s-manifests', {
      name: 'generate-k8s-manifests',
      schema: z.object({
        imageId: z.string(),
        appName: z.string().optional(),
        sessionId: z.string().optional(),
      }),
      handler: jest.fn(async (params: any) => {
        return Success({
          manifests: ['deployment.yaml', 'service.yaml'],
          appName: params.appName || 'app',
        });
      }),
    });

    // Create router with mocks
    router = new ToolRouter({
      sessionManager,
      logger,
      tools: mockTools,
      aiAssistant: createHostAIAssistant(logger, true),
    });
  });

  afterEach(() => {
    if (sessionManager && typeof sessionManager.close === 'function') {
      sessionManager.close();
    }
  });

  describe('Out-of-order tool execution', () => {
    it('should automatically run preconditions when calling deploy first', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'deploy',
        params: {
          imageId: 'myapp:latest',
          namespace: 'production',
        },
        context,
      });

      // Check that router executed multiple tools
      expect(result.executedTools.length).toBeGreaterThan(1);

      // Verify the sequence included necessary preconditions
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('build-image');
      expect(result.executedTools).toContain('prepare-cluster');
      expect(result.executedTools).toContain('generate-k8s-manifests');
      expect(result.executedTools).toContain('deploy');

      // Verify final result is successful
      expect(result.result.ok).toBe(true);
      if (result.result.ok) {
        expect(result.result.value).toHaveProperty('deployment');
      }

      // Verify tools were called
      const analyzeHandler = mockTools.get('analyze-repo').handler;
      expect(analyzeHandler).toHaveBeenCalled();

      const buildHandler = mockTools.get('build-image').handler;
      expect(buildHandler).toHaveBeenCalled();
    });

    it('should auto-run analyze-repo before generate-dockerfile', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'generate-dockerfile',
        params: {
          path: './app',
        },
        context,
      });

      // Should run analyze-repo and resolve-base-images first
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('resolve-base-images');
      expect(result.executedTools).toContain('generate-dockerfile');

      expect(result.result.ok).toBe(true);
    });

    it('should skip execution if effects already satisfied', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      // Create the session first
      const sessionId = 'test-session';
      await sessionManager.create(sessionId);

      // First execution
      const firstResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      expect(firstResult.result.ok).toBe(true);
      expect(firstResult.executedTools).toContain('analyze-repo');

      // Check that session was updated with completed steps
      const sessionAfterFirst = await sessionManager.get(sessionId);
      expect(sessionAfterFirst).toBeDefined();
      expect(sessionAfterFirst?.completed_steps).toContain('analyzed_repo');

      // Second execution with same session
      const secondResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      // Should skip execution (idempotency)
      expect(secondResult.executedTools.length).toBe(0);
      expect(secondResult.result.ok).toBe(true);
      if (secondResult.result.ok) {
        expect(secondResult.result.value).toHaveProperty('skipped', true);
      }
    });
  });

  describe('Force flag functionality', () => {
    it('should re-run tool when force flag is set', async () => {
      const sessionId = 'force-test-session';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      // Create the session first
      await sessionManager.create(sessionId);

      // First execution
      const firstResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      expect(firstResult.result.ok).toBe(true);
      expect(firstResult.executedTools).toContain('analyze-repo');

      // Force re-execution
      const forcedResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.', force: true },
        sessionId,
        force: true,
        context,
      });

      // Should execute despite already completed
      expect(forcedResult.executedTools).toContain('analyze-repo');
      expect(forcedResult.result.ok).toBe(true);

      // Verify handler was called twice
      const handler = mockTools.get('analyze-repo').handler;
      expect(handler).toHaveBeenCalledTimes(2);
    });

    it('should bypass idempotency checks with force flag', async () => {
      const sessionId = 'force-bypass-session';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      // Create the session first
      await sessionManager.create(sessionId);

      // Set up initial state
      await router.route({
        toolName: 'build-image',
        params: {
          path: '.',
          imageId: 'app:v1',
        },
        sessionId,
        context,
      });

      // Clear mock calls
      jest.clearAllMocks();

      // Force re-run
      const result = await router.route({
        toolName: 'build-image',
        params: {
          path: '.',
          imageId: 'app:v2',
          force: true,
        },
        sessionId,
        force: true,
        context,
      });

      expect(result.executedTools).toContain('build-image');
      const handler = mockTools.get('build-image').handler;
      expect(handler).toHaveBeenCalledWith(
        expect.objectContaining({ imageId: 'app:v2' }),
        expect.anything(),
      );
    });
  });

  describe('Session state management', () => {
    it('should track completed steps across tool executions', async () => {
      const sessionId = 'state-tracking-session';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      // Create the session first
      await sessionManager.create(sessionId);

      // Execute first tool
      await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      // Check session state
      const session1 = await sessionManager.get(sessionId);
      expect(session1.completed_steps).toContain('analyzed_repo');

      // Execute dependent tool
      await router.route({
        toolName: 'resolve-base-images',
        params: { framework: 'nodejs' },
        sessionId,
        context,
      });

      // Check updated state
      const session2 = await sessionManager.get(sessionId);
      expect(session2.completed_steps).toContain('analyzed_repo');
      expect(session2.completed_steps).toContain('resolved_base_images');
    });

    it('should store tool results in session', async () => {
      const sessionId = 'results-storage-session';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      // Create the session first
      await sessionManager.create(sessionId);

      const result = await router.route({
        toolName: 'analyze-repo',
        params: { path: './src' },
        sessionId,
        context,
      });

      expect(result.result.ok).toBe(true);

      const session = await sessionManager.get(sessionId);
      expect(session.results).toBeDefined();
      expect(session.results['analyze-repo']).toBeDefined();
      expect(session.results['analyze-repo']).toHaveProperty('framework', 'nodejs');
    });
  });

  describe('Error handling', () => {
    it('should handle tool execution failures gracefully', async () => {
      // Create failing tool
      mockTools.set('failing-tool', {
        name: 'failing-tool',
        schema: z.object({ param: z.string() }),
        handler: jest.fn(async () => {
          return Failure('Tool execution failed');
        }),
      });

      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'failing-tool',
        params: { param: 'test' },
        context,
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool execution failed');
      }
    });

    it('should handle missing tool gracefully', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'non-existent-tool',
        params: {},
        context,
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool not found');
      }
    });

    it('should fail fast if precondition tool fails', async () => {
      // Make analyze-repo fail
      mockTools.get('analyze-repo').handler = jest.fn(async () => {
        return Failure('Analysis failed');
      });

      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'generate-dockerfile',
        params: { path: '.' },
        context,
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Failed to satisfy precondition');
      }

      // Should not have executed generate-dockerfile
      expect(result.executedTools).not.toContain('generate-dockerfile');
    });
  });

  describe('Tool execution order', () => {
    it('should execute tools in correct dependency order', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'deploy',
        params: {
          imageId: 'myapp:latest',
        },
        context,
      });

      expect(result.result.ok).toBe(true);

      // Verify execution order
      const analyzeIndex = result.executedTools.indexOf('analyze-repo');
      const buildIndex = result.executedTools.indexOf('build-image');
      const deployIndex = result.executedTools.indexOf('deploy');

      // analyze-repo should come before build-image
      expect(analyzeIndex).toBeLessThan(buildIndex);
      // build-image should come before deploy
      expect(buildIndex).toBeLessThan(deployIndex);
    });

    it('should handle parallel preconditions efficiently', async () => {
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      const result = await router.route({
        toolName: 'deploy',
        params: {
          imageId: 'app:v1',
          namespace: 'staging',
        },
        context,
      });

      expect(result.result.ok).toBe(true);

      // Both prepare-cluster and build-image are preconditions
      expect(result.executedTools).toContain('prepare-cluster');
      expect(result.executedTools).toContain('build-image');
      expect(result.executedTools).toContain('deploy');
    });
  });

  describe('Session update consolidation', () => {
    it('should handle successful session updates atomically', async () => {
      const sessionId = 'atomic-update-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      const result = await router.route({
        toolName: 'analyze-repo',
        params: { path: './test' },
        sessionId,
        context,
      });

      expect(result.result.ok).toBe(true);

      // Verify session was updated with all fields in one operation
      const session = await sessionManager.get(sessionId);
      expect(session).toBeDefined();
      expect(session?.updatedAt).toBeDefined();
      expect(session?.completed_steps).toContain('analyzed_repo');
      expect(session?.results?.['analyze-repo']).toBeDefined();
    });

    it('should recover gracefully when session update returns null', async () => {
      const sessionId = 'null-update-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      // Mock update to return null once
      const originalUpdate = sessionManager.update;
      let updateCallCount = 0;
      sessionManager.update = jest.fn(async (id, state) => {
        updateCallCount++;
        if (updateCallCount === 1) {
          // First call returns null to simulate failure
          return null;
        }
        return originalUpdate.call(sessionManager, id, state);
      });

      const result = await router.route({
        toolName: 'analyze-repo',
        params: { path: './test' },
        sessionId,
        context,
      });

      // Should still succeed by falling back to get()
      expect(result.result.ok).toBe(true);
      expect(sessionManager.update).toHaveBeenCalled();

      // Restore original update
      sessionManager.update = originalUpdate;
    });

    it('should maintain session consistency across multiple tool executions', async () => {
      const sessionId = 'consistency-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      // Execute multiple tools in sequence
      const analyzeResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      expect(analyzeResult.result.ok).toBe(true);

      // Force re-execution with different params
      const dockerfileResult = await router.route({
        toolName: 'generate-dockerfile',
        params: { path: './app' },
        sessionId,
        force: true,
        context,
      });

      expect(dockerfileResult.result.ok).toBe(true);

      // Verify session maintains all results
      const finalSession = await sessionManager.get(sessionId);
      expect(finalSession?.results?.['analyze-repo']).toBeDefined();
      expect(finalSession?.results?.['generate-dockerfile']).toBeDefined();
      expect(finalSession?.completed_steps).toContain('analyzed_repo');
      expect(finalSession?.completed_steps).toContain('dockerfile_generated');
    });

    it('should not create race conditions with concurrent updates', async () => {
      const sessionId = 'concurrent-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      // Create a single mock tool without dependencies for simpler testing
      mockTools.set('tool-a', {
        name: 'tool-a',
        schema: z.object({ data: z.string(), sessionId: z.string().optional() }),
        handler: jest.fn(async (params: any) => {
          return Success({ result: 'tool-a-result', data: params.data });
        }),
      });

      // Execute single tool first to verify basic functionality
      const singleResult = await router.route({
        toolName: 'tool-a',
        params: { data: 'test-data' },
        sessionId,
        context,
      });

      expect(singleResult.result.ok).toBe(true);

      // Check session was updated
      const sessionAfterSingle = await sessionManager.get(sessionId);
      expect(sessionAfterSingle).toBeDefined();
      expect(sessionAfterSingle?.results).toBeDefined();
      expect(sessionAfterSingle?.results?.['tool-a']).toBeDefined();

      // Now test with multiple tools
      const independentTools = ['tool-b', 'tool-c'];
      independentTools.forEach(toolName => {
        mockTools.set(toolName, {
          name: toolName,
          schema: z.object({ data: z.string(), sessionId: z.string().optional() }),
          handler: jest.fn(async (params: any) => {
            // Simulate some async work
            await new Promise(resolve => setTimeout(resolve, 10));
            return Success({ result: `${toolName}-result`, data: params.data });
          }),
        });
      });

      // Execute tools in parallel
      const promises = independentTools.map(toolName =>
        router.route({
          toolName,
          params: { data: `data-${toolName}` },
          sessionId,
          context,
        })
      );

      const results = await Promise.all(promises);

      // All should succeed
      results.forEach((result) => {
        expect(result.result.ok).toBe(true);
      });

      // Session should contain all results
      const finalSession = await sessionManager.get(sessionId);
      expect(finalSession).toBeDefined();
      expect(finalSession?.results).toBeDefined();

      // Check all tools are present
      ['tool-a', 'tool-b', 'tool-c'].forEach(toolName => {
        expect(finalSession?.results?.[toolName]).toBeDefined();
        expect(finalSession?.results?.[toolName]).toHaveProperty('result', `${toolName}-result`);
      });
    });

    it('should preserve session state on partial failure', async () => {
      const sessionId = 'partial-failure-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      // First tool succeeds
      const firstResult = await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      expect(firstResult.result.ok).toBe(true);

      // Make resolve-base-images fail
      const originalHandler = mockTools.get('resolve-base-images').handler;
      mockTools.get('resolve-base-images').handler = jest.fn(async () => {
        return Failure('Intentional failure for testing');
      });

      // Try to execute a tool that depends on resolve-base-images
      const secondResult = await router.route({
        toolName: 'generate-dockerfile',
        params: { path: '.' },
        sessionId,
        context,
      });

      expect(secondResult.result.ok).toBe(false);

      // Session should still contain the successful analyze-repo result
      const session = await sessionManager.get(sessionId);
      expect(session?.results?.['analyze-repo']).toBeDefined();
      expect(session?.completed_steps).toContain('analyzed_repo');

      // Restore original handler
      mockTools.get('resolve-base-images').handler = originalHandler;
    });

    it('should update timestamps consistently', async () => {
      const sessionId = 'timestamp-test';
      const context: ToolContext = {
        sessionManager,
        logger,
      };

      await sessionManager.create(sessionId);

      const initialSession = await sessionManager.get(sessionId);
      const initialTimestamp = initialSession?.updatedAt;

      // Wait a bit to ensure timestamp difference
      await new Promise(resolve => setTimeout(resolve, 10));

      await router.route({
        toolName: 'analyze-repo',
        params: { path: '.' },
        sessionId,
        context,
      });

      const updatedSession = await sessionManager.get(sessionId);
      const updatedTimestamp = updatedSession?.updatedAt;

      // Timestamp should be updated
      expect(updatedTimestamp).toBeDefined();
      expect(new Date(updatedTimestamp!).getTime()).toBeGreaterThan(
        new Date(initialTimestamp!).getTime()
      );
    });
  });
});