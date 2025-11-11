/**
 * Unit Tests: Generate Dockerfile Tool
 * Tests the generate-dockerfile tool functionality with error scenarios
 */

import { jest } from '@jest/globals';
import { promises as fs } from 'node:fs';
import type { ToolContext } from '@/mcp/context';

// Mock filesystem
jest.mock('node:fs', () => ({
  promises: {
    readFile: jest.fn(),
    access: jest.fn(),
    stat: jest.fn(),
    constants: {
      R_OK: 4,
      W_OK: 2,
      X_OK: 1,
      F_OK: 0,
    },
  },
  constants: {
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,
    F_OK: 0,
  },
}));

// Mock validation library
jest.mock('@/lib/validation', () => ({
  validatePath: jest.fn().mockImplementation(async (pathStr: string, options: any) => {
    // Default: return success
    return { ok: true, value: pathStr };
  }),
  validateImageName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateK8sName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateNamespace: jest.fn().mockImplementation((ns: string) => ({ ok: true, value: ns })),
}));

// Mock validation-helpers
jest.mock('@/lib/validation-helpers', () => ({
  validatePathOrFail: jest.fn().mockImplementation(async (...args: any[]) => {
    const { validatePath } = require('@/lib/validation');
    return validatePath(...args);
  }),
}));

// Mock knowledge loader
const mockGetKnowledgeForCategory = jest.fn();
jest.mock('@/knowledge', () => ({
  getKnowledgeForCategory: mockGetKnowledgeForCategory,
}));

// Mock logger
jest.mock('@/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  })),
}));

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

function createMockToolContext(): ToolContext {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import after mocks are set up
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import type { GenerateDockerfileParams } from '@/tools/generate-dockerfile/schema';

const mockFs = fs as jest.Mocked<typeof fs>;

describe('generate-dockerfile', () => {
  let mockContext: ToolContext;
  let config: GenerateDockerfileParams;

  beforeEach(() => {
    mockContext = createMockToolContext();
    config = {
      repositoryPath: '/test/repo',
      language: 'node',
      framework: 'express',
      environment: 'production',
      targetPlatform: 'linux/amd64',
    };

    // Reset all mocks
    jest.clearAllMocks();

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => false, isDirectory: () => true } as any);
    mockFs.readFile.mockRejectedValue(new Error('ENOENT: no such file'));

    // Default knowledge matches
    mockGetKnowledgeForCategory.mockReturnValue([
      {
        id: 'base-image-1',
        text: 'FROM node:18-alpine\nUse official Node.js image',
        category: 'base-image',
        tags: ['base-image', 'node', 'alpine'],
        weight: 0.9,
      },
      {
        id: 'security-1',
        text: 'Use non-root user for security\nUSER node',
        category: 'security',
        tags: ['security', 'best-practice'],
        weight: 0.85,
      },
    ]);
  });

  describe('Happy Path', () => {
    it('should generate Dockerfile plan for Node.js project', async () => {
      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.repositoryInfo).toBeDefined();
        expect(result.value.recommendations).toBeDefined();
        expect(result.value.recommendations.buildStrategy).toBeDefined();
        expect(result.value.recommendations.baseImages).toBeDefined();
        expect(result.value.recommendations.securityConsiderations).toBeDefined();
        expect(result.value.nextAction).toBeDefined();
        expect(result.value.nextAction.action).toBe('create-files');
        expect(result.value.summary).toContain('ACTION REQUIRED');
        expect(result.value.summary).toContain('Create Dockerfile');
      }
    });

    it('should detect existing Dockerfile and provide enhancement guidance', async () => {
      const existingDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(existingDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.existingDockerfile).toBeDefined();
        expect(result.value.existingDockerfile?.analysis).toBeDefined();
        expect(result.value.existingDockerfile?.guidance).toBeDefined();
        expect(result.value.nextAction).toBeDefined();
        expect(result.value.nextAction.action).toBe('update-files');
        expect(result.value.summary).toContain('ACTION REQUIRED');
        expect(result.value.summary).toContain('Update Dockerfile');
      }
    });

    it('should recommend multi-stage build for Java projects', async () => {
      config.language = 'java';
      config.framework = 'spring-boot';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.buildStrategy.multistage).toBe(true);
        expect(result.value.summary).toContain('Multi-stage');
      }
    });
  });

  describe('Error Handling', () => {
    it('should fail when repository path is not provided', async () => {
      config.repositoryPath = '';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Path is required');
      }
    });

    it('should fail when repository path does not exist', async () => {
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'Path does not exist: /nonexistent/repo',
        guidance: {
          message: 'Path does not exist: /nonexistent/repo',
          hint: 'The specified path could not be found on the filesystem',
        },
      });

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });

    it('should fail when path is not a directory', async () => {
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'Path is not a directory: /test/file.txt',
        guidance: {
          message: 'Path is not a directory: /test/file.txt',
          hint: 'The specified path exists but is a file, not a directory',
        },
      });

      const result = await generateDockerfileTool.handler(
        { ...config, repositoryPath: '/test/file.txt' },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('not a directory');
      }
    });

    it('should handle permission errors when reading directory', async () => {
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'EACCES: permission denied',
        guidance: {
          message: 'EACCES: permission denied',
          hint: 'You do not have permission to access this directory',
        },
      });

      const result = await generateDockerfileTool.handler(
        { ...config, repositoryPath: '/test/restricted' },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeTruthy();
      }
    });

    it('should handle corrupted existing Dockerfile gracefully', async () => {
      // Corrupted Dockerfile content
      mockFs.readFile.mockResolvedValue('INVALID DOCKERFILE CONTENT\x00\x00\x00');

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should handle corrupted Dockerfile and continue
      // Either enhance the corrupted one or create new
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations).toBeDefined();
      }
    });

    it('should handle empty string repository path', async () => {
      const result = await generateDockerfileTool.handler(
        { ...config, repositoryPath: '' },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeTruthy();
      }
    });

    it('should handle missing language parameter', async () => {
      delete config.language;

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should handle missing language (auto-detect)
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.repositoryInfo).toBeDefined();
      }
    });

    it('should handle invalid environment parameter', async () => {
      config.environment = 'invalid-env' as any;

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should still process with invalid environment
      expect(result.ok).toBe(true);
    });

    it('should handle network errors when querying knowledge base', async () => {
      mockGetKnowledgeForCategory.mockImplementation(() => {
        throw new Error('Network error: Unable to fetch knowledge');
      });

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should handle knowledge base errors gracefully
      // Knowledge system errors are typically caught internally
      expect(result).toBeDefined();
    });

    it('should handle empty knowledge base results', async () => {
      mockGetKnowledgeForCategory.mockReturnValue([]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.knowledgeMatches).toBeUndefined();
        expect(result.value.recommendations).toBeDefined();
      }
    });

    it('should handle very long repository paths', async () => {
      const longPath = '/test/' + 'a'.repeat(1000);
      config.repositoryPath = longPath;

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should handle long paths (validation will check)
      expect(result).toBeDefined();
    });

    it('should handle special characters in path', async () => {
      const { validatePath } = await import('@/lib/validation');
      (validatePath as jest.Mock).mockResolvedValueOnce({
        ok: false,
        error: 'Invalid path characters',
      });

      config.repositoryPath = '/test/repo with spaces & special!chars';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
    });
  });

  describe('Existing Dockerfile Analysis', () => {
    it('should analyze multi-stage Dockerfile correctly', async () => {
      const multistageDockerfile = `FROM node:18-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY package*.json ./
RUN npm ci --only=production
USER node
EXPOSE 3000
CMD ["node", "dist/index.js"]`;

      mockFs.readFile.mockResolvedValue(multistageDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.existingDockerfile) {
        expect(result.value.existingDockerfile.analysis.isMultistage).toBe(true);
        expect(result.value.existingDockerfile.analysis.hasNonRootUser).toBe(true);
        expect(result.value.existingDockerfile.analysis.baseImages.length).toBeGreaterThan(1);
      }
    });

    it('should detect missing security features in existing Dockerfile', async () => {
      const insecureDockerfile = `FROM node:latest
COPY . .
RUN npm install
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(insecureDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.existingDockerfile) {
        expect(result.value.existingDockerfile.analysis.hasNonRootUser).toBe(false);
        expect(result.value.existingDockerfile.analysis.hasHealthCheck).toBe(false);
        expect(result.value.existingDockerfile.analysis.securityPosture).not.toBe('good');
      }
    });

    it('should handle Dockerfile read errors gracefully', async () => {
      mockFs.readFile.mockRejectedValue(new Error('EACCES: permission denied'));

      const result = await generateDockerfileTool.handler(config, mockContext);

      // Tool should continue without existing Dockerfile analysis
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.existingDockerfile).toBeUndefined();
        expect(result.value.nextAction.action).toBe('create-files');
        expect(result.value.summary).toContain('ACTION REQUIRED');
        expect(result.value.summary).toContain('Create Dockerfile');
      }
    });
  });

  describe('Build Strategy Recommendations', () => {
    it('should recommend single-stage for Python projects', async () => {
      config.language = 'python';
      config.framework = 'flask';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.buildStrategy.multistage).toBe(false);
      }
    });

    it('should recommend multi-stage for Go projects', async () => {
      config.language = 'go';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.buildStrategy.multistage).toBe(true);
        expect(result.value.recommendations.buildStrategy.reason).toContain('Multi-stage');
      }
    });

    it('should recommend multi-stage for .NET projects', async () => {
      config.language = 'dotnet';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.buildStrategy.multistage).toBe(true);
      }
    });

    it('should recommend multi-stage for Rust projects', async () => {
      config.language = 'rust';

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.buildStrategy.multistage).toBe(true);
      }
    });
  });

  describe('Metadata', () => {
    it('should have correct metadata', () => {
      expect(generateDockerfileTool.version).toBe('2.0.0');
      expect(generateDockerfileTool.metadata.knowledgeEnhanced).toBe(true);
    });

    it('should have chain hints', () => {
      expect(generateDockerfileTool.chainHints.success).toContain('fix-dockerfile');
      expect(generateDockerfileTool.chainHints.failure).toContain('repository analysis');
    });
  });
});
