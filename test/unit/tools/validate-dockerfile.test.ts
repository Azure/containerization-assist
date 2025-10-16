/**
 * Unit Tests: Validate Dockerfile Tool
 * Tests the validate-dockerfile tool functionality for allowlist/denylist validation
 */

import { jest } from '@jest/globals';
import { appConfig } from '../../../src/config/app-config';

jest.mock('../../../src/config/app-config', () => ({
  appConfig: {
    validation: {
      imageAllowlist: [],
      imageDenylist: [],
    },
  },
}));

jest.mock('../../../src/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  })),
}));

import tool from '../../../src/tools/validate-dockerfile/tool';
import { createLogger } from '../../../src/lib/logger';

const mockLogger = (createLogger as jest.Mock)();

const mockSessionFacade = {
  id: 'test-session-123',
  get: jest.fn(),
  set: jest.fn(),
  storeResult: jest.fn(),
  pushStep: jest.fn(),
};

function createMockToolContext() {
  return {
    logger: mockLogger,
    session: mockSessionFacade,
  } as any;
}

describe('validate-dockerfile tool', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    appConfig.validation.imageAllowlist = [];
    appConfig.validation.imageDenylist = [];
  });

  describe('validateImageAgainstRules logic', () => {
    const sampleDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["npm", "start"]`;

    describe('allowlist behavior', () => {
      it('should allow images matching allowlist pattern', async () => {
        appConfig.validation.imageAllowlist = ['^node:.*-alpine$'];

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
          expect(result.value.baseImages).toHaveLength(1);
          expect(result.value.baseImages[0]?.allowed).toBe(true);
          expect(result.value.baseImages[0]?.denied).toBe(false);
          expect(result.value.baseImages[0]?.matchedAllowRule).toBe('^node:.*-alpine$');
          expect(result.value.violations).toHaveLength(0);
        }
      });

      it('should reject images not matching allowlist in strict mode', async () => {
        appConfig.validation.imageAllowlist = ['^alpine:.*'];

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile, strictMode: true },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(false);
          expect(result.value.baseImages[0]?.allowed).toBe(false);
          expect(result.value.violations.length).toBeGreaterThan(0);
          expect(result.value.violations[0]).toContain('does not match any allowlist pattern');
        }
      });

      it('should allow images not matching allowlist when strict mode is off', async () => {
        appConfig.validation.imageAllowlist = ['^alpine:.*'];

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile, strictMode: false },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
          expect(result.value.baseImages[0]?.allowed).toBe(true);
          expect(result.value.violations).toHaveLength(0);
        }
      });

      it('should match multiple allowlist patterns', async () => {
        appConfig.validation.imageAllowlist = ['^python:.*', '^node:.*-alpine$', '^golang:.*'];

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
          expect(result.value.baseImages[0]?.matchedAllowRule).toBe('^node:.*-alpine$');
        }
      });
    });

    describe('denylist behavior', () => {
      it('should deny images matching denylist pattern', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const dockerfileWithLatest = `FROM node:latest
CMD ["npm", "start"]`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: dockerfileWithLatest },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(false);
          expect(result.value.baseImages[0]?.denied).toBe(true);
          expect(result.value.baseImages[0]?.allowed).toBe(false);
          expect(result.value.baseImages[0]?.matchedDenyRule).toBe('.*:latest$');
          expect(result.value.violations.length).toBeGreaterThan(0);
          expect(result.value.violations[0]).toContain('matches denylist pattern');
        }
      });

      it('should deny images even if they match allowlist', async () => {
        appConfig.validation.imageAllowlist = ['^node:.*'];
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const dockerfileWithLatest = `FROM node:latest
CMD ["npm", "start"]`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: dockerfileWithLatest },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(false);
          expect(result.value.baseImages[0]?.denied).toBe(true);
          expect(result.value.baseImages[0]?.allowed).toBe(false);
        }
      });

      it('should match multiple denylist patterns', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$', '^ubuntu:18\\.04$', '.*-rc$'];

        const dockerfileWithRC = `FROM node:20-rc
CMD ["npm", "start"]`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: dockerfileWithRC },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(false);
          expect(result.value.baseImages[0]?.matchedDenyRule).toBe('.*-rc$');
        }
      });

      it('should allow images not matching any denylist pattern', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$', '^ubuntu:18\\.04$'];

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
          expect(result.value.baseImages[0]?.denied).toBe(false);
        }
      });
    });

    describe('complex regex patterns', () => {
      it('should support complex allowlist regex patterns', async () => {
        appConfig.validation.imageAllowlist = [
          '^(alpine|node|python):(\\d+\\.\\d+|\\d+-alpine)$',
        ];

        const validDockerfiles = [
          'FROM alpine:3.18',
          'FROM node:18-alpine',
          'FROM python:3.11',
        ];

        for (const dockerfile of validDockerfiles) {
          const context = createMockToolContext();
          const result = await tool.handler(
            { dockerfile },
            context,
          );

          expect(result.ok).toBe(true);
          if (result.ok) {
            expect(result.value.passed).toBe(true);
          }
        }
      });

      it('should support complex denylist regex patterns', async () => {
        appConfig.validation.imageDenylist = [
          '.*:(latest|master|main|dev|development)$',
        ];

        const deniedTags = ['latest', 'master', 'main', 'dev', 'development'];

        for (const tag of deniedTags) {
          const dockerfile = `FROM node:${tag}`;
          const context = createMockToolContext();
          const result = await tool.handler(
            { dockerfile },
            context,
          );

          expect(result.ok).toBe(true);
          if (result.ok) {
            expect(result.value.passed).toBe(false);
            expect(result.value.baseImages[0]?.denied).toBe(true);
          }
        }
      });
    });

    describe('multiple FROM statements', () => {
      it('should validate all base images in multi-stage builds', async () => {
        appConfig.validation.imageAllowlist = ['^node:.*-alpine$', '^nginx:.*-alpine$'];

        const multiStageDockerfile = `FROM node:18-alpine AS builder
WORKDIR /app
COPY . .
RUN npm run build

FROM nginx:1.24-alpine
COPY --from=builder /app/dist /usr/share/nginx/html`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: multiStageDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
          expect(result.value.baseImages).toHaveLength(2);
          expect(result.value.baseImages[0]?.image).toBe('node:18-alpine');
          expect(result.value.baseImages[1]?.image).toBe('nginx:1.24-alpine');
          expect(result.value.baseImages[0]?.allowed).toBe(true);
          expect(result.value.baseImages[1]?.allowed).toBe(true);
        }
      });

      it('should report violations for any denied image in multi-stage builds', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const multiStageDockerfile = `FROM node:18-alpine AS builder
WORKDIR /app

FROM nginx:latest
COPY --from=builder /app/dist /usr/share/nginx/html`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: multiStageDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(false);
          expect(result.value.baseImages).toHaveLength(2);
          expect(result.value.baseImages[0]?.denied).toBe(false);
          expect(result.value.baseImages[1]?.denied).toBe(true);
          expect(result.value.violations.length).toBeGreaterThan(0);
        }
      });
    });

    describe('edge cases', () => {
      it('should pass when no allowlist or denylist is configured', async () => {
        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.passed).toBe(true);
        }
      });

      it('should handle empty dockerfile gracefully', async () => {
        const context = createMockToolContext();
        const result = await tool.handler(
          {},
          context,
        );

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('Either path or dockerfile content is required');
        }
      });

      it('should handle dockerfile without FROM statement', async () => {
        const dockerfileWithoutFrom = `WORKDIR /app
COPY . .
CMD ["npm", "start"]`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: dockerfileWithoutFrom },
          context,
        );

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('No FROM instructions found');
        }
      });

      it('should provide correct line numbers for violations', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const dockerfile = `# Comment
FROM node:latest
WORKDIR /app`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.baseImages[0]?.line).toBe(2);
          expect(result.value.violations[0]).toContain('Line 2');
        }
      });
    });

    describe('summary statistics', () => {
      it('should correctly count allowed, denied, and unknown images', async () => {
        appConfig.validation.imageAllowlist = ['^node:.*'];
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const dockerfile = `FROM node:18-alpine
FROM python:3.11
FROM node:latest`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile, strictMode: true },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.summary.totalImages).toBe(3);
          expect(result.value.summary.allowedImages).toBe(1);
          expect(result.value.summary.deniedImages).toBe(1);
          expect(result.value.summary.unknownImages).toBe(1);
        }
      });
    });

    describe('workflow hints', () => {
      it('should suggest build-image when validation passes', async () => {
        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok && result.value.workflowHints) {
          expect(result.value.workflowHints.nextStep).toBe('build-image');
          expect(result.value.workflowHints.message).toContain('validated successfully');
        }
      });

      it('should suggest fix-dockerfile when validation fails', async () => {
        appConfig.validation.imageDenylist = ['.*:latest$'];

        const dockerfile = `FROM node:latest`;

        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok && result.value.workflowHints) {
          expect(result.value.workflowHints.nextStep).toBe('fix-dockerfile');
          expect(result.value.workflowHints.message).toContain('validation failed');
        }
      });
    });

    describe('session integration', () => {
      it('should include baseImages property in results when run with a valid Dockerfile and session', async () => {
        const context = createMockToolContext();
        const result = await tool.handler(
          { dockerfile: sampleDockerfile },
          context,
        );

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toHaveProperty('passed');
          expect(result.value).toHaveProperty('baseImages');
        }
      });
    });
  });
});
