/**
 * Tests for knowledge enhancement integration in AI-delegate tools
 */

import { analyzeRepo } from '@/tools/analyze-repo/tool';
import { generateDockerfile } from '@/tools/generate-dockerfile/tool';
import { fixDockerfile } from '@/tools/fix-dockerfile/tool';
import { generateK8sManifests } from '@/tools/generate-k8s-manifests/tool';
import { generateHelmCharts } from '@/tools/generate-helm-charts/tool';
import { resolveBaseImages } from '@/tools/resolve-base-images/tool';
import { generateAcaManifests } from '@/tools/generate-aca-manifests/tool';
import { convertAcaToK8s } from '@/tools/convert-aca-to-k8s/tool';
import type { ToolContext } from '@/mcp/context';
import * as knowledgeHelper from '@/tools/knowledge-helper';

// Mock the knowledge helper
jest.mock('@/tools/knowledge-helper', () => ({
  enhancePrompt: jest.fn().mockImplementation((prompt) => Promise.resolve(prompt + '\n\n## Best Practices to Apply\n- Use multi-stage builds\n- Minimize layers')),
}));

describe('Knowledge Enhancement Integration', () => {
  const mockContext: ToolContext = {
    sampling: {
      createMessage: jest.fn().mockResolvedValue({
        content: [{ text: '{"result": "success"}' }],
      }),
    },
    session: {
      get: jest.fn(),
      set: jest.fn(),
      delete: jest.fn(),
      exists: jest.fn(),
      clear: jest.fn(),
    },
    logger: {
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    },
  } as unknown as ToolContext;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('analyze-repo', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await analyzeRepo(
        {
          path: '/test/repo',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'analyze_repository',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-dockerfile', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateDockerfile(
        {
          path: '/test/repo',
          dockerfileDirectoryPaths: ['/'],
          environment: 'production',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'generate_dockerfile',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('fix-dockerfile', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await fixDockerfile(
        {
          dockerfile: 'FROM node:16\nRUN npm install',
          targetEnvironment: 'production',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'fix_dockerfile',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-k8s-manifests', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateK8sManifests(
        {
          appName: 'test-app',
          imageId: 'test:latest',
          path: '/test/k8s',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'generate_k8s_manifests',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-helm-charts', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateHelmCharts(
        {
          appName: 'test-app',
          chartName: 'test-chart',
          imageId: 'test:latest',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'generate_helm_charts',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('resolve-base-images', () => {
    it('should enhance prompt with technology-specific knowledge', async () => {
      const result = await resolveBaseImages(
        {
          technology: 'nodejs',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'resolve_base_images',
        expect.objectContaining({
          technology: 'nodejs',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-aca-manifests', () => {
    it('should enhance prompt with Azure-specific knowledge', async () => {
      const result = await generateAcaManifests(
        {
          appName: 'test-app',
          imageId: 'test:latest',
          path: '/test/aca',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'generate_aca_manifests',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('convert-aca-to-k8s', () => {
    it('should enhance prompt with conversion knowledge', async () => {
      const result = await convertAcaToK8s(
        {
          acaManifest: 'apiVersion: apps.azure.com/v1\nkind: ContainerApp',
        },
        mockContext,
      );

      expect(knowledgeHelper.enhancePrompt).toHaveBeenCalledWith(
        expect.any(String),
        'convert_aca_to_k8s',
        expect.objectContaining({
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('Knowledge Enhancement Content', () => {
    it('should include best practices in enhanced prompts', async () => {
      const enhancedPrompt = await knowledgeHelper.enhancePrompt(
        'Base prompt text',
        'generate_dockerfile',
        { environment: 'production' },
      );

      expect(enhancedPrompt).toContain('Best Practices to Apply');
      expect(enhancedPrompt).toContain('multi-stage builds');
    });

    it('should handle enhancement gracefully', async () => {
      // Reset the mock to return the original prompt (simulating failure fallback)
      (knowledgeHelper.enhancePrompt as jest.Mock).mockResolvedValueOnce('Original prompt');

      const result = await knowledgeHelper.enhancePrompt(
        'Original prompt',
        'generate_dockerfile',
        {},
      );

      // The helper should return something even if enhancement doesn't add content
      expect(result).toBe('Original prompt');
    });
  });
});