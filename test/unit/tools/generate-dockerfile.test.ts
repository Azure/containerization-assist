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

// Mock knowledge matcher
jest.mock('@/knowledge/matcher', () => ({
  getKnowledgeSnippets: jest.fn(),
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
import * as knowledgeMatcher from '@/knowledge/matcher';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import type { GenerateDockerfileParams } from '@/tools/generate-dockerfile/schema';

// Spy on getKnowledgeSnippets
const mockGetKnowledgeSnippets = jest.spyOn(knowledgeMatcher, 'getKnowledgeSnippets').mockImplementation(jest.fn());

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

    // Setup default knowledge mock implementation with empty array
    mockGetKnowledgeSnippets.mockReturnValue([]);

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => false, isDirectory: () => true } as any);
    mockFs.readFile.mockRejectedValue(new Error('ENOENT: no such file'));
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

    it('should handle empty knowledge base results', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([]);

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

  describe('Base Image Recommendations', () => {
    it('should categorize distroless images correctly', async () => {
      // Mock knowledge with base-image tag to trigger categorization
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-distroless',
          text: 'FROM gcr.io/distroless/nodejs18-debian11 Use distroless image for minimal attack surface',
          category: 'base-image',
          tags: ['base-image', 'distroless', 'node'], // base-image tag triggers baseImages categorization
          weight: 0.95,
          source: 'base-image-distroless',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.baseImages).toBeDefined();
        const distrolessImage = result.value.recommendations.baseImages?.find(
          (img) => img.category === 'distroless',
        );
        expect(distrolessImage).toBeDefined();
      }
    });

    it('should categorize security images correctly (chainguard)', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-chainguard',
          text: 'FROM cgr.dev/chainguard/node:latest Use Chainguard hardened image for enhanced security',
          category: 'base-image',
          tags: ['base-image', 'security', 'node'],
          weight: 0.92,
          source: 'base-image-chainguard',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const securityImage = result.value.recommendations.baseImages?.find(
          (img) => img.category === 'security',
        );
        expect(securityImage).toBeDefined();
        expect(securityImage?.image).toContain('chainguard');
      }
    });

    it('should categorize security images correctly (wolfi)', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-wolfi',
          text: 'FROM cgr.dev/chainguard/wolfi-base:latest Use Wolfi-based image for security',
          category: 'base-image',
          tags: ['base-image', 'security', 'wolfi'],
          weight: 0.91,
          source: 'base-image-wolfi',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const securityImage = result.value.recommendations.baseImages?.find(
          (img) => img.category === 'security',
        );
        expect(securityImage).toBeDefined();
      }
    });

    it('should categorize size-optimized images (alpine)', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-alpine',
          text: 'FROM node:18-alpine Use Alpine Linux for smaller image size (50MB)',
          category: 'base-image',
          tags: ['base-image', 'alpine', 'node'],
          weight: 0.88,
          source: 'base-image-alpine',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const alpineImage = result.value.recommendations.baseImages?.find(
          (img) => img.category === 'size',
        );
        expect(alpineImage).toBeDefined();
        expect(alpineImage?.size).toBe('50MB');
      }
    });

    it('should categorize size-optimized images (slim)', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-slim',
          text: 'FROM node:18-slim Use slim variant for reduced size (200 MB)',
          category: 'base-image',
          tags: ['base-image', 'slim', 'node'],
          weight: 0.87,
          source: 'base-image-slim',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const slimImage = result.value.recommendations.baseImages?.find((img) =>
          img.image.includes('slim'),
        );
        expect(slimImage?.category).toBe('size');
        expect(slimImage?.size).toBe('200MB');
      }
    });

    it('should substitute version in runtime images', async () => {
      config.languageVersion = '20';
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-node',
          text: 'FROM node:18-alpine Use Node.js Alpine image',
          category: 'base-image',
          tags: ['base-image', 'node', 'alpine'],
          weight: 0.9,
          source: 'base-image-node',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.recommendations.baseImages?.[0]) {
        expect(result.value.recommendations.baseImages[0].image).toBe('node:20-alpine');
      }
    });

    it('should substitute version in maven/gradle images', async () => {
      config.language = 'java';
      config.languageVersion = '21';
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-maven',
          text: 'FROM maven:3.9-openjdk-17 Use Maven with OpenJDK',
          category: 'base-image',
          tags: ['base-image', 'java', 'maven'],
          weight: 0.9,
          source: 'base-image-maven',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.recommendations.baseImages?.[0]) {
        expect(result.value.recommendations.baseImages[0].image).toBe('maven:3.9-openjdk-21');
      }
    });

    it('should substitute version in gradle images', async () => {
      config.language = 'java';
      config.languageVersion = '21';
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-gradle',
          text: 'FROM gradle:8.5-jdk-17-alpine Use Gradle with JDK',
          category: 'base-image',
          tags: ['base-image', 'java', 'gradle'],
          weight: 0.89,
          source: 'base-image-gradle',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.recommendations.baseImages?.[0]) {
        expect(result.value.recommendations.baseImages[0].image).toBe(
          'gradle:8.5-jdk-21-alpine',
        );
      }
    });

    it('should handle images with no version to substitute', async () => {
      config.languageVersion = '20';
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-image-custom',
          text: 'FROM custom-node-image:latest Use custom image',
          category: 'base-image',
          tags: ['base-image', 'node'],
          weight: 0.7,
          source: 'base-image-custom',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.recommendations.baseImages?.[0]) {
        // Should remain unchanged if no version pattern found
        expect(result.value.recommendations.baseImages[0].image).toBe('custom-node-image:latest');
      }
    });

    it('should limit base images to top 2', async () => {
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-1',
          text: 'FROM node:18-alpine',
          category: 'base-image',
          tags: ['base-image'],
          weight: 0.95,
          source: 'base-1',
        },
        {
          id: 'base-2',
          text: 'FROM node:18-slim',
          category: 'base-image',
          tags: ['base-image'],
          weight: 0.9,
          source: 'base-2',
        },
        {
          id: 'base-3',
          text: 'FROM node:18',
          category: 'base-image',
          tags: ['base-image'],
          weight: 0.85,
          source: 'base-3',
        },
        {
          id: 'base-4',
          text: 'FROM node:18-bullseye',
          category: 'base-image',
          tags: ['base-image'],
          weight: 0.8,
          source: 'base-4',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.baseImages?.length).toBeLessThanOrEqual(2);
      }
    });
  });

  describe('Knowledge Filtering and Limiting', () => {
    it('should limit security recommendations to top 5', async () => {
      const securitySnippets = Array.from({ length: 10 }, (_, i) => ({
        id: `security-${i}`,
        text: `Security recommendation ${i}`,
        category: 'security',
        tags: ['security'],
        weight: 0.9 - i * 0.05,
        source: `security-${i}`,
      }));

      mockGetKnowledgeSnippets.mockReturnValue(securitySnippets);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(
          result.value.recommendations.securityConsiderations?.length,
        ).toBeLessThanOrEqual(5);
      }
    });

    it('should limit optimizations to top 5', async () => {
      const optimizationSnippets = Array.from({ length: 10 }, (_, i) => ({
        id: `optimization-${i}`,
        text: `Optimization ${i}`,
        category: 'optimization',
        tags: ['optimization'],
        weight: 0.85 - i * 0.05,
        source: `optimization-${i}`,
      }));

      mockGetKnowledgeSnippets.mockReturnValue(optimizationSnippets);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.optimizations?.length).toBeLessThanOrEqual(5);
      }
    });

    it('should limit best practices to top 5', async () => {
      const bestPracticeSnippets = Array.from({ length: 10 }, (_, i) => ({
        id: `best-practice-${i}`,
        text: `Best practice ${i}`,
        category: 'best-practice',
        tags: ['best-practice'],
        weight: 0.8 - i * 0.05,
        source: `best-practice-${i}`,
      }));

      mockGetKnowledgeSnippets.mockReturnValue(bestPracticeSnippets);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.recommendations.bestPractices?.length).toBeLessThanOrEqual(5);
      }
    });
  });

  describe('Enhancement Guidance Strategy', () => {
    it('should provide targeted guidance for Dockerfile with poor security', async () => {
      const poorSecurityDockerfile = `FROM node:latest
COPY . .
RUN npm install
EXPOSE 3000
CMD ["npm", "start"]`;

      mockFs.readFile.mockResolvedValue(poorSecurityDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.existingDockerfile) {
        expect(result.value.existingDockerfile.analysis.securityPosture).toBe('poor');
        expect(result.value.existingDockerfile.guidance).toBeDefined();
        // Strategy should be 'major-overhaul' for poor security
        expect(result.value.existingDockerfile.guidance?.strategy).toBe('major-overhaul');
      }
    });

    it('should provide minimal guidance for well-structured Dockerfile', async () => {
      const wellStructuredDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER node
HEALTHCHECK CMD node healthcheck.js
EXPOSE 3000
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(wellStructuredDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.existingDockerfile) {
        expect(result.value.existingDockerfile.analysis.securityPosture).toBe('good');
        // Strategy should be 'minor-tweaks' for well-structured Dockerfile
        expect(result.value.existingDockerfile.guidance?.strategy).toBe('minor-tweaks');
      }
    });

    it('should handle multi-stage Dockerfile with existing guidance', async () => {
      const multistageWithIssues = `FROM node:18 AS builder
WORKDIR /app
COPY . .
RUN npm install
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
CMD ["node", "dist/index.js"]`;

      mockFs.readFile.mockResolvedValue(multistageWithIssues);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok && result.value.existingDockerfile) {
        expect(result.value.existingDockerfile.analysis.isMultistage).toBe(true);
        expect(result.value.existingDockerfile.analysis.hasNonRootUser).toBe(false);
        expect(result.value.existingDockerfile.guidance).toBeDefined();
      }
    });
  });

  describe('NextAction Instructions', () => {
    it('should provide create-files action when no Dockerfile exists', async () => {
      mockFs.readFile.mockRejectedValue(new Error('ENOENT'));

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.action).toBe('create-files');
        expect(result.value.nextAction.instruction).toBeDefined();
        expect(result.value.nextAction.instruction.length).toBeGreaterThan(0);
      }
    });

    it('should provide update-files action when Dockerfile exists', async () => {
      const existingDockerfile = 'FROM node:18\nCOPY . .\nCMD ["node", "index.js"]';
      mockFs.readFile.mockResolvedValue(existingDockerfile);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.action).toBe('update-files');
        expect(result.value.nextAction.instruction).toBeDefined();
      }
    });

    it('should include base image selection in instructions', async () => {
      mockFs.readFile.mockRejectedValue(new Error('ENOENT'));
      mockGetKnowledgeSnippets.mockReturnValue([
        {
          id: 'base-1',
          text: 'FROM node:18-alpine',
          category: 'base-image',
          tags: ['base-image'],
          weight: 0.9,
          source: 'base-1',
        },
      ]);

      const result = await generateDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.instruction).toContain('base image');
      }
    });
  });
});
