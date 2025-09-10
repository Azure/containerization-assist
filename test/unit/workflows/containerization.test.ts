import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ToolContext } from '../../../src/mcp/context/types';
import type { ContainerizationWorkflowParams } from '../../../src/workflows/types';

// Mock all the tool imports
jest.mock('../../../src/tools/analyze-repo', () => ({
  analyzeRepo: jest.fn(),
}));

jest.mock('../../../src/tools/generate-dockerfile', () => ({
  generateDockerfile: jest.fn(),
}));

jest.mock('../../../src/tools/build-image', () => ({
  buildImage: jest.fn(),
}));

jest.mock('../../../src/tools/scan', () => ({
  scanImage: jest.fn(),
}));

jest.mock('../../../src/tools/tag-image/tool', () => ({
  tagImage: jest.fn(),
}));

jest.mock('../../../src/lib/base-images', () => ({
  getRecommendedBaseImage: jest.fn().mockReturnValue('node:18-alpine'),
}));

describe('Containerization Workflow', () => {
  let mockToolContext: ToolContext;
  let mockSessionManager: any;

  beforeEach(() => {
    mockSessionManager = {
      get: jest.fn(),
      create: jest.fn(),
      update: jest.fn(),
    };

    mockToolContext = {
      logger: {
        info: jest.fn(),
        warn: jest.fn(),
        error: jest.fn(),
        debug: jest.fn(),
      },
      sessionManager: mockSessionManager,
      signal: undefined,
    } as any;

    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('runContainerizationWorkflow', () => {
    it('should exist and be a function', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      expect(typeof runContainerizationWorkflow).toBe('function');
    });

    it('should require sessionManager in toolContext', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      
      const contextWithoutSessionManager = {
        ...mockToolContext,
        sessionManager: undefined,
      };

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      await expect(
        runContainerizationWorkflow(params, contextWithoutSessionManager)
      ).rejects.toThrow('sessionManager is required in toolContext');
    });

    it('should initialize workflow context correctly', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');

      // Mock successful analysis
      (analyzeRepo as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          language: 'javascript',
          framework: 'express',
          recommendations: {
            baseImage: 'node:18-alpine',
          },
        },
      });

      // Mock session manager
      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      // This will fail at dockerfile generation, but that's OK for testing initialization
      await runContainerizationWorkflow(params, mockToolContext);

      // Verify session operations
      expect(mockSessionManager.get).toHaveBeenCalledWith('test-session');
      expect(mockSessionManager.create).toHaveBeenCalledWith('test-session');
      expect(mockSessionManager.update).toHaveBeenCalledWith('test-session', {
        status: 'analyzing',
        stage: 'analyze-repository',
      });
    });

    it('should handle analyze-repo failures gracefully', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');

      // Mock failed analysis
      (analyzeRepo as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Repository analysis failed',
      });

      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Analysis failed');
      expect(result.sessionId).toBe('test-session');
      expect(result.metadata).toBeDefined();
      expect(result.metadata.steps).toBeDefined();
      expect(result.metadata.startTime).toBeDefined();
      expect(result.metadata.endTime).toBeDefined();
      expect(result.metadata.duration).toBeGreaterThanOrEqual(0);
    });

    it('should handle abort signals', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');

      const abortController = new AbortController();
      abortController.abort();

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(
        params,
        mockToolContext,
        { abortSignal: abortController.signal }
      );

      expect(result.success).toBe(false);
      expect(result.error).toBe('Workflow aborted before start');
    });

    it('should contain all required workflow steps', () => {
      const workflowPath = join(__dirname, '../../../src/workflows/containerization.ts');
      const content = readFileSync(workflowPath, 'utf-8');
      
      // Verify all steps are defined
      expect(content).toContain('analyze-repository');
      expect(content).toContain('generate-dockerfile');
      expect(content).toContain('build-image');
      expect(content).toContain('scan-image');
      expect(content).toContain('tag-image');
    });

    it('should handle dockerfile generation failures', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');
      const { generateDockerfile } = await import('../../../src/tools/generate-dockerfile');

      // Mock successful analysis
      (analyzeRepo as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          language: 'javascript',
          framework: 'express',
        },
      });

      // Mock failed dockerfile generation
      (generateDockerfile as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Dockerfile generation failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Dockerfile generation failed');
    });

    it('should handle build failures', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');
      const { generateDockerfile } = await import('../../../src/tools/generate-dockerfile');
      const { buildImage } = await import('../../../src/tools/build-image');

      // Mock successful analysis
      (analyzeRepo as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          language: 'javascript',
          framework: 'express',
        },
      });

      // Mock successful dockerfile generation
      (generateDockerfile as jest.Mock).mockResolvedValue({
        ok: true,
        value: {
          path: '/test/Dockerfile',
          content: 'FROM node:18-alpine\nWORKDIR /app',
        },
      });

      // Mock failed build
      (buildImage as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Build failed',
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toContain('Build failed');
    });

    it('should treat scan failures as warnings, not hard failures', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');
      const { generateDockerfile } = await import('../../../src/tools/generate-dockerfile');
      const { buildImage } = await import('../../../src/tools/build-image');
      const { scanImage } = await import('../../../src/tools/scan');
      const { tagImage } = await import('../../../src/tools/tag-image/tool');

      // Mock successful steps up to scan
      (analyzeRepo as jest.Mock).mockResolvedValue({
        ok: true,
        value: { language: 'javascript' },
      });

      (generateDockerfile as jest.Mock).mockResolvedValue({
        ok: true,
        value: { path: '/test/Dockerfile' },
      });

      (buildImage as jest.Mock).mockResolvedValue({
        ok: true,
        value: { imageId: 'test-image' },
      });

      // Mock failed scan
      (scanImage as jest.Mock).mockResolvedValue({
        ok: false,
        error: 'Scanner not available',
      });

      // Mock successful tagging
      (tagImage as jest.Mock).mockResolvedValue({
        ok: true,
        value: { tags: ['test:latest'] },
      });

      mockSessionManager.get.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(params, mockToolContext);

      // Workflow should still succeed despite scan failure
      expect(result.success).toBe(true);
      expect(mockToolContext.logger.warn).toHaveBeenCalledWith('Image scan found issues');
    });

    it('should handle general exceptions', async () => {
      const { runContainerizationWorkflow } = await import('../../../src/workflows/containerization');
      const { analyzeRepo } = await import('../../../src/tools/analyze-repo');

      // Mock exception
      (analyzeRepo as jest.Mock).mockRejectedValue(new Error('Unexpected error'));

      mockSessionManager.get.mockResolvedValue(null);
      mockSessionManager.create.mockResolvedValue({ id: 'test-session' });
      mockSessionManager.update.mockResolvedValue(undefined);

      const params: ContainerizationWorkflowParams = {
        sessionId: 'test-session',
        projectPath: '/test/project',
      };

      const result = await runContainerizationWorkflow(params, mockToolContext);

      expect(result.success).toBe(false);
      expect(result.error).toBe('Unexpected error');
    });
  });

  describe('containerizationWorkflow export', () => {
    it('should export workflow configuration', async () => {
      const { containerizationWorkflow } = await import('../../../src/workflows/containerization');
      
      expect(containerizationWorkflow).toBeDefined();
      expect(containerizationWorkflow.name).toBe('containerization-workflow');
      expect(containerizationWorkflow.description).toContain('Complete containerization pipeline');
      expect(typeof containerizationWorkflow.execute).toBe('function');
      expect(containerizationWorkflow.schema).toBeDefined();
      expect(containerizationWorkflow.schema.type).toBe('object');
      expect(containerizationWorkflow.schema.required).toContain('sessionId');
      expect(containerizationWorkflow.schema.required).toContain('projectPath');
    });

    it('should have proper schema properties', async () => {
      const { containerizationWorkflow } = await import('../../../src/workflows/containerization');
      
      const schema = containerizationWorkflow.schema;
      expect(schema.properties).toBeDefined();
      expect(schema.properties.sessionId).toBeDefined();
      expect(schema.properties.projectPath).toBeDefined();
      expect(schema.properties.buildOptions).toBeDefined();
      expect(schema.properties.scanOptions).toBeDefined();

      // Check buildOptions structure
      expect(schema.properties.buildOptions.type).toBe('object');
      expect(schema.properties.buildOptions.properties).toBeDefined();
      
      // Check scanOptions structure
      expect(schema.properties.scanOptions.type).toBe('object');
      expect(schema.properties.scanOptions.properties).toBeDefined();
    });
  });
});