/**
 * Policy YAML Loading Tests
 * Verify that YAML policy files are loaded correctly
 */

import { describe, it, expect } from '@jest/globals';
import * as path from 'node:path';
import { loadPolicy } from '@/config/policy-io';

describe('Policy YAML Loading', () => {
  const policiesDir = path.join(process.cwd(), 'policies');

  it('should load security-baseline.yaml policy', () => {
    const policyPath = path.join(policiesDir, 'security-baseline.yaml');
    const result = loadPolicy(policyPath);

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.version).toBe('2.0');
      expect(result.value.metadata?.name).toBe('Security Baseline');
      expect(result.value.rules.length).toBe(5);
      expect(result.value.defaults?.enforcement).toBe('strict');

      // Verify specific rules exist
      const ruleIds = result.value.rules.map(r => r.id);
      expect(ruleIds).toContain('block-root-user');
      expect(ruleIds).toContain('block-secrets-in-env');
    }
  });

  it('should load base-images.yaml policy', () => {
    const policyPath = path.join(policiesDir, 'base-images.yaml');
    const result = loadPolicy(policyPath);

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.version).toBe('2.0');
      expect(result.value.metadata?.name).toBe('Base Image Governance');
      expect(result.value.rules.length).toBe(8);
      expect(result.value.defaults?.enforcement).toBe('advisory');

      // Verify Microsoft recommendation rule exists
      const microsoftRule = result.value.rules.find(r => r.id === 'recommend-microsoft-images');
      expect(microsoftRule).toBeDefined();
      expect(microsoftRule?.priority).toBe(85);
    }
  });

  it('should load container-best-practices.yaml policy', () => {
    const policyPath = path.join(policiesDir, 'container-best-practices.yaml');
    const result = loadPolicy(policyPath);

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.version).toBe('2.0');
      expect(result.value.metadata?.name).toBe('Container Best Practices');
      expect(result.value.rules.length).toBe(10);
      expect(result.value.defaults?.enforcement).toBe('advisory');

      // Verify specific rules exist
      const ruleIds = result.value.rules.map(r => r.id);
      expect(ruleIds).toContain('require-healthcheck');
      expect(ruleIds).toContain('recommend-multistage');
    }
  });

  it('should fall back to TypeScript data when file does not exist', () => {
    const result = loadPolicy('/nonexistent/policy.yaml');

    expect(result.ok).toBe(true);
    if (result.ok) {
      // Should load the default TypeScript policy
      expect(result.value.version).toBe('2.0');
      expect(result.value.rules.length).toBeGreaterThan(0);
    }
  });

  it('should sort rules by priority descending', () => {
    const policyPath = path.join(policiesDir, 'security-baseline.yaml');
    const result = loadPolicy(policyPath);

    expect(result.ok).toBe(true);
    if (result.ok) {
      const priorities = result.value.rules.map(r => r.priority);
      const sortedPriorities = [...priorities].sort((a, b) => b - a);
      expect(priorities).toEqual(sortedPriorities);
    }
  });
});
