/**
 * Unit tests for Merged Policy Evaluator
 */

import { MergedPolicyEvaluator, createMergedEvaluator, type PolicyMergeOptions } from '@/config/policy-merged-evaluator';
import type { RegoEvaluator, RegoPolicyResult } from '@/config/policy-rego';
import { createLogger } from '@/lib/logger';
import { Success, Failure } from '@types';

describe('MergedPolicyEvaluator', () => {
  const logger = createLogger({ level: 'silent' });

  // Mock evaluators for testing
  const mockEvaluator1: RegoEvaluator = {
    policyPaths: ['policy1.rego'],
    evaluate: jest.fn(async () => ({
      allow: false,
      violations: [
        {
          rule: 'rule1',
          category: 'security',
          severity: 'block',
          message: 'Violation 1',
          priority: 80,
        },
      ],
      warnings: [],
      suggestions: [],
    })),
    evaluatePolicy: jest.fn(async (result) => {
      if (!result.ok) return result as any;
      return Success({ blocking: [], warnings: [], suggestions: [] });
    }),
    queryConfig: jest.fn(async () => null),
    close: jest.fn(),
  };

  const mockEvaluator2: RegoEvaluator = {
    policyPaths: ['policy2.yaml'],
    evaluate: jest.fn(async () => ({
      allow: true,
      violations: [],
      warnings: [
        {
          rule: 'rule2',
          category: 'optimization',
          severity: 'warn',
          message: 'Warning 1',
          priority: 60,
        },
      ],
      suggestions: [],
    })),
    evaluatePolicy: jest.fn(async (result) => {
      if (!result.ok) return result as any;
      return Success({ blocking: [], warnings: [], suggestions: [] });
    }),
    queryConfig: jest.fn(async () => null),
    close: jest.fn(),
  };

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Constructor', () => {
    it('should create merged evaluator instance', () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      expect(merged).toBeDefined();
      expect(merged.policyPaths).toEqual([]);
    });

    it('should combine policy paths', () => {
      const merged = new MergedPolicyEvaluator(
        [mockEvaluator1, mockEvaluator2],
        logger,
        ['policy1.rego', 'policy2.yaml']
      );

      expect(merged.policyPaths).toHaveLength(2);
    });
  });

  describe('evaluate', () => {
    it('should merge violations from multiple evaluators', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      const result = await merged.evaluate('FROM alpine');

      expect(result.violations).toHaveLength(1);
      expect(result.warnings).toHaveLength(1);
      expect(result.allow).toBe(false); // Blocked due to violation
    });

    it('should evaluate all policies in parallel', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      await merged.evaluate('FROM alpine');

      expect(mockEvaluator1.evaluate).toHaveBeenCalledWith('FROM alpine');
      expect(mockEvaluator2.evaluate).toHaveBeenCalledWith('FROM alpine');
    });

    it('should sort violations by priority (highest first)', async () => {
      const mockHighPriority: RegoEvaluator = {
        policyPaths: ['high.rego'],
        evaluate: jest.fn(async () => ({
          allow: false,
          violations: [
            {
              rule: 'high-priority',
              category: 'security',
              severity: 'block',
              message: 'High',
              priority: 100,
            },
          ],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const mockLowPriority: RegoEvaluator = {
        policyPaths: ['low.rego'],
        evaluate: jest.fn(async () => ({
          allow: false,
          violations: [
            {
              rule: 'low-priority',
              category: 'security',
              severity: 'block',
              message: 'Low',
              priority: 30,
            },
          ],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator([mockLowPriority, mockHighPriority], logger, []);

      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(2);
      expect(result.violations[0].rule).toBe('high-priority');
      expect(result.violations[1].rule).toBe('low-priority');
    });

    it('should deduplicate violations by rule name (smart mode by default)', async () => {
      const mockDuplicate1: RegoEvaluator = {
        policyPaths: ['dup1.rego'],
        evaluate: jest.fn(async () => ({
          allow: false,
          violations: [
            {
              rule: 'duplicate-rule',
              category: 'security',
              severity: 'block',
              message: 'First occurrence',
              priority: 80,
            },
          ],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const mockDuplicate2: RegoEvaluator = {
        policyPaths: ['dup2.rego'],
        evaluate: jest.fn(async () => ({
          allow: false,
          violations: [
            {
              rule: 'duplicate-rule',
              category: 'security',
              severity: 'block',
              message: 'Second occurrence',
              priority: 90,
            },
          ],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator([mockDuplicate1, mockDuplicate2], logger, []);

      const result = await merged.evaluate('test');

      // Should have one merged violation (smart deduplication)
      expect(result.violations).toHaveLength(1);
      // Higher priority comes first, message indicates additional issue
      expect(result.violations[0].message).toContain('Second occurrence');
      expect(result.violations[0].message).toContain('additional issue');
    });

    it('should handle evaluator errors gracefully', async () => {
      const mockFailingEvaluator: RegoEvaluator = {
        policyPaths: ['failing.rego'],
        evaluate: jest.fn(async () => {
          throw new Error('Evaluation failed');
        }),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator([mockFailingEvaluator, mockEvaluator1], logger, []);

      const result = await merged.evaluate('test');

      // Should have error violation + rule1 violation
      expect(result.violations.length).toBeGreaterThan(0);
      expect(result.violations.some((v) => v.rule === 'policy-evaluation-error')).toBe(true);
    });

    it('should categorize by severity', async () => {
      const mockMixed: RegoEvaluator = {
        policyPaths: ['mixed.rego'],
        evaluate: jest.fn(async () => ({
          allow: false,
          violations: [
            {
              rule: 'blocking',
              category: 'security',
              severity: 'block',
              message: 'Block',
            },
          ],
          warnings: [
            {
              rule: 'warning',
              category: 'best-practices',
              severity: 'warn',
              message: 'Warn',
            },
          ],
          suggestions: [
            {
              rule: 'suggestion',
              category: 'optimization',
              severity: 'suggest',
              message: 'Suggest',
            },
          ],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator([mockMixed], logger, []);

      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      expect(result.warnings).toHaveLength(1);
      expect(result.suggestions).toHaveLength(1);
    });

    it('should include summary in results', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      const result = await merged.evaluate('test');

      expect(result.summary).toBeDefined();
      expect(result.summary?.total_violations).toBe(1);
      expect(result.summary?.total_warnings).toBe(1);
      expect(result.summary?.total_suggestions).toBe(0);
    });

    it('should handle empty evaluator list', async () => {
      const merged = new MergedPolicyEvaluator([], logger, []);

      const result = await merged.evaluate('test');

      expect(result.allow).toBe(true);
      expect(result.violations).toHaveLength(0);
      expect(result.warnings).toHaveLength(0);
      expect(result.suggestions).toHaveLength(0);
    });
  });

  describe('evaluatePolicy', () => {
    it('should evaluate with Success Result wrapper', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1], logger, []);

      const inputResult = Success('FROM alpine');
      const result = await merged.evaluatePolicy(inputResult);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.blocking).toBeDefined();
        expect(result.value.warnings).toBeDefined();
        expect(result.value.suggestions).toBeDefined();
      }
    });

    it('should pass through Failure Result', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1], logger, []);

      const inputResult = Failure('Test error');
      const result = await merged.evaluatePolicy(inputResult);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Test error');
      }
    });
  });

  describe('queryConfig', () => {
    it('should return config from first evaluator that returns non-null', async () => {
      const mockWithConfig: RegoEvaluator = {
        policyPaths: ['with-config.rego'],
        evaluate: jest.fn(async () => ({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => ({ baseImage: 'alpine:3.20' })),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockWithConfig], logger, []);

      const config = await merged.queryConfig('test.config', {});

      expect(config).toEqual({ baseImage: 'alpine:3.20' });
      expect(mockWithConfig.queryConfig).toHaveBeenCalled();
    });

    it('should return null if no evaluator returns config', async () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      const config = await merged.queryConfig('test.config', {});

      expect(config).toBeNull();
    });

    it('should handle queryConfig errors and continue to next evaluator', async () => {
      const mockFailingConfig: RegoEvaluator = {
        policyPaths: ['failing-config.rego'],
        evaluate: jest.fn(async () => ({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => {
          throw new Error('Config query failed');
        }),
        close: jest.fn(),
      };

      const mockWithConfig: RegoEvaluator = {
        policyPaths: ['with-config.rego'],
        evaluate: jest.fn(async () => ({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [],
        })),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => ({ baseImage: 'ubuntu:22.04' })),
        close: jest.fn(),
      };

      const merged = new MergedPolicyEvaluator(
        [mockFailingConfig, mockWithConfig],
        logger,
        []
      );

      const config = await merged.queryConfig('test.config', {});

      expect(config).toEqual({ baseImage: 'ubuntu:22.04' });
    });
  });

  describe('close', () => {
    it('should close all evaluators', () => {
      const merged = new MergedPolicyEvaluator([mockEvaluator1, mockEvaluator2], logger, []);

      merged.close();

      expect(mockEvaluator1.close).toHaveBeenCalled();
      expect(mockEvaluator2.close).toHaveBeenCalled();
    });

    it('should handle close errors gracefully', () => {
      const mockFailingClose: RegoEvaluator = {
        policyPaths: ['failing.rego'],
        evaluate: jest.fn(),
        evaluatePolicy: jest.fn(),
        queryConfig: jest.fn(async () => null),
        close: jest.fn(() => {
          throw new Error('Close failed');
        }),
      };

      const merged = new MergedPolicyEvaluator([mockFailingClose, mockEvaluator1], logger, []);

      // Should not throw
      expect(() => merged.close()).not.toThrow();
      expect(mockFailingClose.close).toHaveBeenCalled();
      expect(mockEvaluator1.close).toHaveBeenCalled();
    });
  });
});

describe('createMergedEvaluator', () => {
  const logger = createLogger({ level: 'silent' });

  it('should create merged evaluator from array', () => {
    const mockEvaluator: RegoEvaluator = {
      policyPaths: ['test.rego'],
      evaluate: jest.fn(),
      evaluatePolicy: jest.fn(),
      queryConfig: jest.fn(async () => null),
      close: jest.fn(),
    };

    const merged = createMergedEvaluator([mockEvaluator], logger);

    expect(merged).toBeInstanceOf(MergedPolicyEvaluator);
    expect(merged.policyPaths).toEqual(['test.rego']);
  });

  it('should combine policy paths from all evaluators', () => {
    const mockEvaluator1: RegoEvaluator = {
      policyPaths: ['policy1.rego', 'policy2.rego'],
      evaluate: jest.fn(),
      evaluatePolicy: jest.fn(),
      queryConfig: jest.fn(async () => null),
      close: jest.fn(),
    };

    const mockEvaluator2: RegoEvaluator = {
      policyPaths: ['policy3.yaml'],
      evaluate: jest.fn(),
      evaluatePolicy: jest.fn(),
      queryConfig: jest.fn(async () => null),
      close: jest.fn(),
    };

    const merged = createMergedEvaluator([mockEvaluator1, mockEvaluator2], logger);

    expect(merged.policyPaths).toEqual(['policy1.rego', 'policy2.rego', 'policy3.yaml']);
  });

  it('should handle empty evaluator array', () => {
    const merged = createMergedEvaluator([], logger);

    expect(merged).toBeInstanceOf(MergedPolicyEvaluator);
    expect(merged.policyPaths).toEqual([]);
  });

  it('should pass options to merged evaluator', () => {
    const mockEvaluator: RegoEvaluator = {
      policyPaths: ['test.rego'],
      evaluate: jest.fn(),
      evaluatePolicy: jest.fn(),
      queryConfig: jest.fn(async () => null),
      close: jest.fn(),
    };

    const merged = createMergedEvaluator([mockEvaluator], logger, { deduplication: 'strict' });

    expect(merged).toBeInstanceOf(MergedPolicyEvaluator);
  });
});

describe('Policy Deduplication', () => {
  const logger = createLogger({ level: 'silent' });

  // Helper function to create mock evaluators
  const createMockEvaluator = (
    name: string,
    violations: Array<{
      rule: string;
      category: string;
      severity: 'block' | 'warn' | 'suggest';
      message: string;
      description?: string;
      priority?: number;
    }>
  ): RegoEvaluator => ({
    policyPaths: [name],
    evaluate: jest.fn(async () => ({
      allow: violations.length === 0,
      violations: violations.filter((v) => v.severity === 'block'),
      warnings: violations.filter((v) => v.severity === 'warn'),
      suggestions: violations.filter((v) => v.severity === 'suggest'),
    })),
    evaluatePolicy: jest.fn(async (result) => {
      if (!result.ok) return result as any;
      return Success({ blocking: [], warnings: [], suggestions: [] });
    }),
    queryConfig: jest.fn(async () => null),
    close: jest.fn(),
  });

  describe('Smart Deduplication (Default)', () => {
    it('should keep unique violations unchanged', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'rule-a', category: 'test', severity: 'block', message: 'Issue A' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'rule-b', category: 'test', severity: 'block', message: 'Issue B' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(2);
      expect(result.violations.map((v) => v.rule)).toContain('rule-a');
      expect(result.violations.map((v) => v.rule)).toContain('rule-b');
    });

    it('should deduplicate identical violations', async () => {
      const violation = {
        rule: 'same-rule',
        category: 'test',
        severity: 'block' as const,
        message: 'Identical message',
        description: 'Same description',
      };

      const evaluator1 = createMockEvaluator('policy1', [violation]);
      const evaluator2 = createMockEvaluator('policy2', [violation]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].rule).toBe('same-rule');
      expect(result.violations[0].message).toBe('Identical message');
    });

    it('should merge violations with different messages', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'First message',
          description: 'First description',
          priority: 80,
        },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Second message',
          description: 'Second description',
          priority: 60,
        },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].rule).toBe('same-rule');
      expect(result.violations[0].message).toContain('additional issue');
      expect(result.violations[0].description).toContain('First description');
      expect(result.violations[0].description).toContain('Second description');
      expect(result.violations[0].priority).toBe(80); // Higher priority kept
    });

    it('should respect priority when merging', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        {
          rule: 'same-rule',
          category: 'low-priority',
          severity: 'block',
          message: 'Low priority message',
          priority: 40,
        },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        {
          rule: 'same-rule',
          category: 'high-priority',
          severity: 'block',
          message: 'High priority message',
          priority: 90,
        },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Category should come from higher priority violation
      expect(result.violations[0].category).toBe('high-priority');
      expect(result.violations[0].priority).toBe(90);
    });

    it('should handle violations with different descriptions but same message', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Same message',
          description: 'Description from Rego policy',
        },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Same message',
          description: 'Description from CEL policy',
        },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Should merge descriptions with separator
      expect(result.violations[0].description).toContain('Description from Rego policy');
      expect(result.violations[0].description).toContain('Description from CEL policy');
      expect(result.violations[0].description).toContain('---');
    });

    it('should handle multiple additional issues in message', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'First' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Second' },
      ]);
      const evaluator3 = createMockEvaluator('policy3', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Third' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2, evaluator3], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Should say "issues" (plural) for multiple additional
      expect(result.violations[0].message).toContain('2 additional issues');
    });
  });

  describe('Strict Deduplication', () => {
    it('should keep only first occurrence per rule name', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'First occurrence',
          priority: 50,
        },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Second occurrence (should be filtered)',
          priority: 100, // Even with higher priority, this should be filtered in strict mode
        },
      ]);

      const merged = new MergedPolicyEvaluator(
        [evaluator1, evaluator2],
        logger,
        [],
        { deduplication: 'strict' }
      );
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Due to priority sorting, higher priority comes first, then strict mode keeps it
      expect(result.violations[0].message).toBe('Second occurrence (should be filtered)');
    });

    it('should not merge descriptions in strict mode', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Message 1',
          description: 'Description 1',
        },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        {
          rule: 'same-rule',
          category: 'test',
          severity: 'block',
          message: 'Message 2',
          description: 'Description 2',
        },
      ]);

      const merged = new MergedPolicyEvaluator(
        [evaluator1, evaluator2],
        logger,
        [],
        { deduplication: 'strict' }
      );
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Description should NOT be merged
      expect(result.violations[0].description).not.toContain('---');
    });
  });

  describe('No Deduplication', () => {
    it('should keep all violations including duplicates', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Message 1' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Message 2' },
      ]);

      const merged = new MergedPolicyEvaluator(
        [evaluator1, evaluator2],
        logger,
        [],
        { deduplication: 'none' }
      );
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(2);
      expect(result.violations.map((v) => v.message)).toContain('Message 1');
      expect(result.violations.map((v) => v.message)).toContain('Message 2');
    });
  });

  describe('Warnings and Suggestions', () => {
    it('should apply deduplication to warnings', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'warning-rule', category: 'test', severity: 'warn', message: 'Warning 1' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'warning-rule', category: 'test', severity: 'warn', message: 'Warning 2' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.warnings).toHaveLength(1);
      expect(result.warnings[0].message).toContain('additional issue');
    });

    it('should apply deduplication to suggestions', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'suggest-rule', category: 'test', severity: 'suggest', message: 'Suggest 1' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'suggest-rule', category: 'test', severity: 'suggest', message: 'Suggest 2' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.suggestions).toHaveLength(1);
      expect(result.suggestions[0].message).toContain('additional issue');
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty description arrays', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Message 1' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'same-rule', category: 'test', severity: 'block', message: 'Message 2' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // No description should be set
      expect(result.violations[0].description).toBeUndefined();
    });

    it('should handle violations with default priority', async () => {
      const evaluator1 = createMockEvaluator('policy1', [
        { rule: 'same-rule', category: 'test1', severity: 'block', message: 'No priority 1' },
      ]);
      const evaluator2 = createMockEvaluator('policy2', [
        { rule: 'same-rule', category: 'test2', severity: 'block', message: 'No priority 2' },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator1, evaluator2], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      // Should still merge even with default priorities
      expect(result.violations[0].message).toContain('additional issue');
    });

    it('should handle single violation without modification', async () => {
      const evaluator = createMockEvaluator('policy', [
        {
          rule: 'unique-rule',
          category: 'test',
          severity: 'block',
          message: 'Unique message',
          description: 'Unique description',
        },
      ]);

      const merged = new MergedPolicyEvaluator([evaluator], logger, []);
      const result = await merged.evaluate('test');

      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].message).toBe('Unique message');
      expect(result.violations[0].description).toBe('Unique description');
    });
  });
});
