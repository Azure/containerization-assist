/**
 * CEL Policy Schema Definitions
 *
 * Defines the YAML structure for user-provided CEL (Common Expression Language) policies.
 * CEL policies provide a simpler alternative to Rego for basic validation rules.
 *
 * @module config/policy-cel-schema
 */

import { z } from 'zod';

/**
 * CEL policy rule definition
 *
 * Each rule defines a validation condition using CEL expression syntax.
 * When the condition evaluates to true, the rule is considered matched
 * and a violation is reported.
 *
 * @example
 * ```yaml
 * - name: require-user
 *   category: security
 *   severity: block
 *   priority: 100
 *   condition: '!input.content.contains("USER")'
 *   message: "Dockerfile must specify USER directive"
 *   description: "Running as root is a security risk"
 * ```
 */
export const celPolicyRuleSchema = z.object({
  /**
   * Unique rule identifier
   *
   * Used for tracking violations and deduplication.
   * Should be kebab-case and descriptive.
   */
  name: z.string().min(1, 'Rule name required'),

  /**
   * Category classification
   *
   * Common categories: security, optimization, best-practices, health, resources
   */
  category: z.string().min(1, 'Category required'),

  /**
   * Severity level determines enforcement behavior
   *
   * - `block`: Prevents operation, returns error
   * - `warn`: Logs warning, allows operation
   * - `suggest`: Informational suggestion only
   */
  severity: z.enum(['block', 'warn', 'suggest'], {
    errorMap: () => ({ message: 'Severity must be block, warn, or suggest' }),
  }),

  /**
   * Priority for sorting (optional, default: 50)
   *
   * Higher priority violations appear first in results.
   * Range: 0-100 (100 = highest priority)
   */
  priority: z.number().int().min(0).max(100).optional().default(50),

  /**
   * CEL expression that returns boolean
   *
   * Expression is evaluated against input data.
   * When expression returns true, the rule is matched and a violation is reported.
   *
   * Available context:
   * - `input.content` - String content (Dockerfile, manifest, etc.)
   * - `input.*` - Additional fields depending on tool context
   *
   * @example
   * ```yaml
   * condition: '!input.content.contains("USER")'
   * condition: 'input.content.matches("FROM\\s+latest")'
   * condition: |
   *   input.content.contains("FROM") &&
   *   !input.content.contains("HEALTHCHECK")
   * ```
   */
  condition: z.string().min(1, 'Condition expression required'),

  /**
   * Human-readable message shown when rule matches
   *
   * Should be actionable and explain what's wrong.
   */
  message: z.string().min(1, 'Message required'),

  /**
   * Optional detailed description
   *
   * Provides additional context about why this rule exists
   * and how to fix violations.
   */
  description: z.string().optional(),
});

/**
 * Inferred TypeScript type for CEL policy rule
 */
export type CelPolicyRule = z.infer<typeof celPolicyRuleSchema>;

/**
 * CEL policy set metadata
 *
 * Provides identifying information about the policy set.
 */
export const celPolicyMetadataSchema = z.object({
  /**
   * Policy set name (required)
   *
   * Should be descriptive and unique.
   */
  name: z.string().min(1, 'Policy name required'),

  /**
   * Policy version (optional)
   *
   * Recommended format: semver (e.g., "1.0.0")
   */
  version: z.string().optional(),

  /**
   * Policy description (optional)
   *
   * Brief explanation of what this policy set enforces.
   */
  description: z.string().optional(),
});

/**
 * Inferred TypeScript type for CEL policy metadata
 */
export type CelPolicyMetadata = z.infer<typeof celPolicyMetadataSchema>;

/**
 * CEL policy set specification
 *
 * Contains the list of validation rules.
 */
export const celPolicySpecSchema = z.object({
  /**
   * Array of policy rules (at least one required)
   *
   * Rules are evaluated in order and all matching rules
   * will be reported as violations.
   */
  rules: z.array(celPolicyRuleSchema).min(1, 'At least one rule required'),
});

/**
 * Inferred TypeScript type for CEL policy spec
 */
export type CelPolicySpec = z.infer<typeof celPolicySpecSchema>;

/**
 * Complete CEL policy set document
 *
 * Follows Kubernetes-style resource format with apiVersion, kind, metadata, and spec.
 *
 * @example
 * ```yaml
 * apiVersion: policy.containerization-assist.dev/v1
 * kind: PolicySet
 * metadata:
 *   name: custom-security-rules
 *   version: "1.0.0"
 *   description: "Custom security validation rules"
 * spec:
 *   rules:
 *     - name: require-user
 *       category: security
 *       severity: block
 *       priority: 100
 *       condition: '!input.content.contains("USER")'
 *       message: "Dockerfile must specify USER directive"
 * ```
 */
export const celPolicySetSchema = z.object({
  /**
   * API version (must be exactly this value)
   *
   * Future versions may use v2, v3, etc.
   */
  apiVersion: z.literal('policy.containerization-assist.dev/v1'),

  /**
   * Resource kind (must be exactly this value)
   */
  kind: z.literal('PolicySet'),

  /**
   * Policy metadata
   */
  metadata: celPolicyMetadataSchema,

  /**
   * Policy specification
   */
  spec: celPolicySpecSchema,
});

/**
 * Inferred TypeScript type for complete CEL policy set
 */
export type CelPolicySet = z.infer<typeof celPolicySetSchema>;

/**
 * Validation result for CEL policy sets
 */
export interface CelPolicyValidationResult {
  success: boolean;
  data?: CelPolicySet;
  errors?: z.ZodError;
}

/**
 * Validate CEL policy set structure
 *
 * Parses and validates a CEL policy set against the schema.
 * Returns success/failure with parsed data or detailed error information.
 *
 * @param data - Unknown data to validate (typically from YAML.parse)
 * @returns Validation result with parsed data or errors
 *
 * @example
 * ```typescript
 * const parsed = yaml.load(yamlContent);
 * const result = validateCelPolicySet(parsed);
 *
 * if (result.success) {
 *   console.log('Valid policy:', result.data.metadata.name);
 * } else {
 *   console.error('Validation errors:', result.errors.errors);
 * }
 * ```
 */
export function validateCelPolicySet(data: unknown): CelPolicyValidationResult {
  const result = celPolicySetSchema.safeParse(data);

  if (result.success) {
    return { success: true, data: result.data };
  } else {
    return { success: false, errors: result.error };
  }
}

/**
 * Format Zod validation errors as human-readable string
 *
 * @param errors - Zod error object from validation
 * @returns Formatted error message
 *
 * @example
 * ```typescript
 * const result = validateCelPolicySet(data);
 * if (!result.success) {
 *   const message = formatValidationErrors(result.errors);
 *   console.error('Policy validation failed:', message);
 * }
 * ```
 */
export function formatValidationErrors(errors: z.ZodError): string {
  return errors.errors
    .map((e) => {
      const path = e.path.length > 0 ? `${e.path.join('.')}: ` : '';
      return `${path}${e.message}`;
    })
    .join(', ');
}
