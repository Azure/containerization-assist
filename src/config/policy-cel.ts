/**
 * CEL Policy Evaluator
 *
 * Evaluates CEL-based policies against input content.
 * Implements RegoEvaluator interface for seamless integration with existing policy infrastructure.
 *
 * CEL (Common Expression Language) provides a simpler alternative to Rego for basic validation rules.
 * See: https://github.com/google/cel-spec
 *
 * @module config/policy-cel
 */

import { readFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import yaml from 'js-yaml';
import type { Logger } from 'pino';
import { evaluate, parse, type ParsedExpression } from '@marcbachmann/cel-js';
import { type Result, Success, Failure } from '@types';
import { ERROR_MESSAGES, extractErrorMessage } from '@/lib/errors';
import type {
  RegoEvaluator,
  RegoPolicyResult,
  RegoPolicyViolation,
  CategorizedViolations,
} from './policy-rego';
import {
  celPolicySetSchema,
  formatValidationErrors,
  type CelPolicySet,
  type CelPolicyRule,
} from './policy-cel-schema';

/**
 * Compiled CEL program with metadata
 *
 * Stores the compiled CEL expression program alongside the original rule
 * for efficient evaluation without re-parsing.
 */
interface CompiledCelRule {
  /** Original rule definition */
  rule: CelPolicyRule;
  /** Compiled CEL expression function */
  program: ParsedExpression;
}

/**
 * CEL Policy Evaluator
 *
 * Implements RegoEvaluator interface to integrate seamlessly
 * with existing policy infrastructure. Evaluates CEL expressions
 * defined in YAML policy files.
 *
 * **Capabilities:**
 * - Expression-based validation rules
 * - Fast evaluation (no external process overhead)
 * - Priority-based ordering
 * - Severity-based categorization (block, warn, suggest)
 *
 * **Limitations:**
 * - Cannot generate configuration objects (queryConfig always returns null)
 * - Less expressive than Rego for complex logic
 * - No support for functions beyond CEL built-ins
 *
 * @example
 * ```typescript
 * const evaluator = new CelPolicyEvaluator(policySet, logger, ['policy.yaml']);
 * await evaluator.initialize();
 *
 * const result = await evaluator.evaluate('FROM alpine\nUSER appuser');
 * console.log('Allow:', result.allow);
 * console.log('Violations:', result.violations);
 * ```
 */
export class CelPolicyEvaluator implements RegoEvaluator {
  private compiledRules: CompiledCelRule[] = [];
  private initialized = false;

  constructor(
    private policySet: CelPolicySet,
    private logger: Logger,
    public policyPaths: string[]
  ) {
    this.logger.debug(
      {
        policyName: policySet.metadata.name,
        ruleCount: policySet.spec.rules.length,
        policyPaths,
      },
      'CEL policy evaluator created'
    );
  }

  /**
   * Initialize by compiling all CEL expressions
   *
   * This must be called before evaluation. Compilation happens once at
   * initialization for performance.
   *
   * @returns Result indicating success or compilation errors
   */
  async initialize(): Promise<Result<void>> {
    if (this.initialized) {
      this.logger.debug('CEL policy already initialized');
      return Success(undefined);
    }

    try {
      this.logger.debug(
        { ruleCount: this.policySet.spec.rules.length },
        'Compiling CEL expressions'
      );

      for (const rule of this.policySet.spec.rules) {
        try {
          // Compile CEL expression into reusable program
          const program = parse(rule.condition);

          this.compiledRules.push({
            rule,
            program,
          });

          this.logger.debug(
            {
              ruleName: rule.name,
              severity: rule.severity,
              condition: rule.condition.substring(0, 50),
            },
            'CEL rule compiled successfully'
          );
        } catch (error) {
          const message = extractErrorMessage(error);
          this.logger.error(
            {
              ruleName: rule.name,
              condition: rule.condition,
              error: message,
            },
            'Failed to compile CEL rule'
          );

          return Failure(`Failed to compile CEL rule '${rule.name}': ${message}`, {
            message: 'CEL expression compilation failed',
            hint: `Check syntax in rule: ${rule.name}`,
            resolution: 'Verify CEL expression syntax and try again',
          });
        }
      }

      this.initialized = true;

      this.logger.info(
        {
          policyName: this.policySet.metadata.name,
          ruleCount: this.compiledRules.length,
        },
        'CEL policy initialized successfully'
      );

      return Success(undefined);
    } catch (error) {
      const message = extractErrorMessage(error);
      this.logger.error({ error: message }, 'Failed to initialize CEL policy');
      return Failure(`Failed to initialize CEL policy: ${message}`);
    }
  }

  /**
   * Evaluate policy against input
   *
   * Runs all compiled CEL expressions against the input and categorizes
   * matching rules by severity.
   *
   * @param input - String content (Dockerfile, manifest, etc.) or object with content field
   * @returns Promise resolving to policy evaluation result
   */
  async evaluate(input: string | Record<string, unknown>): Promise<RegoPolicyResult> {
    if (!this.initialized) {
      this.logger.warn('CEL policy not initialized, initializing now');
      const initResult = await this.initialize();
      if (!initResult.ok) {
        // Return policy evaluation failure
        return {
          allow: false,
          violations: [
            {
              rule: 'cel-initialization-error',
              category: 'system',
              severity: 'block',
              message: `Failed to initialize CEL policy: ${initResult.error}`,
            },
          ],
          warnings: [],
          suggestions: [],
          summary: {
            total_violations: 1,
            total_warnings: 0,
            total_suggestions: 0,
          },
        };
      }
    }

    // Normalize input to object with 'input' key for CEL evaluation
    const inputData = typeof input === 'string' ? { content: input } : input;

    const violations: RegoPolicyViolation[] = [];
    const warnings: RegoPolicyViolation[] = [];
    const suggestions: RegoPolicyViolation[] = [];

    this.logger.debug(
      { ruleCount: this.compiledRules.length },
      'Evaluating CEL policy rules'
    );

    for (const { rule, program } of this.compiledRules) {
      try {
        // Evaluate CEL expression with input context
        const result = program({ input: inputData });

        // CEL expressions should return boolean
        const matched = Boolean(result);

        if (matched) {
          const violation: RegoPolicyViolation = {
            rule: rule.name,
            category: rule.category,
            severity: rule.severity,
            message: rule.message,
            description: rule.description,
            priority: rule.priority,
          };

          // Categorize by severity
          if (rule.severity === 'block') {
            violations.push(violation);
          } else if (rule.severity === 'warn') {
            warnings.push(violation);
          } else {
            suggestions.push(violation);
          }

          this.logger.debug(
            {
              ruleName: rule.name,
              severity: rule.severity,
              category: rule.category,
            },
            'CEL rule matched'
          );
        }
      } catch (error) {
        const message = extractErrorMessage(error);
        this.logger.error(
          {
            ruleName: rule.name,
            condition: rule.condition,
            error: message,
          },
          'CEL rule evaluation failed'
        );

        // Add evaluation error as blocking violation
        violations.push({
          rule: `${rule.name}-evaluation-error`,
          category: 'system',
          severity: 'block',
          message: `Policy evaluation failed: ${message}`,
          description: `Rule '${rule.name}' encountered runtime error`,
        });
      }
    }

    // Sort violations by priority (higher priority first)
    const sortByPriority = (a: RegoPolicyViolation, b: RegoPolicyViolation) =>
      (b.priority ?? 50) - (a.priority ?? 50);

    violations.sort(sortByPriority);
    warnings.sort(sortByPriority);
    suggestions.sort(sortByPriority);

    const allow = violations.length === 0;

    this.logger.info(
      {
        allow,
        violations: violations.length,
        warnings: warnings.length,
        suggestions: suggestions.length,
      },
      'CEL policy evaluation completed'
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
  }

  /**
   * Evaluate with Result wrapper (compatibility method)
   *
   * Wraps evaluate() to work with Result pattern used throughout the codebase.
   *
   * @param result - Result containing content to evaluate
   * @param _packageName - Package name (unused, for interface compatibility)
   * @returns Result containing categorized violations
   */
  async evaluatePolicy<T>(
    result: Result<T>,
    _packageName?: string
  ): Promise<Result<CategorizedViolations>> {
    if (!result.ok) {
      return result as Result<never>;
    }

    try {
      const policyResult = await this.evaluate(
        result.value as string | Record<string, unknown>
      );

      return Success({
        blocking: policyResult.violations,
        warnings: policyResult.warnings,
        suggestions: policyResult.suggestions,
      });
    } catch (error) {
      const message = extractErrorMessage(error);
      this.logger.error({ error: message }, 'CEL policy evaluation failed');
      return Failure(`CEL policy evaluation failed: ${message}`);
    }
  }

  /**
   * Query configuration (not supported in CEL)
   *
   * CEL is expression-based and cannot generate configuration objects like Rego.
   * This method always returns null with a warning log.
   *
   * **Rationale:** CEL is designed for validation, not configuration generation.
   * For configuration queries, use Rego policies instead.
   *
   * @param packageName - Package name to query
   * @param _input - Input data (unused)
   * @returns Always returns null
   */
  async queryConfig<T = unknown>(
    packageName: string,
    _input: Record<string, unknown>
  ): Promise<T | null> {
    this.logger.warn(
      { packageName, policyName: this.policySet.metadata.name },
      'CEL policies do not support queryConfig - returning null'
    );
    return null;
  }

  /**
   * Clean up resources
   *
   * CEL evaluator has no external resources to clean up,
   * but implements this method for interface compatibility.
   */
  close(): void {
    this.logger.debug('CEL policy evaluator closed');
    this.compiledRules = [];
    this.initialized = false;
  }
}

/**
 * Load CEL policy from YAML file
 *
 * Reads, parses, and validates a CEL policy file, then creates an initialized evaluator.
 *
 * @param policyPath - Path to .yaml or .yml policy file
 * @param logger - Logger instance
 * @returns Promise resolving to Result containing RegoEvaluator or error
 *
 * @example
 * ```typescript
 * const result = await loadCelPolicy('/path/to/policy.yaml', logger);
 * if (result.ok) {
 *   const evaluation = await result.value.evaluate('FROM alpine');
 *   console.log('Violations:', evaluation.violations);
 * }
 * ```
 */
export async function loadCelPolicy(
  policyPath: string,
  logger: Logger
): Promise<Result<RegoEvaluator>> {
  try {
    logger.debug({ policyPath }, 'Loading CEL policy');

    // Validate file exists
    if (!existsSync(policyPath)) {
      return Failure(`Policy file not found: ${policyPath}`, {
        message: 'CEL policy file does not exist',
        hint: `Attempted to load: ${policyPath}`,
        resolution: 'Ensure the policy file path is correct',
      });
    }

    // Validate file extension
    if (!policyPath.endsWith('.yaml') && !policyPath.endsWith('.yml')) {
      return Failure('Only .yaml or .yml files supported for CEL policies', {
        message: 'Invalid CEL policy file format',
        hint: `File: ${policyPath}`,
        resolution: 'CEL policies must use .yaml or .yml extension',
      });
    }

    // Read and parse YAML
    const content = await readFile(policyPath, 'utf-8');
    const parsed = yaml.load(content);

    if (!parsed || typeof parsed !== 'object') {
      return Failure('Invalid YAML format', {
        message: 'Failed to parse YAML content',
        hint: `File: ${policyPath}`,
        resolution: 'Ensure file contains valid YAML',
      });
    }

    // Validate schema
    const validation = celPolicySetSchema.safeParse(parsed);
    if (!validation.success) {
      const errorMessage = formatValidationErrors(validation.error);

      logger.error(
        { policyPath, errors: errorMessage },
        'CEL policy validation failed'
      );

      return Failure('Invalid CEL policy format', {
        message: 'CEL policy validation failed',
        hint: errorMessage,
        resolution: 'Check policy structure against schema (see docs)',
      });
    }

    const policySet = validation.data;

    logger.info(
      {
        policyPath,
        name: policySet.metadata.name,
        version: policySet.metadata.version,
        ruleCount: policySet.spec.rules.length,
      },
      'CEL policy loaded, initializing evaluator'
    );

    // Create evaluator
    const evaluator = new CelPolicyEvaluator(policySet, logger, [policyPath]);

    // Initialize (compile all rules)
    const initResult = await evaluator.initialize();
    if (!initResult.ok) {
      return initResult as Result<never>;
    }

    logger.info({ policyPath }, 'CEL policy loaded and initialized successfully');

    return Success(evaluator);
  } catch (error) {
    const message = extractErrorMessage(error);
    logger.error({ policyPath, error: message }, 'Failed to load CEL policy');

    return Failure(ERROR_MESSAGES.POLICY_LOAD_FAILED(message), {
      message: 'Failed to load CEL policy',
      hint: `Error: ${message}`,
      resolution: 'Check policy file syntax and structure',
    });
  }
}
