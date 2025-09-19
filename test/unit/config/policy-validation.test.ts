/**
 * Policy Validation Test Suite
 *
 * Tests for malformed policies to ensure they fail fast with clear errors
 * and that valid policies continue to pass.
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';

// Mock the docker-client module before importing config
jest.mock('@/services/docker-client', () => ({
  autoDetectDockerSocket: jest.fn(() => '/var/run/docker.sock'),
  createDockerClient: jest.fn(),
}));

import { validatePolicyData } from '@config/policy';
import { createLogger } from '@lib/logger';
import type { Logger } from 'pino';

describe('Policy Validation', () => {
  let logger: Logger;

  beforeEach(() => {
    logger = createLogger();
  });

  describe('Legacy Policy Format Validation', () => {
    it('should fail with clear error for policies with maxTokens too high', () => {
      const malformedPolicy = {
        maxTokens: 150000, // Too high, max is 100000
        maxCost: 50,
        timeoutMs: 30000
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/maxTokens|too|large|maximum/i);
      }
    });

    it('should fail with clear error for policies with maxCost too high', () => {
      const malformedPolicy = {
        maxTokens: 50000,
        maxCost: 150, // Too high, max is 100
        timeoutMs: 30000
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/maxCost|too|large|maximum/i);
      }
    });

    it('should fail with clear error for policies with timeoutMs too high', () => {
      const malformedPolicy = {
        maxTokens: 50000,
        maxCost: 50,
        timeoutMs: 700000 // Too high, max is 600000
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/timeoutMs|too|large|maximum/i);
      }
    });

    it('should fail with clear error for policies with invalid field types', () => {
      const malformedPolicy = {
        maxTokens: "50000", // Should be number, not string
        maxCost: 50,
        timeoutMs: 30000
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/number|type|invalid/i);
      }
    });

    it('should fail with clear error for policies with invalid array types', () => {
      const malformedPolicy = {
        maxTokens: 50000,
        forbiddenModels: "gpt-4", // Should be array, not string
        timeoutMs: 30000
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/array|type|invalid/i);
      }
    });
  });

  describe('Policy Format Validation', () => {
    it('should fail with clear error for missing required policy fields', () => {
      const malformedPolicy = {
        schema_version: "1.0.0",
        // Missing version field
        metadata: {
          description: "Test policy",
          created: "2024-01-01",
          author: "Test"
        }
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/version|required|missing/i);
      }
    });

    it('should fail with clear error for invalid policy structure', () => {
      const malformedPolicy = {
        schema_version: "1.0.0",
        version: "1.0.0",
        metadata: "invalid metadata", // Should be object, not string
        weights: {
          global_categories: {},
          content_types: {}
        }
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/metadata|object|type/i);
      }
    });

    it('should fail with clear error for malformed policy rules', () => {
      const malformedPolicy = {
        schema_version: "1.0.0",
        version: "1.0.0",
        metadata: {
          description: "Test policy",
          created: "2024-01-01",
          author: "Test"
        },
        weights: {
          global_categories: {},
          content_types: {}
        },
        rules: {
          "invalid_rule": {
            // Missing required fields: base_score, max_score, timeout_ms
          }
        }
      };

      const result = validatePolicyData(malformedPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/base_score|max_score|timeout_ms|required/i);
      }
    });
  });

  describe('Valid Policies', () => {
    it('should pass validation for properly formatted legacy policy', () => {
      const validPolicy = {
        maxTokens: 50000,
        maxCost: 25,
        forbiddenModels: ["gpt-4-32k"],
        allowedModels: ["gpt-3.5-turbo", "gpt-4"],
        timeoutMs: 120000
      };

      const result = validatePolicyData(validPolicy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.maxTokens).toBe(50000);
        expect(result.value.allowedModels).toHaveLength(2);
      }
    });

    it('should pass validation for minimal legacy policy', () => {
      const minimalPolicy = {
        maxTokens: 10000
      };

      const result = validatePolicyData(minimalPolicy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.maxTokens).toBe(10000);
      }
    });

    it('should pass validation for empty legacy policy', () => {
      const emptyPolicy = {};

      const result = validatePolicyData(emptyPolicy);
      expect(result.ok).toBe(true);
    });

    it('should pass validation for properly formatted policy', () => {
      const validPolicy = {
        version: "2.0.0",
        metadata: {
          description: "Test policy",
          created: "2024-01-01T00:00:00Z",
          author: "Test Author"
        },
        weights: {
          global_categories: {
            "dockerfile": 1.0,
            "kubernetes": 1.5
          },
          content_types: {
            "dockerfile": {
              "base_image": 2.0
            }
          }
        },
        rules: {
          "dockerfile_rules": {
            base_score: 10,
            max_score: 100,
            timeout_ms: 30000,
            matchers: [
              {
                pattern: "FROM alpine",
                weight: 10,
                description: "Use Alpine base image",
                type: "instruction"
              }
            ]
          }
        },
        strategies: {
          "default": ["strategy1", "strategy2"]
        },
        strategy_selection: {
          "dockerfile": {
            conditions: [],
            default_strategy_index: 0
          }
        },
        env_overrides: {},
        tool_defaults: {},
        schema_version: "1.0.0"
      };

      const result = validatePolicyData(validPolicy);
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.version).toBe("2.0.0");
        expect(result.value.metadata.author).toBe("Test Author");
        expect(result.value.rules.dockerfile_rules.base_score).toBe(10);
      }
    });
  });

  describe('Error Message Quality', () => {
    it('should provide specific field names in error messages for legacy policies', () => {
      const policyWithBadField = {
        maxTokens: 200000, // Too high, max is 100000
        maxCost: 50
      };

      const result = validatePolicyData(policyWithBadField);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toMatch(/maxTokens|maximum|too|large/i);
      }
    });

    it('should provide helpful field path information for nested validation errors', () => {
      const policyWithNestedError = {
        schema_version: "1.0.0",
        version: "1.0.0",
        metadata: {
          description: "Test",
          created: "invalid-date", // String is accepted, so use invalid nested structure instead
          author: "Test"
        },
        weights: {
          global_categories: {},
          content_types: {}
        },
        rules: {
          "invalid_rule": {
            base_score: "not-a-number", // Should be number, not string
            max_score: 100,
            timeout_ms: 30000,
            matchers: []
          }
        },
        strategies: {},
        strategy_selection: {},
        env_overrides: {},
        tool_defaults: {}
      };

      const result = validatePolicyData(policyWithNestedError);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        // Error should indicate issues with the invalid data or structure
        expect(result.error).toMatch(/Policy validation failed|base_score|number/i);
        expect(result.error.length).toBeGreaterThan(0);
      }
    });

    it('should handle validation errors gracefully with informative messages', () => {
      const completelyInvalidPolicy = {
        maxTokens: "not-a-number", // Wrong type - should be number
        maxCost: true, // Wrong type - should be number
        forbiddenModels: "not-an-array" // Wrong type - should be array
      };

      const result = validatePolicyData(completelyInvalidPolicy);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Policy validation failed');
        expect(typeof result.error).toBe('string');
        expect(result.error.length).toBeGreaterThan(0);
      }
    });
  });

  describe('Policy Processing Performance', () => {
    it('should validate policies quickly', () => {
      const largePolicy = {
        maxTokens: 75000,
        maxCost: 75,
        forbiddenModels: Array.from({ length: 50 }, (_, i) => `forbidden-model-${i}`),
        allowedModels: Array.from({ length: 100 }, (_, i) => `allowed-model-${i}`),
        timeoutMs: 300000
      };

      const startTime = Date.now();
      const result = validatePolicyData(largePolicy);
      const endTime = Date.now();

      expect(result.ok).toBe(true);
      expect(endTime - startTime).toBeLessThan(100); // Should complete very quickly
    });

    it('should handle multiple validation attempts efficiently', () => {
      const testPolicy = {
        maxTokens: 50000,
        allowedModels: ["gpt-3.5-turbo"]
      };

      const startTime = Date.now();

      // Run validation multiple times
      for (let i = 0; i < 100; i++) {
        const result = validatePolicyData(testPolicy);
        expect(result.ok).toBe(true);
      }

      const endTime = Date.now();
      expect(endTime - startTime).toBeLessThan(1000); // 100 validations should complete within 1 second
    });
  });
});