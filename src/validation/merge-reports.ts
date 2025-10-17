import type {
  ValidationReport,
  ValidationResult,
  ValidationSeverity,
  ValidationGrade,
} from './core-types';

/**
 * Create a unique key for a validation result to detect duplicates
 */
function getResultKey(result: ValidationResult): string {
  const ruleId = result.ruleId || 'unknown';
  const location = result.metadata?.location || 'unknown';
  const message = result.message || result.errors.join('|') || result.warnings?.join('|') || '';
  return `${ruleId}:${location}:${message}`;
}

/**
 * Compare two severities and return the worse one
 */
function worseSeverity(a: ValidationSeverity, b: ValidationSeverity): ValidationSeverity {
  const severityOrder: Record<ValidationSeverity, number> = {
    error: 3,
    warning: 2,
    info: 1,
  };

  const aOrder = severityOrder[a] || 0;
  const bOrder = severityOrder[b] || 0;

  return aOrder >= bOrder ? a : b;
}

/**
 * Compare two grades and return the worse one
 */
function worseGrade(a: ValidationGrade, b: ValidationGrade): ValidationGrade {
  const gradeOrder: Record<ValidationGrade, number> = {
    F: 5,
    D: 4,
    C: 3,
    B: 2,
    A: 1,
  };

  const aOrder = gradeOrder[a];
  const bOrder = gradeOrder[b];

  return aOrder >= bOrder ? a : b;
}

/**
 * Merge two validation reports, de-duplicating results and recalculating scores
 *
 * @param a - First validation report
 * @param b - Second validation report
 * @returns Merged validation report with de-duplicated results
 */
export function mergeReports(a: ValidationReport, b: ValidationReport): ValidationReport {
  // De-duplicate by ruleId:line:message
  const byKey = new Map<string, ValidationResult>();

  // Process results from report A
  for (const result of a.results) {
    const key = getResultKey(result);
    byKey.set(key, result);
  }

  // Process results from report B
  for (const result of b.results) {
    const key = getResultKey(result);

    // If duplicate exists, keep the one with worse severity
    if (byKey.has(key)) {
      const existing = byKey.get(key);
      if (existing) {
        const existingSeverity = existing.metadata?.severity || ('info' as ValidationSeverity);
        const newSeverity = result.metadata?.severity || ('info' as ValidationSeverity);

        // Keep the result with worse severity
        if (worseSeverity(newSeverity, existingSeverity) === newSeverity) {
          byKey.set(key, result);
        }
      } else {
        byKey.set(key, result);
      }
    } else {
      byKey.set(key, result);
    }
  }

  // Convert back to array
  const mergedResults = Array.from(byKey.values());

  // Recalculate counts
  let errors = 0;
  let warnings = 0;
  let info = 0;
  let passed = 0;
  let failed = 0;

  for (const result of mergedResults) {
    if (result.passed) {
      passed++;
    } else {
      failed++;
    }

    const severity = result.metadata?.severity;
    if (severity === 'error') {
      errors++;
    } else if (severity === 'warning') {
      warnings++;
    } else if (severity === 'info') {
      info++;
    }
  }

  // Recalculate score using worst-case
  const score = Math.min(a.score, b.score);

  // Use worse grade
  const grade = worseGrade(a.grade, b.grade);

  return {
    results: mergedResults,
    score,
    grade,
    passed,
    failed,
    errors,
    warnings,
    info,
    timestamp: new Date().toISOString(),
  };
}

/**
 * Merge multiple validation reports
 */
export function mergeMultipleReports(reports: ValidationReport[]): ValidationReport {
  if (reports.length === 0) {
    return {
      results: [],
      score: 100,
      grade: 'A',
      passed: 0,
      failed: 0,
      errors: 0,
      warnings: 0,
      info: 0,
      timestamp: new Date().toISOString(),
    };
  }

  const firstReport = reports[0];
  if (!firstReport || reports.length === 1) {
    return (
      firstReport || {
        results: [],
        score: 100,
        grade: 'A',
        passed: 0,
        failed: 0,
        errors: 0,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      }
    );
  }

  // Merge all reports sequentially
  let merged: ValidationReport = firstReport;
  for (let i = 1; i < reports.length; i++) {
    const report = reports[i];
    if (report) {
      merged = mergeReports(merged, report);
    }
  }

  return merged;
}
