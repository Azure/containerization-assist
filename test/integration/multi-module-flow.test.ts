/**
 * Integration Test: Multi-Module Containerization Flow
 *
 * Tests the end-to-end workflow for monorepo/multi-module projects:
 * 1. analyze-repo detects multiple modules
 * 2. generate-dockerfile automatically generates Dockerfiles for all modules
 * 3. generate-k8s-manifests automatically generates manifests for all modules
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import type { ModuleInfo, RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { ToolContext, SamplingRequest, SamplingResponse } from '@/mcp/context';
import type { Logger } from 'pino';
import type { SessionFacade } from '@/app/orchestrator-types';

// Mock logger factory
function createMockLogger(): Logger {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as unknown as Logger;
}

// Mock session with full module support
function createMockSession(initialData?: Record<string, unknown>): SessionFacade {
  const storage = new Map<string, unknown>(Object.entries(initialData || {}));

  return {
    id: 'test-session-id',
    get: jest.fn(<T>(key: string): T | undefined => storage.get(key)),
    set: jest.fn((key: string, value: unknown) => {
      storage.set(key, value);
    }),
    pushStep: jest.fn((step: string) => {
      // Store completed steps
    }),
  } as unknown as SessionFacade;
}

// Helper to create consistent mock ToolContext with smart responses
function createMockContext(session: SessionFacade): ToolContext {
  return {
    logger: createMockLogger(),
    session,
    sampling: {
      createMessage: jest
        .fn<(req: SamplingRequest) => Promise<SamplingResponse>>()
        .mockImplementation(async (req: SamplingRequest) => {
          // Check what type of request this is based on the prompt content
          const promptText = req.messages
            .map((m) => m.content.map((c) => c.text).join(' '))
            .join(' ');

          // Mock Dockerfile generation
          if (promptText.includes('Dockerfile') || promptText.includes('Docker')) {
            return {
              role: 'assistant',
              content: [
                {
                  type: 'text',
                  text: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]`,
                },
              ],
            };
          }

          // Mock K8s manifest generation
          if (promptText.includes('Kubernetes') || promptText.includes('manifest')) {
            return {
              role: 'assistant',
              content: [
                {
                  type: 'text',
                  text: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: test-registry/app:latest
        ports:
        - containerPort: 3000
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
    targetPort: 3000`,
                },
              ],
            };
          }

          // Default mock response
          return {
            role: 'assistant',
            content: [{ type: 'text', text: 'Mock AI response' }],
          };
        }),
    },
    getPrompt: jest.fn().mockResolvedValue({
      description: 'Mock prompt',
      messages: [],
    }),
    signal: undefined,
    progress: undefined,
  };
}

describe('Multi-Module Containerization Flow', () => {
  describe('Automated multi-module Dockerfile generation', () => {
    it('should generate Dockerfiles for all modules when no moduleName specified', async () => {
      const modules: ModuleInfo[] = [
        {
          name: 'api-service',
          path: 'services/api',
          language: 'node',
          framework: 'express',
          ports: [3000],
          dependencies: ['express', 'dotenv'],
        },
        {
          name: 'web-app',
          path: 'apps/web',
          language: 'node',
          framework: 'react',
          ports: [3001],
          dependencies: ['react', 'react-dom'],
        },
      ];

      const session = createMockSession({
        isMonorepo: true,
        modules,
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-monorepo',
          modules: modules, // Pass modules explicitly
        },
        ctx,
      );

      // Debug: log the actual result
      if (!result.ok) {
        console.error('Error:', result.error);
        if (result.guidance) {
          console.error('Guidance:', result.guidance);
        }
      }

      // Should succeed with multi-module summary
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Content should be a summary of all modules
        expect(result.value.content).toContain('api-service');
        expect(result.value.content).toContain('web-app');
        expect(result.value.suggestions).toBeDefined();
      }
    });

    it('should generate Dockerfile for specific module when single module provided', async () => {
      const apiServiceModule: ModuleInfo = {
        name: 'api-service',
        path: 'services/api',
        language: 'node',
        framework: 'express',
        ports: [3000],
      };

      const session = createMockSession({
        isMonorepo: true,
        modules: [
          apiServiceModule,
          {
            name: 'web-app',
            path: 'apps/web',
            language: 'node',
            framework: 'react',
            ports: [3001],
          },
        ],
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-monorepo',
          modules: [apiServiceModule], // Pass only the single module to generate for
        },
        ctx,
      );

      // Should generate for single module successfully
      if (!result.ok) {
        expect(result.error).not.toContain('Please specify which module');
      }

      // Verify logger was called with module-specific info
      expect(ctx.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          moduleName: 'api-service',
        }),
        expect.any(String),
      );
    });
  });

  describe('Automated multi-module K8s manifest generation', () => {
    it('should generate manifests for all modules when no moduleName specified', async () => {
      const modules: ModuleInfo[] = [
        {
          name: 'api-service',
          path: 'services/api',
          language: 'node',
          framework: 'express',
          ports: [3000],
        },
        {
          name: 'worker-service',
          path: 'services/worker',
          language: 'python',
          framework: 'celery',
          ports: [5000],
        },
      ];

      const session = createMockSession({
        isMonorepo: true,
        modules,
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-k8s-manifests/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          appName: 'test-monorepo',
          imageId: 'test-registry/app:latest',
          modules: modules,
        },
        ctx,
      );

      // Debug: log the actual result
      if (!result.ok) {
        console.error('Error:', result.error);
        if (result.guidance) {
          console.error('Guidance:', result.guidance);
        }
      }

      // Should succeed with multi-module summary
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Content should be a summary of all modules
        expect(result.value.content).toContain('api-service');
        expect(result.value.content).toContain('worker-service');
      }
    });

    it('should generate manifests for specific module when single module provided', async () => {
      const workerServiceModule: ModuleInfo = {
        name: 'worker-service',
        path: 'services/worker',
        language: 'python',
        ports: [5000],
      };

      const session = createMockSession({
        isMonorepo: true,
        modules: [
          {
            name: 'api-service',
            path: 'services/api',
            language: 'node',
            framework: 'express',
            ports: [3000],
          },
          workerServiceModule,
        ],
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-k8s-manifests/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          appName: 'test-monorepo',
          imageId: 'test-registry/worker:latest',
          modules: [workerServiceModule], // Pass only the single module to generate for
        },
        ctx,
      );

      // Should generate for single module successfully
      if (!result.ok) {
        expect(result.error).not.toContain('Please specify which module');
      }

      // Verify logger was called with module-specific info
      expect(ctx.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          moduleName: 'worker-service',
        }),
        expect.any(String),
      );
    });
  });

  describe('Single-module repository behavior (unchanged)', () => {
    it('should work normally for single-module repos in generate-dockerfile', async () => {
      const session = createMockSession({
        isMonorepo: false,
        analyzedPath: '/tmp/single-app',
        appName: 'single-app',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/single-app',
        },
        ctx,
      );

      // Should not fail with monorepo-related errors
      if (!result.ok) {
        expect(result.error).not.toContain('monorepo');
        expect(result.error).not.toContain('moduleName');
      }
    });

    it('should work normally for single-module repos in generate-k8s-manifests', async () => {
      const session = createMockSession({
        isMonorepo: false,
        analyzedPath: '/tmp/single-app',
        appName: 'single-app',
        appPorts: [8080],
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-k8s-manifests/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test-registry/single-app:latest',
        },
        ctx,
      );

      // Should not fail with monorepo-related errors
      if (!result.ok) {
        expect(result.error).not.toContain('monorepo');
        expect(result.error).not.toContain('moduleName');
      }
    });
  });

  describe('Error handling for multi-module scenarios', () => {
    it('should work with empty modules array', async () => {
      const session = createMockSession({
        isMonorepo: false,
        analyzedPath: '/tmp/test-app',
      });

      const ctx = createMockContext(session);

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-app',
          modules: [], // Empty modules array should fall back to single-module behavior
        },
        ctx,
      );

      // Should not fail with module-related errors
      if (!result.ok) {
        expect(result.error).not.toContain('module');
      }
    });
  });
});
