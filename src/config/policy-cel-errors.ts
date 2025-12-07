/**
 * CEL Policy Error Formatting Utilities
 *
 * Provides detailed, actionable error messages for CEL policy issues.
 * Enhances developer experience when authoring and debugging policies.
 *
 * @module config/policy-cel-errors
 */

import type { CelPolicyRule } from './policy-cel-schema';

/**
 * Common CEL syntax mistakes and their fixes
 *
 * Maps error message patterns to user-friendly suggestions.
 */
const COMMON_MISTAKES: Record<string, string> = {
  'unclosed string': 'Check for unmatched quotes in your condition string',
  'unexpected token': 'Verify CEL expression syntax - check for typos or invalid operators',
  undefined: 'Ensure you are accessing valid fields (input.content, not input.nonexistent)',
  'is not a function':
    'Method call syntax: use input.content.contains("text"), not contains(input.content, "text")',
  'is not defined': 'Check for typos in variable or function names',
  'cannot access':
    'The field you are trying to access does not exist on the object. Available: input.content',
  'type mismatch': 'Ensure you are comparing compatible types (e.g., string to string)',
  'division by zero': 'Check for potential division by zero in arithmetic expressions',
  'no such key': 'The map/object does not contain the key you are trying to access',
};

/**
 * Error details structure for consistent error handling
 */
export interface CelErrorDetails {
  /** Short error message for Result.error */
  message: string;
  /** Detailed context with rule information */
  hint: string;
  /** Actionable guidance for fixing the issue */
  resolution: string;
}

/**
 * Format compilation error with context and suggestions
 *
 * Provides detailed error information when a CEL expression fails to compile.
 * Includes rule context, the failing condition, and suggestions for common mistakes.
 *
 * @param rule - The rule that failed to compile
 * @param error - The error thrown during compilation
 * @returns Formatted error details with hint and resolution
 *
 * @example
 * ```typescript
 * try {
 *   parse(rule.condition);
 * } catch (error) {
 *   const details = formatCompilationError(rule, error as Error);
 *   return Failure(`Failed to compile CEL rule '${rule.name}'`, details);
 * }
 * ```
 */
export function formatCompilationError(rule: CelPolicyRule, error: Error): CelErrorDetails {
  const errorMessage = error.message.toLowerCase();

  // Find matching common mistake
  let suggestion = 'Verify CEL expression syntax and try again';
  for (const [pattern, fix] of Object.entries(COMMON_MISTAKES)) {
    if (errorMessage.includes(pattern.toLowerCase())) {
      suggestion = fix;
      break;
    }
  }

  return {
    message: 'CEL expression compilation failed',
    hint: formatErrorHint(rule, error),
    resolution: `${suggestion}\n\nCEL language reference: https://github.com/google/cel-spec/blob/master/doc/langdef.md`,
  };
}

/**
 * Format evaluation error with context
 *
 * Provides detailed error information when a CEL expression fails at runtime.
 * Includes available input fields and links to examples.
 *
 * @param rule - The rule that failed during evaluation
 * @param error - The error thrown during evaluation
 * @returns Formatted error details with hint and resolution
 *
 * @example
 * ```typescript
 * try {
 *   program({ input: inputData });
 * } catch (error) {
 *   const details = formatEvaluationError(rule, error as Error);
 *   // Log or include in violation
 * }
 * ```
 */
export function formatEvaluationError(rule: CelPolicyRule, error: Error): CelErrorDetails {
  return {
    message: 'CEL expression evaluation failed',
    hint: formatErrorHint(rule, error),
    resolution:
      'Check that your condition works with the provided input structure.\n' +
      'Available fields: input.content (string)\n' +
      'See policy examples: policies.user.examples/cel/',
  };
}

/**
 * Format detailed error hint with rule context
 *
 * Creates a multi-line error hint that includes all relevant rule information
 * and the error message.
 *
 * @param rule - The rule associated with the error
 * @param error - The error that occurred
 * @returns Formatted hint string with rule context
 */
function formatErrorHint(rule: CelPolicyRule, error: Error): string {
  const lines = [
    `Rule: ${rule.name}`,
    `Category: ${rule.category}`,
    `Severity: ${rule.severity}`,
    '',
    'Condition:',
    formatConditionWithContext(rule.condition),
    '',
    `Error: ${error.message}`,
  ];

  return lines.join('\n');
}

/**
 * Format condition with line numbers for multi-line expressions
 *
 * For single-line conditions, returns the condition with indentation.
 * For multi-line conditions, adds line numbers for easier debugging.
 *
 * @param condition - The CEL condition expression
 * @returns Formatted condition string
 *
 * @example
 * Single-line:
 * ```
 *   input.content.contains("test")
 * ```
 *
 * Multi-line:
 * ```
 *    1| input.content.contains("FROM") &&
 *    2| !input.content.contains("USER")
 * ```
 */
function formatConditionWithContext(condition: string): string {
  const lines = condition.split('\n');

  if (lines.length === 1) {
    return `  ${condition}`;
  }

  // Multi-line: add line numbers
  return lines
    .map((line, idx) => `  ${(idx + 1).toString().padStart(2, ' ')}| ${line}`)
    .join('\n');
}

/**
 * Phase of policy file processing
 */
export type PolicyFilePhase = 'read' | 'parse' | 'validate' | 'compile';

/**
 * Create user-friendly error message for policy file loading
 *
 * Provides phase-specific error messages with appropriate guidance for
 * each stage of policy loading: reading, parsing YAML, validating schema,
 * or compiling CEL expressions.
 *
 * @param policyPath - Path to the policy file
 * @param phase - The phase where the error occurred
 * @param error - The error that occurred
 * @returns Formatted error details with phase-specific guidance
 *
 * @example
 * ```typescript
 * if (!existsSync(policyPath)) {
 *   const details = formatPolicyFileError(policyPath, 'read', new Error('File not found'));
 *   return Failure(`Policy file not found: ${policyPath}`, details);
 * }
 * ```
 */
export function formatPolicyFileError(
  policyPath: string,
  phase: PolicyFilePhase,
  error: Error
): CelErrorDetails {
  const phaseMessages: Record<PolicyFilePhase, CelErrorDetails> = {
    read: {
      message: 'Failed to read policy file',
      hint: `Could not read file: ${policyPath}\nError: ${error.message}`,
      resolution: 'Ensure the file exists and is readable',
    },
    parse: {
      message: 'Failed to parse policy YAML',
      hint: `Invalid YAML syntax in: ${policyPath}\nError: ${error.message}`,
      resolution:
        'Check YAML syntax:\n' +
        '  - Use proper indentation (spaces, not tabs)\n' +
        '  - Quote strings with colons or special characters\n' +
        '  - Ensure valid YAML structure',
    },
    validate: {
      message: 'Policy file structure is invalid',
      hint: `Schema validation failed: ${policyPath}\nError: ${error.message}`,
      resolution:
        'Required structure:\n' +
        '  apiVersion: policy.containerization-assist.dev/v1\n' +
        '  kind: PolicySet\n' +
        '  metadata: { name: "..." }\n' +
        '  spec: { rules: [...] }\n\n' +
        'See examples: policies.user.examples/cel/',
    },
    compile: {
      message: 'Failed to compile CEL expressions',
      hint: `Compilation error in: ${policyPath}\nError: ${error.message}`,
      resolution:
        'Review CEL expression syntax in rule conditions.\n' +
        'CEL reference: https://github.com/google/cel-spec/blob/master/doc/langdef.md',
    },
  };

  return phaseMessages[phase];
}

/**
 * Format schema validation error with field context
 *
 * Creates a user-friendly error message that shows the field path
 * and the actual value that failed validation.
 *
 * @param fieldPath - Path to the field that failed validation (e.g., ['spec', 'rules', '0', 'name'])
 * @param errorMessage - The validation error message
 * @param value - Optional value that failed validation
 * @returns Formatted error string
 *
 * @example
 * ```typescript
 * const message = formatSchemaError(['spec', 'rules', '0', 'severity'], 'Invalid enum value', 'error');
 * // Returns: "spec.rules.0.severity: Invalid enum value (got: \"error\")"
 * ```
 */
export function formatSchemaError(
  fieldPath: string[],
  errorMessage: string,
  value?: unknown
): string {
  const field = fieldPath.length > 0 ? fieldPath.join('.') : 'root';
  const valueStr = value !== undefined ? ` (got: ${JSON.stringify(value)})` : '';

  return `${field}: ${errorMessage}${valueStr}`;
}

/**
 * Detect if error is a common CEL mistake and return suggestion
 *
 * Analyzes the error message to identify common mistakes and provides
 * targeted suggestions for fixing them.
 *
 * @param errorMessage - The error message to analyze
 * @returns Suggestion string if a common mistake is detected, undefined otherwise
 *
 * @example
 * ```typescript
 * const suggestion = detectCommonMistake('contains is not a function');
 * // Returns: "Method call syntax: use input.content.contains(\"text\"), not contains(input.content, \"text\")"
 * ```
 */
export function detectCommonMistake(errorMessage: string): string | undefined {
  const lowerMessage = errorMessage.toLowerCase();

  for (const [pattern, suggestion] of Object.entries(COMMON_MISTAKES)) {
    if (lowerMessage.includes(pattern.toLowerCase())) {
      return suggestion;
    }
  }

  return undefined;
}
