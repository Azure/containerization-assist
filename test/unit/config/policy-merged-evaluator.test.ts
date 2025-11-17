/**
 * Unit tests for Merged Policy Evaluator
 */

import { MergedPolicyEvaluator, createMergedEvaluator } from '@/config/policy-merged-evaluator';
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

    it('should deduplicate violations by rule name', async () => {
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
              message: 'Second occurrence (should be filtered)',
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

      // Should only have one violation (first occurrence)
      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].message).toBe('Second occurrence (should be filtered)'); // Higher priority comes first
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
});
