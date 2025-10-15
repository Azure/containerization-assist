/**
 * Tests for knowledge enhancement integration in AI-delegate tools
 */

import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import generateHelmChartsTool from '@/tools/generate-helm-charts/tool';
import resolveBaseImagesTool from '@/tools/resolve-base-images/tool';
import generateAcaManifestsTool from '@/tools/generate-aca-manifests/tool';
import convertAcaToK8sTool from '@/tools/convert-aca-to-k8s/tool';
import type { ToolContext } from '@/mcp/context';
import * as promptEngine from '@/ai/prompt-engine';
import { TOPICS } from '@/types/topics';

// Mock the prompt engine
jest.mock('@/ai/prompt-engine', () => ({
  buildMessages: jest.fn().mockImplementation(() => Promise.resolve({
    messages: [
      { role: 'user', content: [{ type: 'text', text: 'Test prompt with knowledge' }] }
    ]
  })),
}));

// Mock the MCP message converter
jest.mock('@/mcp/ai/message-converter', () => ({
  toMCPMessages: jest.fn().mockImplementation((messages) => ({
    messages: messages.messages || [
      { role: 'user', content: [{ type: 'text', text: 'Test prompt' }] }
    ]
  })),
}));

// Mock the sampling runner
jest.mock('@/mcp/ai/sampling-runner', () => ({
  sampleWithRerank: jest.fn().mockImplementation(async (ctx, buildRequest, score, options) => {
    // Call buildRequest to get the request details and check for hints
    const request = await buildRequest();
    const hints = request?.modelPreferences?.hints;

    let text = '';
    if (hints?.some((h: any) => h.name === 'repo-analysis')) {
      // For analyze-repo - return JSON analysis
      text = JSON.stringify({
        language: 'JavaScript',
        framework: 'Express',
        dependencies: ['express'],
        suggestedPorts: [3000],
        buildSystem: { type: 'npm' },
        entryPoint: 'index.js'
      });
    } else if (hints?.some((h: any) => h.name === 'kubernetes-manifests')) {
      // For K8s manifests - return YAML
      text = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
        - name: test-app
          image: test:latest
---
apiVersion: v1
kind: Service
metadata:
  name: test-app
spec:
  selector:
    app: test-app
  ports:
    - port: 80
      targetPort: 8080`;
    } else {
      // Default - return Dockerfile for generate-dockerfile and others
      text = 'FROM node:18-alpine\nWORKDIR /app\nCOPY . .\nRUN npm install\nCMD ["npm", "start"]';
    }

    return Promise.resolve({
      ok: true,
      value: {
        text,
        winner: {
          score: 85,
          scoreBreakdown: {
            structure: 20,
            completeness: 25,
            clarity: 20,
            actionability: 20
          }
        }
      }
    });
  }),
}));

// Mock fs module to prevent actual file operations
jest.mock('node:fs', () => ({
  promises: {
    stat: jest.fn().mockImplementation((filePath) => {
      // Mock stat for analyze-repo
      return Promise.resolve({
        isDirectory: () => true,
        isFile: () => false,
      });
    }),
    writeFile: jest.fn().mockResolvedValue(undefined),
    mkdir: jest.fn().mockResolvedValue(undefined),
    readdir: jest.fn().mockImplementation((path, options) => {
      // Mock directory listing for analyze-repo
      if (path === '/test/repo') {
        return Promise.resolve([
          { name: 'package.json', isDirectory: () => false },
          { name: 'src', isDirectory: () => true },
          { name: 'README.md', isDirectory: () => false },
        ]);
      }
      if (path.includes('/test/repo/src')) {
        return Promise.resolve([{ name: 'index.js', isDirectory: () => false }]);
      }
      return Promise.resolve([]);
    }),
    readFile: jest.fn().mockImplementation((filePath) => {
      // Mock config file contents for analyze-repo
      if (filePath.includes('package.json')) {
        return Promise.resolve(JSON.stringify({
          name: 'test-app',
          version: '1.0.0',
          dependencies: { express: '^4.0.0' },
          scripts: { start: 'node index.js' }
        }));
      }
      return Promise.reject(new Error('File not found'));
    }),
  },
}));

// Mock path module for consistent behavior
jest.mock('node:path', () => ({
  ...jest.requireActual('node:path'),
  resolve: jest.fn((cwd, p) => `/test/${p || ''}`),
  join: jest.fn((...parts) => parts.join('/')),
  isAbsolute: jest.fn((p) => typeof p === 'string' && p.startsWith('/')),
}));

describe('Knowledge Enhancement Integration', () => {
  // Create a mock that returns different responses based on the input
  const createMessageMock = jest.fn().mockImplementation((params) => {
    // Check for hints or other parameters to determine the response type
    const hints = params?.modelPreferences?.hints;

    if (hints?.some((h: any) => h.name === 'json-output' || h.name === 'code-analysis' || h.name === 'repo-analysis')) {
      // For analyze-repo - return JSON analysis
      return Promise.resolve({
        content: [{ text: JSON.stringify({
          language: 'JavaScript',
          framework: 'Express',
          dependencies: ['express', 'mongodb'],
          suggestedPorts: [3000],
          buildSystem: { type: 'npm' },
          entryPoint: 'server.js'
        }) }],
      });
    } else if (hints?.some((h: any) => h.name === 'kubernetes-manifests')) {
      // For K8s manifests - return YAML
      return Promise.resolve({
        content: [{ text: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: test:latest` }],
      });
    } else {
      // Default - return Dockerfile for generate-dockerfile and others
      return Promise.resolve({
        content: [{ text: 'FROM node:18-alpine\nWORKDIR /app\nCOPY . .\nRUN npm install\nCMD ["npm", "start"]' }],
      });
    }
  });

  const mockContext: ToolContext = {
    sampling: {
      createMessage: createMessageMock,
    },
    session: {
      id: 'test-session-id',
      get: jest.fn(),
      set: jest.fn(),
      delete: jest.fn(),
      exists: jest.fn(),
      clear: jest.fn(),
      pushStep: jest.fn(),
      storeResult: jest.fn(),
      getResult: jest.fn(),
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

  describe('analyze-repo (v4.0.0 - now deterministic, no AI)', () => {
    it('should successfully analyze repository with pre-provided modules (legacy mode)', async () => {
      const result = await analyzeRepoTool.run(
        {
          repositoryPathAbsoluteUnix: '/test/repo',
          sessionId: 'test-session-123',
          modules: [
            {
              name: 'test-module',
              modulePathAbsoluteUnix: '/test/repo',
              language: 'java',
              buildSystem: { type: 'maven' },
              ports: [8080],
            },
          ],
        },
        mockContext,
      );

      // analyze-repo v4.0.0 is now deterministic - accepts pre-provided modules or parses config files
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('modules');
        expect(result.value.modules).toHaveLength(1);
        expect(result.value).toHaveProperty('analyzedPath', '/test/repo');
      }
    });

    it('should analyze repository deterministically when modules not provided', async () => {
      // v4.0.0 uses deterministic file parsing, not AI
      const result = await analyzeRepoTool.run(
        {
          repositoryPathAbsoluteUnix: '/test/repo',
          sessionId: 'test-session-123',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('modules');
        expect(result.value.modules.length).toBeGreaterThan(0);
        // Should detect express from mocked package.json
        expect(result.value.modules?.[0].frameworks?.[0]?.name).toBe('express');
      }
      // Verify AI was NOT called (v4.0.0 is deterministic)
      expect(promptEngine.buildMessages).not.toHaveBeenCalled();
    });
  });

  describe('generate-dockerfile', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateDockerfileTool.run(
        {
          repositoryPath: '/test/repo',
          environment: 'production',
        },
        mockContext,
      );

      // Now uses a single call with unified topic
      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.DOCKERFILE,
          tool: 'generate-dockerfile',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('fix-dockerfile', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await fixDockerfileTool.run(
        {
          dockerfile: 'FROM node:16\nRUN npm install',
          targetEnvironment: 'production',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.FIX_DOCKERFILE,
          tool: 'fix-dockerfile',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-k8s-manifests', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateK8sManifestsTool.run(
        {
          appName: 'test-app',
          imageId: 'test:latest',
          path: '/test/k8s',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.KUBERNETES,
          tool: 'generate-k8s-manifests',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-helm-charts', () => {
    it('should enhance prompt with knowledge base', async () => {
      const result = await generateHelmChartsTool.run(
        {
          appName: 'test-app',
          chartName: 'test-chart',
          imageId: 'test:latest',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.GENERATE_HELM_CHARTS,
          tool: 'generate-helm-charts',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('resolve-base-images', () => {
    it('should enhance prompt with technology-specific knowledge', async () => {
      const result = await resolveBaseImagesTool.run(
        {
          technology: 'nodejs',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.RESOLVE_BASE_IMAGES,
          tool: 'resolve-base-images',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('generate-aca-manifests', () => {
    it('should enhance prompt with Azure-specific knowledge', async () => {
      const result = await generateAcaManifestsTool.run(
        {
          appName: 'test-app',
          imageId: 'test:latest',
          path: '/test/aca',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.GENERATE_ACA_MANIFESTS,
          tool: 'generate-aca-manifests',
          environment: 'production',
        }),
      );

      expect(result.ok).toBe(true);
    });
  });

  describe('convert-aca-to-k8s', () => {
    it('should enhance prompt with conversion knowledge', async () => {
      const result = await convertAcaToK8sTool.run(
        {
          acaManifest: 'apiVersion: apps.azure.com/v1\nkind: ContainerApp',
        },
        mockContext,
      );

      expect(promptEngine.buildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.CONVERT_ACA_TO_K8S,
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
        topic: TOPICS.DOCKERFILE_GENERATION,
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
      await generateDockerfileTool.run(
        {
          repositoryPath: '/test/repo',
          environment: 'production',
        },
        mockContext,
      );

      // Verify the prompt engine was invoked
      expect(promptEngine.buildMessages).toHaveBeenCalled();
    });
  });
});
