/**
 * Rego Policy Evaluation Module
 * Uses OPA CLI for policy evaluation instead of WASM
 *
 * This module provides integration with Open Policy Agent (OPA) for policy evaluation.
 * It supports loading and evaluating Rego policies against Dockerfile and Kubernetes content.
 */

import { readFile, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import type { Logger } from 'pino';
import { type Result, Success, Failure } from '@/types';
import { ERROR_MESSAGES } from '@/lib/errors';

const execFileAsync = promisify(execFile);

/**
 * Policy violation returned from Rego evaluation
 */
export interface RegoPolicyViolation {
  rule: string;
  message: string;
  severity: 'block' | 'warn' | 'suggest';
  category: string;
  priority?: number;
  description?: string;
}

/**
 * Rego policy evaluation result
 */
export interface RegoPolicyResult {
  allow: boolean;
  violations: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
  summary?: {
    total_violations: number;
    total_warnings: number;
    total_suggestions: number;
  };
}

/**
 * Rego policy evaluator interface
 */
export interface RegoEvaluator {
  /**
   * Evaluate policy against input
   * @param input - Content to evaluate (Dockerfile text, K8s manifest, etc.)
   */
  evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult>;

  /**
   * Clean up resources
   */
  close(): void;

  /**
   * Policy file path(s)
   */
  policyPaths: string[];
}

/**
 * Get path to OPA binary
 */
function getOpaBinaryPath(): string {
  // First try project's node_modules
  const localOpa = join(process.cwd(), 'node_modules', '.bin', 'opa');
  if (existsSync(localOpa)) {
    return localOpa;
  }

  // Fall back to system OPA
  return 'opa';
}

/**
 * Load and compile a Rego policy from file
 *
 * @param policyPath - Path to .rego policy file
 * @param logger - Logger instance for diagnostics
 * @returns Result containing RegoEvaluator or error
 *
 * @example
 * ```typescript
 * const result = await loadRegoPolicy('policies/security.rego', logger);
 * if (result.ok) {
 *   const evalResult = await result.value.evaluate(dockerfileContent);
 *   if (!evalResult.allow) {
 *     console.log('Violations:', evalResult.violations);
 *   }
 * }
 * ```
 */
export async function loadRegoPolicy(
  policyPath: string,
  logger: Logger,
): Promise<Result<RegoEvaluator>> {
  try {
    // Validate file exists
    if (!existsSync(policyPath)) {
      return Failure(`Policy file not found: ${policyPath}`, {
        message: 'Rego policy file does not exist',
        hint: `Attempted to load: ${policyPath}`,
        resolution: 'Ensure the policy file path is correct',
      });
    }

    // Validate .rego extension
    if (!policyPath.endsWith('.rego')) {
      return Failure('Only .rego policy files are supported', {
        message: 'Invalid policy file format',
        hint: `File: ${policyPath}`,
        resolution: 'Provide a .rego policy file',
      });
    }

    // Read the policy file to validate it
    const policyContent = await readFile(policyPath, 'utf-8');

    logger.info({ policyPath, size: policyContent.length }, 'Loading Rego policy');

    // Test that OPA binary is available
    const opaBinary = getOpaBinaryPath();
    try {
      await execFileAsync(opaBinary, ['version']);
    } catch {
      return Failure('OPA binary not found', {
        message: 'Open Policy Agent (OPA) CLI is not installed',
        hint: 'OPA is required for policy evaluation',
        resolution: 'Install OPA: https://www.openpolicyagent.org/docs/latest/#running-opa',
      });
    }

    logger.info({ policyPath }, 'Rego policy loaded successfully');

    // Create evaluator
    const evaluator: RegoEvaluator = {
      policyPaths: [policyPath],
      evaluate: async (input: string | Record<string, unknown>) => {
        return evaluateRegoPolicy(policyPath, input, logger);
      },
      close: () => {
        logger.debug({ policyPath }, 'Cleaning up Rego policy resources');
      },
    };

    return Success(evaluator);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(message), {
      message: 'Failed to load Rego policy',
      hint: `Error: ${message}`,
      resolution: 'Check policy file syntax and OPA compatibility',
    });
  }
}

/**
 * Evaluate Rego policy against input using OPA CLI
 *
 * @param policyPaths - Path(s) to the Rego policy file(s)
 * @param input - Content to evaluate
 * @param logger - Logger instance
 * @returns Policy evaluation result
 */
async function evaluateRegoPolicy(
  policyPaths: string | string[],
  input: string | Record<string, unknown>,
  logger: Logger,
): Promise<RegoPolicyResult> {
  try {
    // Convert input to the format expected by the policy
    const inputData = typeof input === 'string' ? { content: input } : input;

    logger.debug({ inputType: typeof input }, 'Evaluating Rego policy');

    // Create a temporary file for input
    const inputFile = join(tmpdir(), `opa-input-${Date.now()}-${Math.random().toString(36).substr(2, 9)}.json`);
    await writeFile(inputFile, JSON.stringify(inputData));

    try {
      const opaBinary = getOpaBinaryPath();

      // Build args with multiple -d flags for each policy file (OPA will merge them)
      const paths = Array.isArray(policyPaths) ? policyPaths : [policyPaths];
      const policyArgs = paths.flatMap(p => ['-d', p]);

      // Run OPA eval command to evaluate the policy
      // Use -f json for JSON output and query data.containerization to get all results
      const { stdout, stderr } = await execFileAsync(
        opaBinary,
        [
          'eval',
          ...policyArgs,
          '-i', inputFile,
          '-f', 'json',
          'data.containerization',
        ],
        {
          maxBuffer: 10 * 1024 * 1024, // 10MB buffer
        },
      );

      if (stderr) {
        logger.debug({ stderr }, 'OPA eval stderr output');
      }

      // Parse the OPA output
      const combinedResult: any = {
        allow: true,
        violations: [],
        warnings: [],
        suggestions: [],
      };

      try {
        const output = JSON.parse(stdout);

        // OPA JSON format: { result: [{ expressions: [{ value: ... }] }] }
        if (
          output?.result && Array.isArray(output.result) && output.result.length > 0
        ) {
          const firstResult = output.result[0];
          if (
            firstResult?.expressions &&
            Array.isArray(firstResult.expressions) &&
            firstResult.expressions.length > 0
          ) {
            const containerization = firstResult.expressions[0]?.value;

            if (containerization) {
              // Merge results from all policies (security, best_practices, base_images)
              const namespaces = ['security', 'best_practices', 'base_images'];

              for (const ns of namespaces) {
                const nsResult = containerization[ns]?.result;
                if (nsResult) {
                  // Merge allow (false if any policy blocks)
                  if (nsResult.allow === false) {
                    combinedResult.allow = false;
                  }

                  // Merge violations, warnings, suggestions
                  if (nsResult.violations) {
                    const violations = Array.isArray(nsResult.violations)
                      ? nsResult.violations
                      : Object.values(nsResult.violations);
                    combinedResult.violations.push(...violations);
                  }

                  if (nsResult.warnings) {
                    const warnings = Array.isArray(nsResult.warnings)
                      ? nsResult.warnings
                      : Object.values(nsResult.warnings);
                    combinedResult.warnings.push(...warnings);
                  }

                  if (nsResult.suggestions) {
                    const suggestions = Array.isArray(nsResult.suggestions)
                      ? nsResult.suggestions
                      : Object.values(nsResult.suggestions);
                    combinedResult.suggestions.push(...suggestions);
                  }
                }
              }
            }
          }
        }
      } catch (parseError) {
        logger.warn({ stdout, parseError }, 'Failed to parse OPA output');
      }

      const violations: RegoPolicyViolation[] = combinedResult.violations;
      const warnings: RegoPolicyViolation[] = combinedResult.warnings;
      const suggestions: RegoPolicyViolation[] = combinedResult.suggestions;
      const allow = Boolean(combinedResult.allow);

      logger.info(
        {
          allow,
          violations: violations.length,
          warnings: warnings.length,
          suggestions: suggestions.length,
        },
        'Rego policy evaluation completed',
      );

      return {
        allow,
        violations,
        warnings,
        suggestions,
        summary: {
          total_violations: violations.length,
          total_warnings: warnings.length,
          total_suggestions: suggestions.length,
        },
      };
    } finally {
      // Clean up temp file
      try {
        const { unlink } = await import('node:fs/promises');
        await unlink(inputFile);
      } catch {
        // Ignore cleanup errors
      }
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error({ error: message }, 'Rego policy evaluation failed');

    // Return a safe default on error
    return {
      allow: false,
      violations: [
        {
          rule: 'policy-evaluation-error',
          category: 'system',
          message: `Policy evaluation failed: ${message}`,
          severity: 'block',
        },
      ],
      warnings: [],
      suggestions: [],
    };
  }
}

/**
 * Load and merge multiple Rego policy files
 *
 * OPA CLI automatically merges multiple policy files when passed with multiple -d flags.
 * All policies will be evaluated together and their results combined.
 *
 * @param policyPaths - Array of .rego policy file paths
 * @param logger - Logger instance
 * @returns Result containing RegoEvaluator or error
 */
export async function loadAndMergeRegoPolicies(
  policyPaths: string[],
  logger: Logger,
): Promise<Result<RegoEvaluator>> {
  if (policyPaths.length === 0) {
    return Failure('No policy paths provided');
  }

  // Validate all policy files exist
  for (const policyPath of policyPaths) {
    if (!existsSync(policyPath)) {
      return Failure(`Policy file not found: ${policyPath}`, {
        message: 'Rego policy file does not exist',
        hint: `Attempted to load: ${policyPath}`,
        resolution: 'Ensure all policy file paths are correct',
      });
    }

    if (!policyPath.endsWith('.rego')) {
      return Failure(`Invalid policy file format: ${policyPath}`, {
        message: 'All policy files must be .rego format',
        hint: `File: ${policyPath}`,
        resolution: 'Provide only .rego policy files',
      });
    }
  }

  logger.info({ policyCount: policyPaths.length, policies: policyPaths }, 'Loading and merging Rego policies');

  // Test that OPA binary is available
  const opaBinary = getOpaBinaryPath();
  try {
    await execFileAsync(opaBinary, ['version']);
  } catch {
    return Failure('OPA binary not found', {
      message: 'Open Policy Agent (OPA) CLI is not installed',
      hint: 'OPA is required for policy evaluation',
      resolution: 'Install OPA: https://www.openpolicyagent.org/docs/latest/#running-opa',
    });
  }

  // Create evaluator that will evaluate all policies together
  const evaluator: RegoEvaluator = {
    policyPaths,
    evaluate: async (input: string | Record<string, unknown>) => {
      return evaluateRegoPolicy(policyPaths, input, logger);
    },
    close: () => {
      logger.debug({ policyPaths }, 'Cleaning up merged Rego policy resources');
    },
  };

  logger.info({ policyPaths }, 'Rego policies loaded and ready for merged evaluation');

  return Success(evaluator);
}
