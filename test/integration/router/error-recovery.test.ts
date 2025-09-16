/**
 * Integration tests for error recovery and rerouting
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import pino from 'pino';
import { createToolRouter, type ToolRouter } from '@mcp/tool-router';
import type { Step } from '@mcp/tool-graph';
import { Failure, Success, type Result } from '@types';
import { MockSessionManager } from './fixtures/mock-session';
import { createMockContext } from './fixtures/mock-context';
import {
  createMockToolsMap,
  resetExecutionLog,
  executionLog,
} from './fixtures/mock-tools';

describe('Error Recovery and Rerouting', () => {
  let router: ToolRouter;
  let sessionManager: MockSessionManager;
  let logger: pino.Logger;
  let mockContext: any;

  beforeEach(() => {
    resetExecutionLog();
    sessionManager = new MockSessionManager();
    logger = pino({ level: 'silent' });
    mockContext = createMockContext();

    router = createToolRouter({
      sessionManager,
      logger,
      tools: createMockToolsMap(),
    });
  });

  afterEach(async () => {
    await sessionManager.clear();
  });

  describe('handling tool failures', () => {
    it('should stop execution when a prerequisite fails', async () => {
      const tools = createMockToolsMap();

      // Make analyze-repo fail
      tools.set('analyze_repo', {
        name: 'analyze_repo',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'analyze_repo',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Failure('Repository analysis failed', { recoverable: false });
        },
      });

      const failingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await failingRouter.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Repository analysis failed');
      }

      // Should have stopped after analyze-repo failed
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toEqual(['analyze_repo']);
      expect(toolOrder).not.toContain('build_image');
    });

    it('should return detailed error when middle of chain fails', async () => {
      const tools = createMockToolsMap();

      // Make generate-dockerfile fail
      tools.set('generate_dockerfile', {
        name: 'generate_dockerfile',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'generate_dockerfile',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Failure('Dockerfile generation failed: invalid base image');
        },
      });

      const failingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await failingRouter.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('generate_dockerfile');
        expect(result.result.error).toContain('Dockerfile generation failed');
      }

      // Should have run up to the failure point
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toContain('analyze_repo');
      expect(toolOrder).toContain('resolve_base_images');
      expect(toolOrder).toContain('generate_dockerfile');
      expect(toolOrder).not.toContain('build_image');
    });
  });

  describe('recoverable errors', () => {
    it('should handle recoverable errors gracefully', async () => {
      const tools = createMockToolsMap();

      let totalCalls = 0;
      tools.set('build_image', {
        name: 'build_image',
        handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
          totalCalls++;
          executionLog.push({
            tool: 'build_image',
            executed: true,
            params,
            timestamp: new Date(),
          });

          // Fail first time, succeed second time
          if (totalCalls === 1) {
            return Failure('Build failed: timeout', { recoverable: true });
          }
          return Success({
            imageId: 'recovered-build',
            imageName: params.imageName || 'test',
          });
        },
      });

      const recoveringRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await recoveringRouter.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
      });

      // First attempt should fail
      expect(result.result.ok).toBe(false);

      // Try again with same session (totalCalls will be 2)
      resetExecutionLog();
      const retryResult = await recoveringRouter.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
        sessionId: result.sessionState.sessionId,
        force: true, // Force retry
      });

      if (!retryResult.result.ok) {
        console.error('Retry failed:', retryResult.result.error);
        console.error('Total calls was:', totalCalls);
      }
      expect(retryResult.result.ok).toBe(true);
      if (retryResult.result.ok) {
        expect(retryResult.result.value).toMatchObject({
          imageId: 'recovered-build',
        });
      }
    });
  });

  describe('error propagation', () => {
    it('should include context in error messages', async () => {
      const tools = createMockToolsMap();

      tools.set('push_image', {
        name: 'push_image',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'push_image',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Failure('Authentication failed: invalid credentials');
        },
      });

      const failingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await failingRouter.route({
        context: mockContext,
        toolName: 'push_image',
        params: { imageId: 'test', registry: 'private.registry.io' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Authentication failed');
      }

      // Verify prerequisites ran successfully before failure
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toContain('analyze_repo');
      expect(toolOrder).toContain('build_image');
      expect(toolOrder[toolOrder.length - 1]).toBe('push_image');
    });
  });

  describe('partial completion on error', () => {
    it('should preserve completed steps even when later steps fail', async () => {
      const tools = createMockToolsMap();
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Make deploy fail
      tools.set('deploy', {
        name: 'deploy',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'deploy',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Failure('Deployment failed: cluster unavailable');
        },
      });

      const failingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await failingRouter.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(false);

      // Check that completed steps were preserved
      const getResult = await sessionManager.get(session.sessionId);
      expect(getResult.ok).toBe(true);
      if (!getResult.ok) return;
      const updatedSession = getResult.value;
      const completedSteps = updatedSession?.completed_steps as Step[];

      // Prerequisites should be marked complete
      expect(completedSteps).toContain('analyzed_repo');
      expect(completedSteps).toContain('built_image');
      expect(completedSteps).toContain('k8s_prepared');
      expect(completedSteps).toContain('manifests_generated');

      // But not the failed step
      expect(completedSteps).not.toContain('deployed');
    });

    it('should allow resuming after partial failure', async () => {
      const tools = createMockToolsMap();
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      let deployAttempts = 0;
      tools.set('deploy', {
        name: 'deploy',
        handler: async (params: Record<string, unknown>): Promise<Result<unknown>> => {
          deployAttempts++;
          executionLog.push({
            tool: 'deploy',
            executed: true,
            params,
            timestamp: new Date(),
          });

          if (deployAttempts === 1) {
            return Failure('Deployment failed: temporary network issue');
          }
          return Success({ deployed: true });
        },
      });

      const router = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      // First attempt - will fail at deploy
      const firstAttempt = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      expect(firstAttempt.result.ok).toBe(false);


      // Second attempt - should skip completed prerequisites
      resetExecutionLog();
      const secondAttempt = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
        force: true, // Force retry of deploy only
      });

      expect(secondAttempt.result.ok).toBe(true);

      // Should only run deploy, not prerequisites
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].tool).toBe('deploy');
    });
  });

  describe('missing tool handling', () => {
    it('should handle missing tool gracefully', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'non-existent-tool',
        params: {},
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool not found');
        expect(result.result.error).toContain('non-existent-tool');
      }
      expect(result.executedTools).toEqual([]);
    });

    it('should handle missing prerequisite tool', async () => {
      const tools = createMockToolsMap();

      // Remove a critical prerequisite tool
      tools.delete('analyze_repo');

      const incompleteRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await incompleteRouter.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool not found');
        expect(result.result.error).toContain('analyze_repo');
      }
    });
  });

  describe('exception handling', () => {
    it('should catch and wrap thrown exceptions', async () => {
      const tools = createMockToolsMap();

      tools.set('scan', {
        name: 'scan',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'scan',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          throw new Error('Unexpected scanner crash');
        },
      });

      const crashingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await crashingRouter.route({
        context: mockContext,
        toolName: 'scan_image',
        params: { imageId: 'test' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool execution failed');
        expect(result.result.error).toContain('Unexpected scanner crash');
      }
    });

    it('should handle async rejection', async () => {
      const tools = createMockToolsMap();

      tools.set('prepare_cluster', {
        name: 'prepare_cluster',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'prepare_cluster',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Promise.reject(new Error('Async operation failed'));
        },
      });

      const rejectingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await rejectingRouter.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {},
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Tool execution failed');
      }
    });
  });

  describe('session error handling', () => {
    it('should handle session creation failure', async () => {
      const failingSessionManager = {
        ...sessionManager,
        create: async () => Failure('Session creation failed'),
      };

      const routerWithBadSession = createToolRouter({
        sessionManager: failingSessionManager as any,
        logger,
        tools: createMockToolsMap(),
      });

      const result = await routerWithBadSession.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('Failed to get or create session');
      }
    });

    it('should handle session update failure gracefully', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Override update to fail
      sessionManager.update = async () => Failure('Update failed');

      const result = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      // Tool should still execute successfully
      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
    });
  });

  describe('complex error scenarios', () => {
    it('should handle errors in deeply nested prerequisite chains', async () => {
      const tools = createMockToolsMap();

      // Make a mid-chain tool fail
      tools.set('resolve_base_images', {
        name: 'resolve_base_images',
        handler: async (): Promise<Result<unknown>> => {
          executionLog.push({
            tool: 'resolve_base_images',
            executed: true,
            params: {},
            timestamp: new Date(),
          });
          return Failure('Cannot determine base image: unsupported framework');
        },
      });

      const failingRouter = createToolRouter({
        sessionManager,
        logger,
        tools,
      });

      const result = await failingRouter.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
      });

      expect(result.result.ok).toBe(false);
      if (!result.result.ok) {
        expect(result.result.error).toContain('resolve_base_images');
        expect(result.result.error).toContain('unsupported framework');
      }

      // Should have executed up to the failure point
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toContain('analyze_repo');
      expect(toolOrder).toContain('resolve_base_images');

      // Should not have continued past the failure
      expect(toolOrder).not.toContain('generate_dockerfile');
      expect(toolOrder).not.toContain('build_image');
      expect(toolOrder).not.toContain('deploy');
    });
  });
});