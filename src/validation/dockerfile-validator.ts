/**
 * Dockerfile Validator - Security and best practice enforcement (Functional)
 *
 * Invariant: All validation rules maintain backwards compatibility
 * Trade-off: Runtime parsing cost over build-time validation for flexibility
 */

import * as dockerParser from 'docker-file-parser';
import type { CommandEntry } from 'docker-file-parser';
import validateDockerfileSyntax from 'validate-dockerfile';
import { Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import {
  DockerfileValidationRule,
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
} from './core-types';
import { combine, ValidationFunction } from './pipeline';
import {
  SUDO_INSTALL,
  AS_CLAUSE,
  LATEST_TAG,
  PACKAGE_FILES,
  PASSWORD_PATTERN,
  API_KEY_PATTERN,
  SECRET_PATTERN,
  TOKEN_PATTERN,
} from '@/lib/regex-patterns';

type DockerCommand = CommandEntry;

/**
 * Get argument value from docker command
 */
const getArgValue = (command: DockerCommand): string => {
  if (typeof command.args === 'string') {
    return command.args;
  }
  if (Array.isArray(command.args)) {
    return command.args.join(' ');
  }
  if (typeof command.args === 'object' && command.args !== null) {
    // ENV supports both space-separated and = formats
    return Object.entries(command.args)
      .map(([k, v]) => `${k}=${v}`)
      .join(' ');
  }
  return '';
};

/**
 * Validation rules for Dockerfile analysis
 */
const DOCKERFILE_RULES: DockerfileValidationRule[] = [
  {
    id: 'no-root-user',
    name: 'Non-root user required',
    description: 'Container should run as non-root user for security',
    check: (commands: CommandEntry[]) => {
      const userCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'USER');
      if (userCommands.length === 0) return false;

      const lastUser = userCommands[userCommands.length - 1];
      if (!lastUser) return false;
      const userArg = getArgValue(lastUser);
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
    check: (commands: CommandEntry[]) => {
      const runCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'RUN');
      return !runCommands.some((cmd: DockerCommand) => {
        const args = getArgValue(cmd);
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
    check: (commands: CommandEntry[]) => {
      const fromCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'FROM');
      return fromCommands.every((cmd: DockerCommand) => {
        const image = getArgValue(cmd);
        // Multi-stage builds use 'FROM image AS stage' syntax
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
    check: (commands: CommandEntry[]) => {
      return commands.some((cmd: CommandEntry) => cmd.name === 'HEALTHCHECK');
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
    check: (commands: CommandEntry[]) => {
      const copyCommands = commands.filter((cmd: DockerCommand) => cmd.name === 'COPY');

      let packageCopyIndex = -1;
      let sourceCopyIndex = -1;

      copyCommands.forEach((cmd: DockerCommand, index: number) => {
        const args = getArgValue(cmd);
        if (PACKAGE_FILES.test(args)) {
          if (packageCopyIndex === -1) packageCopyIndex = index;
        }
        if ((args.includes(' . ') || args.endsWith(' .')) && !args.includes('*.')) {
          if (sourceCopyIndex === -1) sourceCopyIndex = index;
        }
      });

      // Optimal layer caching requires dependencies before source code
      if (sourceCopyIndex === -1) return true;
      // Layer invalidation occurs when source changes before dependency install
      if (packageCopyIndex === -1) return false;
      return packageCopyIndex < sourceCopyIndex;
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
    check: (commands: CommandEntry[]) => {
      const suspicious = [PASSWORD_PATTERN, API_KEY_PATTERN, SECRET_PATTERN, TOKEN_PATTERN];

      return !commands.some((cmd: DockerCommand) => {
        if (cmd.name === 'ENV' || cmd.name === 'ARG') {
          const value = getArgValue(cmd);
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
    check: (commands: CommandEntry[]) => {
      const fromCount = commands.filter((cmd: DockerCommand) => cmd.name === 'FROM').length;
      const totalLines = commands.length;

      // Multi-stage builds reduce final image size significantly
      if (totalLines < 10) return true;

      // Complex builds benefit from separate build and runtime stages
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

/**
 * Parse Dockerfile content into commands
 */
const parseDockerfile = (content: string): Result<DockerCommand[]> => {
  try {
    const commands = dockerParser.parse(content);
    return Success(commands);
  } catch (error) {
    return Failure(`Failed to parse Dockerfile: ${extractErrorMessage(error)}`);
  }
};

/**
 * Validate Dockerfile syntax using external validator
 */
const validateSyntax = (content: string): Result<boolean> => {
  const basicValidation = validateDockerfileSyntax(content) as {
    valid: boolean;
    line?: number;
    message?: string;
    priority?: number;
    errors?: Array<{ line: number; message: string; level?: string }>;
  };

  if (!basicValidation.valid) {
    // Check if there are true syntax/parse errors (priority 0)
    interface DockerfileError {
      line: number;
      message: string;
      level?: string;
      priority?: number;
    }

    const syntaxErrors =
      (basicValidation.errors as DockerfileError[])?.filter(
        (err) =>
          err.priority === 0 &&
          (err.message.includes('Invalid instruction') ||
            err.message.includes('Missing or misplaced FROM')),
      ) || [];

    if (syntaxErrors.length > 0) {
      return Failure(`Failed to parse Dockerfile: ${syntaxErrors[0]?.message || 'Unknown error'}`);
    }
  }

  return Success(true);
};

/**
 * Create validation function for a single rule
 */
const createRuleValidator = (
  rule: DockerfileValidationRule,
): ValidationFunction<DockerCommand[]> => {
  return (commands: DockerCommand[]): Result<ValidationResult> => {
    const passed = rule.check(commands);

    const result: ValidationResult = {
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
    };

    return Success(result);
  };
};

/**
 * Calculate validation grade from score
 */
const calculateGrade = (score: number): ValidationGrade => {
  if (score >= 90) return 'A';
  if (score >= 80) return 'B';
  if (score >= 70) return 'C';
  if (score >= 60) return 'D';
  return 'F';
};

/**
 * Create validation report from results
 */
const createReport = (results: ValidationResult[]): ValidationReport => {
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
  const grade = calculateGrade(score);

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
};

/**
 * Validate Dockerfile content using functional pipeline
 */
export const validateDockerfileContent = (dockerfileContent: string): ValidationReport => {
  // First check syntax
  const syntaxResult = validateSyntax(dockerfileContent);
  if (!syntaxResult.ok) {
    return {
      results: [
        {
          ruleId: 'parse-error',
          isValid: false,
          passed: false,
          errors: [syntaxResult.error],
          warnings: [],
          message: syntaxResult.error,
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

  // Parse Dockerfile
  const parseResult = parseDockerfile(dockerfileContent);
  if (!parseResult.ok) {
    return {
      results: [
        {
          ruleId: 'parse-error',
          isValid: false,
          passed: false,
          errors: [parseResult.error],
          warnings: [],
          message: parseResult.error,
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

  const commands = parseResult.value;

  // Create validators for each rule
  const validators = DOCKERFILE_RULES.map((rule) => createRuleValidator(rule));

  // Run all validations
  const combinedValidator = combine(validators);
  const validationResult = combinedValidator(commands);

  if (validationResult.ok) {
    // Extract individual results from combined validation
    const results: ValidationResult[] = [];

    for (const rule of DOCKERFILE_RULES) {
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

    return createReport(results);
  } else {
    return {
      results: [
        {
          ruleId: 'validation-error',
          isValid: false,
          passed: false,
          errors: [validationResult.error],
          warnings: [],
          message: validationResult.error,
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
};

/**
 * Get all Dockerfile validation rules
 */
export const getDockerfileRules = (): DockerfileValidationRule[] => {
  return [...DOCKERFILE_RULES];
};

/**
 * Get Dockerfile rules by category
 */
export const getDockerfileRulesByCategory = (
  category: ValidationCategory,
): DockerfileValidationRule[] => {
  return DOCKERFILE_RULES.filter((rule) => rule.category === category);
};

/**
 * Create a Dockerfile validator factory for backward compatibility
 */
export const createDockerfileValidator = (): {
  validate: (dockerfileContent: string) => ValidationReport;
  getRules: () => DockerfileValidationRule[];
  getCategory: (category: ValidationCategory) => DockerfileValidationRule[];
} => ({
  validate: validateDockerfileContent,
  getRules: getDockerfileRules,
  getCategory: getDockerfileRulesByCategory,
});
