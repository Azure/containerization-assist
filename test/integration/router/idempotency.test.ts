/**
 * Integration tests for idempotency behavior
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import pino from 'pino';
import { createToolRouter, type ToolRouter } from '../../../src/mcp/tool-router';
import type { Step } from '../../../src/mcp/tool-graph';
import {
  createMockToolsMap,
  resetExecutionLog,
  executionLog,
} from './fixtures/mock-tools';
import { MockSessionManager } from './fixtures/mock-session';
import { createMockContext } from './fixtures/mock-context';

describe('Idempotency Behavior', () => {
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

  describe('preventing redundant execution', () => {
    it('should not re-run tool when effects are already satisfied', async () => {
      // First execution
      const firstRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './src' },
      });

      expect(firstRun.result.ok).toBe(true);
      expect(firstRun.executedTools).toEqual(['analyze_repo']);

      // Second execution with same session
      resetExecutionLog();
      const secondRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './src' },
        sessionId: firstRun.sessionState.sessionId,
      });

      expect(secondRun.result.ok).toBe(true);
      expect(secondRun.executedTools).toEqual([]);
      expect(executionLog.length).toBe(0);

      // Result should indicate skipping
      if (secondRun.result.ok) {
        expect(secondRun.result.value).toEqual({
          skipped: true,
          reason: 'Effects already satisfied',
        });
      }
    });

    it('should skip multiple tools when their effects are satisfied', async () => {
      // Complete a full deployment chain
      const initialRun = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
      });

      expect(initialRun.result.ok).toBe(true);
      const initialToolCount = executionLog.length;
      expect(initialToolCount).toBeGreaterThan(5);

      // Try to run various tools that should be skipped
      const toolsToTest = [
        'analyze_repo',
        'build_image',
        'prepare_cluster',
        'generate_k8s_manifests',
        'deploy',
      ];

      for (const tool of toolsToTest) {
        resetExecutionLog();
        const result = await router.route({
          context: mockContext,
          toolName: tool,
          params: { manifestPath: './k8s', imageName: 'test' },
          sessionId: initialRun.sessionState.sessionId,
        });

        expect(result.result.ok).toBe(true);
        expect(result.executedTools).toEqual([]);
        expect(executionLog.length).toBe(0);

        if (result.result.ok) {
          expect(result.result.value).toEqual({
            skipped: true,
            reason: 'Effects already satisfied',
          });
        }
      }
    });
  });

  describe('session state persistence', () => {
    it('should maintain completed_steps across multiple operations', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Run first tool
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      let currentSessionResult = await sessionManager.get(session.sessionId);
      expect(currentSessionResult.ok).toBe(true);
      if (!currentSessionResult.ok) return;
      expect(currentSessionResult.value?.completed_steps).toContain('analyzed_repo');

      // Run second tool
      await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {},
        sessionId: session.sessionId,
      });

      currentSessionResult = await sessionManager.get(session.sessionId);
      expect(currentSessionResult.ok).toBe(true);
      if (!currentSessionResult.ok) return;
      expect(currentSessionResult.value?.completed_steps).toContain('analyzed_repo');
      expect(currentSessionResult.value?.completed_steps).toContain('k8s_prepared');

      // Run third tool that depends on first
      resetExecutionLog();
      await router.route({
        context: mockContext,
        toolName: 'resolve_base_images',
        params: {},
        sessionId: session.sessionId,
      });

      // Should not re-run analyze-repo
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');
      expect(toolOrder).toEqual(['resolve_base_images']);

      // All steps should be preserved
      currentSessionResult = await sessionManager.get(session.sessionId);
      expect(currentSessionResult.ok).toBe(true);
      if (!currentSessionResult.ok) return;
      const steps = currentSessionResult.value?.completed_steps as Step[];
      expect(steps).toContain('analyzed_repo');
      expect(steps).toContain('k8s_prepared');
      expect(steps).toContain('resolved_base_images');
    });

    it('should accumulate results in session', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Run multiple tools
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test-app' },
        sessionId: session.sessionId,
      });

      const finalSessionResult = await sessionManager.get(session.sessionId);
      expect(finalSessionResult.ok).toBe(true);
      if (!finalSessionResult.ok) return;
      const finalSession = finalSessionResult.value;
      expect(finalSession?.results).toBeDefined();
      expect(finalSession?.results?.['analyze_repo']).toBeDefined();
      expect(finalSession?.results?.['build_image']).toBeDefined();
    });
  });

  describe('effect tracking accuracy', () => {
    it('should correctly track single tool effects', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;
      await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: { context: 'test' },
        sessionId: session.sessionId,
      });

      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      const steps = updatedSession?.completed_steps as Step[];
      expect(steps).toEqual(['k8s_prepared']);
    });

    it('should track multiple effects from single tool', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      await router.route({
        context: mockContext,
        toolName: 'generate_dockerfile',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      const steps = updatedSession?.completed_steps as Step[];
      expect(steps).toContain('dockerfile_generated');
      // Also includes effects from prerequisites
      expect(steps).toContain('analyzed_repo');
      expect(steps).toContain('resolved_base_images');
    });

    it('should not duplicate effects when tools share them', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Run analyze-repo directly
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      // Run another tool that would also provide analyzed_repo
      await router.route({
        context: mockContext,
        toolName: 'resolve_base_images',
        params: {},
        sessionId: session.sessionId,
      });

      const updatedResult = await sessionManager.get(session.sessionId);
      expect(updatedResult.ok).toBe(true);
      if (!updatedResult.ok) return;
      const updatedSession = updatedResult.value;
      const steps = updatedSession?.completed_steps as Step[];

      // Should have exactly one instance of analyzed_repo
      const analyzeCount = steps.filter(s => s === 'analyzed_repo').length;
      expect(analyzeCount).toBe(1);
    });
  });

  describe('partial completion scenarios', () => {
    it('should handle tool with some but not all effects satisfied', async () => {
      // This test depends on the actual tool graph structure
      // For tools with multiple effects, we test partial satisfaction

      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo'] as Step[],
      });

      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // generate-dockerfile provides dockerfile_generated
      // but requires both analyzed_repo and resolved_base_images
      const result = await router.route({
        context: mockContext,
        toolName: 'generate_dockerfile',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should only run missing prerequisites
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');
      expect(toolOrder).toContain('resolve_base_images');
      expect(toolOrder).toContain('generate_dockerfile');
    });
  });

  describe('idempotency with different parameters', () => {
    it('should still skip execution even with different parameters', async () => {
      // First run with one set of parameters
      const firstRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './src' },
      });

      expect(firstRun.result.ok).toBe(true);

      // Second run with different parameters but same session
      resetExecutionLog();
      const secondRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './different-path' },
        sessionId: firstRun.sessionState.sessionId,
      });

      // Should still skip because effects are satisfied
      expect(secondRun.result.ok).toBe(true);
      expect(secondRun.executedTools).toEqual([]);
      expect(executionLog.length).toBe(0);
    });
  });

  describe('idempotency boundaries', () => {
    it('should maintain separate idempotency per session', async () => {
      // First session
      const session1 = await sessionManager.create();
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session1.sessionId,
      });

      // Second session
      const session2 = await sessionManager.create();
      resetExecutionLog();
      const result = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session2.sessionId,
      });

      // Should execute in new session despite being run in session1
      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['analyze_repo']);
      expect(executionLog.length).toBe(1);
    });

    it('should create new session when none specified', async () => {
      // First run without session
      const firstRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
      });

      expect(firstRun.result.ok).toBe(true);
      expect(firstRun.sessionState.sessionId).toBeDefined();

      // Second run without session (should create new session)
      resetExecutionLog();
      const secondRun = await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
      });

      expect(secondRun.result.ok).toBe(true);
      expect(secondRun.executedTools).toEqual(['analyze_repo']);
      expect(secondRun.sessionState.sessionId).not.toBe(firstRun.sessionState.sessionId);
    });
  });

  describe('complex idempotency scenarios', () => {
    it('should handle interleaved tool execution', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Run tool A
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      // Run unrelated tool B
      await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {},
        sessionId: session.sessionId,
      });

      // Run tool C that depends on A
      resetExecutionLog();
      await router.route({
        context: mockContext,
        toolName: 'resolve_base_images',
        params: {},
        sessionId: session.sessionId,
      });

      // Should not re-run A
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');
      expect(toolOrder).toEqual(['resolve_base_images']);

      // Try to re-run B
      resetExecutionLog();
      const rerunB = await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {},
        sessionId: session.sessionId,
      });

      expect(rerunB.executedTools).toEqual([]);
      if (rerunB.result.ok) {
        expect(rerunB.result.value).toEqual({
          skipped: true,
          reason: 'Effects already satisfied',
        });
      }
    });

    it('should handle failed prerequisite chains gracefully', async () => {
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo', 'dockerfile_generated'] as Step[],
      });

      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Try to build (should skip prerequisites)
      const result = await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should only run build_image
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toEqual(['build_image']);

      // Now try to re-run build_image
      resetExecutionLog();
      const rerun = await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
        sessionId: session.sessionId,
      });

      expect(rerun.executedTools).toEqual([]);
      expect(executionLog.length).toBe(0);
    });
  });
});