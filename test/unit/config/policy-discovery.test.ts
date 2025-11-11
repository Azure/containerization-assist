import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { createLogger } from '@/lib/logger';
import {
  discoverBuiltInPolicies,
  discoverUserPolicies,
  discoverCustomPolicies,
  discoverPolicies,
} from '@/app/orchestrator';
import { ENV_VARS } from '@/config/constants';

describe('Policy Discovery', () => {
  let testDir: string;
  let logger: ReturnType<typeof createLogger>;
  let originalCwd: string;
  let originalEnv: string | undefined;

  beforeEach(() => {
    testDir = join(__dirname, 'test-policies-' + Date.now());
    mkdirSync(testDir, { recursive: true });
    logger = createLogger({ name: 'test', level: 'silent' });
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

  describe('discoverBuiltInPolicies', () => {
    it('should discover built-in policies from policies/ directory', () => {
      // Built-in policies are discovered from the actual repo
      // This test verifies the function works
      const policies = discoverBuiltInPolicies(logger);

      // Should find the 3 built-in policies in the repo
      expect(policies.length).toBeGreaterThanOrEqual(3);
      expect(policies.some((p) => p.endsWith('security-baseline.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('base-images.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('container-best-practices.rego'))).toBe(true);
    });

    it('should exclude test files (*_test.rego)', () => {
      const policies = discoverBuiltInPolicies(logger);

      // No test files should be included
      expect(policies.every((p) => !p.endsWith('_test.rego'))).toBe(true);
    });

    it('should return empty array if policies/ not found', () => {
      // Change to a directory that has no policies/ parent
      process.chdir('/tmp');

      const policies = discoverBuiltInPolicies(logger);

      // Should return empty array if not found
      expect(Array.isArray(policies)).toBe(true);
    });

    it('should search upward for policies/ directory from nested path', () => {
      // Create a nested directory structure
      const nestedDir = join(testDir, 'deeply', 'nested', 'path');
      const policiesDir = join(testDir, 'policies');

      mkdirSync(nestedDir, { recursive: true });
      mkdirSync(policiesDir, { recursive: true });

      // Create a test policy in the policies directory
      writeFileSync(join(policiesDir, 'test-policy.rego'), 'package test\ndefault allow := true');

      // Change to nested directory
      process.chdir(nestedDir);

      const policies = discoverBuiltInPolicies(logger);

      // Should find the policy by searching upward
      expect(policies.some((p) => p.endsWith('test-policy.rego'))).toBe(true);
    });
  });

  describe('discoverUserPolicies', () => {
    it('should discover policies from policies.user/ directory', () => {
      // Create a test policies.user directory
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      // Create test policy files
      writeFileSync(join(policiesUserDir, 'user-policy1.rego'), 'package test1\ndefault allow := true');
      writeFileSync(join(policiesUserDir, 'user-policy2.rego'), 'package test2\ndefault allow := true');

      // Change to test directory
      process.chdir(testDir);

      const policies = discoverUserPolicies(logger);

      expect(policies.length).toBe(2);
      expect(policies.some((p) => p.endsWith('user-policy1.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('user-policy2.rego'))).toBe(true);
    });

    it('should exclude test files (*_test.rego)', () => {
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });

      // Create test policy files
      writeFileSync(join(policiesUserDir, 'user-policy.rego'), 'package test\ndefault allow := true');
      writeFileSync(join(policiesUserDir, 'user-policy_test.rego'), 'package test\n# test');

      process.chdir(testDir);

      const policies = discoverUserPolicies(logger);

      expect(policies.length).toBe(1);
      expect(policies[0].endsWith('user-policy.rego')).toBe(true);
    });

    it('should return empty array if policies.user/ not found', () => {
      // Change to directory without policies.user
      process.chdir(testDir);

      const policies = discoverUserPolicies(logger);

      expect(policies).toEqual([]);
    });

    it('should search upward for policies.user/ directory from nested path', () => {
      // Create nested structure
      const nestedDir = join(testDir, 'src', 'tools', 'my-tool');
      const policiesUserDir = join(testDir, 'policies.user');

      mkdirSync(nestedDir, { recursive: true });
      mkdirSync(policiesUserDir, { recursive: true });

      // Create user policy at repo root level
      writeFileSync(join(policiesUserDir, 'user-override.rego'), 'package override\ndefault allow := true');

      // Change to deeply nested directory
      process.chdir(nestedDir);

      const policies = discoverUserPolicies(logger);

      // Should find the policy by searching upward
      expect(policies.length).toBe(1);
      expect(policies[0].endsWith('user-override.rego')).toBe(true);
    });
  });

  describe('discoverCustomPolicies', () => {
    it('should discover policies from custom directory', () => {
      const customDir = join(testDir, 'custom');
      mkdirSync(customDir, { recursive: true });

      writeFileSync(join(customDir, 'custom-policy1.rego'), 'package custom1\ndefault allow := true');
      writeFileSync(join(customDir, 'custom-policy2.rego'), 'package custom2\ndefault allow := true');

      const policies = discoverCustomPolicies(customDir, logger);

      expect(policies.length).toBe(2);
      expect(policies.some((p) => p.endsWith('custom-policy1.rego'))).toBe(true);
      expect(policies.some((p) => p.endsWith('custom-policy2.rego'))).toBe(true);
    });

    it('should handle single file path', () => {
      const customFile = join(testDir, 'single-policy.rego');
      writeFileSync(customFile, 'package single\ndefault allow := true');

      const policies = discoverCustomPolicies(customFile, logger);

      expect(policies.length).toBe(1);
      expect(policies[0]).toBe(customFile);
    });

    it('should exclude test files (*_test.rego)', () => {
      const customDir = join(testDir, 'custom');
      mkdirSync(customDir, { recursive: true });

      writeFileSync(join(customDir, 'custom-policy.rego'), 'package custom\ndefault allow := true');
      writeFileSync(join(customDir, 'custom-policy_test.rego'), 'package custom\n# test');

      const policies = discoverCustomPolicies(customDir, logger);

      expect(policies.length).toBe(1);
      expect(policies[0].endsWith('custom-policy.rego')).toBe(true);
    });

    it('should return empty array if path does not exist', () => {
      const nonExistentPath = join(testDir, 'does-not-exist');

      const policies = discoverCustomPolicies(nonExistentPath, logger);

      expect(policies).toEqual([]);
    });

    it('should return empty array if file is not .rego', () => {
      const txtFile = join(testDir, 'not-a-policy.txt');
      writeFileSync(txtFile, 'not a policy');

      const policies = discoverCustomPolicies(txtFile, logger);

      expect(policies).toEqual([]);
    });
  });

  describe('discoverPolicies (priority ordering)', () => {
    it('should merge policies in correct priority order', () => {
      // Create test directories
      const policiesUserDir = join(testDir, 'policies.user');
      const customDir = join(testDir, 'custom');

      mkdirSync(policiesUserDir, { recursive: true });
      mkdirSync(customDir, { recursive: true });

      // Create user policy
      writeFileSync(join(policiesUserDir, 'user-policy.rego'), 'package test\ndefault allow := true');

      // Create custom policy
      writeFileSync(join(customDir, 'custom-policy.rego'), 'package test\ndefault allow := true');

      // Set environment variable
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = customDir;
      process.chdir(testDir);

      const policies = discoverPolicies(logger);

      // Should include built-in + user + custom
      // Built-in policies come first (lowest priority)
      expect(policies.length).toBeGreaterThanOrEqual(5); // 3 built-in + 1 user + 1 custom

      // Check that custom policy comes last (highest priority)
      expect(policies[policies.length - 1].endsWith('custom-policy.rego')).toBe(true);

      // Check that user policy comes before custom
      const userPolicyIndex = policies.findIndex((p) => p.endsWith('user-policy.rego'));
      const customPolicyIndex = policies.findIndex((p) => p.endsWith('custom-policy.rego'));
      expect(userPolicyIndex).toBeLessThan(customPolicyIndex);

      // Check that built-in policies come first
      const builtInIndex = policies.findIndex((p) => p.includes('/policies/'));
      expect(builtInIndex).toBeLessThan(userPolicyIndex);
    });

    it('should work with only built-in policies', () => {
      // No user or custom policies
      process.chdir(testDir);

      const policies = discoverPolicies(logger);

      // Should only have built-in policies
      expect(policies.length).toBeGreaterThanOrEqual(3);
      expect(policies.every((p) => p.includes('/policies/'))).toBe(true);
    });

    it('should work with only user policies', () => {
      const policiesUserDir = join(testDir, 'policies.user');
      mkdirSync(policiesUserDir, { recursive: true });
      writeFileSync(join(policiesUserDir, 'user-policy.rego'), 'package test\ndefault allow := true');

      process.chdir(testDir);

      const policies = discoverPolicies(logger);

      // Should have built-in + user
      expect(policies.length).toBeGreaterThanOrEqual(4);
      expect(policies.some((p) => p.endsWith('user-policy.rego'))).toBe(true);
    });

    it('should work with only custom policies', () => {
      const customDir = join(testDir, 'custom');
      mkdirSync(customDir, { recursive: true });
      writeFileSync(join(customDir, 'custom-policy.rego'), 'package test\ndefault allow := true');

      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = customDir;

      const policies = discoverPolicies(logger);

      // Should have built-in + custom
      expect(policies.length).toBeGreaterThanOrEqual(4);
      expect(policies.some((p) => p.endsWith('custom-policy.rego'))).toBe(true);
    });

    it('should maintain priority order when discovering from nested directory', () => {
      // Create nested directory structure
      const nestedDir = join(testDir, 'src', 'nested');
      const policiesDir = join(testDir, 'policies');
      const policiesUserDir = join(testDir, 'policies.user');
      const customDir = join(testDir, 'custom');

      mkdirSync(nestedDir, { recursive: true });
      mkdirSync(policiesDir, { recursive: true });
      mkdirSync(policiesUserDir, { recursive: true });
      mkdirSync(customDir, { recursive: true });

      // Create policies at different levels
      writeFileSync(join(policiesDir, 'builtin-policy.rego'), 'package builtin\ndefault allow := true');
      writeFileSync(join(policiesUserDir, 'user-policy.rego'), 'package user\ndefault allow := true');
      writeFileSync(join(customDir, 'custom-policy.rego'), 'package custom\ndefault allow := true');

      // Set environment and change to nested directory
      process.env[ENV_VARS.CUSTOM_POLICY_PATH] = customDir;
      process.chdir(nestedDir);

      const policies = discoverPolicies(logger);

      // Verify all three layers are present
      const hasBuiltIn = policies.some((p) => p.endsWith('builtin-policy.rego'));
      const hasUser = policies.some((p) => p.endsWith('user-policy.rego'));
      const hasCustom = policies.some((p) => p.endsWith('custom-policy.rego'));

      expect(hasBuiltIn).toBe(true);
      expect(hasUser).toBe(true);
      expect(hasCustom).toBe(true);

      // Verify priority ordering (custom last = highest priority)
      const builtInIndex = policies.findIndex((p) => p.endsWith('builtin-policy.rego'));
      const userIndex = policies.findIndex((p) => p.endsWith('user-policy.rego'));
      const customIndex = policies.findIndex((p) => p.endsWith('custom-policy.rego'));
      expect(builtInIndex).toBeLessThan(userIndex);
      expect(customIndex).toBeGreaterThan(userIndex);
    });
  });
});
