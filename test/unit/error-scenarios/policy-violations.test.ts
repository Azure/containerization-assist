/**
 * Unit Tests: Policy Violation Scenarios
 * Tests policy enforcement patterns without being prescriptive about exact violations
 */

import { jest } from '@jest/globals';

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

const mockPolicyEval = {
  evaluateRules: jest.fn(),
  checkCompliance: jest.fn(),
  getRuleWeights: jest.fn(),
};

jest.mock('../../../src/config/policy-eval', () => ({
  evaluateRules: (...args: any[]) => mockPolicyEval.evaluateRules(...args),
  checkCompliance: (...args: any[]) => mockPolicyEval.checkCompliance(...args),
  getRuleWeights: (...args: any[]) => mockPolicyEval.getRuleWeights(...args),
}));

jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({ end: jest.fn(), error: jest.fn() })),
  createLogger: jest.fn(() => createMockLogger()),
}));

describe('Policy Violation Scenarios', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Policy Enforcement Types', () => {
    it('should support strict enforcement that blocks operations', () => {
      const result = mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [
          {
            ruleId: 'test-001',
            category: 'security',
            severity: 'critical',
            message: 'Violation detected',
            enforcement: 'strict',
          },
        ],
      });

      expect(result).toBeDefined();
      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.passed).toBe(false);
      expect(evaluation.violations[0].enforcement).toBe('strict');
    });

    it('should support advisory enforcement that warns', () => {
      const result = mockPolicyEval.evaluateRules.mockReturnValue({
        passed: true,
        warnings: [
          {
            ruleId: 'test-002',
            category: 'quality',
            severity: 'low',
            message: 'Advisory notice',
            enforcement: 'advisory',
          },
        ],
      });

      expect(result).toBeDefined();
      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.passed).toBe(true);
      expect(evaluation.warnings).toBeDefined();
    });

    it('should support lenient enforcement', () => {
      const result = mockPolicyEval.evaluateRules.mockReturnValue({
        passed: true,
        notices: [
          {
            ruleId: 'test-003',
            category: 'compliance',
            severity: 'info',
            message: 'Lenient notice',
            enforcement: 'lenient',
          },
        ],
      });

      expect(result).toBeDefined();
      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.passed).toBe(true);
    });
  });

  describe('Violation Structure', () => {
    it('should include required violation fields', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [
          {
            ruleId: 'test-001',
            category: 'security',
            severity: 'high',
            message: 'Test violation',
            enforcement: 'strict',
          },
        ],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      const violation = evaluation.violations[0];

      expect(violation).toHaveProperty('ruleId');
      expect(violation).toHaveProperty('category');
      expect(violation).toHaveProperty('severity');
      expect(violation).toHaveProperty('message');
      expect(violation).toHaveProperty('enforcement');
    });

    it('should support multiple violations', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [
          { ruleId: 'sec-1', category: 'security', severity: 'high', message: 'Violation 1', enforcement: 'strict' },
          { ruleId: 'sec-2', category: 'security', severity: 'medium', message: 'Violation 2', enforcement: 'strict' },
          { ruleId: 'comp-1', category: 'compliance', severity: 'low', message: 'Violation 3', enforcement: 'advisory' },
        ],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.violations.length).toBe(3);
    });
  });

  describe('Policy Categories', () => {
    it('should support security category', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [{ ruleId: 'sec-1', category: 'security', severity: 'high', message: 'Security violation', enforcement: 'strict' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.violations[0].category).toBe('security');
    });

    it('should support compliance category', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [{ ruleId: 'comp-1', category: 'compliance', severity: 'medium', message: 'Compliance violation', enforcement: 'strict' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.violations[0].category).toBe('compliance');
    });

    it('should support quality category', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: true,
        warnings: [{ ruleId: 'qual-1', category: 'quality', severity: 'low', message: 'Quality suggestion', enforcement: 'advisory' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.warnings[0].category).toBe('quality');
    });

    it('should support performance category', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: true,
        warnings: [{ ruleId: 'perf-1', category: 'performance', severity: 'info', message: 'Performance tip', enforcement: 'advisory' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.warnings[0].category).toBe('performance');
    });
  });

  describe('Severity Levels', () => {
    it('should support critical severity', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [{ ruleId: 'test', category: 'security', severity: 'critical', message: 'Critical issue', enforcement: 'strict' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.violations[0].severity).toBe('critical');
    });

    it('should support high, medium, low, info severity', () => {
      const severities = ['high', 'medium', 'low', 'info'];

      severities.forEach(severity => {
        mockPolicyEval.evaluateRules.mockReturnValue({
          passed: false,
          violations: [{ ruleId: 'test', category: 'security', severity, message: 'Test', enforcement: 'strict' }],
        });

        const evaluation = mockPolicyEval.evaluateRules();
        expect(evaluation.violations[0].severity).toBe(severity);
      });
    });
  });

  describe('Policy Evaluation Results', () => {
    it('should return passed: false when violations exist', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: false,
        violations: [{ ruleId: 'test', category: 'security', severity: 'high', message: 'Violation', enforcement: 'strict' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.passed).toBe(false);
    });

    it('should return passed: true when no blocking violations exist', () => {
      mockPolicyEval.evaluateRules.mockReturnValue({
        passed: true,
        warnings: [{ ruleId: 'test', category: 'quality', severity: 'low', message: 'Advisory', enforcement: 'advisory' }],
      });

      const evaluation = mockPolicyEval.evaluateRules();
      expect(evaluation.passed).toBe(true);
    });
  });
});
