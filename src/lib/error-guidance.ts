/**
 * Error guidance pattern matching system for consistent, reusable error handling
 */

import type { ErrorGuidance } from '@/types';
import { extractErrorMessage } from './error-utils';

/**
 * Pattern definition for matching errors and generating guidance
 */
export interface ErrorPattern {
  /** Test if this pattern matches the given error */
  match: (error: unknown) => boolean;
  /** Generate guidance for a matched error */
  guidance: (error: unknown) => ErrorGuidance;
}

/**
 * Create error guidance builder with pattern matching
 *
 * @param patterns - Array of error patterns to check in order
 * @param defaultGuidance - Optional default guidance when no pattern matches
 * @returns Function that extracts guidance from errors using pattern matching
 *
 * @example
 * ```typescript
 * const patterns = [
 *   statusCodePattern(404, {
 *     message: 'Resource not found',
 *     hint: 'The requested resource does not exist',
 *     resolution: 'Verify the resource name and try again',
 *   }),
 * ];
 * const extractGuidance = createErrorGuidanceBuilder(patterns);
 * const guidance = extractGuidance(error);
 * ```
 */
export function createErrorGuidanceBuilder(
  patterns: ErrorPattern[],
  defaultGuidance?: (error: unknown) => ErrorGuidance,
) {
  return function extractGuidance(error: unknown): ErrorGuidance {
    // Try each pattern in order
    for (const pattern of patterns) {
      if (pattern.match(error)) {
        return pattern.guidance(error);
      }
    }

    // Use default guidance or generic fallback
    if (defaultGuidance) {
      return defaultGuidance(error);
    }

    return {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred',
      resolution: 'Check the error message and logs for more details',
    };
  };
}

/**
 * Create pattern that matches error message substrings (case-insensitive)
 *
 * @param substring - Substring to search for in error message
 * @param guidance - Guidance to return when matched
 * @returns ErrorPattern that checks for the substring
 *
 * @example
 * ```typescript
 * messagePattern('ECONNREFUSED', {
 *   message: 'Connection refused',
 *   hint: 'Cannot connect to service',
 *   resolution: 'Ensure the service is running',
 * })
 * ```
 */
export function messagePattern(substring: string, guidance: ErrorGuidance): ErrorPattern {
  return {
    match: (error: unknown) => {
      const message = extractErrorMessage(error).toLowerCase();
      return message.includes(substring.toLowerCase());
    },
    guidance: () => guidance,
  };
}

/**
 * Create pattern with custom match function
 *
 * @param matchFn - Custom function to test if pattern matches
 * @param guidance - Guidance to return when matched
 * @returns ErrorPattern with custom matching logic
 *
 * @example
 * ```typescript
 * customPattern(
 *   (error) => error instanceof TypeError && error.message.includes('null'),
 *   {
 *     message: 'Null reference error',
 *     hint: 'A required value was null or undefined',
 *     resolution: 'Check for missing required parameters',
 *   }
 * )
 * ```
 */
export function customPattern(
  matchFn: (error: unknown) => boolean,
  guidance: ErrorGuidance | ((error: unknown) => ErrorGuidance),
): ErrorPattern {
  return {
    match: matchFn,
    guidance: typeof guidance === 'function' ? guidance : () => guidance,
  };
}
