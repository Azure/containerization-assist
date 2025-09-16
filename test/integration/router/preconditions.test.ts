/**
 * Integration tests for automatic precondition satisfaction
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

describe('Automatic Precondition Satisfaction', () => {
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

  describe('single missing precondition', () => {
    it('should automatically satisfy missing analyzed_repo step', async () => {
      // Create session with no completed steps
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // Try to generate dockerfile (requires analyzed_repo)
      const result = await router.route({
        context: mockContext,
        toolName: 'resolve_base_images',
        params: {},
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
      expect(result.executedTools).toContain('resolve_base_images');

      // Verify order
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder[0]).toBe('analyze_repo');
      expect(toolOrder[1]).toBe('resolve_base_images');
    });

    it('should automatically satisfy missing built_image step', async () => {
      // Try to scan image without building first
      const result = await router.route({
        context: mockContext,
        toolName: 'scan_image',
        params: { imageId: 'auto' },
      });

      expect(result.result.ok).toBe(true);

      // Should run full build chain
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toContain('analyze_repo');
      expect(toolOrder).toContain('generate_dockerfile');
      expect(toolOrder).toContain('build_image');
      expect(toolOrder[toolOrder.length - 1]).toBe('scan_image');
    });
  });

  describe('multiple missing preconditions', () => {
    it('should satisfy all missing preconditions in correct order', async () => {
      // deploy requires: built_image, k8s_prepared, manifests_generated
      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
      });

      expect(result.result.ok).toBe(true);

      // Check all prerequisites were satisfied
      const executedTools = new Set(executionLog.map(e => e.tool));
      expect(executedTools.has('analyze_repo')).toBe(true);
      expect(executedTools.has('build_image')).toBe(true);
      expect(executedTools.has('prepare_cluster')).toBe(true);
      expect(executedTools.has('generate_k8s_manifests')).toBe(true);

      // Verify final tool was executed
      const lastTool = executionLog[executionLog.length - 1].tool;
      expect(lastTool).toBe('deploy');
    });

    it('should handle parallel preconditions efficiently', async () => {
      // Create session with analyzed_repo already complete
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo'] as Step[],
      });

      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // deploy has three parallel branches from analyzed_repo
      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should not re-run analyze-repo
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');

      // Should run the three branches
      expect(toolOrder).toContain('prepare_cluster'); // Independent
      expect(toolOrder).toContain('generate_k8s_manifests'); // Depends on analyzed_repo
      expect(toolOrder).toContain('build_image'); // Depends on analyzed_repo
    });
  });

  describe('transitive dependencies', () => {
    it('should resolve deep dependency chains', async () => {
      // build_image requires dockerfile_generated
      // dockerfile_generated requires analyzed_repo and resolved_base_images
      // resolved_base_images requires analyzed_repo

      const result = await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: { imageName: 'test' },
      });

      expect(result.result.ok).toBe(true);

      // Check execution order respects dependencies
      const toolOrder = executionLog.map(e => e.tool);
      const analyzeIdx = toolOrder.indexOf('analyze_repo');
      const resolveIdx = toolOrder.indexOf('resolve_base_images');
      const generateIdx = toolOrder.indexOf('generate_dockerfile');
      const buildIdx = toolOrder.indexOf('build_image');

      expect(analyzeIdx).toBeLessThan(resolveIdx);
      expect(resolveIdx).toBeLessThan(generateIdx);
      expect(generateIdx).toBeLessThan(buildIdx);
    });
  });

  describe('partial satisfaction', () => {
    it('should only run missing prerequisites', async () => {
      // Pre-satisfy some but not all prerequisites
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo', 'k8s_prepared'] as Step[],
      });

      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should not run already completed steps
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');
      expect(toolOrder).not.toContain('prepare_cluster');

      // Should run missing steps
      expect(toolOrder).toContain('build_image');
      expect(toolOrder).toContain('generate_k8s_manifests');
      expect(toolOrder).toContain('deploy');
    });

    it('should handle diamond dependencies correctly', async () => {
      // When multiple tools require the same prerequisite,
      // it should only be run once

      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // generate-dockerfile and generate_k8s_manifests both require analyzed_repo
      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Count how many times analyze-repo was run
      const analyzeCount = executionLog.filter(e => e.tool === 'analyze_repo').length;
      expect(analyzeCount).toBe(1);
    });
  });

  describe('canExecute checks', () => {
    it('should correctly identify when tool can execute', async () => {
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo', 'resolved_base_images', 'dockerfile_generated'] as Step[],
      });
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      const canExecute = await router.canExecute('build_image', session.sessionId);
      expect(canExecute.canExecute).toBe(true);
      expect(canExecute.missingSteps).toEqual([]);
    });

    it('should identify missing preconditions', async () => {
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo'] as Step[],
      });
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      const canExecute = await router.canExecute('build_image', session.sessionId);
      expect(canExecute.canExecute).toBe(false);
      expect(canExecute.missingSteps).toContain('dockerfile_generated');
    });

    it('should handle tools with no preconditions', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      const canExecute = await router.canExecute('prepare_cluster', session.sessionId);
      expect(canExecute.canExecute).toBe(true);
      expect(canExecute.missingSteps).toEqual([]);
    });
  });

  describe('execution planning', () => {
    it('should generate minimal execution plan', () => {
      const plan = router.getExecutionPlan('build_image');

      expect(plan).toEqual([
        'analyze_repo',
        'resolve_base_images',
        'generate_dockerfile',
        'build_image',
      ]);
    });

    it('should adjust plan based on completed steps', () => {
      const completed = new Set<Step>(['analyzed_repo', 'resolved_base_images']);
      const plan = router.getExecutionPlan('build_image', completed);

      expect(plan).toEqual([
        'generate_dockerfile',
        'build_image',
      ]);
    });

    it('should return single tool when all preconditions met', () => {
      const completed = new Set<Step>(['analyzed_repo', 'resolved_base_images', 'dockerfile_generated']);
      const plan = router.getExecutionPlan('build_image', completed);

      expect(plan).toEqual(['build_image']);
    });
  });

  describe('parameter passing', () => {
    it('should pass parameters through precondition chain', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: {
          imageName: 'my-app',
          tag: 'v1.2.3',
          path: './custom-path',
        },
      });

      expect(result.result.ok).toBe(true);

      // Check that path was passed to analyze-repo
      const analyzeExec = executionLog.find(e => e.tool === 'analyze_repo');
      expect(analyzeExec?.params.path).toBe('./custom-path');

      // Check that final params reached build_image
      const buildExec = executionLog.find(e => e.tool === 'build_image');
      expect(buildExec?.params.imageName).toBe('my-app');
      expect(buildExec?.params.tag).toBe('v1.2.3');
    });
  });

  describe('state accumulation', () => {
    it('should accumulate completed steps across execution', async () => {
      const sessionResult = await sessionManager.create();
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      // First execution
      await router.route({
        context: mockContext,
        toolName: 'analyze_repo',
        params: { path: './' },
        sessionId: session.sessionId,
      });

      const getResult = await sessionManager.get(session.sessionId);
      expect(getResult.ok).toBe(true);
      if (!getResult.ok) return;
      let updatedSession = getResult.value;
      expect(updatedSession?.completed_steps).toContain('analyzed_repo');

      // Second execution
      await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {},
        sessionId: session.sessionId,
      });

      const getResult2 = await sessionManager.get(session.sessionId);
      expect(getResult2.ok).toBe(true);
      if (!getResult2.ok) return;
      updatedSession = getResult2.value;
      expect(updatedSession?.completed_steps).toContain('analyzed_repo');
      expect(updatedSession?.completed_steps).toContain('k8s_prepared');

      // Third execution using accumulated state
      resetExecutionLog();
      await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: { manifestPath: './k8s' },
        sessionId: session.sessionId,
      });

      // Should not re-run analyze-repo or prepare_cluster
      const finalTools = executionLog.map(e => e.tool);
      expect(finalTools).not.toContain('analyze_repo');
      expect(finalTools).not.toContain('prepare_cluster');
    });
  });
});