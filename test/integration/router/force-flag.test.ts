/**
 * Integration tests for force flag functionality
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import pino from 'pino';
import { createToolRouter, type IToolRouter } from '@mcp/tool-router';
import type { Step } from '@mcp/tool-graph';
import {
  createMockToolsMap,
  resetExecutionLog,
  executionLog,
} from './fixtures/mock-tools';
import { MockSessionManager } from './fixtures/mock-session';
import { createMockContext } from './fixtures/mock-context';

describe('Force Flag Functionality', () => {
  let router: IToolRouter;
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

  describe('bypassing idempotency checks', () => {
    it('should re-run tool when force=true even if effects are satisfied', async () => {
      // First run: analyze-repo
      const firstRun = await router.route({
        context: mockContext,
        toolName: 'analyze-repo',
        params: { path: './src' },
      });

      expect(firstRun.result.ok).toBe(true);
      expect(firstRun.executedTools).toEqual(['analyze-repo']);
      expect(executionLog.length).toBe(1);

      // Second run WITHOUT force: should skip
      resetExecutionLog();
      const secondRun = await router.route({
        context: mockContext,
        toolName: 'analyze-repo',
        params: { path: './src' },
        sessionId: firstRun.sessionState.sessionId,
      });

      expect(secondRun.result.ok).toBe(true);
      if (secondRun.result.ok) {
        expect(secondRun.result.value).toEqual({
          skipped: true,
          reason: 'Effects already satisfied',
        });
      }
      expect(secondRun.executedTools).toEqual([]);
      expect(executionLog.length).toBe(0);

      // Third run WITH force: should re-execute
      resetExecutionLog();
      const thirdRun = await router.route({
        context: mockContext,
        toolName: 'analyze-repo',
        params: { path: './src' },
        sessionId: firstRun.sessionState.sessionId,
        force: true,
      });

      expect(thirdRun.result.ok).toBe(true);
      expect(thirdRun.executedTools).toEqual(['analyze-repo']);
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].tool).toBe('analyze-repo');
    });
  });

  describe('bypassing precondition checks', () => {
    it('should skip prerequisite execution when force=true', async () => {
      // Try to run build-image with force flag
      // Should NOT run analyze-repo, generate-dockerfile first
      const result = await router.route({
        context: mockContext,
        toolName: 'build-image',
        params: {
          imageName: 'forced-build',
          dockerfilePath: './Dockerfile',
        },
        force: true,
      });

      if (!result.result.ok) {
        console.error('Force flag test failed:', result.result.error);
      }
      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['build-image']);
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].tool).toBe('build-image');

      // Session should still record the effect
      const completedSteps = result.sessionState.completed_steps as Step[];
      expect(completedSteps).toContain('built_image');
    });

    it('should allow out-of-order execution with force=true', async () => {
      // Run deploy directly with force
      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: {
          manifestPath: './k8s',
          namespace: 'forced',
        },
        force: true,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['deploy']);
      expect(executionLog.length).toBe(1);

      // Verify only deploy was run
      expect(executionLog[0].tool).toBe('deploy');
      expect(executionLog[0].params.namespace).toBe('forced');
    });
  });

  describe('force flag with session state', () => {
    it('should override existing session state when forced', async () => {
      // Create session with completed steps
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo', 'dockerfile_generated', 'built_image'] as Step[],
        results: {
          'build-image': {
            imageId: 'old-image',
            tag: 'v1.0',
          },
        },
      });

      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Force re-run build-image
      const result = await router.route({
        context: mockContext,
        toolName: 'build-image',
        params: {
          imageName: 'new-image',
          tag: 'v2.0',
        },
        sessionId: session.sessionId,
        force: true,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['build-image']);

      // Verify new results replaced old ones
      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      expect(updatedSession?.results?.['build-image']).toMatchObject({
        imageName: 'new-image',
        tag: 'v2.0',
      });
    });
  });

  describe('selective forcing', () => {
    it('should only force the specified tool, not its prerequisites', async () => {
      // First, complete the full chain normally
      const initialRun = await router.route({
        context: mockContext,
        toolName: 'push-image',
        params: { imageId: 'initial' },
      });

      expect(initialRun.result.ok).toBe(true);
      const initialCount = executionLog.length;
      expect(initialCount).toBeGreaterThan(1); // Multiple tools run

      // Now force re-run only push-image
      resetExecutionLog();
      const forcedRun = await router.route({
        context: mockContext,
        toolName: 'push-image',
        params: { imageId: 'forced', registry: 'newregistry.io' },
        sessionId: initialRun.sessionState.sessionId,
        force: true,
      });

      expect(forcedRun.result.ok).toBe(true);
      expect(forcedRun.executedTools).toEqual(['push-image']);
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].params.registry).toBe('newregistry.io');
    });
  });

  describe('force flag combinations', () => {
    it('should handle force=true with missing session gracefully', async () => {
      // Force flag without session should create new session and execute
      const result = await router.route({
        context: mockContext,
        toolName: 'scan-image',
        params: { imageId: 'test-image' },
        force: true,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['scan-image']);
      expect(result.sessionState.sessionId).toBeDefined();
    });

    it('should handle force=false explicitly', async () => {
      // Complete a tool first
      const firstRun = await router.route({
        context: mockContext,
        toolName: 'prepare-cluster',
        params: { context: 'test' },
      });

      expect(firstRun.result.ok).toBe(true);

      // Explicitly set force=false (should still check idempotency)
      resetExecutionLog();
      const secondRun = await router.route({
        context: mockContext,
        toolName: 'prepare-cluster',
        params: { context: 'test' },
        sessionId: firstRun.sessionState.sessionId,
        force: false,
      });

      expect(secondRun.result.ok).toBe(true);
      if (secondRun.result.ok) {
        expect(secondRun.result.value).toEqual({
          skipped: true,
          reason: 'Effects already satisfied',
        });
      }
      expect(executionLog.length).toBe(0);
    });
  });

  describe('force flag with failed prerequisites', () => {
    it('should execute tool despite missing prerequisites when forced', async () => {
      // Create session with no completed steps
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Try to run a tool with many prerequisites using force
      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: {
          manifestPath: './forced-deploy',
          namespace: 'production',
        },
        sessionId: session.sessionId,
        force: true,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['deploy']);

      // Verify session recorded the effect despite missing prerequisites
      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      const completedSteps = updatedSession?.completed_steps as Step[];
      expect(completedSteps).toContain('deployed');

      // But NOT the prerequisite steps
      expect(completedSteps).not.toContain('analyzed_repo');
      expect(completedSteps).not.toContain('built_image');
      expect(completedSteps).not.toContain('k8s_prepared');
    });
  });

  describe('force flag effect tracking', () => {
    it('should still update completed_steps when forced', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Run with force flag
      const result = await router.route({
        context: mockContext,
        toolName: 'generate-dockerfile',
        params: { path: './' },
        sessionId: session.sessionId,
        force: true,
      });

      expect(result.result.ok).toBe(true);

      // Check that effects were recorded
      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      const completedSteps = updatedSession?.completed_steps as Step[];
      expect(completedSteps).toContain('dockerfile_generated');
    });

    it('should allow multiple forced executions in sequence', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Force multiple tools in sequence
      const tools = ['analyze-repo', 'build-image', 'scan-image'];

      for (const tool of tools) {
        resetExecutionLog();
        const result = await router.route({
        context: mockContext,
          toolName: tool,
          params: { imageId: 'test', path: './' },
          sessionId: session.sessionId,
          force: true,
        });

        expect(result.result.ok).toBe(true);
        expect(result.executedTools).toEqual([tool]);
        expect(executionLog.length).toBe(1);
      }

      // Verify all effects were recorded
      const finalResult = await sessionManager.get(session.sessionId);
      expect(finalResult.ok).toBe(true);
      if (!finalResult.ok) return;
      const finalSession = finalResult.value;
      const completedSteps = finalSession?.completed_steps as Step[];
      expect(completedSteps).toContain('analyzed_repo');
      expect(completedSteps).toContain('built_image');
      expect(completedSteps).toContain('scanned_image');
    });
  });
});