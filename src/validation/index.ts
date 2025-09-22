/**
 * Validation module exports - Functional API with Result<T> pattern
 */

export * from './core-types';
// Only export types that are part of the public API
export type { ValidationFunction, Validator } from './pipeline';
export {
  validateDockerfileContent,
  getDockerfileRules,
  getDockerfileRulesByCategory,
  createDockerfileValidator,
} from './dockerfile-validator';

export {
  createKubernetesValidator,
  validateKubernetes as validateKubernetesManifests,
  type KubernetesValidatorInstance,
} from './kubernetes-validator';

import { validateDockerfileContent } from './dockerfile-validator';

export const validateDockerfile = validateDockerfileContent;
export type {
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
  DockerfileValidationRule,
  KubernetesValidationRule,
} from './core-types';

import type { ValidationReport, ValidationResult } from './core-types';

/** Formats validation reports as markdown for human consumption */
export function formatValidationReport(report: ValidationReport): string {
  const lines: string[] = [
    '# Validation Report',
    '',
    `**Score**: ${report.score}/100 (${report.grade})`,
    `**Timestamp**: ${report.timestamp}`,
    `**Results**: ${report.passed} passed, ${report.failed} failed`,
    '',
  ];

  if (report.errors > 0) {
    lines.push('## ❌ Errors');
    for (const result of report.results) {
      if (!result.passed && result.metadata?.severity === 'error') {
        lines.push(`- ${result.message}`);
        if (result.suggestions?.[0]) {
          lines.push(`  💡 **Fix**: ${result.suggestions[0]}`);
        }
      }
    }
    lines.push('');
  }

  if (report.warnings > 0) {
    lines.push('## ⚠️ Warnings');
    for (const result of report.results) {
      if (!result.passed && result.metadata?.severity === 'warning') {
        lines.push(`- ${result.message}`);
        if (result.suggestions?.[0]) {
          lines.push(`  💡 **Fix**: ${result.suggestions[0]}`);
        }
      }
    }
    lines.push('');
  }

  if (report.info > 0) {
    lines.push('## ℹ️ Information');
    for (const result of report.results) {
      if (!result.passed && result.metadata?.severity === 'info') {
        lines.push(`- ${result.message}`);
        if (result.suggestions?.[0]) {
          lines.push(`  💡 **Suggestion**: ${result.suggestions[0]}`);
        }
      }
    }
    lines.push('');
  }

  const passed = report.results.filter((r: ValidationResult) => r.passed);
  if (passed.length > 0) {
    lines.push('## ✅ Passed');
    for (const result of passed.slice(0, 5)) {
      lines.push(`- ${result.message}`);
    }
    if (passed.length > 5) {
      lines.push(`- ... and ${passed.length - 5} more`);
    }
  }

  return lines.join('\n');
}

/** Returns human-readable validation status with score and issue counts */
export function getValidationSummary(report: ValidationReport): string {
  if (report.score >= 90) {
    return `✅ Excellent! Score: ${report.score}/100 (${report.grade})`;
  } else if (report.score >= 70) {
    return `👍 Good. Score: ${report.score}/100 (${report.grade}) - ${report.errors} errors, ${report.warnings} warnings`;
  } else {
    return `⚠️ Needs improvement. Score: ${report.score}/100 (${report.grade}) - ${report.errors} errors, ${report.warnings} warnings`;
  }
}
