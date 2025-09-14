/**
 * Integration tests for out-of-order tool execution
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import pino from 'pino';
import { ToolRouter } from '@mcp/tool-router';
import type { Step } from '@mcp/tool-graph';
import {
  createMockToolsMap,
  resetExecutionLog,
  executionLog,
} from './fixtures/mock-tools';
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

    router = new ToolRouter({
      sessionManager,
      logger,
      tools: createMockToolsMap(),
    });
  });

  afterEach(async () => {
    await sessionManager.clear();
  });

  describe('deploy without prerequisites', () => {
    it('should automatically run analyze-repo, build-image, prepare-cluster, and generate-k8s-manifests', async () => {
      const result = await router.route({
        toolName: 'deploy',
        params: {
          manifestPath: './k8s',
          namespace: 'production',
        },
        context: mockContext,
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('build-image');
      expect(result.executedTools).toContain('prepare-cluster');
      expect(result.executedTools).toContain('generate-k8s-manifests');
      expect(result.executedTools).toContain('deploy');

      // Verify execution order from log
      const toolOrder = executionLog.map(e => e.tool);

      // Check that all required tools were executed
      expect(toolOrder).toContain('analyze-repo');
      expect(toolOrder).toContain('resolve-base-images');
      expect(toolOrder).toContain('generate-dockerfile');
      expect(toolOrder).toContain('build-image');
      expect(toolOrder).toContain('prepare-cluster');
      expect(toolOrder).toContain('generate-k8s-manifests');
      expect(toolOrder).toContain('deploy');

      // Verify order constraints (dependencies must be respected)
      const analyzeIdx = toolOrder.indexOf('analyze-repo');
      const resolveIdx = toolOrder.indexOf('resolve-base-images');
      const generateDockerIdx = toolOrder.indexOf('generate-dockerfile');
      const buildIdx = toolOrder.indexOf('build-image');
      const deployIdx = toolOrder.indexOf('deploy');

      // analyze-repo must come before its dependents
      expect(analyzeIdx).toBeLessThan(resolveIdx);
      expect(analyzeIdx).toBeLessThan(generateDockerIdx);
      expect(analyzeIdx).toBeLessThan(buildIdx);

      // resolve-base-images must come before generate-dockerfile
      expect(resolveIdx).toBeLessThan(generateDockerIdx);

      // generate-dockerfile must come before build-image
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

  describe('push-image without build', () => {
    it('should automatically run analyze-repo, generate-dockerfile, and build-image first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'push-image',
        params: {
          imageId: 'auto-generated',
          registry: 'myregistry.io',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('generate-dockerfile');
      expect(result.executedTools).toContain('build-image');
      expect(result.executedTools).toContain('push-image');

      // Verify execution order
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toEqual([
        'analyze-repo',
        'resolve-base-images',
        'generate-dockerfile',
        'build-image',
        'push-image',
      ]);

      // Verify the push used the built image ID
      const pushExecution = executionLog.find(e => e.tool === 'push-image');
      expect(pushExecution?.params.imageId).toBeDefined();
    });
  });

  describe('scan-image without build', () => {
    it('should automatically build the image first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'scan-image',
        params: {
          imageId: 'will-be-built',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('build-image');
      expect(result.executedTools).toContain('scan-image');

      // Verify proper order
      const toolOrder = executionLog.map(e => e.tool);
      const analyzeIndex = toolOrder.indexOf('analyze-repo');
      const buildIndex = toolOrder.indexOf('build-image');
      const scanIndex = toolOrder.indexOf('scan-image');

      expect(analyzeIndex).toBeLessThan(buildIndex);
      expect(buildIndex).toBeLessThan(scanIndex);
    });
  });

  describe('generate-dockerfile without analysis', () => {
    it('should automatically run analyze-repo and resolve-base-images first', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'generate-dockerfile',
        params: {
          path: './src',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toContain('analyze-repo');
      expect(result.executedTools).toContain('resolve-base-images');
      expect(result.executedTools).toContain('generate-dockerfile');

      // Verify execution order
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).toEqual([
        'analyze-repo',
        'resolve-base-images',
        'generate-dockerfile',
      ]);
    });
  });

  describe('partial prerequisites satisfied', () => {
    it('should only run missing prerequisites', async () => {
      // Pre-populate session with some completed steps
      const session = await sessionManager.createWithState({
        completed_steps: ['analyzed_repo'] as Step[],
        results: {
          'analyze-repo': {
            framework: 'node',
            packageManager: 'npm',
          },
        },
      });

      const result = await router.route({
        context: mockContext,
        toolName: 'build-image',
        params: {
          imageName: 'test-app',
        },
        sessionId: session.sessionId,
      });

      expect(result.result.ok).toBe(true);

      // Should NOT re-run analyze-repo
      const toolOrder = executionLog.map(e => e.tool);
      expect(toolOrder).not.toContain('analyze-repo');

      // Should run missing prerequisites
      expect(toolOrder).toEqual([
        'resolve-base-images',
        'generate-dockerfile',
        'build-image',
      ]);
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
      const executedTools = new Set(executionLog.map(e => e.tool));
      expect(executedTools.has('analyze-repo')).toBe(true);
      expect(executedTools.has('build-image')).toBe(true);
      expect(executedTools.has('prepare-cluster')).toBe(true);
      expect(executedTools.has('generate-k8s-manifests')).toBe(true);
      expect(executedTools.has('deploy')).toBe(true);

      // Verify order constraints
      const toolOrder = executionLog.map(e => e.tool);
      const analyzeIndex = toolOrder.indexOf('analyze-repo');
      const buildIndex = toolOrder.indexOf('build-image');
      const deployIndex = toolOrder.indexOf('deploy');

      // analyze must come before build and deploy
      expect(analyzeIndex).toBeLessThan(buildIndex);
      expect(analyzeIndex).toBeLessThan(deployIndex);

      // build must come before deploy
      expect(buildIndex).toBeLessThan(deployIndex);
    });
  });

  describe('tools with no dependencies', () => {
    it('should execute prepare-cluster directly without prerequisites', async () => {
      const result = await router.route({
        context: mockContext,
        toolName: 'prepare-cluster',
        params: {
          context: 'minikube',
        },
      });

      expect(result.result.ok).toBe(true);
      expect(result.executedTools).toEqual(['prepare-cluster']);
      expect(executionLog.length).toBe(1);
      expect(executionLog[0].tool).toBe('prepare-cluster');
    });
  });

  describe('execution plan', () => {
    it('should provide correct execution plan for deploy', () => {
      const plan = router.getExecutionPlan('deploy');

      expect(plan).toContain('analyze-repo');
      expect(plan).toContain('build-image');
      expect(plan).toContain('prepare-cluster');
      expect(plan).toContain('generate-k8s-manifests');
      expect(plan[plan.length - 1]).toBe('deploy');
    });

    it('should provide minimal plan when some steps completed', () => {
      const completedSteps = new Set<Step>(['analyzed_repo', 'k8s_prepared']);
      const plan = router.getExecutionPlan('deploy', completedSteps);

      // Should not include analyze-repo or prepare-cluster
      expect(plan).not.toContain('analyze-repo');
      expect(plan).not.toContain('prepare-cluster');

      // Should include missing steps
      expect(plan).toContain('build-image');
      expect(plan).toContain('generate-k8s-manifests');
      expect(plan[plan.length - 1]).toBe('deploy');
    });
  });
});