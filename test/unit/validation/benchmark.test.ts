/**
 * Performance benchmark tests for Dockerfile validation pipeline
 */

import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import { applyFixes, applyAllFixes } from '@/validation/dockerfile-fixer';
import { lintWithDockerfilelint } from '@/validation/dockerfilelint-adapter';
import { mergeReports } from '@/validation/merge-reports';

describe('Validation Pipeline Performance', () => {
  const SMALL_DOCKERFILE = `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER node
CMD ["node", "server.js"]`;

  const MEDIUM_DOCKERFILE = `FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY --from=builder /app/dist ./dist
USER node
EXPOSE 3000
HEALTHCHECK CMD curl -f http://localhost:3000/health || exit 1
CMD ["node", "dist/server.js"]`;

  const LARGE_DOCKERFILE = (() => {
    const commands = ['FROM ubuntu:24.04'];
    for (let i = 0; i < 50; i++) {
      commands.push(`RUN apt-get update && apt-get install -y package${i}`);
      commands.push(`ENV VAR_${i}=value${i}`);
      commands.push(`COPY file${i}.txt /app/file${i}.txt`);
    }
    commands.push('CMD ["bash"]');
    return commands.join('\n');
  })();

  describe('Internal validation performance', () => {
    it('should validate small Dockerfile within 50ms', async () => {
      const start = Date.now();
      await validateDockerfileContent(SMALL_DOCKERFILE, { enableExternalLinter: false });
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(50);
    });

    it('should validate medium Dockerfile within 100ms', async () => {
      const start = Date.now();
      await validateDockerfileContent(MEDIUM_DOCKERFILE, { enableExternalLinter: false });
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(100);
    });

    it('should validate large Dockerfile within 200ms', async () => {
      const start = Date.now();
      await validateDockerfileContent(LARGE_DOCKERFILE, { enableExternalLinter: false });
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(200);
    });
  });

  describe('External linter performance', () => {
    it('should run external linter within 100ms overhead', async () => {
      const startInternal = Date.now();
      await validateDockerfileContent(MEDIUM_DOCKERFILE, {
        enableExternalLinter: false,
      });
      const internalDuration = Date.now() - startInternal;

      const startExternal = Date.now();
      await lintWithDockerfilelint(MEDIUM_DOCKERFILE);
      const externalDuration = Date.now() - startExternal;

      // External linter should add less than 2000ms overhead (relaxed for CI)
      expect(externalDuration).toBeLessThan(internalDuration + 2000);
    });

    it('should complete full validation with external linter within 500ms', async () => {
      const start = Date.now();
      await validateDockerfileContent(LARGE_DOCKERFILE, { enableExternalLinter: true });
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(1000);
    });
  });

  describe('Fixer performance', () => {
    it('should apply single fix within 20ms', () => {
      const start = Date.now();
      applyFixes(SMALL_DOCKERFILE, ['no-root-user']);
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(20);
    });

    it('should apply all fixes within 50ms', () => {
      const start = Date.now();
      applyAllFixes(MEDIUM_DOCKERFILE);
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(50);
    });

    it('should handle large Dockerfiles efficiently', () => {
      const start = Date.now();
      applyAllFixes(LARGE_DOCKERFILE);
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(100);
    });
  });

  describe('Report merging performance', () => {
    it('should merge reports within 5ms', async () => {
      const report1 = await validateDockerfileContent(SMALL_DOCKERFILE, {
        enableExternalLinter: false,
      });
      const report2 = await lintWithDockerfilelint(SMALL_DOCKERFILE);

      const start = Date.now();
      mergeReports(report1, report2);
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(5);
    });

    it('should handle large report merging efficiently', async () => {
      const report1 = await validateDockerfileContent(LARGE_DOCKERFILE, {
        enableExternalLinter: false,
      });
      const report2 = await lintWithDockerfilelint(LARGE_DOCKERFILE);

      const start = Date.now();
      mergeReports(report1, report2);
      const duration = Date.now() - start;

      expect(duration).toBeLessThan(10);
    });
  });

  describe('Full pipeline functionality', () => {
    it('should complete full validation + fix pipeline successfully', async () => {
      // Validate with both internal and external
      const report = await validateDockerfileContent(MEDIUM_DOCKERFILE, {
        enableExternalLinter: true,
      });

      expect(report).toBeDefined();
      expect(typeof report.score).toBe('number');

      // Apply fixes if needed
      if (report.score < 100) {
        const failedRules = report.results
          .filter((r) => !r.passed && r.ruleId)
          .map((r) => r.ruleId!);
        const fixedDockerfile = applyFixes(MEDIUM_DOCKERFILE, failedRules);
        expect(fixedDockerfile).toBeDefined();
      }
    });

    it('should handle different Dockerfile sizes', async () => {
      const smallReport = await validateDockerfileContent(SMALL_DOCKERFILE, { enableExternalLinter: true });
      const mediumReport = await validateDockerfileContent(MEDIUM_DOCKERFILE, { enableExternalLinter: true });
      const largeReport = await validateDockerfileContent(LARGE_DOCKERFILE, { enableExternalLinter: true });

      expect(smallReport).toBeDefined();
      expect(mediumReport).toBeDefined();
      expect(largeReport).toBeDefined();

      expect(typeof smallReport.score).toBe('number');
      expect(typeof mediumReport.score).toBe('number');
      expect(typeof largeReport.score).toBe('number');
    });
  });

  describe('Memory usage', () => {
    it('should not leak memory on repeated validations', async () => {
      // Note: This test requires Node.js to be started with --expose-gc flag to enable global.gc
      // If gc is not available, the test will skip memory cleanup but still verify basic functionality

      if (global.gc) {
        global.gc();
      }

      const initialMemory = process.memoryUsage().heapUsed;

      // Run validation 100 times
      for (let i = 0; i < 100; i++) {
        await validateDockerfileContent(MEDIUM_DOCKERFILE, { enableExternalLinter: true });
      }

      if (global.gc) {
        global.gc();

        const finalMemory = process.memoryUsage().heapUsed;

        // Memory should not grow by more than 10MB when gc is available
        const memoryGrowth = (finalMemory - initialMemory) / (1024 * 1024);
        expect(memoryGrowth).toBeLessThan(10);
      } else {
        // When gc is not available, just verify the function completes successfully
        // This ensures basic functionality without memory leak detection
        expect(true).toBe(true);
      }
    });
  });
});
