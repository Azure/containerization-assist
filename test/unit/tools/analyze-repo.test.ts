/**
 * Unit tests for analyze-repo tool (deterministic version)
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import type { ToolContext } from '@/mcp/context';

// Mock the validation library to bypass path validation in tests
jest.mock('@/lib/validation', () => ({
  validatePath: jest.fn().mockImplementation(async (pathStr: string, options: any) => {
    // Always return success with the path for tests
    return { ok: true, value: pathStr };
  }),
  validateImageName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateK8sName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateNamespace: jest.fn().mockImplementation((ns: string) => ({ ok: true, value: ns })),
}));

// Mock dependencies - fs/promises
jest.mock('node:fs/promises', () => ({
  stat: jest.fn(),
  readdir: jest.fn(),
  readFile: jest.fn(),
}));

// Mock node:fs to support 'import { promises as fs } from node:fs'
jest.mock('node:fs', () => ({
  promises: {
    stat: jest.fn(),
    readdir: jest.fn(),
    readFile: jest.fn(),
  },
}));

// Import the mocked fs after setting up the mock
import { promises as fs } from 'node:fs';
import analyzeTool from '@/tools/analyze-repo/tool';

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
      // Mock filesystem operations for both node:fs and node:fs/promises
      const statMock = jest.fn().mockResolvedValue({
        isDirectory: () => true,
        isFile: () => false
      });
      const readdirMock = jest.fn().mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false, isFile: () => true },
            { name: 'src', isDirectory: () => true, isFile: () => false },
            { name: 'README.md', isDirectory: () => false, isFile: () => true },
          ]);
        }
        if (dirPath.includes('/test/repo/src')) {
          return Promise.resolve([{ name: 'index.js', isDirectory: () => false, isFile: () => true }]);
        }
        return Promise.resolve([]);
      });
      const readFileMock = jest.fn().mockImplementation((filePath: string) => {
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

      (fs.stat as jest.Mock).mockImplementation(statMock);
      (fs.readdir as jest.Mock).mockImplementation(readdirMock);
      (fs.readFile as jest.Mock).mockImplementation(readFileMock);

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/repo',
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
      const statMock = jest.fn().mockResolvedValue({
        isDirectory: () => true,
        isFile: () => false
      });
      const readdirMock = jest.fn().mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'pom.xml', isDirectory: () => false, isFile: () => true },
            { name: 'src', isDirectory: () => true, isFile: () => false },
          ]);
        }
        return Promise.resolve([]);
      });
      const readFileMock = jest.fn().mockImplementation((filePath: string) => {
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

      (fs.stat as jest.Mock).mockImplementation(statMock);
      (fs.readdir as jest.Mock).mockImplementation(readdirMock);
      (fs.readFile as jest.Mock).mockImplementation(readFileMock);

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/repo',
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
      const statMock = jest.fn().mockResolvedValue({
        isDirectory: () => true,
        isFile: () => false
      });
      const readdirMock = jest.fn().mockImplementation((dirPath: string) => {
        if (dirPath === '/test/repo') {
          return Promise.resolve([
            { name: 'api', isDirectory: () => true, isFile: () => false },
            { name: 'worker', isDirectory: () => true, isFile: () => false },
          ]);
        }
        if (dirPath.includes('/test/repo/api')) {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false, isFile: () => true },
            { name: 'src', isDirectory: () => true, isFile: () => false },
          ]);
        }
        if (dirPath.includes('/test/repo/worker')) {
          return Promise.resolve([
            { name: 'package.json', isDirectory: () => false, isFile: () => true },
            { name: 'src', isDirectory: () => true, isFile: () => false },
          ]);
        }
        return Promise.resolve([]);
      });
      const readFileMock = jest.fn().mockImplementation((filePath: string) => {
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

      (fs.stat as jest.Mock).mockImplementation(statMock);
      (fs.readdir as jest.Mock).mockImplementation(readdirMock);
      (fs.readFile as jest.Mock).mockImplementation(readFileMock);

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/repo',
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
      const statMock = jest.fn().mockResolvedValue({
        isDirectory: () => true,
        isFile: () => false
      });
      (fs.stat as jest.Mock).mockImplementation(statMock);

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/repo',
          modules: [{
            name: 'my-service',
            modulePath: '/test/repo',
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
      // Mock validation to fail for this test
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'Path is not a directory: /test/file.txt',
        guidance: {
          message: 'Path is not a directory: /test/file.txt',
          hint: 'The specified path exists but is a file, not a directory',
        }
      });

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/file.txt',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a directory');
      }
    });

    it('should fail if no modules detected in repository', async () => {
      const statMock = jest.fn().mockResolvedValue({
        isDirectory: () => true,
        isFile: () => false
      });
      const readdirMock = jest.fn().mockImplementation((dirPath: string) => {
        return Promise.resolve([
          { name: 'README.md', isDirectory: () => false, isFile: () => true },
          { name: 'LICENSE', isDirectory: () => false, isFile: () => true },
        ]);
      });
      (fs.stat as jest.Mock).mockImplementation(statMock);
      (fs.readdir as jest.Mock).mockImplementation(readdirMock);

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/test/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No modules detected');
      }
    });

    it('should fail if repository does not exist', async () => {
      // Mock validation to fail for this test
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'Path does not exist: /nonexistent/repo',
        guidance: {
          message: 'Path does not exist: /nonexistent/repo',
          hint: 'The specified path could not be found on the filesystem',
        }
      });

      const result = await analyzeTool.handler(
        {
          repositoryPath: '/nonexistent/repo',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });
  });

  describe('Metadata', () => {
    it('should have correct metadata for v4.0.0 deterministic version', () => {
      expect(analyzeTool.version).toBe('4.0.0');
      expect(analyzeTool.metadata.knowledgeEnhanced).toBe(false);
    });
  });
});
