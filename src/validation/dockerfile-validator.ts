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
import { extractErrorMessage } from '@/lib/errors';
import { createLogger } from '@/lib/logger';
import {
  DockerfileValidationRule,
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
} from './core-types';
import { lintWithDockerfilelint } from './dockerfilelint-adapter';
import { mergeReports } from './merge-reports';
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
import type { DockerPlatform } from '@/tools/shared/schemas';

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
 * Check if a specific line contains a HEALTHCHECK instruction
 * @param lines Pre-split lines array for efficient lookup
 * @param lineNumber 1-based line number
 */
const isHealthCheckLine = (lines: string[], lineNumber: number): boolean => {
  const line = lines[lineNumber - 1]; // lineNumber is 1-based
  return line?.trim().toUpperCase().startsWith('HEALTHCHECK') ?? false;
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
    // Split content once for efficient line lookups
    const lines = content.split('\n');

    // Check if there are true syntax/parse errors (priority 0)
    // Exclude HEALTHCHECK instruction which validate-dockerfile incorrectly flags as invalid
    const syntaxErrors =
      (
        basicValidation.errors as Array<{
          line: number;
          message: string;
          level?: string;
          priority?: number;
        }>
      )?.filter(
        (err) =>
          err.priority === 0 &&
          (err.message.includes('Missing or misplaced FROM') ||
            (err.message.includes('Invalid instruction') && !isHealthCheckLine(lines, err.line))),
      ) || [];

    if (syntaxErrors.length > 0) {
      return Failure(`Failed to parse Dockerfile: ${syntaxErrors[0]?.message || 'Unknown error'}`);
    }
  }

  return Success(true);
};

/**
 * Security-critical rules that cap the grade
 */
const SECURITY_RULES = ['no-root-user', 'no-sudo-install', 'no-secrets'];

/**
 * Severity weights for scoring
 */
const SEVERITY_WEIGHTS = {
  [ValidationSeverity.ERROR]: 15,
  [ValidationSeverity.WARNING]: 4,
  [ValidationSeverity.INFO]: 1,
};

/**
 * Calculate validation grade from score
 */
const calculateGrade = (score: number, hasCriticalSecurity: boolean): ValidationGrade => {
  let grade: ValidationGrade;
  if (score >= 90) grade = 'A';
  else if (score >= 80) grade = 'B';
  else if (score >= 70) grade = 'C';
  else if (score >= 60) grade = 'D';
  else grade = 'F';

  // Cap at C if critical security issues
  if (hasCriticalSecurity && (grade === 'A' || grade === 'B')) {
    grade = 'C';
  }

  return grade;
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

  // Calculate weighted score
  const deductions =
    errors * SEVERITY_WEIGHTS[ValidationSeverity.ERROR] +
    warnings * SEVERITY_WEIGHTS[ValidationSeverity.WARNING] +
    info * SEVERITY_WEIGHTS[ValidationSeverity.INFO];

  const score = Math.max(0, 100 - deductions);

  // Check for critical security failures
  const hasCriticalSecurity = results.some(
    (r) =>
      SECURITY_RULES.includes(r.ruleId || '') &&
      r.metadata?.severity === ValidationSeverity.ERROR &&
      !r.passed,
  );

  const grade = calculateGrade(score, hasCriticalSecurity);

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
 * Detect BuildKit features in Dockerfile content
 */
function detectBuildKitFeatures(content: string): {
  syntax?: string;
  hasHeredocs: boolean;
  hasMounts: boolean;
  hasSecrets: boolean;
} {
  const lines = content.split('\n');
  const syntaxLine = lines.find((l) => l.startsWith('# syntax='));

  const syntaxMatch = syntaxLine?.replace('# syntax=', '').trim();
  return {
    ...(syntaxMatch && { syntax: syntaxMatch }),
    hasHeredocs: /RUN\s+<</.test(content),
    hasMounts: /RUN\s+--mount/.test(content),
    hasSecrets: /RUN\s+--mount=type=secret/.test(content),
  };
}

/**
 * Validate BuildKit Dockerfile
 * Used when standard parser fails due to BuildKit syntax
 */
function validateBuildKitDockerfile(content: string): ValidationReport {
  const results: ValidationResult[] = [];
  const lines = content.split('\n');

  lines.forEach((line, idx) => {
    const lineNumber = idx + 1;
    const trimmedLine = line.trim();

    // Skip comments and empty lines
    if (!trimmedLine || trimmedLine.startsWith('#')) {
      return;
    }

    // Check for :latest tags
    if (line.includes('latest') && line.startsWith('FROM')) {
      results.push({
        ruleId: 'specific-base-image',
        isValid: false,
        passed: false,
        errors: [`Line ${lineNumber}: Avoid using :latest tag`],
        warnings: [],
        message: `✗ Use specific version tags: Line ${lineNumber}`,
        suggestions: ['Replace :latest with specific version (e.g., node:20-alpine)'],
        metadata: {
          severity: ValidationSeverity.WARNING,
          location: `line ${lineNumber}`,
          aiEnhanced: false,
        },
      });
    }

    // Check for potential secrets
    if (/password|api_key|secret|token/i.test(line) && !line.startsWith('#')) {
      // Skip if it's in a comment or mount directive (secrets are OK in BuildKit mounts)
      if (!line.includes('--mount=type=secret')) {
        // Extract the variable name from ENV/ARG statements
        const envMatch = line.match(/^\s*(?:ENV|ARG)\s+([A-Z_][A-Z0-9_]*)\s*=/i);
        const variableName = envMatch ? envMatch[1] : 'secret';

        results.push({
          ruleId: 'no-secrets',
          isValid: false,
          passed: false,
          errors: [`Line ${lineNumber}: Potential secret exposed`],
          warnings: [],
          message: `✗ No hardcoded secrets: Do not hardcode secrets in Dockerfile (found: ${variableName})`,
          suggestions: ['Use build arguments or runtime environment variables'],
          metadata: {
            severity: ValidationSeverity.ERROR,
            location: `line ${lineNumber}`,
            aiEnhanced: false,
          },
        });
      }
    }

    // Check for root user
    if (line.startsWith('USER root') || line.startsWith('USER 0')) {
      results.push({
        ruleId: 'no-root-user',
        isValid: false,
        passed: false,
        errors: [`Line ${lineNumber}: Container runs as root user`],
        warnings: [],
        message: `✗ Non-root user required: Line ${lineNumber}`,
        suggestions: ['Add USER directive with non-root user (e.g., USER node)'],
        metadata: {
          severity: ValidationSeverity.ERROR,
          location: `line ${lineNumber}`,
          aiEnhanced: false,
        },
      });
    }

    // Check for package manager optimizations
    if (line.includes('apt-get install') && !line.includes('--no-install-recommends')) {
      results.push({
        ruleId: 'optimize-package-install',
        isValid: false,
        passed: false,
        errors: [`Line ${lineNumber}: Missing --no-install-recommends flag`],
        warnings: [],
        message: `✗ Optimize package install: Line ${lineNumber}`,
        suggestions: ['Add --no-install-recommends to apt-get install commands'],
        metadata: {
          severity: ValidationSeverity.WARNING,
          location: `line ${lineNumber}`,
          aiEnhanced: false,
        },
      });
    }
  });

  // Add positive results for detected BuildKit features
  const buildKit = detectBuildKitFeatures(content);
  if (buildKit.syntax) {
    results.push({
      ruleId: 'buildkit-syntax',
      isValid: true,
      passed: true,
      errors: [],
      warnings: [],
      message: `✓ Uses BuildKit syntax: ${buildKit.syntax}`,
      suggestions: [],
      metadata: {
        severity: ValidationSeverity.INFO,
        location: 'BuildKit syntax',
        aiEnhanced: false,
      },
    });
  }

  if (buildKit.hasMounts) {
    results.push({
      ruleId: 'buildkit-mounts',
      isValid: true,
      passed: true,
      errors: [],
      warnings: [],
      message: '✓ Uses BuildKit mount optimizations',
      suggestions: [],
      metadata: {
        severity: ValidationSeverity.INFO,
        location: 'BuildKit mounts',
        aiEnhanced: false,
      },
    });
  }

  return createReport(results);
}
/**
 * Validate Dockerfile content using functional pipeline
 */
export const validateDockerfileContent = async (
  dockerfileContent: string,
  options?: {
    enableExternalLinter?: boolean;
    targetPlatform?: DockerPlatform;
    strictPlatformValidation?: boolean;
  },
): Promise<ValidationReport> => {
  // Detect BuildKit features first
  const buildKit = detectBuildKitFeatures(dockerfileContent);

  // Prepare validation rules
  const validationRules = [...DOCKERFILE_RULES];

  if (buildKit.syntax || buildKit.hasHeredocs || buildKit.hasMounts) {
    // Log BuildKit features detected
    const logger = createLogger({ name: 'dockerfile-validator' });
    logger.info({ buildKit }, 'BuildKit features detected');

    // Try standard parsing first
    const parseResult = parseDockerfile(dockerfileContent);
    if (parseResult.ok) {
      // Standard validation path works despite BuildKit features
      const commands = parseResult.value;

      // Run all validation rules directly
      const results: ValidationResult[] = [];

      for (const rule of validationRules) {
        const passed = rule.check(commands);

        // Special handling for no-secrets rule to include the specific secret name
        let message = passed ? `✓ ${rule.name}` : `✗ ${rule.name}: ${rule.message}`;
        if (!passed && rule.id === 'no-secrets') {
          // Extract the secret variable name from the ENV/ARG commands
          const secretVariable = commands.find((cmd: DockerCommand) => {
            if (cmd.name === 'ENV' || cmd.name === 'ARG') {
              const value = getArgValue(cmd);
              const suspicious = [PASSWORD_PATTERN, API_KEY_PATTERN, SECRET_PATTERN, TOKEN_PATTERN];
              return suspicious.some((pattern) => pattern.test(value));
            }
            return false;
          });

          if (secretVariable) {
            const value = getArgValue(secretVariable);
            const variableMatch = value.match(/^([A-Z_][A-Z0-9_]*)\s*=/i);
            const variableName = variableMatch ? variableMatch[1] : 'secret';
            message = `✗ ${rule.name}: ${rule.message} (found: ${variableName})`;
          }
        }

        results.push({
          ruleId: rule.id,
          isValid: passed,
          passed,
          errors: passed ? [] : [`${rule.name}: ${rule.message}`],
          warnings: [],
          message,
          suggestions: !passed && rule.fix ? [rule.fix] : [],
          metadata: {
            severity: rule.severity,
          },
        });
      }

      const internalReport = createReport(results);

      // Add external linter if enabled
      if (options?.enableExternalLinter !== false) {
        try {
          const externalReport = await lintWithDockerfilelint(dockerfileContent);
          return mergeReports(internalReport, externalReport);
        } catch (error) {
          console.warn('External linter failed, using internal validation only:', error);
          return internalReport;
        }
      }

      return internalReport;
    }

    // Fall back to BuildKit-specific validation
    const buildKitReport = validateBuildKitDockerfile(dockerfileContent);

    // Add external linter if enabled (it might handle BuildKit better)
    if (options?.enableExternalLinter !== false) {
      try {
        const externalReport = await lintWithDockerfilelint(dockerfileContent);
        return mergeReports(buildKitReport, externalReport);
      } catch (error) {
        console.warn(
          'External linter failed with BuildKit file, using BuildKit-specific validation:',
          error,
        );
        return buildKitReport;
      }
    }

    return buildKitReport;
  }

  // Standard validation path for non-BuildKit files
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

  // Run all validation rules directly
  const results: ValidationResult[] = [];

  for (const rule of validationRules) {
    const passed = rule.check(commands);

    // Special handling for no-secrets rule to include the specific secret name
    let message = passed ? `✓ ${rule.name}` : `✗ ${rule.name}: ${rule.message}`;
    if (!passed && rule.id === 'no-secrets') {
      // Extract the secret variable name from the ENV/ARG commands
      const secretVariable = commands.find((cmd: DockerCommand) => {
        if (cmd.name === 'ENV' || cmd.name === 'ARG') {
          const value = getArgValue(cmd);
          const suspicious = [PASSWORD_PATTERN, API_KEY_PATTERN, SECRET_PATTERN, TOKEN_PATTERN];
          return suspicious.some((pattern) => pattern.test(value));
        }
        return false;
      });

      if (secretVariable) {
        const value = getArgValue(secretVariable);
        const variableMatch = value.match(/^([A-Z_][A-Z0-9_]*)\s*=/i);
        const variableName = variableMatch ? variableMatch[1] : 'secret';
        message = `✗ ${rule.name}: ${rule.message} (found: ${variableName})`;
      }
    }

    results.push({
      ruleId: rule.id,
      isValid: passed,
      passed,
      errors: passed ? [] : [`${rule.name}: ${rule.message}`],
      warnings: [],
      message,
      suggestions: !passed && rule.fix ? [rule.fix] : [],
      metadata: {
        severity: rule.severity,
      },
    });
  }

  const internalReport = createReport(results);

  // Add external linter if enabled (default: true)
  if (options?.enableExternalLinter !== false) {
    try {
      const externalReport = await lintWithDockerfilelint(dockerfileContent);
      return mergeReports(internalReport, externalReport);
    } catch (error) {
      // If external linter fails, just return internal results
      console.warn('External linter failed, using internal validation only:', error);
      return internalReport;
    }
  }

  return internalReport;
};
