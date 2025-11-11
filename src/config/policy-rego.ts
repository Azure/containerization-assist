/**
 * Rego Policy Evaluation Module
 * Hybrid approach: WASM (fast, no deps) + OPA CLI fallback
 *
 * This module provides integration with Open Policy Agent (OPA) for policy evaluation.
 * It supports loading and evaluating Rego policies against Dockerfile and Kubernetes content.
 *
 * **Evaluation Strategy:**
 * 1. Pre-compiled WASM bundle (policies/compiled/policies.wasm) - Fast, zero-dependency
 * 2. OPA CLI fallback for custom .rego files - Requires OPA installation
 */

import { readFile, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { join, dirname } from 'node:path';
import tmp from 'tmp';
import type { Logger } from 'pino';
import { loadPolicy as loadWasmPolicy, LoadedPolicy as WasmPolicy } from '@open-policy-agent/opa-wasm';
import { type Result, Success, Failure } from '@/types';
import { ERROR_MESSAGES } from '@/lib/errors';

const execFileAsync = promisify(execFile);

/**
 * Built-in policy mappings
 * Maps policy filenames to their WASM bundle entrypoint names
 */
const BUILT_IN_POLICY_MODULES: Record<string, string> = {
  'security-baseline.rego': 'containerization/security/result',
  'base-images.rego': 'containerization/base_images/result',
  'container-best-practices.rego': 'containerization/best_practices/result',
};

/**
 * Cached project root to avoid blocking the event loop on repeated calls
 */
let cachedProjectRoot: string | undefined;

/**
 * Get project root directory
 * Works in both ESM and CJS builds
 *
 * Note: This searches from process.cwd() upward, which works for:
 * - Direct package usage (policies/ in package root)
 * - Monorepos (policies/ in package directory)
 *
 * For packages installed in node_modules, the WASM bundle is shipped
 * in the package and resolved relative to the built dist/ directory.
 */
function getProjectRoot(): string {
  if (cachedProjectRoot) {
    return cachedProjectRoot;
  }

  // Try to find policies directory by walking up from current working directory
  let currentDir = process.cwd();
  while (currentDir !== dirname(currentDir)) {
    const policiesDir = join(currentDir, 'policies');
    if (existsSync(policiesDir)) {
      cachedProjectRoot = currentDir;
      return currentDir;
    }
    currentDir = dirname(currentDir);
  }

  // Fall back to cwd (works for npm packages since policies/ is at package root)
  cachedProjectRoot = process.cwd();
  return cachedProjectRoot;
}

const projectRoot = getProjectRoot();

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
 * Categorized policy violations for tool consumption
 */
export interface CategorizedViolations {
  blocking: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
}

/**
 * OPA CLI JSON output structure
 */
interface OpaEvalOutput {
  result?: Array<{
    expressions?: Array<{
      value?: {
        [namespace: string]: {
          result?: {
            allow?: boolean;
            violations?: RegoPolicyViolation[] | Record<string, RegoPolicyViolation>;
            warnings?: RegoPolicyViolation[] | Record<string, RegoPolicyViolation>;
            suggestions?: RegoPolicyViolation[] | Record<string, RegoPolicyViolation>;
          };
        };
      };
    }>;
  }>;
}

/**
 * Internal combined result during policy evaluation
 */
interface CombinedPolicyResult {
  allow: boolean;
  violations: RegoPolicyViolation[];
  warnings: RegoPolicyViolation[];
  suggestions: RegoPolicyViolation[];
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
   * Evaluate policy against input with Result wrapper
   * @param result - Result containing content to evaluate
   * @param packageName - Package name to query (optional)
   * @returns Result containing policy evaluation
   */
  evaluatePolicy<T>(
    result: Result<T>,
    packageName?: string,
  ): Promise<Result<CategorizedViolations>>;

  /**
   * Query policy for configuration data
   * @param packageName - OPA package name to query (e.g., 'containerization.generation_config')
   * @param input - Input data for the query
   * @returns Configuration object from policy
   */
  queryConfig<T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null>;

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
 * Helper function to implement evaluatePolicy wrapper around evaluate
 * This provides a convenience method for tests that wraps evaluate with Result handling
 */
function createEvaluatePolicyWrapper(
  evaluate: (input: string | Record<string, unknown>) => Promise<RegoPolicyResult>,
) {
  return async <T>(
    result: Result<T>,
    _packageName?: string,
  ): Promise<Result<CategorizedViolations>> => {
    if (!result.ok) {
      return result as Result<never>;
    }

    try {
      const policyResult = await evaluate(result.value as string | Record<string, unknown>);

      return Success({
        blocking: policyResult.violations,
        warnings: policyResult.warnings,
        suggestions: policyResult.suggestions,
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return Failure(`Policy evaluation failed: ${message}`);
    }
  };
}

/**
 * Get path to OPA binary
 */
function getOpaBinaryPath(): string {
  // Determine platform-specific OPA binary name
  const opaBinaryName = process.platform === 'win32' ? 'opa.exe' : 'opa';

  // First try project's node_modules
  const localOpa = join(process.cwd(), 'node_modules', '.bin', opaBinaryName);
  if (existsSync(localOpa)) {
    return localOpa;
  }

  // Fall back to system OPA
  return opaBinaryName;
}

/**
 * Create an error response for missing policy entrypoint mapping
 */
function createPolicyMappingError(policyFileName: string): RegoPolicyResult {
  return {
    allow: false,
    violations: [
      {
        rule: 'policy-mapping-error',
        category: 'system',
        message: `No WASM entrypoint mapping for policy: ${policyFileName}`,
        severity: 'block',
      },
    ],
    warnings: [],
    suggestions: [],
  };
}

/**
 * Evaluate a specific WASM policy module (for single policy file loads)
 */
async function evaluateWasmPolicyModule(
  wasmPolicy: WasmPolicy,
  input: string | Record<string, unknown>,
  entrypointName: string,
  logger: Logger,
): Promise<RegoPolicyResult> {
  try {
    const inputData = typeof input === 'string' ? { content: input } : input;

    logger.debug({ entrypoint: entrypointName }, 'Evaluating WASM policy module');

    const resultSet = wasmPolicy.evaluate(inputData, entrypointName);

    if (!resultSet || resultSet.length === 0 || !resultSet[0]?.result) {
      logger.warn({ entrypoint: entrypointName }, 'WASM policy module returned no results');
      return {
        allow: true,
        violations: [],
        warnings: [],
        suggestions: [],
      };
    }

    const moduleResult = resultSet[0].result as {
      allow?: boolean;
      violations?: RegoPolicyViolation[];
      warnings?: RegoPolicyViolation[];
      suggestions?: RegoPolicyViolation[];
    };

    logger.info(
      {
        allow: moduleResult.allow ?? true,
        violations: moduleResult.violations?.length ?? 0,
        warnings: moduleResult.warnings?.length ?? 0,
        suggestions: moduleResult.suggestions?.length ?? 0,
      },
      'WASM policy module evaluation completed',
    );

    return {
      allow: moduleResult.allow ?? true,
      violations: moduleResult.violations ?? [],
      warnings: moduleResult.warnings ?? [],
      suggestions: moduleResult.suggestions ?? [],
      summary: {
        total_violations: moduleResult.violations?.length ?? 0,
        total_warnings: moduleResult.warnings?.length ?? 0,
        total_suggestions: moduleResult.suggestions?.length ?? 0,
      },
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error({ error: message }, 'WASM policy module evaluation failed');

    return {
      allow: false,
      violations: [
        {
          rule: 'policy-evaluation-error',
          category: 'system',
          message: `WASM policy evaluation failed: ${message}`,
          severity: 'block',
        },
      ],
      warnings: [],
      suggestions: [],
    };
  }
}


/**
 * Load and compile a Rego policy from file
 *
 * Hybrid approach:
 * 1. Check for pre-compiled WASM (fast, no OPA required)
 * 2. Fall back to OPA binary for .rego files (requires OPA installed)
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

    // Check for pre-compiled WASM bundle (fast path)
    // Only use WASM when loading one of the built-in policy files
    // This ensures tests that load specific policies get only those policies
    const policyFileName = policyPath.split(/[/\\]/).pop() || '';
    const isBuiltInPolicy = policyFileName in BUILT_IN_POLICY_MODULES;

    const wasmPath = join(projectRoot, 'policies', 'compiled', 'policies.wasm');
    if (isBuiltInPolicy && existsSync(wasmPath)) {
      try {
        logger.info({ wasmPath, policyFile: policyFileName }, 'Loading pre-compiled WASM policy bundle (fast path)');
        const wasmBytes = await readFile(wasmPath);
        const wasmPolicy = await loadWasmPolicy(wasmBytes);

        const evaluator: RegoEvaluator = {
          policyPaths: [policyPath],
          evaluate: async (input: string | Record<string, unknown>) => {
            // Get the WASM entrypoint for the requested policy module
            const entrypointName = BUILT_IN_POLICY_MODULES[policyFileName];

            // This should always be defined since isBuiltInPolicy is true
            // but TypeScript can't verify this statically
            if (!entrypointName) {
              logger.error({ policyFileName }, 'No entrypoint mapping found for built-in policy');
              return createPolicyMappingError(policyFileName);
            }

            return evaluateWasmPolicyModule(wasmPolicy, input, entrypointName, logger);
          },
          evaluatePolicy: createEvaluatePolicyWrapper(async (input) => {
            const entrypointName = BUILT_IN_POLICY_MODULES[policyFileName];
            if (!entrypointName) {
              return createPolicyMappingError(policyFileName);
            }
            return evaluateWasmPolicyModule(wasmPolicy, input, entrypointName, logger);
          }),
          queryConfig: async <T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
            try {
              logger.debug({ packageName, wasmMode: true }, 'Querying WASM policy for config');
              const inputData = input;

              // Query the WASM policy for the package
              const resultSet = wasmPolicy.evaluate(inputData, packageName);

              if (!resultSet || resultSet.length === 0 || !resultSet[0]?.result) {
                logger.debug({ packageName }, 'WASM policy returned no config data');
                return null;
              }

              logger.debug({ packageName, hasValue: true }, 'WASM policy config query successful');
              return resultSet[0].result as T;
            } catch (error) {
              const message = error instanceof Error ? error.message : String(error);
              logger.warn({ error: message, packageName }, 'WASM policy config query failed, returning null');
              return null;
            }
          },
          close: () => {
            logger.debug({ policyPath }, 'Cleaning up WASM policy resources');
          },
        };

        logger.info({ policyModule: policyFileName }, 'WASM policy module loaded successfully');
        return Success(evaluator);
      } catch (wasmError) {
        logger.warn(
          { error: wasmError },
          'Failed to load WASM bundle, falling back to OPA binary',
        );
        // Continue to OPA binary fallback
      }
    }

    // Fall back to OPA binary evaluation
    logger.info({ policyPath }, 'Using OPA binary for policy evaluation (requires OPA installed)');

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
        hint: 'OPA is required for custom .rego policy evaluation. For built-in policies, run: npm run build:policies',
        resolution: 'Install OPA: https://www.openpolicyagent.org/docs/latest/#running-opa',
      });
    }

    logger.info({ policyPath }, 'Rego policy loaded successfully via OPA binary');

    // Create evaluator
    const evaluator: RegoEvaluator = {
      policyPaths: [policyPath],
      evaluate: async (input: string | Record<string, unknown>) => {
        return evaluateRegoPolicy(policyPath, input, logger);
      },
      evaluatePolicy: createEvaluatePolicyWrapper(async (input) => {
        return evaluateRegoPolicy(policyPath, input, logger);
      }),
      queryConfig: async <T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
        return queryRegoConfig<T>(policyPath, packageName, input, logger);
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
 * Query Rego policy for configuration data using OPA CLI
 *
 * @param policyPaths - Path(s) to the Rego policy file(s)
 * @param packageName - Package name to query (e.g., 'containerization.generation_config')
 * @param input - Input data for the query
 * @param logger - Logger instance
 * @returns Configuration data or null if not found
 */
async function queryRegoConfig<T = unknown>(
  policyPaths: string | string[],
  packageName: string,
  input: Record<string, unknown>,
  logger: Logger,
): Promise<T | null> {
  try {
    logger.debug({ packageName, inputKeys: Object.keys(input) }, 'Querying Rego policy for config');

    // Create a secure temporary file for input using tmp package
    const tmpFile = tmp.fileSync({ prefix: 'opa-input-', postfix: '.json' });
    await writeFile(tmpFile.name, JSON.stringify(input));

    try {
      const opaBinary = getOpaBinaryPath();

      // Build args with multiple -d flags for each policy file
      const paths = Array.isArray(policyPaths) ? policyPaths : [policyPaths];
      const policyArgs = paths.flatMap(p => ['-d', p]);

      // Query the specific package for configuration
      const query = `data.${packageName}`;
      const { stdout, stderr } = await execFileAsync(
        opaBinary,
        [
          'eval',
          ...policyArgs,
          '-i', tmpFile.name,
          '-f', 'json',
          query,
        ],
        {
          maxBuffer: 10 * 1024 * 1024, // 10MB buffer
        },
      );

      if (stderr) {
        logger.debug({ stderr }, 'OPA eval stderr output');
      }

      // Parse the OPA output
      const output: unknown = JSON.parse(stdout);
      const opaOutput = output as OpaEvalOutput;

      // OPA JSON format: { result: [{ expressions: [{ value: ... }] }] }
      if (
        opaOutput?.result && Array.isArray(opaOutput.result) && opaOutput.result.length > 0
      ) {
        const firstResult = opaOutput.result[0];
        if (
          firstResult?.expressions &&
          Array.isArray(firstResult.expressions) &&
          firstResult.expressions.length > 0
        ) {
          const value = firstResult.expressions[0]?.value;

          if (value !== undefined && value !== null) {
            logger.debug({ packageName, hasValue: true }, 'Policy config query successful');
            return value as T;
          }
        }
      }

      logger.debug({ packageName }, 'Policy config query returned no data');
      return null;
    } finally {
      // Clean up temp file
      try {
        tmpFile.removeCallback();
      } catch {
        // Ignore cleanup errors
      }
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.warn({ error: message, packageName }, 'Policy config query failed, returning null');
    return null;
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

    // Create a secure temporary file for input using tmp package
    const tmpFile = tmp.fileSync({ prefix: 'opa-input-', postfix: '.json' });
    await writeFile(tmpFile.name, JSON.stringify(inputData));

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
          '-i', tmpFile.name,
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
      const combinedResult: CombinedPolicyResult = {
        allow: true,
        violations: [],
        warnings: [],
        suggestions: [],
      };

      try {
        const output: unknown = JSON.parse(stdout);
        const opaOutput = output as OpaEvalOutput;

        // OPA JSON format: { result: [{ expressions: [{ value: ... }] }] }
        if (
          opaOutput?.result && Array.isArray(opaOutput.result) && opaOutput.result.length > 0
        ) {
          const firstResult = opaOutput.result[0];
          if (
            firstResult?.expressions &&
            Array.isArray(firstResult.expressions) &&
            firstResult.expressions.length > 0
          ) {
            const containerization = firstResult.expressions[0]?.value;

            if (containerization) {
              // Dynamically merge results from ALL policy namespaces
              // This allows custom policies in any namespace (e.g., platform, compliance, etc.)
              for (const [, nsObj] of Object.entries(containerization)) {
                // Only process keys where value is an object with a 'result' property
                if (
                  nsObj &&
                  typeof nsObj === 'object' &&
                  'result' in nsObj &&
                  nsObj.result &&
                  typeof nsObj.result === 'object'
                ) {
                  const nsResult = nsObj.result;

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
          violationDetails: violations.map(v => ({ rule: v.rule, message: v.message })),
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
      // Clean up temp file using tmp's cleanup mechanism
      try {
        tmpFile.removeCallback();
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
    evaluatePolicy: createEvaluatePolicyWrapper(async (input) => {
      return evaluateRegoPolicy(policyPaths, input, logger);
    }),
    queryConfig: async <T = unknown>(packageName: string, input: Record<string, unknown>): Promise<T | null> => {
      return queryRegoConfig<T>(policyPaths, packageName, input, logger);
    },
    close: () => {
      logger.debug({ policyPaths }, 'Cleaning up merged Rego policy resources');
    },
  };

  logger.info({ policyPaths }, 'Rego policies loaded and ready for merged evaluation');

  return Success(evaluator);
}

/**
 * Simplified policy loading API for tests
 * Wrapper around loadAndMergeRegoPolicies for convenience
 *
 * @param options - Loading options
 * @param options.policiesPath - Directory containing policies
 * @param options.filePattern - Glob pattern for policy files (e.g., '*.rego', 'validation.rego')
 * @param logger - Optional logger instance
 * @returns Policy evaluator or failure
 */
export async function loadPolicies(
  options: {
    policiesPath: string;
    filePattern?: string;
  },
  logger?: Logger,
): Promise<Result<RegoPolicy>> {
  const log = logger || (await import('@/lib/logger')).createLogger({ name: 'policy-loader', level: 'info' });

  // Use glob to find matching policy files
  const { glob } = await import('glob');
  const pattern = options.filePattern || '*.rego';
  const fullPattern = join(options.policiesPath, pattern);

  const policyFiles = await glob(fullPattern);

  if (policyFiles.length === 0) {
    return Failure(`No policy files found matching pattern: ${fullPattern}`, {
      message: 'Policy files not found',
      hint: `No .rego files matched pattern: ${pattern}`,
      resolution: 'Check the policiesPath and filePattern parameters',
    });
  }

  log.debug({ policyFiles, pattern: fullPattern }, 'Found policy files');

  // Load and merge all found policies
  return loadAndMergeRegoPolicies(policyFiles, log);
}

/**
 * Convenience type alias for RegoEvaluator used in tests
 */
export type RegoPolicy = RegoEvaluator;
