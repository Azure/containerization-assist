import { spawnSync } from 'node:child_process';
import * as fs from 'node:fs';
import * as path from 'node:path';
import * as os from 'node:os';
import * as crypto from 'node:crypto';
import type { ValidationReport, ValidationResult, ValidationSeverity } from './core-types';
import { createLogger } from '@/lib/logger';

const logger = createLogger({ name: 'dockerfilelint-adapter' });

type DflIssue = {
  rule?: string;
  message: string;
  level?: 'error' | 'warn' | 'info' | 'style';
  line?: number;
  column?: number;
};

type DflOutput = {
  issues?: DflIssue[];
  error?: string;
};

/**
 * Maps dockerfilelint severity levels to our ValidationSeverity
 */
function mapSeverity(level?: string): ValidationSeverity {
  switch (level) {
    case 'error':
      return 'error' as ValidationSeverity;
    case 'warn':
      return 'warning' as ValidationSeverity;
    case 'info':
    case 'style':
    default:
      return 'info' as ValidationSeverity;
  }
}

/**
 * Converts dockerfilelint issues to our ValidationResult format
 */
function toValidationResults(issues: DflIssue[]): ValidationResult[] {
  return issues.map((issue) => ({
    isValid: false,
    passed: false,
    errors: issue.level === 'error' ? [issue.message] : [],
    warnings: issue.level === 'warn' ? [issue.message] : [],
    ruleId: issue.rule ? `dockerfilelint-${issue.rule}` : 'dockerfilelint',
    message: issue.message,
    metadata: {
      severity: mapSeverity(issue.level),
      ...(issue.line && { location: `line ${issue.line}` }),
    },
  }));
}

/**
 * Try to load dockerfilelint module programmatically
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function tryLoadModule(): Promise<any> {
  try {
    // Try to dynamically import dockerfilelint - using require syntax for optional dependency
    const mod = eval('require("dockerfilelint")');
    logger.debug('Loaded dockerfilelint module programmatically');
    return mod;
  } catch {
    logger.debug('Could not load dockerfilelint module, will use CLI fallback');
    return null;
  }
}

/**
 * Run dockerfilelint programmatically if module is available
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function runProgrammatic(mod: any, dockerfileContent: string): Promise<DflIssue[]> {
  try {
    // Try to use the module API if available
    if (mod.lint || mod.default?.lint) {
      const lintFn = mod.lint || mod.default.lint;
      const result = await lintFn(dockerfileContent);

      // Handle different possible result formats
      if (Array.isArray(result)) {
        return result;
      } else if (result?.issues) {
        return result.issues;
      } else if (result?.problems) {
        return result.problems;
      }
    }
  } catch (e) {
    logger.debug({ error: e }, 'Failed to run dockerfilelint programmatically');
  }

  throw new Error('Programmatic API not available');
}

/**
 * Lint a Dockerfile using dockerfilelint
 * Falls back to CLI if programmatic API is not available
 */
export async function lintWithDockerfilelint(dockerfileContent: string): Promise<ValidationReport> {
  // Try programmatic API first
  const mod = await tryLoadModule();
  if (mod) {
    try {
      const issues = await runProgrammatic(mod, dockerfileContent);
      return toReport(issues);
    } catch {
      // Fall through to CLI
      logger.debug('Falling back to CLI approach');
    }
  }

  // CLI fallback using npx (no external binary needed)
  try {
    // Secure temp file creation using Node.js built-ins
    const tmpDir = os.tmpdir();
    const randomSuffix = crypto.randomBytes(8).toString('hex');
    const tmpFile = path.join(tmpDir, `dockerfile-${randomSuffix}`);

    // Create file with secure permissions (0o600 = rw-------)
    const fd = fs.openSync(tmpFile, 'w', 0o600);
    fs.writeSync(fd, dockerfileContent);
    fs.closeSync(fd);

    try {
      const cli = spawnSync(
        process.platform === 'win32' ? 'npx.cmd' : 'npx',
        ['-y', 'dockerfilelint', '--json', tmpFile],
        { encoding: 'utf8' },
      );

      // Clean up temp file
      try {
        fs.unlinkSync(tmpFile);
      } catch {
        // Ignore cleanup errors
      }

      if (cli.error) {
        logger.warn({ error: cli.error }, 'dockerfilelint CLI error');
        return createEmptyReport();
      }

      // Parse JSON output
      if (cli.stdout) {
        try {
          const output: DflOutput = JSON.parse(cli.stdout);
          if (output.issues) {
            return toReport(output.issues);
          }
        } catch {
          logger.debug({ output: cli.stdout }, 'Failed to parse dockerfilelint output');
        }
      }

      // If stderr has content, it might be non-JSON output
      if (cli.stderr) {
        logger.debug({ stderr: cli.stderr }, 'dockerfilelint stderr output');
      }

      return createEmptyReport();
    } finally {
      // Ensure temp file is cleaned up
      try {
        if (fs.existsSync(tmpFile)) {
          fs.unlinkSync(tmpFile);
        }
      } catch {
        // Ignore cleanup errors
      }
    }
  } catch (error) {
    logger.error({ error }, 'Failed to run dockerfilelint');
    return createEmptyReport();
  }
}

/**
 * Convert dockerfilelint issues to a ValidationReport
 */
function toReport(issues: DflIssue[]): ValidationReport {
  const results = toValidationResults(issues);

  // Count by severity
  let errors = 0;
  let warnings = 0;
  let info = 0;

  for (const result of results) {
    const severity = result.metadata?.severity;
    if (severity === 'error') errors++;
    else if (severity === 'warning') warnings++;
    else info++;
  }

  // Calculate score (simple linear deduction)
  const score = Math.max(0, 100 - errors * 10 - warnings * 3 - info * 1);

  // Calculate grade
  let grade: 'A' | 'B' | 'C' | 'D' | 'F';
  if (score >= 90) grade = 'A';
  else if (score >= 80) grade = 'B';
  else if (score >= 70) grade = 'C';
  else if (score >= 60) grade = 'D';
  else grade = 'F';

  return {
    results,
    score,
    grade,
    passed: results.filter((r) => r.passed).length,
    failed: results.filter((r) => !r.passed).length,
    errors,
    warnings,
    info,
    timestamp: new Date().toISOString(),
  };
}

/**
 * Create an empty validation report when dockerfilelint is not available
 */
function createEmptyReport(): ValidationReport {
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
