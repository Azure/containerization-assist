/**
 * Policy Constraints Adapter - Clean interface between policy system and prompt engine
 *
 * This module consolidates all policy-to-prompt formatting logic in one place,
 * providing a clean separation of concerns between policy evaluation and prompt building.
 */

import { buildPolicyConstraints } from '@/config/policy-prompt';
import { createLogger } from '@/lib/logger';

const logger = createLogger().child({ module: 'policy-constraints-adapter' });

/**
 * Options for formatting policy constraints
 */
export interface ConstraintFormattingOptions {
  /** Maximum character budget for constraints */
  maxChars?: number;
  /** Include policy metadata in output */
  includeMetadata?: boolean;
  /** Format as bullet points vs. narrative */
  bulletFormat?: boolean;
  /** Include priority/weight indicators */
  includePriority?: boolean;
}

/**
 * Result of constraint formatting with metadata
 */
export interface FormattedConstraints {
  /** Ready-to-inject text for the prompt */
  text: string;
  /** Number of constraints included */
  count: number;
  /** Character count of formatted text */
  charCount: number;
  /** Constraints that were truncated due to budget */
  truncated: boolean;
}

/**
 * Default formatting options
 */
const DEFAULT_OPTIONS: ConstraintFormattingOptions = {
  maxChars: 2000,
  bulletFormat: true,
  includeMetadata: false,
  includePriority: false,
};

/**
 * Format policy constraints for prompt injection with budget control.
 * This is the main adapter function that bridges policy system to prompt engine.
 *
 * @param tool - Tool name for context
 * @param environment - Environment (e.g., 'production', 'development')
 * @param options - Formatting options
 * @returns Formatted constraints ready for prompt injection
 */
export function formatPolicyConstraints(
  tool: string,
  environment: string,
  options: ConstraintFormattingOptions = {},
): FormattedConstraints {
  const opts = { ...DEFAULT_OPTIONS, ...options };

  try {
    // Get raw constraints from policy system
    const constraints = buildPolicyConstraints({ tool, environment });

    if (!constraints || constraints.length === 0) {
      return {
        text: '',
        count: 0,
        charCount: 0,
        truncated: false,
      };
    }

    // Format based on options
    const formatted = opts.bulletFormat
      ? formatAsBullets(constraints, opts)
      : formatAsNarrative(constraints, opts);

    // Apply budget control
    const result = applyBudget(formatted, opts.maxChars || 2000);

    logger.debug(
      {
        tool,
        environment,
        originalCount: constraints.length,
        finalCount: result.count,
        truncated: result.truncated,
      },
      'Formatted policy constraints',
    );

    return result;
  } catch (error) {
    logger.warn({ error, tool, environment }, 'Failed to format policy constraints');
    return {
      text: '',
      count: 0,
      charCount: 0,
      truncated: false,
    };
  }
}

/**
 * Format constraints as bullet points
 */
function formatAsBullets(constraints: string[], options: ConstraintFormattingOptions): string {
  const header = 'You must follow these organizational policies:';
  const bullets = constraints.map((c, i) => {
    if (options.includePriority) {
      // Add priority indicator for first few items
      const priority = i < 3 ? '[HIGH] ' : '';
      return `- ${priority}${c}`;
    }
    return `- ${c}`;
  });

  return [header, ...bullets].join('\n');
}

/**
 * Format constraints as narrative text
 */
function formatAsNarrative(constraints: string[], _options: ConstraintFormattingOptions): string {
  const intro = 'Follow these organizational policies: ';
  const joined = constraints.join('. ');

  // Ensure proper sentence ending
  const narrative = joined.endsWith('.') ? joined : `${joined}.`;

  return intro + narrative;
}

/**
 * Apply character budget to formatted text
 */
function applyBudget(text: string, maxChars: number): FormattedConstraints {
  const lines = text.split('\n');
  const originalCount = lines.filter((l) => l.startsWith('-')).length;

  if (text.length <= maxChars) {
    return {
      text,
      count: originalCount,
      charCount: text.length,
      truncated: false,
    };
  }

  // Truncate intelligently at line boundaries
  let accumulated = '';
  let count = 0;
  let truncated = false;

  for (const line of lines) {
    const potential = accumulated ? `${accumulated}\n${line}` : line;

    if (potential.length > maxChars - 20) {
      // Leave room for truncation indicator
      truncated = true;
      if (accumulated) {
        accumulated += '\n[Additional constraints truncated]';
      }
      break;
    }

    accumulated = potential;
    if (line.startsWith('-')) {
      count++;
    }
  }

  return {
    text: accumulated,
    count,
    charCount: accumulated.length,
    truncated,
  };
}

/**
 * Get a single formatted string for system message.
 * Convenience function for simple use cases.
 *
 * @param tool - Tool name
 * @param environment - Environment
 * @param maxChars - Maximum character budget
 * @returns Formatted constraint text or undefined if no constraints
 */
export function getSystemConstraintText(
  tool: string,
  environment: string,
  maxChars = 2000,
): string | undefined {
  const result = formatPolicyConstraints(tool, environment, { maxChars });
  return result.text || undefined;
}
