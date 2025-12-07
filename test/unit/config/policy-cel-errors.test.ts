/**
 * Unit tests for CEL Policy Error Formatting Utilities
 */

import {
  formatCompilationError,
  formatEvaluationError,
  formatPolicyFileError,
  formatSchemaError,
  detectCommonMistake,
  type CelErrorDetails,
} from '@/config/policy-cel-errors';
import type { CelPolicyRule } from '@/config/policy-cel-schema';

describe('CEL Error Formatting', () => {
  const sampleRule: CelPolicyRule = {
    name: 'test-rule',
    category: 'security',
    severity: 'block',
    priority: 80,
    condition: 'input.content.contains("test")',
    message: 'Test rule message',
    description: 'Test rule description',
  };

  describe('formatCompilationError', () => {
    it('should format error with rule context', () => {
      const error = new Error('Unexpected token @');
      const result = formatCompilationError(sampleRule, error);

      expect(result.message).toBe('CEL expression compilation failed');
      expect(result.hint).toContain('test-rule');
      expect(result.hint).toContain('security');
      expect(result.hint).toContain('block');
      expect(result.hint).toContain('input.content.contains');
      expect(result.hint).toContain('Unexpected token @');
      expect(result.resolution).toContain('CEL language reference');
    });

    it('should detect common mistake: contains is not a function', () => {
      const error = new Error('contains is not a function');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('input.content.contains');
      expect(result.resolution).not.toBe('Verify CEL expression syntax and try again');
    });

    it('should detect common mistake: undefined', () => {
      const error = new Error('undefined field access');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('input.content');
      expect(result.resolution).toContain('valid fields');
    });

    it('should detect common mistake: unexpected token', () => {
      const error = new Error('Unexpected token in expression');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('typos');
      expect(result.resolution).toContain('invalid operators');
    });

    it('should detect common mistake: type mismatch', () => {
      const error = new Error('Type mismatch in comparison');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('compatible types');
    });

    it('should detect common mistake: is not defined', () => {
      const error = new Error('variable is not defined');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('typos');
    });

    it('should format multi-line conditions with line numbers', () => {
      const multiLineRule: CelPolicyRule = {
        ...sampleRule,
        condition: 'input.content.contains("FROM") &&\n!input.content.contains("USER")',
      };

      const error = new Error('Syntax error on line 2');
      const result = formatCompilationError(multiLineRule, error);

      expect(result.hint).toContain(' 1|');
      expect(result.hint).toContain(' 2|');
      expect(result.hint).toContain('FROM');
      expect(result.hint).toContain('USER');
    });

    it('should format single-line conditions without line numbers', () => {
      const error = new Error('Some error');
      const result = formatCompilationError(sampleRule, error);

      // Should be indented but not have line numbers
      expect(result.hint).toContain('  input.content.contains');
      expect(result.hint).not.toContain('1|');
    });

    it('should fallback to generic suggestion for unknown errors', () => {
      const error = new Error('Some completely unknown error type');
      const result = formatCompilationError(sampleRule, error);

      expect(result.resolution).toContain('Verify CEL expression syntax');
    });
  });

  describe('formatEvaluationError', () => {
    it('should format error with rule context', () => {
      const error = new Error('Cannot read property of undefined');
      const result = formatEvaluationError(sampleRule, error);

      expect(result.message).toBe('CEL expression evaluation failed');
      expect(result.hint).toContain('test-rule');
      expect(result.hint).toContain('security');
      expect(result.hint).toContain('Cannot read property');
    });

    it('should include available fields in resolution', () => {
      const error = new Error('Runtime error');
      const result = formatEvaluationError(sampleRule, error);

      expect(result.resolution).toContain('input.content');
      expect(result.resolution).toContain('Available fields');
    });

    it('should link to policy examples', () => {
      const error = new Error('Runtime error');
      const result = formatEvaluationError(sampleRule, error);

      expect(result.resolution).toContain('policies.user.examples/cel/');
    });

    it('should format multi-line conditions', () => {
      const multiLineRule: CelPolicyRule = {
        ...sampleRule,
        condition: 'line1 &&\nline2 &&\nline3',
      };

      const error = new Error('Error');
      const result = formatEvaluationError(multiLineRule, error);

      expect(result.hint).toContain(' 1|');
      expect(result.hint).toContain(' 2|');
      expect(result.hint).toContain(' 3|');
    });
  });

  describe('formatPolicyFileError', () => {
    const testPath = '/path/to/policy.yaml';

    describe('read phase', () => {
      it('should format read errors', () => {
        const error = new Error('ENOENT: no such file');
        const result = formatPolicyFileError(testPath, 'read', error);

        expect(result.message).toContain('read policy file');
        expect(result.hint).toContain(testPath);
        expect(result.hint).toContain('ENOENT');
        expect(result.resolution).toContain('exists');
        expect(result.resolution).toContain('readable');
      });
    });

    describe('parse phase', () => {
      it('should format parse errors with YAML tips', () => {
        const error = new Error('Invalid YAML at line 5');
        const result = formatPolicyFileError(testPath, 'parse', error);

        expect(result.message).toContain('parse policy YAML');
        expect(result.hint).toContain('Invalid YAML');
        expect(result.resolution).toContain('indentation');
        expect(result.resolution).toContain('spaces, not tabs');
        expect(result.resolution).toContain('Quote strings');
      });
    });

    describe('validate phase', () => {
      it('should format validate errors with structure guide', () => {
        const error = new Error('Missing metadata.name');
        const result = formatPolicyFileError(testPath, 'validate', error);

        expect(result.message).toContain('structure is invalid');
        expect(result.hint).toContain('metadata.name');
        expect(result.resolution).toContain('apiVersion');
        expect(result.resolution).toContain('kind: PolicySet');
        expect(result.resolution).toContain('metadata');
        expect(result.resolution).toContain('spec');
        expect(result.resolution).toContain('examples');
      });
    });

    describe('compile phase', () => {
      it('should format compile errors', () => {
        const error = new Error('Invalid CEL syntax');
        const result = formatPolicyFileError(testPath, 'compile', error);

        expect(result.message).toContain('compile CEL');
        expect(result.hint).toContain('Compilation error');
        expect(result.hint).toContain(testPath);
        expect(result.resolution).toContain('CEL expression syntax');
        expect(result.resolution).toContain('cel-spec');
      });
    });
  });

  describe('formatSchemaError', () => {
    it('should format error with field path', () => {
      const result = formatSchemaError(['spec', 'rules', '0', 'severity'], 'Invalid enum value');

      expect(result).toBe('spec.rules.0.severity: Invalid enum value');
    });

    it('should include value when provided', () => {
      const result = formatSchemaError(
        ['spec', 'rules', '0', 'severity'],
        'Invalid enum value',
        'error'
      );

      expect(result).toBe('spec.rules.0.severity: Invalid enum value (got: "error")');
    });

    it('should handle nested objects in value', () => {
      const result = formatSchemaError(['metadata'], 'Missing required fields', { name: null });

      expect(result).toBe('metadata: Missing required fields (got: {"name":null})');
    });

    it('should handle empty field path', () => {
      const result = formatSchemaError([], 'Invalid root structure');

      expect(result).toBe('root: Invalid root structure');
    });

    it('should handle single field path', () => {
      const result = formatSchemaError(['apiVersion'], 'Required');

      expect(result).toBe('apiVersion: Required');
    });
  });

  describe('detectCommonMistake', () => {
    it('should detect "contains is not a function"', () => {
      const result = detectCommonMistake('TypeError: contains is not a function');

      expect(result).toContain('input.content.contains');
    });

    it('should detect "undefined"', () => {
      const result = detectCommonMistake('Cannot read undefined property');

      expect(result).toContain('valid fields');
    });

    it('should detect "unexpected token"', () => {
      const result = detectCommonMistake('Unexpected token at position 5');

      expect(result).toContain('typos');
    });

    it('should detect "type mismatch"', () => {
      const result = detectCommonMistake('Type mismatch: expected string');

      expect(result).toContain('compatible types');
    });

    it('should detect "is not defined"', () => {
      const result = detectCommonMistake('Reference error: foo is not defined');

      expect(result).toContain('typos');
    });

    it('should detect "cannot access"', () => {
      const result = detectCommonMistake('Cannot access field on null');

      expect(result).toContain('field you are trying to access');
    });

    it('should detect "no such key"', () => {
      const result = detectCommonMistake('No such key: foo');

      expect(result).toContain('does not contain the key');
    });

    it('should detect case-insensitively', () => {
      const result = detectCommonMistake('UNEXPECTED TOKEN in expression');

      expect(result).toContain('typos');
    });

    it('should return undefined for unknown errors', () => {
      const result = detectCommonMistake('Something completely different happened');

      expect(result).toBeUndefined();
    });
  });

  describe('error details structure', () => {
    it('should return proper CelErrorDetails structure', () => {
      const error = new Error('Test error');
      const result: CelErrorDetails = formatCompilationError(sampleRule, error);

      expect(result).toHaveProperty('message');
      expect(result).toHaveProperty('hint');
      expect(result).toHaveProperty('resolution');
      expect(typeof result.message).toBe('string');
      expect(typeof result.hint).toBe('string');
      expect(typeof result.resolution).toBe('string');
    });

    it('should have non-empty fields', () => {
      const error = new Error('Test');
      const result = formatCompilationError(sampleRule, error);

      expect(result.message.length).toBeGreaterThan(0);
      expect(result.hint.length).toBeGreaterThan(0);
      expect(result.resolution.length).toBeGreaterThan(0);
    });
  });
});
