/**
 * Integration tests for out-of-order tool execution
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import pino from 'pino';
import { createToolRouter, type ToolRouter } from '@/mcp/tool-router';
import type { Step } from '@/mcp/tool-graph';
import { createMockToolsMap, resetExecutionLog, executionLog } from './fixtures/mock-tools';
import { MockSessionManager } from './fixtures/mock-session';
import { createMockContext } from './fixtures/mock-context';

describe('Out-of-Order Tool Execution', () => {
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

  describe('deploy without prerequisites', () => {
    it('should automatically run analyze-repo, build_image, prepare_cluster, and generate_k8s_manifests', async () => {
      const result = await router.route({
        toolName: 'deploy',
        params: {
          manifestPath: './k8s',
          namespace: 'production',
        },
        context: mockContext,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
      expect(result.executedTools).toContain('build_image');
      expect(result.executedTools).toContain('prepare_cluster');
      expect(result.executedTools).toContain('generate_k8s_manifests');
      expect(result.executedTools).toContain('deploy');

      // Verify execution order from log
      const toolOrder = executionLog.map((e) => e.tool);

      // Check that all required tools were executed
      expect(toolOrder).toContain('analyze_repo');
      expect(toolOrder).toContain('resolve_base_images');
      expect(toolOrder).toContain('generate_dockerfile');
      expect(toolOrder).toContain('build_image');
      expect(toolOrder).toContain('prepare_cluster');
      expect(toolOrder).toContain('generate_k8s_manifests');
      expect(toolOrder).toContain('deploy');

      // Verify order constraints (dependencies must be respected)
      const analyzeIdx = toolOrder.indexOf('analyze_repo');
      const resolveIdx = toolOrder.indexOf('resolve_base_images');
      const generateDockerIdx = toolOrder.indexOf('generate_dockerfile');
      const buildIdx = toolOrder.indexOf('build_image');
      const deployIdx = toolOrder.indexOf('deploy');

      // analyze-repo must come before its dependents
      expect(analyzeIdx).toBeLessThan(resolveIdx);
      expect(analyzeIdx).toBeLessThan(generateDockerIdx);
      expect(analyzeIdx).toBeLessThan(buildIdx);

      // resolve_base_images must come before generate_dockerfile
      expect(resolveIdx).toBeLessThan(generateDockerIdx);

      // generate_dockerfile must come before build_image
      expect(generateDockerIdx).toBeLessThan(buildIdx);

      // All prerequisites must come before deploy
      expect(buildIdx).toBeLessThan(deployIdx);
      expect(toolOrder[toolOrder.length - 1]).toBe('deploy');

      // Verify session state
      const completedSteps = result.sessionState.completed_steps as Step[];
      expect(completedSteps).toContain('analyzed_repo');
      expect(completedSteps).toContain('built_image');
      expect(completedSteps).toContain('k8s_prepared');
      expect(completedSteps).toContain('manifests_generated');
      expect(completedSteps).toContain('deployed');
    });
  });

  describe('push_image without build', () => {
    it('should automatically run analyze-repo, generate_dockerfile, and build_image first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'push_image',
        params: {
          imageId: 'auto-generated',
          registry: 'myregistry.io',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
      expect(result.executedTools).toContain('generate_dockerfile');
      expect(result.executedTools).toContain('build_image');
      expect(result.executedTools).toContain('push_image');

      // Verify execution order
      const toolOrder = executionLog.map((e) => e.tool);
      expect(toolOrder).toEqual([
        'analyze_repo',
        'resolve_base_images',
        'generate_dockerfile',
        'build_image',
        'push_image',
      ]);

      // Verify the push used the built image ID
      const pushExecution = executionLog.find((e) => e.tool === 'push_image');
      expect(pushExecution?.params.imageId).toBeDefined();
    });
  });

  describe('scan without build', () => {
    it('should automatically build the image first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'scan',
        params: {
          imageId: 'will-be-built',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
      expect(result.executedTools).toContain('build_image');
      expect(result.executedTools).toContain('scan');

      // Verify proper order
      const toolOrder = executionLog.map((e) => e.tool);
      const analyzeIndex = toolOrder.indexOf('analyze_repo');
      const buildIndex = toolOrder.indexOf('build_image');
      const scanIndex = toolOrder.indexOf('scan');

      expect(analyzeIndex).toBeLessThan(buildIndex);
      expect(buildIndex).toBeLessThan(scanIndex);
    });
  });

  describe('generate_dockerfile without analysis', () => {
    it('should automatically run analyze-repo and resolve_base_images first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'generate_dockerfile',
        params: {
          path: './src',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze_repo');
      expect(result.executedTools).toContain('resolve_base_images');
      expect(result.executedTools).toContain('generate_dockerfile');

      // Verify execution order
      const toolOrder = executionLog.map((e) => e.tool);
      expect(toolOrder).toEqual(['analyze_repo', 'resolve_base_images', 'generate_dockerfile']);
    });
  });

  describe('partial prerequisites satisfied', () => {
    it('should only run missing prerequisites', async () => {
      // Pre-populate session with some completed steps
      const sessionResult = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo'] as Step[],
        results: {
          analyze_repo: {
            framework: 'node',
            packageManager: 'npm',
          },
        },
      });
      expect(sessionResult.ok).toBe(true);
      if (!sessionResult.ok) return;
      const session = sessionResult.value;

      const result = await router.route({
        context: mockContext,
        toolName: 'build_image',
        params: {
          imageName: 'test-app',
        },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should NOT re-run analyze-repo
      const toolOrder = executionLog.map((e) => e.tool);
      expect(toolOrder).not.toContain('analyze_repo');

      // Should run missing prerequisites
      expect(toolOrder).toEqual(['resolve_base_images', 'generate_dockerfile', 'build_image']);
    });
  });

  describe('complex dependency chains', () => {
    it('should handle multiple branch dependencies correctly', async () => {
      // deploy has three parallel prerequisites:
      // - built_image (requires: analyzed_repo, dockerfile_generated)
      // - k8s_prepared (no requirements)
      // - manifests_generated (requires: analyzed_repo)

      const result = await router.route({
        context: mockContext,
        toolName: 'deploy',
        params: {
          manifestPath: './k8s',
        },
      });

      expect(result.result.ok).toBe(true);

      // Verify all required tools were run
      const executedTools = new Set(executionLog.map((e) => e.tool));
      expect(executedTools.has('analyze_repo')).toBe(true);
      expect(executedTools.has('build_image')).toBe(true);
      expect(executedTools.has('prepare_cluster')).toBe(true);
      expect(executedTools.has('generate_k8s_manifests')).toBe(true);
      expect(executedTools.has('deploy')).toBe(true);

      // Verify order constraints
      const toolOrder = executionLog.map((e) => e.tool);
      const analyzeIndex = toolOrder.indexOf('analyze_repo');
      const buildIndex = toolOrder.indexOf('build_image');
      const deployIndex = toolOrder.indexOf('deploy');

      // analyze must come before build and deploy
      expect(analyzeIndex).toBeLessThan(buildIndex);
      expect(analyzeIndex).toBeLessThan(deployIndex);

      // build must come before deploy
      expect(buildIndex).toBeLessThan(deployIndex);
    });
  });

  describe('tools with no dependencies', () => {
    it('should execute prepare_cluster directly without prerequisites', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'prepare_cluster',
        params: {
          context: 'minikube',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['prepare_cluster']);
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].tool).toBe('prepare_cluster');
    });
  });

  describe('execution plan', () => {
    it('should provide correct execution plan for deploy', () => {
      const plan = router.getExecutionPlan('deploy');

      expect(plan).toContain('analyze_repo');
      expect(plan).toContain('build_image');
      expect(plan).toContain('prepare_cluster');
      expect(plan).toContain('generate_k8s_manifests');
      expect(plan[plan.length - 1]).toBe('deploy');
    });

    it('should provide minimal plan when some steps completed', () => {
      const completedSteps = new Set<Step>(['analyzed_repo', 'k8s_prepared']);
      const plan = router.getExecutionPlan('deploy', completedSteps);

      // Should not include analyze-repo or prepare_cluster
      expect(plan).not.toContain('analyze_repo');
      expect(plan).not.toContain('prepare_cluster');

      // Should include missing steps
      expect(plan).toContain('build_image');
      expect(plan).toContain('generate_k8s_manifests');
      expect(plan[plan.length - 1]).toBe('deploy');
    });
  });
});
