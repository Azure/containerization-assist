import { mergeReports, mergeMultipleReports } from '@/validation/merge-reports';
import type { ValidationReport, ValidationResult, ValidationSeverity } from '@/validation/core-types';

describe('Merge Reports', () => {
  const createMockResult = (
    ruleId: string,
    severity: ValidationSeverity,
    passed: boolean = false,
    location?: string,
  ): ValidationResult => ({
    ruleId,
    isValid: passed,
    passed,
    errors: severity === 'error' && !passed ? [`Error for ${ruleId}`] : [],
    warnings: severity === 'warning' && !passed ? [`Warning for ${ruleId}`] : [],
    message: `Message for ${ruleId}`,
    metadata: {
      severity,
      location,
    },
  });

  const createMockReport = (
    results: ValidationResult[],
    score: number = 80,
    grade: 'A' | 'B' | 'C' | 'D' | 'F' = 'B',
  ): ValidationReport => {
    const errors = results.filter((r) => !r.passed && r.metadata?.severity === 'error').length;
    const warnings = results.filter((r) => !r.passed && r.metadata?.severity === 'warning').length;
    const info = results.filter((r) => !r.passed && r.metadata?.severity === 'info').length;
    const passed = results.filter((r) => r.passed).length;

    return {
      results,
      score,
      grade,
      passed,
      failed: results.length - passed,
      errors,
      warnings,
      info,
      timestamp: new Date().toISOString(),
    };
  };

  describe('mergeReports', () => {
    it('should merge two reports without duplicates', () => {
      const report1 = createMockReport(
        [createMockResult('rule1', 'error', false), createMockResult('rule2', 'warning', false)],
        70,
        'C',
      );

      const report2 = createMockReport(
        [createMockResult('rule3', 'info', false), createMockResult('rule4', 'error', false)],
        60,
        'D',
      );

      const merged = mergeReports(report1, report2);

      expect(merged.results).toHaveLength(4);
      expect(merged.results.map((r) => r.ruleId)).toEqual(['rule1', 'rule2', 'rule3', 'rule4']);
    });

    it('should de-duplicate identical results', () => {
      const report1 = createMockReport([
        createMockResult('rule1', 'error', false, 'line 10'),
        createMockResult('rule2', 'warning', false),
      ]);

      const report2 = createMockReport([
        createMockResult('rule1', 'error', false, 'line 10'), // Duplicate
        createMockResult('rule3', 'info', false),
      ]);

      const merged = mergeReports(report1, report2);

      expect(merged.results).toHaveLength(3);
      expect(merged.results.map((r) => r.ruleId)).toEqual(['rule1', 'rule2', 'rule3']);
    });

    it('should keep worse severity when de-duplicating', () => {
      const report1 = createMockReport([createMockResult('rule1', 'warning', false, 'line 10')]);

      const report2 = createMockReport([
        createMockResult('rule1', 'error', false, 'line 10'), // Same rule, worse severity
      ]);

      const merged = mergeReports(report1, report2);

      expect(merged.results).toHaveLength(1);
      expect(merged.results[0].metadata?.severity).toBe('error');
    });

    it('should use worst score from both reports', () => {
      const report1 = createMockReport([], 85, 'B');
      const report2 = createMockReport([], 65, 'D');

      const merged = mergeReports(report1, report2);

      expect(merged.score).toBe(65);
    });

    it('should use worse grade from both reports', () => {
      const report1 = createMockReport([], 90, 'A');
      const report2 = createMockReport([], 70, 'C');

      const merged = mergeReports(report1, report2);

      expect(merged.grade).toBe('C');
    });

    it('should recalculate counts correctly', () => {
      const report1 = createMockReport([
        createMockResult('rule1', 'error', false),
        createMockResult('rule2', 'warning', false),
        createMockResult('rule3', 'info', false),
        createMockResult('rule4', 'info', true), // passed
      ]);

      const report2 = createMockReport([
        createMockResult('rule5', 'error', false),
        createMockResult('rule6', 'warning', false),
      ]);

      const merged = mergeReports(report1, report2);

      expect(merged.errors).toBe(2);
      expect(merged.warnings).toBe(2);
      expect(merged.info).toBe(2); // rule3 from report1 and rule4 passed counts differently
      expect(merged.passed).toBe(1); // rule4 is passed
      expect(merged.failed).toBe(5); // rule1,2,3,5,6 are failed
    });

    it('should handle empty reports', () => {
      const report1 = createMockReport([], 100, 'A');
      const report2 = createMockReport([createMockResult('rule1', 'error', false)], 85, 'B');

      const merged = mergeReports(report1, report2);

      expect(merged.results).toHaveLength(1);
      expect(merged.errors).toBe(1);
      expect(merged.score).toBe(85);
    });

    it('should generate new timestamp', () => {
      const report1 = createMockReport([]);
      const report2 = createMockReport([]);

      const beforeMerge = new Date().getTime();
      const merged = mergeReports(report1, report2);
      const afterMerge = new Date().getTime();

      const mergedTime = new Date(merged.timestamp).getTime();

      expect(mergedTime).toBeGreaterThanOrEqual(beforeMerge);
      expect(mergedTime).toBeLessThanOrEqual(afterMerge);
    });
  });

  describe('mergeMultipleReports', () => {
    it('should merge multiple reports sequentially', () => {
      const reports = [
        createMockReport([createMockResult('rule1', 'error', false)], 90, 'A'),
        createMockReport([createMockResult('rule2', 'warning', false)], 80, 'B'),
        createMockReport([createMockResult('rule3', 'info', false)], 70, 'C'),
      ];

      const merged = mergeMultipleReports(reports);

      expect(merged.results).toHaveLength(3);
      expect(merged.results.map((r) => r.ruleId)).toEqual(['rule1', 'rule2', 'rule3']);
      expect(merged.score).toBe(70); // Worst score
      expect(merged.grade).toBe('C'); // Worst grade
    });

    it('should handle empty array', () => {
      const merged = mergeMultipleReports([]);

      expect(merged.results).toHaveLength(0);
      expect(merged.score).toBe(100);
      expect(merged.grade).toBe('A');
      expect(merged.passed).toBe(0);
      expect(merged.failed).toBe(0);
    });

    it('should handle single report', () => {
      const report = createMockReport([createMockResult('rule1', 'error', false)], 75, 'C');

      const merged = mergeMultipleReports([report]);

      expect(merged).toEqual(report);
    });

    it('should de-duplicate across all reports', () => {
      const reports = [
        createMockReport([createMockResult('duplicate', 'warning', false, 'line 5')]),
        createMockReport([createMockResult('duplicate', 'warning', false, 'line 5')]),
        createMockReport([createMockResult('duplicate', 'error', false, 'line 5')]),
        createMockReport([createMockResult('unique', 'info', false)]),
      ];

      const merged = mergeMultipleReports(reports);

      expect(merged.results).toHaveLength(2);

      const duplicateResult = merged.results.find((r) => r.ruleId === 'duplicate');
      expect(duplicateResult?.metadata?.severity).toBe('error'); // Worst severity kept
    });
  });

  describe('severity comparison', () => {
    it('should correctly order severities', () => {
      const report1 = createMockReport([createMockResult('rule1', 'info', false, 'line 1')]);

      const report2 = createMockReport([createMockResult('rule1', 'warning', false, 'line 1')]);

      const report3 = createMockReport([createMockResult('rule1', 'error', false, 'line 1')]);

      // Info vs Warning -> Warning wins
      let merged = mergeReports(report1, report2);
      expect(merged.results[0].metadata?.severity).toBe('warning');

      // Warning vs Error -> Error wins
      merged = mergeReports(report2, report3);
      expect(merged.results[0].metadata?.severity).toBe('error');

      // Info vs Error -> Error wins
      merged = mergeReports(report1, report3);
      expect(merged.results[0].metadata?.severity).toBe('error');
    });
  });

  describe('grade comparison', () => {
    it('should correctly order grades', () => {
      const gradeTests: Array<
        ['A' | 'B' | 'C' | 'D' | 'F', 'A' | 'B' | 'C' | 'D' | 'F', 'A' | 'B' | 'C' | 'D' | 'F']
      > = [
        ['A', 'B', 'B'],
        ['A', 'F', 'F'],
        ['C', 'D', 'D'],
        ['B', 'C', 'C'],
        ['D', 'F', 'F'],
      ];

      gradeTests.forEach(([grade1, grade2, expected]) => {
        const report1 = createMockReport([], 100, grade1);
        const report2 = createMockReport([], 100, grade2);
        const merged = mergeReports(report1, report2);

        expect(merged.grade).toBe(expected);
      });
    });
  });
});
