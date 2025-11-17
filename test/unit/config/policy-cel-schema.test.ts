/**
 * Unit tests for CEL Policy Schema
 */

import {
  celPolicySetSchema,
  celPolicyRuleSchema,
  celPolicyMetadataSchema,
  celPolicySpecSchema,
  validateCelPolicySet,
  formatValidationErrors,
  type CelPolicySet,
  type CelPolicyRule,
} from '@/config/policy-cel-schema';
import { z } from 'zod';

describe('CEL Policy Schema', () => {
  describe('celPolicyRuleSchema', () => {
    it('should validate a minimal valid rule', () => {
      const rule = {
        name: 'test-rule',
        category: 'security',
        severity: 'block',
        condition: 'true',
        message: 'Test message',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.name).toBe('test-rule');
        expect(result.data.priority).toBe(50); // Default priority
      }
    });

    it('should validate a complete rule with all fields', () => {
      const rule = {
        name: 'require-user',
        category: 'security',
        severity: 'warn',
        priority: 90,
        condition: '!input.content.contains("USER")',
        message: 'Dockerfile should specify USER',
        description: 'Running as root is a security risk',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.priority).toBe(90);
        expect(result.data.description).toBe('Running as root is a security risk');
      }
    });

    it('should reject rule with invalid severity', () => {
      const rule = {
        name: 'test',
        category: 'security',
        severity: 'critical', // Invalid
        condition: 'true',
        message: 'Test',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.errors[0].message).toContain('Severity must be');
      }
    });

    it('should reject rule with missing required fields', () => {
      const rule = {
        name: 'test',
        category: 'security',
        severity: 'block',
        // Missing condition and message
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(false);
      if (!result.success) {
        const errors = result.error.errors.map((e) => e.path[0]);
        expect(errors).toContain('condition');
        expect(errors).toContain('message');
      }
    });

    it('should reject rule with empty name', () => {
      const rule = {
        name: '',
        category: 'security',
        severity: 'block',
        condition: 'true',
        message: 'Test',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(false);
    });

    it('should reject rule with priority out of bounds (> 100)', () => {
      const rule = {
        name: 'test',
        category: 'security',
        severity: 'block',
        priority: 150,
        condition: 'true',
        message: 'Test',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(false);
    });

    it('should reject rule with priority out of bounds (< 0)', () => {
      const rule = {
        name: 'test',
        category: 'security',
        severity: 'block',
        priority: -10,
        condition: 'true',
        message: 'Test',
      };

      const result = celPolicyRuleSchema.safeParse(rule);
      expect(result.success).toBe(false);
    });

    it('should accept all valid severity levels', () => {
      const severities: Array<'block' | 'warn' | 'suggest'> = ['block', 'warn', 'suggest'];

      for (const severity of severities) {
        const rule = {
          name: 'test',
          category: 'security',
          severity,
          condition: 'true',
          message: 'Test',
        };

        const result = celPolicyRuleSchema.safeParse(rule);
        expect(result.success).toBe(true);
      }
    });
  });

  describe('celPolicyMetadataSchema', () => {
    it('should validate minimal metadata', () => {
      const metadata = {
        name: 'test-policy',
      };

      const result = celPolicyMetadataSchema.safeParse(metadata);
      expect(result.success).toBe(true);
    });

    it('should validate complete metadata', () => {
      const metadata = {
        name: 'security-policy',
        version: '1.0.0',
        description: 'Security validation rules',
      };

      const result = celPolicyMetadataSchema.safeParse(metadata);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.version).toBe('1.0.0');
        expect(result.data.description).toBe('Security validation rules');
      }
    });

    it('should reject metadata with empty name', () => {
      const metadata = {
        name: '',
      };

      const result = celPolicyMetadataSchema.safeParse(metadata);
      expect(result.success).toBe(false);
    });
  });

  describe('celPolicySpecSchema', () => {
    it('should validate spec with single rule', () => {
      const spec = {
        rules: [
          {
            name: 'test',
            category: 'security',
            severity: 'block',
            condition: 'true',
            message: 'Test',
          },
        ],
      };

      const result = celPolicySpecSchema.safeParse(spec);
      expect(result.success).toBe(true);
    });

    it('should validate spec with multiple rules', () => {
      const spec = {
        rules: [
          {
            name: 'rule1',
            category: 'security',
            severity: 'block',
            condition: 'true',
            message: 'Test 1',
          },
          {
            name: 'rule2',
            category: 'optimization',
            severity: 'warn',
            condition: 'false',
            message: 'Test 2',
          },
        ],
      };

      const result = celPolicySpecSchema.safeParse(spec);
      expect(result.success).toBe(true);
      if (result.success) {
        expect(result.data.rules).toHaveLength(2);
      }
    });

    it('should reject spec with empty rules array', () => {
      const spec = {
        rules: [],
      };

      const result = celPolicySpecSchema.safeParse(spec);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.errors[0].message).toContain('At least one rule required');
      }
    });
  });

  describe('celPolicySetSchema', () => {
    it('should validate a complete valid policy set', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: {
          name: 'test-policy',
          version: '1.0.0',
          description: 'Test policy set',
        },
        spec: {
          rules: [
            {
              name: 'require-user',
              category: 'security',
              severity: 'block',
              priority: 100,
              condition: '!input.content.contains("USER")',
              message: 'USER directive required',
              description: 'Running as root is dangerous',
            },
          ],
        },
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(true);
    });

    it('should validate a minimal valid policy set', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: {
          name: 'minimal',
        },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(true);
    });

    it('should reject policy set with wrong apiVersion', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v2', // Wrong version
        kind: 'PolicySet',
        metadata: { name: 'test' },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.errors[0].message).toContain('Invalid literal value');
      }
    });

    it('should reject policy set with wrong kind', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'Policy', // Wrong kind
        metadata: { name: 'test' },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(false);
    });

    it('should reject policy set with missing metadata', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        // Missing metadata
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(false);
    });

    it('should reject policy set with missing spec', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'test' },
        // Missing spec
      };

      const result = celPolicySetSchema.safeParse(policySet);
      expect(result.success).toBe(false);
    });
  });

  describe('validateCelPolicySet', () => {
    it('should return success for valid policy set', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'test' },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = validateCelPolicySet(policySet);
      expect(result.success).toBe(true);
      expect(result.data).toBeDefined();
      expect(result.errors).toBeUndefined();
    });

    it('should return errors for invalid policy set', () => {
      const policySet = {
        apiVersion: 'wrong-version',
        kind: 'PolicySet',
        metadata: { name: 'test' },
        spec: {
          rules: [],
        },
      };

      const result = validateCelPolicySet(policySet);
      expect(result.success).toBe(false);
      expect(result.data).toBeUndefined();
      expect(result.errors).toBeDefined();
    });

    it('should handle completely invalid data', () => {
      const result = validateCelPolicySet('not an object');
      expect(result.success).toBe(false);
      expect(result.errors).toBeDefined();
    });

    it('should handle null input', () => {
      const result = validateCelPolicySet(null);
      expect(result.success).toBe(false);
    });

    it('should handle undefined input', () => {
      const result = validateCelPolicySet(undefined);
      expect(result.success).toBe(false);
    });
  });

  describe('formatValidationErrors', () => {
    it('should format single error', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: '' }, // Invalid: empty name
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'suggest',
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = validateCelPolicySet(policySet);
      if (!result.success && result.errors) {
        const formatted = formatValidationErrors(result.errors);
        expect(formatted).toContain('metadata.name');
        expect(typeof formatted).toBe('string');
      }
    });

    it('should format multiple errors', () => {
      const policySet = {
        apiVersion: 'wrong',
        kind: 'wrong',
        metadata: { name: '' },
        spec: { rules: [] },
      };

      const result = validateCelPolicySet(policySet);
      if (!result.success && result.errors) {
        const formatted = formatValidationErrors(result.errors);
        expect(formatted.length).toBeGreaterThan(0);
        expect(formatted).toContain(','); // Multiple errors separated by comma
      }
    });

    it('should include field paths in formatted errors', () => {
      const policySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: { name: 'test' },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'test',
              severity: 'invalid', // Invalid severity
              condition: 'true',
              message: 'Test',
            },
          ],
        },
      };

      const result = validateCelPolicySet(policySet);
      if (!result.success && result.errors) {
        const formatted = formatValidationErrors(result.errors);
        expect(formatted).toContain('spec.rules.0.severity');
      }
    });
  });

  describe('Type inference', () => {
    it('should correctly infer CelPolicySet type', () => {
      const policySet: CelPolicySet = {
        apiVersion: 'policy.containerization-assist.dev/v1',
        kind: 'PolicySet',
        metadata: {
          name: 'test',
          version: '1.0.0',
          description: 'Test',
        },
        spec: {
          rules: [
            {
              name: 'test',
              category: 'security',
              severity: 'block',
              priority: 75,
              condition: 'true',
              message: 'Test message',
              description: 'Test description',
            },
          ],
        },
      };

      // Type checking should pass at compile time
      expect(policySet.metadata.name).toBe('test');
      expect(policySet.spec.rules).toHaveLength(1);
      expect(policySet.spec.rules[0].priority).toBe(75);
    });

    it('should correctly infer CelPolicyRule type', () => {
      const rule: CelPolicyRule = {
        name: 'test-rule',
        category: 'security',
        severity: 'warn',
        priority: 60,
        condition: 'input.content.contains("test")',
        message: 'Test message',
        description: 'Test description',
      };

      // Type checking should pass at compile time
      expect(rule.name).toBe('test-rule');
      expect(rule.severity).toBe('warn');
    });
  });
});
