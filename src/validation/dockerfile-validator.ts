/**
 * Dockerfile Validator - Security and best practice enforcement
 *
 * Invariant: All validation rules maintain backwards compatibility
 * Trade-off: Runtime parsing cost over build-time validation for flexibility
 */

import * as dockerParser from 'docker-file-parser';
import validateDockerfile from 'validate-dockerfile';

// Type definitions for docker-file-parser commands
interface DockerCommand {
  name: string;
  args: string | string[];
  lineno: number;
}

import { extractErrorMessage } from '../lib/error-utils';
import {
  DockerfileValidationRule,
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
} from './core-types';
import {
  SUDO_INSTALL,
  AS_CLAUSE,
  LATEST_TAG,
  PACKAGE_FILES,
  PASSWORD_PATTERN,
  API_KEY_PATTERN,
  SECRET_PATTERN,
  TOKEN_PATTERN,
} from '../lib/regex-patterns';

export class DockerfileValidator {
  private rules: DockerfileValidationRule[] = [];

  constructor() {
    this.loadRules();
  }

  private loadRules(): void {
    this.rules = [
      {
        id: 'no-root-user',
        name: 'Non-root user required',
        description: 'Container should run as non-root user for security',
        check: (commands: any) => {
          const userCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'USER');
          if (userCommands.length === 0) return false;

          const lastUser = userCommands[userCommands.length - 1];
          const userArg = this.getArgValue(lastUser);
          return userArg !== 'root' && userArg !== '0';
        },
        message: 'Container should run as non-root user',
        severity: ValidationSeverity.ERROR,
        fix: 'Add USER directive with non-root user (e.g., USER node)',
        category: ValidationCategory.SECURITY,
      },

      {
        id: 'no-sudo-install',
        name: 'No sudo in containers',
        description: 'Avoid installing sudo in containers for security',
        check: (commands: any) => {
          const runCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'RUN');
          return !runCommands.some((cmd: any) => {
            const args = this.getArgValue(cmd);
            return SUDO_INSTALL.test(args);
          });
        },
        message: 'Avoid installing sudo in containers',
        severity: ValidationSeverity.WARNING,
        fix: 'Remove sudo installation, use specific user permissions instead',
        category: ValidationCategory.SECURITY,
      },

      {
        id: 'specific-base-image',
        name: 'Use specific version tags',
        description: 'Base images should use specific versions for reproducibility',
        check: (commands: any) => {
          const fromCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'FROM');
          return fromCommands.every((cmd: any) => {
            const image = this.getArgValue(cmd);
            // Extract base image name before AS clause for validation
            const cleanImage = image?.split(AS_CLAUSE)?.[0];
            return cleanImage && cleanImage.includes(':') && !LATEST_TAG.test(cleanImage);
          });
        },
        message: 'Use specific version tags instead of latest',
        severity: ValidationSeverity.WARNING,
        fix: 'Replace :latest with specific version (e.g., node:18-alpine)',
        category: ValidationCategory.BEST_PRACTICE,
      },

      {
        id: 'has-healthcheck',
        name: 'Health check defined',
        description: 'Containers should define health checks for monitoring',
        check: (commands: any) => {
          return commands.some((cmd: any) => cmd.name === 'HEALTHCHECK');
        },
        message: 'Add HEALTHCHECK for container monitoring',
        severity: ValidationSeverity.INFO,
        fix: 'Add HEALTHCHECK CMD curl -f http://localhost/health || exit 1',
        category: ValidationCategory.BEST_PRACTICE,
      },

      {
        id: 'layer-caching-optimization',
        name: 'Optimize layer caching',
        description: 'Dependencies should be copied before application code',
        check: (commands: any) => {
          const copyCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'COPY');

          let packageCopyIndex = -1;
          let sourceCopyIndex = -1;

          copyCommands.forEach((cmd: DockerCommand, index: number) => {
            const args = this.getArgValue(cmd);
            if (PACKAGE_FILES.test(args)) {
              if (packageCopyIndex === -1) packageCopyIndex = index;
            }
            if ((args.includes(' . ') || args.endsWith(' .')) && !args.includes('*.')) {
              if (sourceCopyIndex === -1) sourceCopyIndex = index;
            }
          });

          // Pass if:
          // - No source copy found (nothing to optimize)
          // - Source copy found but package copy comes before it
          // Fail if source copy found but no package copy or package copy comes after
          if (sourceCopyIndex === -1) return true; // No source copy, nothing to optimize
          if (packageCopyIndex === -1) return false; // Source copy but no package copy
          return packageCopyIndex < sourceCopyIndex; // Package should come before source
        },
        message: 'Copy dependency files before source code for better caching',
        severity: ValidationSeverity.INFO,
        fix: 'COPY package*.json ./ before COPY . .',
        category: ValidationCategory.OPTIMIZATION,
      },

      {
        id: 'no-secrets',
        name: 'No hardcoded secrets',
        description: 'Secrets should not be hardcoded in Dockerfile',
        check: (commands: any) => {
          const suspicious = [PASSWORD_PATTERN, API_KEY_PATTERN, SECRET_PATTERN, TOKEN_PATTERN];

          return !commands.some((cmd: DockerCommand) => {
            if (cmd.name === 'ENV' || cmd.name === 'ARG') {
              const value = this.getArgValue(cmd);
              return suspicious.some((pattern) => pattern.test(value));
            }
            return false;
          });
        },
        message: 'Do not hardcode secrets in Dockerfile',
        severity: ValidationSeverity.ERROR,
        fix: 'Use build arguments or runtime environment variables',
        category: ValidationCategory.SECURITY,
      },

      {
        id: 'multi-stage-optimization',
        name: 'Multi-stage builds for production',
        description: 'Complex applications should use multi-stage builds',
        check: (commands: any) => {
          const fromCount = commands.filter((cmd: DockerCommand) => cmd.name === 'FROM').length;
          const totalLines = commands.length;

          // Skip check for simple Dockerfiles
          if (totalLines < 10) return true;

          // For complex Dockerfiles, recommend multi-stage
          return fromCount > 1;
        },
        message: 'Consider multi-stage builds for smaller images',
        severity: ValidationSeverity.INFO,
        fix: 'Use FROM ... AS build pattern for build dependencies',
        category: ValidationCategory.OPTIMIZATION,
      },

      {
        id: 'has-expose',
        name: 'Port exposure documented',
        description: 'Applications should document exposed ports',
        check: (commands: DockerCommand[]) => {
          const hasCmd = commands.some(
            (cmd: DockerCommand) => cmd.name === 'CMD' || cmd.name === 'ENTRYPOINT',
          );
          const hasExpose = commands.some((cmd: DockerCommand) => cmd.name === 'EXPOSE');

          // If no CMD/ENTRYPOINT, this check doesn't apply
          if (!hasCmd) return true;

          return hasExpose;
        },
        message: 'Document exposed ports with EXPOSE instruction',
        severity: ValidationSeverity.INFO,
        fix: 'Add EXPOSE <port> for application ports',
        category: ValidationCategory.BEST_PRACTICE,
      },

      {
        id: 'workdir-set',
        name: 'Working directory set',
        description: 'Set a working directory for better organization',
        check: (commands: DockerCommand[]) => {
          return commands.some((cmd: DockerCommand) => cmd.name === 'WORKDIR');
        },
        message: 'Set WORKDIR for better file organization',
        severity: ValidationSeverity.INFO,
        fix: 'Add WORKDIR /app or appropriate directory',
        category: ValidationCategory.BEST_PRACTICE,
      },
    ];
  }

  private getArgValue(command: any): string {
    if (typeof command.args === 'string') {
      return command.args;
    }
    if (Array.isArray(command.args)) {
      return command.args.join(' ');
    }
    if (typeof command.args === 'object' && command.args !== null) {
      // Handle ENV key=value format
      return Object.entries(command.args)
        .map(([k, v]) => `${k}=${v}`)
        .join(' ');
    }
    return '';
  }

  validate(dockerfileContent: string): ValidationReport {
    // First, validate using validate-dockerfile for syntax errors
    const basicValidation = validateDockerfile(dockerfileContent) as {
      valid: boolean;
      line?: number;
      message?: string;
      priority?: number;
      errors?: any[];
    };

    if (!basicValidation.valid) {
      // Check if there are true syntax/parse errors (priority 0)
      const syntaxErrors =
        (basicValidation.errors as any[])?.filter(
          (err: any) =>
            err.priority === 0 &&
            (err.message.includes('Invalid instruction') ||
              err.message.includes('Missing or misplaced FROM')),
        ) || [];

      if (syntaxErrors.length > 0) {
        // Only return parse error for true syntax issues, not validation warnings
        return {
          results: [
            {
              ruleId: 'parse-error',
              isValid: false,
              passed: false,
              errors: [`Failed to parse Dockerfile: ${syntaxErrors[0].message}`],
              warnings: [],
              message: `Failed to parse Dockerfile: ${syntaxErrors[0].message}`,
              metadata: {
                severity: ValidationSeverity.ERROR,
              },
            },
          ],
          score: 0,
          grade: 'F',
          passed: 0,
          failed: 1,
          errors: 1,
          warnings: 0,
          info: 0,
          timestamp: new Date().toISOString(),
        };
      }
    }

    try {
      // Parse the Dockerfile using docker-file-parser for detailed analysis
      const commands = dockerParser.parse(dockerfileContent);

      const results: ValidationResult[] = [];

      // Run each rule
      for (const rule of this.rules) {
        const passed = rule.check(commands);
        results.push({
          ruleId: rule.id,
          isValid: passed,
          passed,
          errors: passed ? [] : [`${rule.name}: ${rule.message}`],
          warnings: [],
          message: passed ? `✓ ${rule.name}` : `✗ ${rule.name}: ${rule.message}`,
          suggestions: !passed && rule.fix ? [rule.fix] : [],
          metadata: {
            severity: rule.severity,
          },
        });
      }

      return this.createReport(results);
    } catch (error) {
      // If parsing fails, return error report
      return {
        results: [
          {
            ruleId: 'parse-error',
            isValid: false,
            passed: false,
            errors: [`Failed to parse Dockerfile: ${extractErrorMessage(error)}`],
            warnings: [],
            message: `Failed to parse Dockerfile: ${extractErrorMessage(error)}`,
            metadata: {
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
        score: 0,
        grade: 'F',
        passed: 0,
        failed: 1,
        errors: 1,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };
    }
  }

  private createReport(results: ValidationResult[]): ValidationReport {
    const errors = results.filter(
      (r) => !r.passed && r.metadata?.severity === ValidationSeverity.ERROR,
    ).length;
    const warnings = results.filter(
      (r) => !r.passed && r.metadata?.severity === ValidationSeverity.WARNING,
    ).length;
    const info = results.filter(
      (r) => !r.passed && r.metadata?.severity === ValidationSeverity.INFO,
    ).length;
    const passed = results.filter((r) => r.passed).length;
    const total = results.length;

    // Weighted scoring: errors = -15, warnings = -5, info = -2
    const score = Math.max(0, 100 - errors * 15 - warnings * 5 - info * 2);
    const grade = this.calculateGrade(score);

    return {
      results,
      score,
      grade,
      passed,
      failed: total - passed,
      errors,
      warnings,
      info,
      timestamp: new Date().toISOString(),
    };
  }

  private calculateGrade(score: number): ValidationGrade {
    if (score >= 90) return 'A';
    if (score >= 80) return 'B';
    if (score >= 70) return 'C';
    if (score >= 60) return 'D';
    return 'F';
  }

  getRules(): DockerfileValidationRule[] {
    return [...this.rules];
  }

  getCategory(category: ValidationCategory): DockerfileValidationRule[] {
    return this.rules.filter((rule) => rule.category === category);
  }
}
