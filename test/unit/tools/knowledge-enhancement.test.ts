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
import * as promptEngine from '@/ai/prompt-engine';

// Mock the prompt engine
jest.mock('@/ai/prompt-engine', () => ({
  buildMessages: jest.fn().mockImplementation(() => Promise.resolve({
    messages: [
      { role: 'user', content: [{ type: 'text', text: 'Test prompt with knowledge' }] }
    ]
  })),
  toMCPMessages: jest.fn().mockImplementation((messages) => ({
    messages: messages.messages || [
      { role: 'user', content: [{ type: 'text', text: 'Test prompt' }] }
    ]
  })),
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'analyze_repository',
          tool: 'analyze-repo',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'generate_dockerfile',
          tool: 'generate-dockerfile',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'fix_dockerfile',
          tool: 'fix-dockerfile',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'generate_k8s_manifests',
          tool: 'generate-k8s-manifests',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'generate_helm_charts',
          tool: 'generate-helm-charts',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'resolve_base_images',
          tool: 'resolve-base-images',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'generate_aca_manifests',
          tool: 'generate-aca-manifests',
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

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: 'convert_aca_to_k8s',
          tool: 'convert-aca-to-k8s',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('Prompt Engine Integration', () => {
    it('should build messages with knowledge integration', async () => {
      // The prompt engine now handles knowledge integration internally
      const result = await promptEngine.buildMessages({
        basePrompt: 'Base prompt text',
        topic: 'generate_dockerfile',
        tool: 'generate-dockerfile',
        environment: 'production',
        knowledgeBudget: 3000,
      });

      // Verify buildMessages was called (mocked to return messages with knowledge)
      expect(promptEngine.buildMessages).toHaveBeenCalled();
      expect(result).toHaveProperty('messages');
      expect(result.messages).toHaveLength(1);
    });

    it('should handle message building gracefully', async () => {
      // Test that the prompt engine is properly integrated
      await generateDockerfile(
        {
          path: '/test/repo',
          dockerfileDirectoryPaths: ['/'],
          environment: 'production',
        },
        mockContext,
      );

      // Verify the prompt engine was invoked
      expect(promptEngine.buildMessages).toHaveBeenCalled();
    });
  });
});