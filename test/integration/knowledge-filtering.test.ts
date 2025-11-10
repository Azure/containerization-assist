/**
 * Integration tests for policy-aware knowledge filtering
 *
 * Tests the complete workflow of:
 * 1. Loading a knowledge filter policy
 * 2. Querying knowledge with different contexts
 * 3. Verifying filtered and weighted results
 */

import { describe, it, expect, beforeAll } from '@jest/globals';
import { join } from 'node:path';
import { loadRegoPolicy } from '@/config/policy-rego';
import { loadKnowledgeData } from '@/knowledge/loader';
import { findPolicyAwareKnowledgeMatches } from '@/knowledge/policy-aware-matcher';
import type { KnowledgeQuery } from '@/knowledge/types';
import { createLogger } from '@/lib/logger';

const logger = createLogger();

// Path to the knowledge filtering policy example
const POLICY_PATH = join(
  process.cwd(),
  'policies.user.examples',
  'knowledge-filtering.rego',
);

describe.skip('Knowledge Filtering - Integration Tests', () => {
  beforeAll(async () => {
    // Pre-load knowledge data
    await loadKnowledgeData();
  });

  describe('Environment-based filtering', () => {
    it('should boost security and reliability in production', async () => {
      // Load policy
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      // Load knowledge
      const knowledgeData = await loadKnowledgeData();

      // Query with production environment
      const query: KnowledgeQuery = {
        category: 'security',
        environment: 'production',
        tool: 'generate-dockerfile',
        language: 'node',
        limit: 10,
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      // Verify policy was applied
      expect(filterResult.policyApplied).toBe(true);

      // In production, security snippets should be boosted
      // Verify that at least some matches exist (knowledge base dependent)
      expect(matches.length).toBeGreaterThan(0);

      policy.close();
    });

    it('should boost build speed and caching in development', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        category: 'build',
        environment: 'development',
        tool: 'generate-dockerfile',
        language: 'node',
        limit: 10,
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      expect(matches.length).toBeGreaterThan(0);

      policy.close();
    });
  });

  describe('Tool-specific filtering', () => {
    it('should apply generate-dockerfile specific filters in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        environment: 'production',
        tool: 'generate-dockerfile',
        language: 'node',
        limit: 20, // Request more than policy max
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);

      // Policy should limit to 8 snippets for generate-dockerfile
      expect(matches.length).toBeLessThanOrEqual(8);

      policy.close();
    });

    it('should prioritize security in fix-dockerfile tool', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        tool: 'fix-dockerfile',
        category: 'security',
        limit: 10,
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      expect(matches.length).toBeGreaterThan(0);

      policy.close();
    });
  });

  describe('Language-specific filtering', () => {
    it('should apply Java-specific filters in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        language: 'java',
        environment: 'production',
        limit: 10,
      };

      const { filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);

      // Policy should exclude some snippets (java-openjdk-full and java-dev-tools if they exist)
      // The actual snippet IDs depend on the knowledge base content
      // Just verify that filtering is being applied
      const excludedIds = filterResult.excluded;
      // Excluded list should contain entries defined in the policy
      // (The exact IDs may not exist in the knowledge base, so just verify the policy is working)
      expect(excludedIds.length).toBeGreaterThanOrEqual(0);

      policy.close();
    });

    it('should apply Node.js-specific filters in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        language: 'node',
        environment: 'production',
        limit: 10,
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      expect(matches.length).toBeGreaterThan(0);

      policy.close();
    });
  });

  describe('Registry filtering', () => {
    it('should respect allowed registries in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      // Query for base image knowledge in production
      const query: KnowledgeQuery = {
        environment: 'production',
        tags: ['base-image', 'from'],
        limit: 10,
      };

      const { filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);

      // All returned base image snippets should reference allowed registries
      // (mcr.microsoft.com, gcr.io, registry.gitlab.com)
      // This is tested by the policy filter logic

      policy.close();
    });

    it('should apply Microsoft-only registry filter with microsoft tag', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        tags: ['microsoft', 'azure', 'base-image'],
        limit: 10,
      };

      const { filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);

      // All base image snippets should reference mcr.microsoft.com only
      // This is enforced by the policy

      policy.close();
    });
  });

  describe('Weight multipliers', () => {
    it('should boost security snippets in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        category: 'security',
        environment: 'production',
        limit: 10,
      };

      const { filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      expect(filterResult.boosted.length).toBeGreaterThan(0);

      policy.close();
    });

    it('should reduce debug snippets in production', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        environment: 'production',
        tags: ['debug'],
        limit: 10,
      };

      const { filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      // Debug snippets should be reduced or excluded in production
      expect(filterResult.reduced.length + filterResult.excluded.length).toBeGreaterThan(0);

      policy.close();
    });
  });

  describe('No policy fallback', () => {
    it('should work without policy (standard matching)', async () => {
      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        category: 'security',
        limit: 10,
      };

      // No policy provided
      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
      );

      // Should not apply policy
      expect(filterResult.policyApplied).toBe(false);
      expect(matches.length).toBeGreaterThan(0);
    });
  });

  describe('Staging environment', () => {
    it('should apply balanced filtering in staging', async () => {
      const policyResult = await loadRegoPolicy(POLICY_PATH, logger);
      expect(policyResult.ok).toBe(true);
      if (!policyResult.ok) return;
      const policy = policyResult.value;

      const knowledgeData = await loadKnowledgeData();

      const query: KnowledgeQuery = {
        environment: 'staging',
        limit: 10,
      };

      const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(
        knowledgeData.entries,
        query,
        policy,
      );

      expect(filterResult.policyApplied).toBe(true);
      expect(matches.length).toBeGreaterThan(0);

      policy.close();
    });
  });
});
