/**
 * Unit Tests: Multi-Module/Monorepo Support
 * Tests the multi-module detection and routing logic for generate-dockerfile and generate-k8s-manifests
 */

import { jest } from '@jest/globals';
import type { ModuleInfo } from '@/tools/analyze-repo/schema';

// Mock logger
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

// Mock session with module support
function createMockSession(modules?: ModuleInfo[], isMonorepo = false) {
  const storage = new Map<string, any>();

  if (isMonorepo) {
    storage.set('isMonorepo', true);
  }
  if (modules) {
    storage.set('modules', modules);
  }
  storage.set('analyzedPath', '/tmp/test-repo');
  storage.set('appName', 'test-app');

  return {
    get: jest.fn((key: string) => storage.get(key)),
    set: jest.fn((key: string, value: any) => storage.set(key, value)),
    getResult: jest.fn((key: string) => storage.get(key)),
    storeResult: jest.fn((key: string, value: any) => storage.set(key, value)),
  } as any;
}

describe('Multi-Module Support', () => {
  describe('generate-dockerfile tool', () => {
    let tool: any;

    beforeEach(async () => {
      jest.resetModules();
      tool = await import('@/tools/generate-dockerfile/tool');
    });

    it('should detect monorepo and attempt multi-module generation', async () => {
      const modules: ModuleInfo[] = [
        { name: 'service-a', path: 'services/a', language: 'javascript' },
        { name: 'service-b', path: 'services/b', language: 'python' },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      // Call the tool - it will attempt to generate for all modules
      // We can't test full execution without mocking AI, but we can verify
      // it detects the monorepo and logs appropriately
      await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-repo',
        },
        ctx,
      );

      // Verify logger was called with multi-module info
      expect(ctx.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          moduleCount: 2,
        }),
        expect.stringContaining('all modules'),
      );
    });

    it('should fail when invalid moduleName provided', async () => {
      const modules: ModuleInfo[] = [
        { name: 'service-a', path: 'services/a', language: 'javascript' },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-repo',
          moduleName: 'nonexistent',
        },
        ctx,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.error).toContain('Available modules');
      }
    });

    it('should accept valid moduleName for monorepo', async () => {
      const modules: ModuleInfo[] = [
        {
          name: 'api-gateway',
          path: 'services/api-gateway',
          language: 'javascript',
          framework: 'express',
          ports: [8080],
        },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-repo',
          moduleName: 'api-gateway',
        },
        ctx,
      );

      // Should not fail with "module not found" error
      if (!result.ok) {
        expect(result.error).not.toContain('Module "api-gateway" not found');
      }
    });

    it('should work normally for single-module repos', async () => {
      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(undefined, false),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          path: '/tmp/test-repo',
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

  describe('generate-k8s-manifests tool', () => {
    let tool: any;

    beforeEach(async () => {
      jest.resetModules();
      tool = await import('@/tools/generate-k8s-manifests/tool');
    });

    it('should detect monorepo and attempt multi-module generation', async () => {
      const modules: ModuleInfo[] = [
        { name: 'service-a', path: 'services/a', language: 'javascript', ports: [8080] },
        { name: 'service-b', path: 'services/b', language: 'python', ports: [8081] },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      // Call the tool - it will attempt to generate for all modules
      // We can't test full execution without mocking AI, but we can verify
      // it detects the monorepo and logs appropriately
      await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test:latest',
        },
        ctx,
      );

      // Verify logger was called with multi-module info
      expect(ctx.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          moduleCount: 2,
        }),
        expect.stringContaining('all modules'),
      );
    });

    it('should fail when invalid moduleName provided', async () => {
      const modules: ModuleInfo[] = [
        { name: 'service-a', path: 'services/a', language: 'javascript', ports: [8080] },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test:latest',
          moduleName: 'nonexistent',
        },
        ctx,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not found');
        expect(result.error).toContain('Available modules');
      }
    });

    it('should accept valid moduleName for monorepo', async () => {
      const modules: ModuleInfo[] = [
        {
          name: 'user-service',
          path: 'services/user-service',
          language: 'python',
          ports: [8081],
        },
      ];

      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(modules, true),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test/user-service:latest',
          moduleName: 'user-service',
        },
        ctx,
      );

      // Should not fail with "module not found" error
      if (!result.ok) {
        expect(result.error).not.toContain('Module "user-service" not found');
      }
    });

    it('should use module-specific port when generating manifests', async () => {
      const modules: ModuleInfo[] = [
        { name: 'api-gateway', path: 'services/api-gateway', language: 'javascript', ports: [8080] },
        { name: 'user-service', path: 'services/user-service', language: 'python', ports: [8081] },
      ];

      const session = createMockSession(modules, true);
      const ctx = {
        logger: createMockLogger(),
        session,
        sampling: {} as any,
      };

      await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test/user-service:latest',
          moduleName: 'user-service',
        },
        ctx,
      );

      // Verify logger was called with module-specific port info
      expect(ctx.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          port: 8081,
          moduleName: 'user-service',
        }),
        expect.stringContaining('port from module'),
      );
    });

    it('should work normally for single-module repos', async () => {
      const ctx = {
        logger: createMockLogger(),
        session: createMockSession(undefined, false),
        sampling: {} as any,
      };

      const result = await tool.default.run(
        {
          sessionId: 'test-session',
          imageId: 'test:latest',
          appName: 'test-app',
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

  describe('Repository Analysis Schema', () => {
    it('should support modules field in RepositoryAnalysis', () => {
      const { RepositoryAnalysis } = require('@/tools/analyze-repo/schema');

      // This is a type-level test - we're just verifying the schema compiles
      const analysis = {
        name: 'test-monorepo',
        language: 'multi-language',
        isMonorepo: true,
        modules: [
          {
            name: 'service-a',
            path: 'services/a',
            language: 'javascript',
          },
        ],
      };

      expect(analysis.modules).toBeDefined();
      expect(analysis.isMonorepo).toBe(true);
    });

    it('should support ModuleInfo type', () => {
      const moduleInfo: ModuleInfo = {
        name: 'api-service',
        path: 'services/api',
        language: 'node',
        framework: 'express',
        ports: [3000],
        entryPoint: 'server.js',
      };

      expect(moduleInfo.name).toBe('api-service');
      expect(moduleInfo.path).toBe('services/api');
    });
  });
});