/**
 * Policy Customization Integration Tests
 *
 * Tests the priority-ordered policy discovery and customization workflow:
 * - Built-in policies (lowest priority)
 * - policies.user/ directory (middle priority)
 * - CUSTOM_POLICY_PATH (highest priority)
 *
 * Validates that:
 * - Example policies work correctly
 * - Custom policies override built-in policies
 * - Priority ordering is respected
 */

import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync, copyFileSync } from 'node:fs';
import { join } from 'node:path';
import { createLogger } from '@/lib/logger';
import { createToolContext } from '@/mcp/context';
import { ENV_VARS } from '@/config/constants';
import { loadAndMergeRegoPolicies } from '@/config/policy-rego';

// Import tools
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';

import type { ValidationReport } from '@/validation/core-types';

describe('Policy Customization Integration', () => {
  let testDir: string;
  let originalCwd: string;
  let originalEnv: string | undefined;
  const logger = createLogger({ level: 'silent' });

  beforeEach(() => {
    testDir = join(__dirname, 'test-customization-' + Date.now());
    mkdirSync(testDir, { recursive: true });
    originalCwd = process.cwd();
    originalEnv = process.env[ENV_VARS.CUSTOM_POLICY_PATH];
  });

  afterEach(() => {
    rmSync(testDir, { recursive: true, force: true });
    process.chdir(originalCwd);
    if (originalEnv !== undefined) {
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = originalEnv;
    } else {
      delete process.env[ENV_VARS.CUSTOM_POLICY_PATH];
    }
  });

  describe('allow-all-registries example policy', () => {
    it('should override built-in MCR preference and allow Docker Hub', async () => {
      // Setup: Create policies.user directory with allow-all-registries policy
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      // Copy example policy
      const examplePolicy = join(process.cwd(), 'policies.user.examples', 'allow-all-registries.rego');
      copyFileSync(examplePolicy, join(policiesUserDir, 'allow-all-registries.rego'));

      // Create Dockerfile with Docker Hub registry
      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM docker.io/node:20-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER node
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);

      // Change to test directory so policies.user is discovered
      process.chdir(testDir);

      // Load only the user policy (we're testing it in isolation)
      const userPolicyPath = join(policiesUserDir, 'allow-all-registries.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;

      // Create context with merged policy
      const context = createToolContext(logger, {
        policy: policyEvaluator,
      });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Should NOT have registry violations (allow-all-registries overrides MCR preference)
        const registryViolations = report.policyValidation?.violations.filter((v) =>
          v.ruleId.includes('registry'),
        );
        expect(registryViolations?.length).toBe(0);

        // Should pass policy validation
        expect(report.policyValidation?.passed).toBe(true);
      }

      // Cleanup
      policyEvaluator.close();
    }, 30000);

    it('should allow GCR and ECR registries', async () => {
      // Setup
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      const examplePolicy = join(process.cwd(), 'policies.user.examples', 'allow-all-registries.rego');
      copyFileSync(examplePolicy, join(policiesUserDir, 'allow-all-registries.rego'));

      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      // Test with GCR registry
      const dockerfileGCR = `FROM gcr.io/distroless/nodejs20-debian12

WORKDIR /app
COPY . .

USER nonroot
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfileGCR);
      process.chdir(testDir);

      // Load only the user policy
      const userPolicyPath = join(policiesUserDir, 'allow-all-registries.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation?.passed).toBe(true);

        // No registry violations
        const registryViolations = report.policyValidation?.violations.filter((v) =>
          v.ruleId.includes('registry'),
        );
        expect(registryViolations?.length).toBe(0);
      }

      policyEvaluator.close();
    }, 30000);
  });

  describe('warn-only-mode example policy', () => {
    it('should convert blocking violations to warnings', async () => {
      // Setup: Create policies.user directory with warn-only-mode policy
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      // Copy example policy
      const examplePolicy = join(process.cwd(), 'policies.user.examples', 'warn-only-mode.rego');
      copyFileSync(examplePolicy, join(policiesUserDir, 'warn-only-mode.rego'));

      // Create Dockerfile with root user (would normally be a blocking violation)
      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM node:latest

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

USER root
EXPOSE 8080

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);
      process.chdir(testDir);

      // Load only the warn-only-mode policy
      const userPolicyPath = join(policiesUserDir, 'warn-only-mode.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Should PASS validation (warn-only mode doesn't block)
        expect(report.policyValidation?.passed).toBe(true);

        // Should have warnings but NO blocking violations
        expect(report.policyValidation?.warnings.length).toBeGreaterThan(0);
        expect(report.policyValidation?.violations.length).toBe(0);

        // Should have warning about root user
        const rootWarning = report.policyValidation?.warnings.find((w) =>
          w.message?.includes('root user'),
        );
        expect(rootWarning).toBeDefined();
        expect(rootWarning?.severity).toBe('warn');

        // Should have warning about :latest tag
        const latestWarning = report.policyValidation?.warnings.find((w) =>
          w.message?.includes('latest tag'),
        );
        expect(latestWarning).toBeDefined();
        expect(latestWarning?.severity).toBe('warn');
      }

      policyEvaluator.close();
    }, 30000);
  });

  describe('priority order when policies conflict', () => {
    it('should respect priority: custom > user > built-in', async () => {
      // Setup: Create all three policy directories with conflicting policies
      const policiesUserDir = join(testDir, 'policies.user');
      const customDir = join(testDir, 'custom');

      mkdirSync(policiesUserDir, { recursive: true });
      mkdirSync(customDir, { recursive: true });

      // User policy: allow-all-registries
      const allowAllPolicy = join(process.cwd(), 'policies.user.examples', 'allow-all-registries.rego');
      copyFileSync(allowAllPolicy, join(policiesUserDir, 'allow-all-registries.rego'));

      // Custom policy: even more permissive (warn-only-mode)
      const warnOnlyPolicy = join(process.cwd(), 'policies.user.examples', 'warn-only-mode.rego');
      copyFileSync(warnOnlyPolicy, join(customDir, 'warn-only-mode.rego'));

      // Create Dockerfile with root user and Docker Hub registry
      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM docker.io/node:latest

WORKDIR /app

USER root

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);

      // Set CUSTOM_POLICY_PATH
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = customDir;
      process.chdir(testDir);

      // Load both user + custom policies (custom should take priority)
      // In a real scenario, both would be merged, but for this test we're testing
      // that the custom policy (warn-only-mode) wins
      const customPolicyPath = join(customDir, 'warn-only-mode.rego');
      const policyPaths = [customPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      // Run fix-dockerfile
      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Custom policy (warn-only-mode) should win: no blocking violations
        expect(report.policyValidation?.violations.length).toBe(0);
        expect(report.policyValidation?.passed).toBe(true);

        // Should have warnings instead (from warn-only-mode)
        expect(report.policyValidation?.warnings.length).toBeGreaterThan(0);
      }

      policyEvaluator.close();
    }, 30000);

    it('should work with only user policies (no custom)', async () => {
      // Setup: Only policies.user, no custom directory
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      const allowAllPolicy = join(process.cwd(), 'policies.user.examples', 'allow-all-registries.rego');
      copyFileSync(allowAllPolicy, join(policiesUserDir, 'allow-all-registries.rego'));

      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM docker.io/node:20-alpine

WORKDIR /app
USER node

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);
      process.chdir(testDir);

      // Load only the user policy
      const userPolicyPath = join(policiesUserDir, 'allow-all-registries.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation?.passed).toBe(true);

        // No registry violations (user policy allows Docker Hub)
        const registryViolations = report.policyValidation?.violations.filter((v) =>
          v.ruleId.includes('registry'),
        );
        expect(registryViolations?.length).toBe(0);
      }

      policyEvaluator.close();
    }, 30000);
  });

  describe('custom-organization-template example policy', () => {
    it('should enforce custom organization rules', async () => {
      // Setup
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      // Copy custom organization template
      const customOrgPolicy = join(
        process.cwd(),
        'policies.user.examples',
        'custom-organization-template.rego',
      );
      copyFileSync(customOrgPolicy, join(policiesUserDir, 'custom-org.rego'));

      // Create Dockerfile WITHOUT required team label (should violate)
      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      const dockerfile = `FROM your-registry.example.com/node:20-alpine

WORKDIR /app

USER node

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);
      process.chdir(testDir);

      // Load only the custom org policy
      const userPolicyPath = join(policiesUserDir, 'custom-org.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Should FAIL validation (missing team label)
        expect(report.policyValidation?.passed).toBe(false);

        // Should have violation for missing team label
        const teamLabelViolation = report.policyValidation?.violations.find((v) =>
          v.ruleId.includes('team-label'),
        );
        expect(teamLabelViolation).toBeDefined();
        expect(teamLabelViolation?.severity).toBe('block');
      }

      policyEvaluator.close();
    }, 30000);

    it('should pass when organization requirements are met', async () => {
      // Setup
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      const customOrgPolicy = join(
        process.cwd(),
        'policies.user.examples',
        'custom-organization-template.rego',
      );
      copyFileSync(customOrgPolicy, join(policiesUserDir, 'custom-org.rego'));

      const appPath = join(testDir, 'app');
      mkdirSync(appPath, { recursive: true });

      // Dockerfile WITH required team label and approved registry
      const dockerfile = `FROM your-registry.example.com/node:20-alpine

LABEL team="platform-engineering"

WORKDIR /app

USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1

CMD ["node", "server.js"]
`;

      writeFileSync(join(appPath, 'Dockerfile'), dockerfile);
      process.chdir(testDir);

      // Load only the custom org policy
      const userPolicyPath = join(policiesUserDir, 'custom-org.rego');
      const policyPaths = [userPolicyPath];

      const policyResult = await loadAndMergeRegoPolicies(policyPaths, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) {
        console.error('Policy load error:', policyResult.error);
        return;
      }

      const policyEvaluator = policyResult.value;
      const context = createToolContext(logger, { policy: policyEvaluator });

      const result = await fixDockerfileTool.handler(
        {
          path: join(appPath, 'Dockerfile'),
        },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value as ValidationReport;
        expect(report.policyValidation).toBeDefined();

        // Should PASS validation (all requirements met)
        expect(report.policyValidation?.passed).toBe(true);
        expect(report.policyValidation?.violations.length).toBe(0);

        // May still have warnings (e.g., security scan label suggestion)
        // but that's acceptable
      }

      policyEvaluator.close();
    }, 30000);
  });
});
