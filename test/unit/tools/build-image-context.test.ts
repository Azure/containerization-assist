/**
 * Unit Tests: Build Image Context Tool
 * Tests the build-image-context tool - context preparation and security analysis
 */

import { jest } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { createMockValidatePath } from '../../__support__/utilities/mocks';

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

// Mock the validation library to bypass path validation in tests
jest.mock('../../../src/lib/validation', () => ({
  validatePath: createMockValidatePath(),
  validateImageName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateK8sName: jest.fn().mockImplementation((name: string) => ({ ok: true, value: name })),
  validateNamespace: jest.fn().mockImplementation((ns: string) => ({ ok: true, value: ns })),
}));

// Mock validation-helpers
jest.mock('../../../src/lib/validation-helpers', () => ({
  validatePathOrFail: jest.fn().mockImplementation(async (...args: any[]) => {
    const { validatePath } = require('../../../src/lib/validation');
    return validatePath(...args);
  }),
  parseImageName: jest.fn().mockImplementation((imageName: string) => {
    const colonIndex = imageName.lastIndexOf(':');
    if (colonIndex > 0 && !imageName.substring(colonIndex + 1).includes('/')) {
      const imagePath = imageName.substring(0, colonIndex);
      const tag = imageName.substring(colonIndex + 1);
      return {
        ok: true,
        value: {
          repository: imagePath,
          tag: tag || 'latest',
        },
      };
    }
    return {
      ok: true,
      value: {
        repository: imageName,
        tag: 'latest',
      },
    };
  }),
}));

// Mock filesystem
jest.mock('node:fs', () => ({
  promises: {
    access: jest.fn(),
    readFile: jest.fn(),
    writeFile: jest.fn(),
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

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
  createLogger: jest.fn(() => createMockLogger()),
}));

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import after mocks
import { buildImageContext } from '../../../src/tools/build-image-context/tool';
import type { BuildImageParams } from '../../../src/tools/build-image-context/schema';

const mockFs = fs as jest.Mocked<typeof fs>;

describe('buildImageContext', () => {
  let config: BuildImageParams;

  const mockDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
USER appuser
CMD ["node", "index.js"]`;

  beforeEach(() => {
    config = {
      path: '/test/repo',
      dockerfile: 'Dockerfile',
      imageName: 'test-app',
      tags: ['latest', 'v1.0'],
      buildArgs: {},
    };

    jest.clearAllMocks();

    // Default mock implementations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => true, isDirectory: () => false } as any);
    mockFs.readFile.mockResolvedValue(mockDockerfile);
  });

  describe('Context Preparation', () => {
    it('should return build context with validated paths', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.context).toBeDefined();
        expect(result.value.context.buildContextPath).toContain('/test/repo');
        expect(result.value.context.dockerfilePath).toContain('Dockerfile');
        expect(result.value.context.dockerfileRelative).toBe('Dockerfile');
      }
    });

    it('should detect .dockerignore presence', async () => {
      // Mock .dockerignore exists
      mockFs.access.mockResolvedValue(undefined);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.context.hasDockerignore).toBeDefined();
      }
    });

    it('should compute final tags from imageName and tags', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildConfig.finalTags).toContain('test-app:latest');
        expect(result.value.buildConfig.finalTags).toContain('test-app:v1.0');
      }
    });

    it('should handle full tag references in tags array', async () => {
      config.tags = ['registry.io/myapp:prod'];

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildConfig.finalTags).toContain('registry.io/myapp:prod');
      }
    });

    it('should use default tag when no tags provided', async () => {
      config.tags = [];
      config.imageName = 'myapp';

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildConfig.finalTags).toContain('myapp:latest');
      }
    });
  });

  describe('Security Analysis', () => {
    it('should detect secrets in build args', async () => {
      config.buildArgs = {
        API_PASSWORD: 'secret123',
        DB_TOKEN: 'token456',
      };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'secret-in-build-arg')).toBe(true);
        expect(warnings.some((w) => w.message.includes('API_PASSWORD'))).toBe(true);
      }
    });

    it('should detect sudo usage in Dockerfile', async () => {
      const dockerfileWithSudo = `FROM ubuntu:20.04
RUN sudo apt-get update
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithSudo);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'sudo-usage')).toBe(true);
      }
    });

    it('should detect unpinned base images', async () => {
      const dockerfileWithLatest = `FROM node:latest
WORKDIR /app
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithLatest);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'unpinned-base-image')).toBe(true);
      }
    });

    it('should detect missing USER instruction', async () => {
      const dockerfileWithoutUser = `FROM node:18-alpine
WORKDIR /app
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(dockerfileWithoutUser);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'runs-as-root')).toBe(true);
      }
    });

    it('should detect root user directive', async () => {
      const dockerfileWithRoot = `FROM node:18-alpine
USER root
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(dockerfileWithRoot);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'runs-as-root')).toBe(true);
      }
    });

    it('should detect chmod 777', async () => {
      const dockerfileWithChmod = `FROM node:18-alpine
RUN chmod 777 /app
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithChmod);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        expect(warnings.some((w) => w.id === 'overly-permissive-chmod')).toBe(true);
      }
    });

    it('should compute correct risk level', async () => {
      // High risk: multiple high severity warnings
      const highRiskDockerfile = `FROM node
RUN sudo apt-get update
USER root`;

      mockFs.readFile.mockResolvedValue(highRiskDockerfile);
      config.buildArgs = { SECRET_KEY: 'abc123' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(['medium', 'high']).toContain(result.value.securityAnalysis.riskLevel);
      }
    });

    it('should provide remediation for each warning', async () => {
      config.buildArgs = { API_TOKEN: 'secret' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const warnings = result.value.securityAnalysis.warnings;
        warnings.forEach((w) => {
          expect(w.remediation).toBeDefined();
          expect(w.remediation.length).toBeGreaterThan(0);
        });
      }
    });
  });

  describe('BuildKit Analysis', () => {
    it('should detect multi-stage builds', async () => {
      const multiStageDockerfile = `FROM node:18-alpine AS builder
WORKDIR /app
RUN npm ci
FROM node:18-alpine
COPY --from=builder /app/dist ./dist
USER appuser`;

      mockFs.readFile.mockResolvedValue(multiStageDockerfile);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildKitAnalysis.features.multiStage).toBe(true);
        expect(result.value.buildKitAnalysis.features.stageCount).toBe(2);
        expect(result.value.buildKitAnalysis.features.copyFrom).toBe(true);
      }
    });

    it('should detect cache mount usage', async () => {
      const dockerfileWithCache = `FROM node:18-alpine
RUN --mount=type=cache,target=/root/.npm npm ci
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithCache);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildKitAnalysis.features.cacheMount).toBe(true);
        expect(result.value.buildKitAnalysis.recommended).toBe(true);
      }
    });

    it('should detect secret mount usage', async () => {
      const dockerfileWithSecret = `FROM node:18-alpine
RUN --mount=type=secret,id=npmrc npm ci
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithSecret);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildKitAnalysis.features.secretMount).toBe(true);
      }
    });

    it('should recommend BuildKit when features are used', async () => {
      const dockerfileWithBuildKit = `FROM node:18-alpine AS builder
RUN --mount=type=cache,target=/root/.npm npm ci
FROM node:18-alpine
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithBuildKit);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildKitAnalysis.recommended).toBe(true);
      }
    });

    it('should suggest cache mounts for npm', async () => {
      const dockerfileWithNpm = `FROM node:18-alpine
RUN npm install
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithNpm);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const recommendations = result.value.buildKitAnalysis.recommendations;
        expect(recommendations.some((r) => r.includes('npm') && r.includes('cache'))).toBe(true);
      }
    });
  });

  describe('Dockerfile Analysis', () => {
    it('should extract base images', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfileAnalysis.baseImages).toContain('node:18-alpine');
      }
    });

    it('should extract exposed ports', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfileAnalysis.exposedPorts).toContain(3000);
      }
    });

    it('should detect final USER', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfileAnalysis.finalUser).toBe('appuser');
      }
    });

    it('should detect HEALTHCHECK', async () => {
      const dockerfileWithHealthcheck = `FROM node:18-alpine
HEALTHCHECK CMD curl -f http://localhost:3000/ || exit 1
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithHealthcheck);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfileAnalysis.hasHealthcheck).toBe(true);
      }
    });

    it('should estimate layer count', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Our mock Dockerfile has: 2 COPY, 1 RUN = 3 layers
        expect(result.value.dockerfileAnalysis.layerCount).toBeGreaterThan(0);
      }
    });
  });

  describe('Build Command Generation', () => {
    it('should generate valid docker build command', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const cmd = result.value.nextAction.buildCommand;
        expect(cmd.command).toContain('docker build');
        expect(cmd.command).toContain('-t test-app:latest');
        expect(cmd.parts.executable).toBe('docker');
        expect(cmd.parts.subcommand).toBe('build');
      }
    });

    it('should include build args in command', async () => {
      config.buildArgs = { NODE_ENV: 'production' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const cmd = result.value.nextAction.buildCommand;
        expect(cmd.command).toContain('--build-arg NODE_ENV=production');
      }
    });

    it('should include platform in command', async () => {
      config.platform = 'linux/arm64';

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const cmd = result.value.nextAction.buildCommand;
        expect(cmd.command).toContain('--platform linux/arm64');
      }
    });

    it('should set DOCKER_BUILDKIT=1 when BuildKit features detected', async () => {
      const dockerfileWithBuildKit = `FROM node:18-alpine
RUN --mount=type=cache,target=/root/.npm npm ci
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithBuildKit);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const cmd = result.value.nextAction.buildCommand;
        expect(cmd.environment.DOCKER_BUILDKIT).toBe('1');
      }
    });

    it('should include fallback command when BuildKit is used', async () => {
      const dockerfileWithBuildKit = `FROM node:18-alpine AS builder
RUN npm ci
FROM node:18-alpine
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithBuildKit);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.fallbackCommand).toBeDefined();
      }
    });
  });

  describe('Next Action Instructions', () => {
    it('should include pre-checks', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.preChecks.length).toBeGreaterThan(0);
        expect(result.value.nextAction.preChecks.some((c) => c.includes('docker'))).toBe(true);
      }
    });

    it('should include post-build steps', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.nextAction.postBuildSteps.length).toBeGreaterThan(0);
      }
    });

    it('should suggest HEALTHCHECK when missing', async () => {
      const dockerfileWithoutHealthcheck = `FROM node:18-alpine
USER appuser
CMD ["node", "index.js"]`;

      mockFs.readFile.mockResolvedValue(dockerfileWithoutHealthcheck);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const postSteps = result.value.nextAction.postBuildSteps;
        expect(postSteps.some((s) => s.includes('HEALTHCHECK'))).toBe(true);
      }
    });
  });

  describe('Build Args Processing', () => {
    it('should merge user args with defaults', async () => {
      config.buildArgs = { CUSTOM_ARG: 'value' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        const buildArgs = result.value.buildConfig.buildArgs;
        expect(buildArgs.CUSTOM_ARG).toBe('value');
        expect(buildArgs.NODE_ENV).toBeDefined();
        expect(buildArgs.BUILD_DATE).toBeDefined();
        expect(buildArgs.VCS_REF).toBeDefined();
      }
    });

    it('should allow user args to override defaults', async () => {
      config.buildArgs = { NODE_ENV: 'development' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.buildConfig.buildArgs.NODE_ENV).toBe('development');
      }
    });
  });

  describe('Error Handling', () => {
    it('should fail with invalid parameters', async () => {
      const result = await buildImageContext(null as any, createMockToolContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid parameters');
      }
    });

    it('should fail when Dockerfile does not exist', async () => {
      mockFs.access.mockRejectedValue(new Error('ENOENT'));
      mockFs.stat.mockRejectedValue(new Error('ENOENT'));

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(false);
    });

    it('should fail when Dockerfile is not readable', async () => {
      mockFs.readFile.mockRejectedValue(new Error('Permission denied'));

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(false);
    });
  });

  describe('Summary Generation', () => {
    it('should include tag info in summary', async () => {
      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toContain('test-app');
      }
    });

    it('should include security status in summary', async () => {
      config.buildArgs = { API_SECRET: 'token' };

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toMatch(/security|warning/i);
      }
    });

    it('should mention BuildKit when recommended', async () => {
      const dockerfileWithBuildKit = `FROM node:18-alpine
RUN --mount=type=cache,target=/root/.npm npm ci
USER appuser`;

      mockFs.readFile.mockResolvedValue(dockerfileWithBuildKit);

      const result = await buildImageContext(config, createMockToolContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary).toContain('BuildKit');
      }
    });
  });
});
