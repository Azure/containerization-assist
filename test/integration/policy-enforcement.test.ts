/**
 * Policy Enforcement Integration Tests
 *
 * Tests that policies are correctly enforced across the tool chain:
 * - ToolContext passes policy to tools
 * - generate-dockerfile validates plans against policy
 * - fix-dockerfile validates Dockerfiles against policy
 * - Policy violations block execution appropriately
 * - Warnings and suggestions are captured but don't block
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import { createToolContext } from '@/mcp/context';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { mkdirSync, writeFileSync } from 'node:fs';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import { loadPolicy } from '@/config/policy-io';

// Import tools
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';

import type { GenerateDockerfileResult } from '@/tools/generate-dockerfile/schema';
import type { ValidationReport } from '@/validation/core-types';

describe('Policy Enforcement Integration Tests', () => {
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  const logger = createLogger({ level: 'silent' });

  beforeAll(async () => {
    const result = createTestTempDir('policy-enforcement-');
    testDir = result.dir;
    cleanup = result.cleanup;
  });

  afterAll(async () => {
    await cleanup();
  });

  describe('ToolContext Policy Integration', () => {
    it('should pass policy through ToolContext to tools', async () => {
      // Load the security baseline policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);

      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create ToolContext with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      expect(contextWithPolicy.policy).toBeDefined();
      expect(contextWithPolicy.policy).toBe(policyEvaluator);

      // Cleanup
      policyEvaluator.close();
    });

    it('should create ToolContext without policy when not provided', () => {
      const contextWithoutPolicy = createToolContext(logger, {});

      expect(contextWithoutPolicy.policy).toBeUndefined();
    });
  });

  describe('fix-dockerfile Policy Enforcement', () => {
    it('should validate compliant Dockerfile and pass', async () => {
      // Setup: Create a compliant Dockerfile
      const appPath = join(testDir.name, 'compliant-app');
      mkdirSync(appPath, { recursive: true });

      const compliantDockerfile = `FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), compliantDockerfile);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();
        expect(report.policyValidation?.passed).toBe(true);
        expect(report.policyValidation?.violations).toHaveLength(0);
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);

    it('should detect blocking violations in Dockerfile', async () => {
      // Setup: Create a Dockerfile with root user (violation)
      const appPath = join(testDir.name, 'violating-app');
      mkdirSync(appPath, { recursive: true });

      const violatingDockerfile = `FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER root
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), violatingDockerfile);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();
        expect(report.policyValidation?.passed).toBe(false);
        expect(report.policyValidation?.violations.length).toBeGreaterThan(0);

        // Should detect block-root-user violation
        const rootUserViolation = report.policyValidation?.violations.find(
          (v) => v.ruleId === 'block-root-user',
        );
        expect(rootUserViolation).toBeDefined();
        expect(rootUserViolation?.severity).toBe('block');
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);

    it('should detect warnings without blocking execution', async () => {
      // Setup: Create a Dockerfile without HEALTHCHECK (warning, not blocking)
      const appPath = join(testDir.name, 'warning-app');
      mkdirSync(appPath, { recursive: true });

      const dockerfileWithWarnings = `FROM node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER node
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfileWithWarnings);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Should pass despite warnings (warnings don't block)
        expect(report.policyValidation?.passed).toBe(true);
        expect(report.policyValidation?.warnings.length).toBeGreaterThan(0);

        // Should detect require-healthcheck warning
        const healthcheckWarning = report.policyValidation?.warnings.find(
          (w) => w.ruleId === 'require-healthcheck',
        );
        expect(healthcheckWarning).toBeDefined();
        expect(healthcheckWarning?.severity).toBe('warn');
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);

    it('should detect secrets in environment variables', async () => {
      // Setup: Create a Dockerfile with secrets in ENV (violation)
      const appPath = join(testDir.name, 'secrets-app');
      mkdirSync(appPath, { recursive: true });

      const dockerfileWithSecrets = `FROM node:20-alpine

WORKDIR /app

ENV PASSWORD=mysecretpassword
ENV API_KEY=sk_test_123456789

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER node
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfileWithSecrets);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();
        expect(report.policyValidation?.passed).toBe(false);
        expect(report.policyValidation?.violations.length).toBeGreaterThan(0);

        // Should detect block-secrets-in-env violation
        const secretsViolation = report.policyValidation?.violations.find(
          (v) => v.ruleId === 'block-secrets-in-env',
        );
        expect(secretsViolation).toBeDefined();
        expect(secretsViolation?.severity).toBe('block');
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);
  });

  describe('Policy Enforcement Across Tool Chain', () => {
    it('should work without policy when not provided', async () => {
      // Setup: Create a Dockerfile (even with violations)
      const appPath = join(testDir.name, 'no-policy-app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM node:20-alpine
USER root
CMD ["node", "app.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);

      // Create context WITHOUT policy
      const contextWithoutPolicy = createToolContext(logger, {});

      // Run fix-dockerfile - should work without policy
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithoutPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        // Policy validation should be undefined when no policy is provided
        expect(report.policyValidation).toBeUndefined();
      }
    }, 30000);

    it('should handle multiple violations correctly', async () => {
      // Setup: Create a Dockerfile with multiple violations
      const appPath = join(testDir.name, 'multi-violation-app');
      mkdirSync(appPath, { recursive: true });

      const dockerfileMultipleViolations = `FROM node:20-alpine

WORKDIR /app

ENV PASSWORD=secret123
ENV TOKEN=bearer_token

COPY package*.json ./
RUN apt-get update && apt-get upgrade -y
RUN npm ci --only=production

COPY . .

USER root
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfileMultipleViolations);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();
        expect(report.policyValidation?.passed).toBe(false);

        // Should detect multiple violations
        expect(report.policyValidation?.violations.length).toBeGreaterThan(1);

        // Should include both root user and secrets violations
        const violationRuleIds = report.policyValidation?.violations.map((v) => v.ruleId) || [];
        expect(violationRuleIds).toContain('block-root-user');
        expect(violationRuleIds).toContain('block-secrets-in-env');

        // Should also have warnings (apt-get upgrade, missing healthcheck)
        expect(report.policyValidation?.warnings.length).toBeGreaterThan(0);
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);
  });

  describe('Policy Validation Summary', () => {
    it('should provide accurate summary statistics', async () => {
      // Setup: Create a Dockerfile with mixed violations and warnings
      const appPath = join(testDir.name, 'summary-app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM node:20-alpine

WORKDIR /app

ENV PASSWORD=secret

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER root

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);

      // Load policy
      const policyPath = join(process.cwd(), 'policies', 'security-baseline.rego');
      const policyResult = await loadPolicy(policyPath);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;

      const policyEvaluator = policyResult.value;

      // Create context with policy
      const contextWithPolicy = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        contextWithPolicy,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();
        expect(report.policyValidation?.summary).toBeDefined();

        const summary = report.policyValidation?.summary;
        if (summary) {
          // Should have accurate counts
          expect(summary.blockingViolations).toBe(report.policyValidation?.violations.length);
          expect(summary.warnings).toBe(report.policyValidation?.warnings.length);
          expect(summary.suggestions).toBe(report.policyValidation?.suggestions.length);

          // Total matched rules should equal sum of all categories
          const totalMatched =
            (report.policyValidation?.violations.length || 0) +
            (report.policyValidation?.warnings.length || 0) +
            (report.policyValidation?.suggestions.length || 0);
          expect(summary.matchedRules).toBe(totalMatched);
        }
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);
  });
});
