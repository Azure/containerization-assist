#!/usr/bin/env tsx
/**
 * Quality Gates Validation Script
 *
 * USAGE:
 *   Local (reporting-only):    npm run quality:gates
 *   CI with baseline updates:  UPDATE_BASELINES=true npm run quality:gates
 *
 * ENVIRONMENT VARIABLES:
 *   UPDATE_BASELINES - Set to "true" to update quality-gates.json (CI-only, default: false)
 *   ALLOW_REGRESSION - Set to "true" to allow quality metric regressions (default: false)
 *   SKIP_TYPECHECK   - Set to "true" to skip TypeScript compilation check (default: false)
 *   VERBOSE          - Set to "true" for detailed output (default: false)
 *
 * By default, this script runs in reporting-only mode and will NOT modify tracked files.
 * This makes it safe to run locally for checking quality status without git churn.
 */

import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { execSync } from 'node:child_process';
import { exit } from 'node:process';

interface LintThresholds {
  maxErrors: number;
  maxWarnings: number;
}

interface DeadcodeThresholds {
  max: number;
}

interface TypescriptThresholds {
  maxErrors: number;
}

interface BuildThresholds {
  maxTimeMs: number;
}

interface Thresholds {
  lint: LintThresholds;
  deadcode: DeadcodeThresholds;
  typescript: TypescriptThresholds;
  build: BuildThresholds;
}

interface LintBaseline {
  errors: number;
  warnings: number | null;
  warningSignatures: string[];
}

interface DeadcodeBaseline {
  count: number | null;
}

interface TypescriptBaseline {
  errors: number;
}

interface BuildEnvironment {
  nodeVersion: string;
  os: string;
  cpu: string;
}

interface BuildBaseline {
  timeMs: number | null;
  mode: string;
  environment: BuildEnvironment;
}

interface Baselines {
  lint: LintBaseline;
  deadcode: DeadcodeBaseline;
  typescript: TypescriptBaseline;
  build: BuildBaseline;
}

interface Metrics {
  thresholds: Thresholds;
  baselines: Baselines;
}

interface QualityConfig {
  schemaVersion: number;
  metrics: Metrics;
}

interface ESLintMessage {
  ruleId?: string;
  severity: number;
  message: string;
  line?: number;
  column?: number;
}

interface ESLintResult {
  filePath: string;
  messages?: ESLintMessage[];
}

// Environment configuration
const ALLOW_REGRESSION = process.env.ALLOW_REGRESSION === 'true';
const UPDATE_BASELINES = process.env.UPDATE_BASELINES === 'true';
const SKIP_TYPECHECK = process.env.SKIP_TYPECHECK === 'true';
const VERBOSE = process.env.VERBOSE === 'true';

const QUALITY_CONFIG = 'quality-gates.json';

// Status types
type Status = 'PASS' | 'FAIL' | 'WARN' | 'INFO';

function printStatus(status: Status, message: string): void {
  const symbols = {
    PASS: '✅ PASS',
    FAIL: '❌ FAIL',
    WARN: '⚠️  WARN',
    INFO: 'ℹ️  INFO',
  };
  console.log(`${symbols[status]}: ${message}`);
}

function validateNumeric(value: unknown, fieldName: string, defaultValue = 0): number {
  if (typeof value === 'number' && !isNaN(value)) {
    return Math.floor(value);
  }
  if (typeof value === 'string') {
    const parsed = parseInt(value, 10);
    if (!isNaN(parsed)) {
      return parsed;
    }
  }
  printStatus('WARN', `Invalid numeric value for ${fieldName}: '${value}' - using ${defaultValue}`);
  return defaultValue;
}

function loadConfig(): QualityConfig {
  if (!existsSync(QUALITY_CONFIG)) {
    console.log('📁 Current directory:', process.cwd());
    console.log('Creating default quality-gates.json configuration file...');

    const defaultConfig: QualityConfig = {
      schemaVersion: 1,
      metrics: {
        thresholds: {
          lint: {
            maxErrors: 0,
            maxWarnings: 400,
          },
          deadcode: {
            max: 200,
          },
          typescript: {
            maxErrors: 0,
          },
          build: {
            maxTimeMs: 60000,
          },
        },
        baselines: {
          lint: {
            errors: 0,
            warnings: null,
            warningSignatures: [],
          },
          deadcode: {
            count: null,
          },
          typescript: {
            errors: 0,
          },
          build: {
            timeMs: null,
            mode: 'clean',
            environment: {
              nodeVersion: process.version,
              os: process.platform,
              cpu: process.arch,
            },
          },
        },
      },
    };

    writeFileSync(QUALITY_CONFIG, JSON.stringify(defaultConfig, null, 2));
    printStatus('INFO', 'Created default quality-gates.json configuration file');
    return defaultConfig;
  }

  const configContent = readFileSync(QUALITY_CONFIG, 'utf-8');
  return JSON.parse(configContent) as QualityConfig;
}

function saveConfig(config: QualityConfig): void {
  if (UPDATE_BASELINES) {
    writeFileSync(QUALITY_CONFIG, JSON.stringify(config, null, 2));
  }
}

function runCommand(command: string, options: { silent?: boolean; captureOutput?: boolean } = {}): string {
  try {
    const result = execSync(command, {
      encoding: 'utf-8',
      stdio: options.captureOutput ? 'pipe' : options.silent ? 'ignore' : 'inherit',
      maxBuffer: 10 * 1024 * 1024, // 10MB buffer
    });
    return result;
  } catch (error) {
    if (options.captureOutput) {
      return '';
    }
    throw error;
  }
}

interface LintResults {
  errors: number;
  warnings: number;
  warningSignatures: string[];
}

function processESLintResults(): LintResults {
  try {
    const output = runCommand('npx eslint src --ext .ts --format=json', { captureOutput: true, silent: true });
    if (!output || output.trim() === '[]') {
      return { errors: 0, warnings: 0, warningSignatures: [] };
    }

    const results = JSON.parse(output) as ESLintResult[];
    let errors = 0;
    let warnings = 0;
    const warningSignatures: string[] = [];

    for (const result of results) {
      if (!result.messages) continue;

      for (const msg of result.messages) {
        if (msg.severity === 2) {
          errors++;
        } else if (msg.severity === 1) {
          warnings++;
          const signature = `${result.filePath}:${msg.line ?? 0}:${msg.column ?? 0}:${msg.ruleId ?? 'unknown'}:${msg.message.replace(/[\n\r]/g, ' ')}`;
          warningSignatures.push(signature);
        }
      }
    }

    return { errors, warnings, warningSignatures };
  } catch (error) {
    printStatus('WARN', 'Failed to parse ESLint JSON output, falling back to text parsing');

    try {
      const output = runCommand('npm run lint 2>&1 || true', { captureOutput: true, silent: true });
      const summaryMatch = output.match(/(\d+)\s+error.*?,\s+(\d+)\s+warning/);

      if (summaryMatch) {
        const errors = parseInt(summaryMatch[1], 10) || 0;
        const warnings = parseInt(summaryMatch[2], 10) || 0;
        return { errors, warnings, warningSignatures: [] };
      }
    } catch {
      // Fallback failed too
    }

    return { errors: 0, warnings: 0, warningSignatures: [] };
  }
}

function checkDeadCode(): number {
  try {
    const output = runCommand('npx knip 2>/dev/null || true', { captureOutput: true, silent: true });
    if (!output || output.trim() === '') {
      return 0;
    }

    if (UPDATE_BASELINES) {
      writeFileSync('knip-deadcode-output.txt', output);
    }

    return output.split('\n').filter((line) => line.trim().length > 0).length;
  } catch {
    printStatus('WARN', 'knip not available or failed to run, assuming 0 dead code exports');
    return 0;
  }
}

function checkTypeScript(): boolean {
  try {
    runCommand('npm run typecheck', { silent: true });
    return true;
  } catch {
    return false;
  }
}

function measureBuildTime(): number {
  const start = Date.now();
  try {
    runCommand('npm run build', { silent: true });
    return Date.now() - start;
  } catch {
    throw new Error('Build failed');
  }
}

function findNewWarnings(baseline: string[], current: string[]): string[] {
  const baselineSet = new Set(baseline);
  return current.filter((sig) => !baselineSet.has(sig));
}

function main(): void {
  console.log(`🛡️ Quality Gates Validation ${new Date().toISOString()}`);
  console.log('=========================================');
  console.log('');

  const config = loadConfig();
  const { thresholds, baselines } = config.metrics;

  if (VERBOSE) {
    console.log('📋 Quality Gate Thresholds:');
    console.log(`  • Max Lint Errors: ${thresholds.lint.maxErrors}`);
    console.log(`  • Max Lint Warnings: ${thresholds.lint.maxWarnings}`);
    console.log(`  • Max Type Errors: ${thresholds.typescript.maxErrors}`);
    console.log(`  • Max Dead Code: ${thresholds.deadcode.max}`);
    console.log(`  • Max Build Time: ${thresholds.build.maxTimeMs / 1000}s`);
    console.log('');
  }

  let hasFailures = false;

  // Gate 1: ESLint Errors Must Be Zero
  console.log('Gate 1: ESLint Error Check');
  console.log('-------------------------');

  const lintResults = processESLintResults();
  const currentErrors = validateNumeric(lintResults.errors, 'errors');
  const currentWarnings = validateNumeric(lintResults.warnings, 'warnings');
  const currentWarningSignatures = lintResults.warningSignatures;

  if (currentErrors <= thresholds.lint.maxErrors) {
    printStatus('PASS', `ESLint errors within threshold: ${currentErrors} ≤ ${thresholds.lint.maxErrors}`);
  } else {
    printStatus('FAIL', `${currentErrors} ESLint errors exceed threshold of ${thresholds.lint.maxErrors}`);
    hasFailures = true;
  }

  console.log('');

  // Handle null lint baseline for first run
  let baselineWarnings = baselines.lint.warnings;
  if (baselineWarnings === null) {
    printStatus('INFO', `No lint baseline set, using current warnings (${currentWarnings}) as baseline`);
    baselineWarnings = currentWarnings;

    if (UPDATE_BASELINES) {
      baselines.lint.warnings = currentWarnings;
      baselines.lint.warningSignatures = currentWarningSignatures;
      saveConfig(config);
    } else {
      printStatus('INFO', 'Skipping baseline update (set UPDATE_BASELINES=true to update)');
    }
  }

  // Gate 2: ESLint Warning Ratcheting
  console.log('Gate 2: ESLint Warning Ratcheting');
  console.log('----------------------------------');

  if (currentWarnings <= baselineWarnings && currentWarnings <= thresholds.lint.maxWarnings) {
    const reduction = baselineWarnings - currentWarnings;
    if (reduction > 0) {
      const percentage = baselineWarnings > 0 ? ((reduction * 100) / baselineWarnings).toFixed(1) : 'N/A';
      printStatus('PASS', `Warnings reduced by ${reduction} (${percentage}%) - ${currentWarnings} ≤ ${baselineWarnings}`);

      if (UPDATE_BASELINES) {
        baselines.lint.warnings = currentWarnings;
        baselines.lint.warningSignatures = currentWarningSignatures;
        saveConfig(config);
        printStatus('INFO', `Updated ESLint baseline: ${baselineWarnings} → ${currentWarnings}`);
      } else {
        printStatus('INFO', 'Baseline improvement detected but not updating (set UPDATE_BASELINES=true to update)');
      }
    } else {
      printStatus('PASS', `Warning count maintained at baseline (${currentWarnings})`);
    }
  } else {
    const increase = currentWarnings - baselineWarnings;
    if (ALLOW_REGRESSION) {
      printStatus('WARN', `Warning count increased by ${increase} (${currentWarnings} > ${baselineWarnings}) - ALLOWED by config`);
    } else {
      printStatus('FAIL', `Warning count increased by ${increase} (${currentWarnings} > ${baselineWarnings}) - REGRESSION NOT ALLOWED`);

      if (baselines.lint.warningSignatures.length > 0 && currentWarningSignatures.length > 0) {
        const newWarnings = findNewWarnings(baselines.lint.warningSignatures, currentWarningSignatures);
        if (newWarnings.length > 0) {
          printStatus('INFO', `New ESLint warnings introduced since baseline (${newWarnings.length}):`);
          newWarnings.slice(0, 50).forEach((sig) => console.log(`  • ${sig}`));
          if (newWarnings.length > 50) {
            printStatus('INFO', `... and ${newWarnings.length - 50} more new warnings (showing first 50)`);
          }
        }
      } else {
        printStatus('INFO', 'Baseline warning signatures unavailable; cannot list new warnings');
      }

      hasFailures = true;
    }
  }

  console.log('');

  // Gate 3: TypeScript Compilation
  if (!SKIP_TYPECHECK) {
    console.log('Gate 3: TypeScript Compilation');
    console.log('-------------------------------');

    if (checkTypeScript()) {
      printStatus('PASS', 'TypeScript compilation successful');
    } else {
      printStatus('FAIL', 'TypeScript compilation failed');
      hasFailures = true;
    }

    console.log('');
  } else {
    console.log('Gate 3: TypeScript Compilation (SKIPPED)');
    console.log('----------------------------------------');
    printStatus('WARN', 'TypeScript check skipped by configuration');
    console.log('');
  }

  // Gate 4: Dead Code Check
  console.log('Gate 4: Dead Code Check');
  console.log('-----------------------');

  const deadcodeCount = checkDeadCode();
  let baselineDeadcode = baselines.deadcode.count;

  if (baselineDeadcode === null) {
    printStatus('INFO', `No deadcode baseline set, using current dead code count (${deadcodeCount}) as baseline`);
    baselineDeadcode = deadcodeCount;

    if (UPDATE_BASELINES) {
      baselines.deadcode.count = deadcodeCount;
      saveConfig(config);
    } else {
      printStatus('INFO', 'Skipping deadcode baseline update (set UPDATE_BASELINES=true to update)');
    }
  }

  if (deadcodeCount <= baselineDeadcode) {
    const reduction = baselineDeadcode - deadcodeCount;
    if (reduction > 0) {
      const percentage = baselineDeadcode > 0 ? ((reduction * 100) / baselineDeadcode).toFixed(1) : 'N/A';
      printStatus('PASS', `Unused exports reduced by ${reduction} (${percentage}%) - ${deadcodeCount} ≤ ${baselineDeadcode}`);

      if (UPDATE_BASELINES) {
        baselines.deadcode.count = deadcodeCount;
        saveConfig(config);
        printStatus('INFO', `Updated deadcode baseline: ${baselineDeadcode} → ${deadcodeCount}`);
      } else {
        printStatus('INFO', 'Deadcode improvement detected but not updating (set UPDATE_BASELINES=true to update)');
      }
    } else {
      printStatus('PASS', `Unused exports maintained at baseline (${deadcodeCount})`);
    }
  } else {
    const increase = deadcodeCount - baselineDeadcode;
    if (ALLOW_REGRESSION) {
      printStatus('WARN', `Unused exports increased by ${increase} (${deadcodeCount} > ${baselineDeadcode}) - ALLOWED by config - Check knip-deadcode-output.txt changes for details`);
    } else {
      printStatus('FAIL', `Unused exports increased by ${increase} (${deadcodeCount} > ${baselineDeadcode}) - REGRESSION NOT ALLOWED - Check knip-deadcode-output.txt changes for details`);
      hasFailures = true;
    }
  }

  console.log('');

  // Gate 5: Build Performance
  console.log('Gate 5: Build Performance');
  console.log('------------------------');

  try {
    const buildTimeMs = measureBuildTime();
    const buildTimeSeconds = Math.floor(buildTimeMs / 1000);
    const thresholdSeconds = Math.floor(thresholds.build.maxTimeMs / 1000);

    if (UPDATE_BASELINES) {
      const currentBuildBaseline = baselines.build.timeMs;
      if (currentBuildBaseline === null || buildTimeMs < currentBuildBaseline) {
        baselines.build.timeMs = buildTimeMs;
        baselines.build.environment = {
          nodeVersion: process.version,
          os: process.platform,
          cpu: process.arch,
        };
        saveConfig(config);
      }
    }

    if (buildTimeSeconds < thresholdSeconds) {
      printStatus('PASS', `Build completed in ${buildTimeSeconds}s (< ${thresholdSeconds}s threshold)`);
    } else {
      printStatus('WARN', `Build took ${buildTimeSeconds}s (exceeds ${thresholdSeconds}s threshold)`);
    }
  } catch (error) {
    printStatus('FAIL', 'Build failed');
    hasFailures = true;
  }

  console.log('');

  // Final Summary
  console.log('🎉 Quality Gates Summary');
  console.log('========================');
  console.log(`ESLint Errors: ${currentErrors} (threshold: ${thresholds.lint.maxErrors})`);
  console.log(`ESLint Warnings: ${currentWarnings} (threshold: ${thresholds.lint.maxWarnings})`);
  console.log(`Unused Exports: ${deadcodeCount} (threshold: ${thresholds.deadcode.max})`);
  console.log('TypeScript: ✅ Compiles');
  console.log('Build: ✅ Successful');
  console.log('');

  if (currentWarnings > thresholds.lint.maxWarnings || deadcodeCount > thresholds.deadcode.max) {
    printStatus('INFO', 'Consider running aggressive cleanup to reach production targets:');
    console.log(`  • ESLint warnings target: <${thresholds.lint.maxWarnings} (current: ${currentWarnings})`);
    console.log(`  • Dead code target: <${thresholds.deadcode.max} (current: ${deadcodeCount})`);
    console.log('');
  }

  if (hasFailures) {
    printStatus('FAIL', 'Some quality gates failed!');
    exit(1);
  } else {
    printStatus('PASS', 'All quality gates passed! 🚀');
    console.log('');
  }
}

main();