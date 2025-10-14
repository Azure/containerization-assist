/**
 * Unit tests for analyze-repo tool (deterministic version)
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import analyzeTool from '@/tools/analyze-repo/tool';
import type { ToolContext } from '@/mcp/context';

// Mock dependencies
jest.mock('node:fs', () => ({
  promises: {
    stat: jest.fn(),
    readdir: jest.fn(),
    readFile: jest.fn(),
  },
}));

describe('analyze-repo tool (v4.0.0 - deterministic)', () => {
  let mockContext: ToolContext;

  beforeEach(() => {
    jest.clearAllMocks();

    mockContext = {
      logger: {
        info: jest.fn(),
        debug: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
      },
    } as unknown as ToolContext;
  });

  describe('Deterministic parsing', () => {
    it('should parse Node.js project with package.json', async () => {
      // Mock filesystem operations
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => true });
      (fs.readdir as jest.Mock).mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false },
            { name: 'src', isDirectory: () => true },
            { name: 'README.md', isDirectory: () => false },
          ]);
        }
        if (dirPath.includes('/test/repo/src')) {
          return Promise.resolve([{ name: 'index.js', isDirectory: () => false }]);
        }
        return Promise.resolve([]);
      });
      (fs.readFile as jest.Mock).mockImplementation((filePath: string) => {
        if (filePath.includes('package.json')) {
          return Promise.resolve(JSON.stringify({
            name: 'test-app',
            version: '1.0.0',
            dependencies: { express: '^4.18.0' },
            scripts: { start: 'node index.js' },
            engines: { node: '18.x' },
          }));
        }
        return Promise.reject(new Error('File not found'));
      });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.modules).toHaveLength(1);
        expect(result.value.modules?.[0].name).toBe('repo');
        expect(result.value.modules?.[0].frameworks?.[0]?.name).toBe('express');
        expect(result.value.modules?.[0].languageVersion).toBe('18.x');
        expect(result.value.modules?.[0].ports).toContain(3000);
        expect(result.value.modules?.[0].buildSystem?.type).toBe('npm');
        expect(result.value.isMonorepo).toBe(false);
      }
    });

    it('should parse Java Maven project with pom.xml', async () => {
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => true });
      (fs.readdir as jest.Mock).mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'pom.xml', isDirectory: () => false },
            { name: 'src', isDirectory: () => true },
          ]);
        }
        return Promise.resolve([]);
      });
      (fs.readFile as jest.Mock).mockImplementation((filePath: string) => {
        if (filePath.includes('pom.xml')) {
          return Promise.resolve(`<?xml version="1.0"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <artifactId>test-service</artifactId>
  <version>1.0.0</version>
  <properties>
    <java.version>17</java.version>
  </properties>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>
</project>`);
        }
        return Promise.reject(new Error('File not found'));
      });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.modules).toHaveLength(1);
        expect(result.value.modules?.[0].language).toBe('java');
        expect(result.value.modules?.[0].frameworks?.[0]?.name).toBe('spring-boot');
        expect(result.value.modules?.[0].languageVersion).toBe('17');
        expect(result.value.modules?.[0].ports).toContain(8080);
        expect(result.value.modules?.[0].buildSystem?.type).toBe('maven');
      }
    });

    it('should detect monorepo with multiple config files', async () => {
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => true });
      (fs.readdir as jest.Mock).mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'api', isDirectory: () => true },
            { name: 'worker', isDirectory: () => true },
          ]);
        }
        if (dirPath.includes('/test/repo/api')) {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false },
            { name: 'src', isDirectory: () => true },
          ]);
        }
        if (dirPath.includes('/test/repo/worker')) {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false },
            { name: 'src', isDirectory: () => true },
          ]);
        }
        return Promise.resolve([]);
      });
      (fs.readFile as jest.Mock).mockImplementation((filePath: string) => {
        if (filePath.includes('api/package.json')) {
          return Promise.resolve(JSON.stringify({
            name: 'api',
            dependencies: { express: '^4.0.0' },
          }));
        }
        if (filePath.includes('worker/package.json')) {
          return Promise.resolve(JSON.stringify({
            name: 'worker',
            dependencies: { amqplib: '^0.10.0' },
          }));
        }
        return Promise.reject(new Error('File not found'));
      });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.modules.length).toBeGreaterThan(1);
        expect(result.value.isMonorepo).toBe(true);
      }
    });
  });

  describe('Legacy mode with pre-provided modules', () => {
    it('should use pre-provided modules without AI analysis', async () => {
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => true });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/repo',
          modules: [{
            name: 'my-service',
            modulePathAbsoluteUnix: '/test/repo',
            language: 'java',
          }],
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.modules).toHaveLength(1);
        expect(result.value.modules?.[0].name).toBe('my-service');
      }
    });
  });

  describe('Error handling', () => {
    it('should fail if path is not a directory', async () => {
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => false });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/file.txt',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a directory');
      }
    });

    it('should fail if no modules detected in repository', async () => {
      (fs.stat as jest.Mock).mockResolvedValue({ isDirectory: () => true });
      (fs.readdir as jest.Mock).mockImplementation(() => {
        return Promise.resolve([
          { name: 'README.md', isDirectory: () => false },
          { name: 'LICENSE', isDirectory: () => false },
        ]);
      });

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/test/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No modules detected');
      }
    });

    it('should fail if repository does not exist', async () => {
      (fs.stat as jest.Mock).mockRejectedValue(new Error('ENOENT: no such file'));

      const result = await analyzeTool.run(
        {
          sessionId: 'test-session',
          repositoryPathAbsoluteUnix: '/nonexistent/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('failed');
      }
    });
  });

  describe('Metadata', () => {
    it('should have correct metadata for v4.0.0 deterministic version', () => {
      expect(analyzeTool.version).toBe('4.0.0');
      expect(analyzeTool.metadata.knowledgeEnhanced).toBe(false);
      expect(analyzeTool.metadata.samplingStrategy).toBe('none');
      expect(analyzeTool.metadata.enhancementCapabilities).toContain('analysis');
    });
  });
});
