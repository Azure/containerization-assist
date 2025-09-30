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
import type { ToolContext } from '@/mcp/context';

// Mock logger factory
function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

// Mock session with full module support
function createMockSession(initialData?: Record<string, any>) {
  const storage = new Map<string, any>(Object.entries(initialData || {}));

  return {
    get: jest.fn(<T>(key: string): T | undefined => storage.get(key)),
    set: jest.fn((key: string, value: any) => {
      storage.set(key, value);
    }),
    getResult: jest.fn(<T>(key: string): T | undefined => storage.get(key)),
    storeResult: jest.fn((key: string, value: any) => {
      storage.set(key, value);
    }),
  } as any;
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

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-monorepo',
        },
        ctx,
      );

      // Should succeed with multi-module summary
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Content should be a summary of all modules
        expect(result.value.content).toContain('api-service');
        expect(result.value.content).toContain('web-app');
        expect(result.value.suggestions).toBeDefined();
      }

      // Verify session stores multi-module results
      expect(ctx.session.storeResult).toHaveBeenCalledWith(
        'generate-dockerfile-multi',
        expect.objectContaining({
          modules: expect.any(Array),
          dockerfiles: expect.any(Array),
        }),
      );
    });

    it('should generate Dockerfile for specific module when moduleName provided', async () => {
      const modules: ModuleInfo[] = [
        {
          name: 'api-service',
          path: 'services/api',
          language: 'node',
          framework: 'express',
          ports: [3000],
        },
        {
          name: 'web-app',
          path: 'apps/web',
          language: 'node',
          framework: 'react',
          ports: [3001],
        },
      ];

      const session = createMockSession({
        isMonorepo: true,
        modules,
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-monorepo',
          moduleName: 'api-service',
        },
        ctx,
      );

      // Should not generate for all - only for specified module
      if (!result.ok) {
        // Error should not be about "specify moduleName"
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

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      const tool = await import('@/tools/generate-k8s-manifests/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test-registry/app:latest',
        },
        ctx,
      );

      // Should succeed with multi-module summary
      expect(result.ok).toBe(true);

      if (result.ok) {
        // Content should be a summary of all modules
        expect(result.value.content).toContain('api-service');
        expect(result.value.content).toContain('worker-service');
      }

      // Verify session stores multi-module results
      expect(ctx.session.storeResult).toHaveBeenCalledWith(
        'generate-k8s-manifests-multi',
        expect.objectContaining({
          modules: expect.any(Array),
          manifests: expect.any(Array),
        }),
      );
    });

    it('should generate manifests for specific module when moduleName provided', async () => {
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
          ports: [5000],
        },
      ];

      const session = createMockSession({
        isMonorepo: true,
        modules,
        analyzedPath: '/tmp/test-monorepo',
        appName: 'test-monorepo',
      });

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      const tool = await import('@/tools/generate-k8s-manifests/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test-registry/worker:latest',
          moduleName: 'worker-service',
        },
        ctx,
      );

      // Should not generate for all - only for specified module
      if (!result.ok) {
        // Error should not be about "specify moduleName"
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

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

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

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

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
    it('should return error when invalid moduleName specified', async () => {
      const modules: ModuleInfo[] = [
        { name: 'service-a', path: 'services/a', language: 'javascript', ports: [3000] },
      ];

      const session = createMockSession({
        isMonorepo: true,
        modules,
        analyzedPath: '/tmp/test-monorepo',
      });

      const ctx: ToolContext = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      const tool = await import('@/tools/generate-dockerfile/tool');
      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          moduleName: 'nonexistent-module',
        },
        ctx,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.error).toContain('Available modules');
      }
    });
  });
});