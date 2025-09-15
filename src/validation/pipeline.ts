/**
 * Validation Pipeline Utilities - Functional validation composition
 */

import { Result, Success, Failure } from '../types';
import { ValidationResult } from './core-types';

/**
 * A validator function that takes input and returns a Result
 */
export type Validator<T> = (input: T) => Result<T>;

/**
 * A validation function that returns ValidationResult
 */
export type ValidationFunction<T> = (input: T) => Result<ValidationResult>;

/**
 * Pipe multiple validators together - all must pass
 */
export const pipe = <T>(...validators: Validator<T>[]): Validator<T> => {
  return (input: T): Result<T> => {
    let current = input;

    for (const validator of validators) {
      const result = validator(current);
      if (!result.ok) {
        return result;
      }
      current = result.value;
    }

    return Success(current);
  };
};

/**
 * Combine multiple validation functions - collect all results
 */
export const combine = <T>(validators: ValidationFunction<T>[]): ValidationFunction<T> => {
  return (input: T): Result<ValidationResult> => {
    const results: ValidationResult[] = [];

    for (const validator of validators) {
      const result = validator(input);
      if (result.ok) {
        results.push(result.value);
      } else {
        results.push({
          isValid: false,
          errors: [result.error],
          warnings: [],
        });
      }
    }
    const allErrors = results.flatMap((r) => r.errors);
    const allWarnings = results.flatMap((r) => r.warnings || []);
    const allSuggestions = results.flatMap((r) => r.suggestions || []);

    const aggregated: ValidationResult = {
      isValid: allErrors.length === 0,
      errors: allErrors,
      warnings: allWarnings,
      suggestions: allSuggestions,
      confidence:
        results.length > 0
          ? results.reduce((sum, r) => sum + (r.confidence || 0), 0) / results.length
          : 0,
      metadata: {
        validationTime: Date.now(),
        rulesApplied: results.flatMap((r) => r.metadata?.rulesApplied || []),
      },
    };

    return Success(aggregated);
  };
};

/**
 * Create a validation function from a simple predicate
 */
export const createValidator = <T>(
  name: string,
  predicate: (input: T) => boolean,
  errorMessage: string,
  warningMessage?: string,
): ValidationFunction<T> => {
  return (input: T): Result<ValidationResult> => {
    const isValid = predicate(input);

    const result: ValidationResult = {
      isValid,
      errors: isValid ? [] : [errorMessage],
      warnings: isValid && warningMessage ? [warningMessage] : [],
      confidence: isValid ? 1.0 : 0.0,
      metadata: {
        rulesApplied: [name],
        validationTime: Date.now(),
      },
    };

    return Success(result);
  };
};

/**
 * Create a validator that checks for required fields
 */
export const validateRequired = <T extends Record<string, unknown>>(
  requiredFields: string[],
): Validator<T> => {
  return (input: T): Result<T> => {
    const missing = requiredFields.filter(
      (field) => input[field] === undefined || input[field] === null || input[field] === '',
    );

    if (missing.length > 0) {
      return Failure(`Missing required fields: ${missing.join(', ')}`);
    }

    return Success(input);
  };
};

/**
 * Create a validator that checks parameter types
 */
export const validateTypes = <T extends Record<string, unknown>>(
  typeSpec: Record<string, string>,
): Validator<T> => {
  return (input: T): Result<T> => {
    const errors: string[] = [];

    for (const [field, expectedType] of Object.entries(typeSpec)) {
      const value = input[field];
      if (value !== undefined) {
        const actualType = typeof value;
        if (actualType !== expectedType) {
          errors.push(`Field '${field}' expected ${expectedType}, got ${actualType}`);
        }
      }
    }

    if (errors.length > 0) {
      return Failure(errors.join('; '));
    }

    return Success(input);
  };
};

/**
 * Create a validator that normalizes parameter values
 */
export const normalizeParameters = <T extends Record<string, unknown>>(
  normalizers: Record<string, (value: unknown) => unknown> = {},
): Validator<T> => {
  return (input: T): Result<T> => {
    const normalized = { ...input };

    for (const [field, normalizer] of Object.entries(normalizers)) {
      if (normalized[field] !== undefined) {
        try {
          (normalized as Record<string, unknown>)[field] = normalizer(normalized[field]);
        } catch (error) {
          return Failure(`Failed to normalize field '${field}': ${error}`);
        }
      }
    }

    return Success(normalized);
  };
};

/**
 * Create a validator from a Zod schema
 */
export const validateSchema = <T>(schema: { parse: (input: unknown) => T }): Validator<T> => {
  return (input: unknown): Result<T> => {
    try {
      const validated = schema.parse(input);
      return Success(validated);
    } catch (error) {
      return Failure(`Schema validation failed: ${error}`);
    }
  };
};

/**
 * Create a conditional validator
 */
export const when = <T>(
  condition: (input: T) => boolean,
  validator: Validator<T>,
): Validator<T> => {
  return (input: T): Result<T> => {
    if (condition(input)) {
      return validator(input);
    }
    return Success(input);
  };
};

/**
 * Create a validator that applies business rules
 */
export const validateBusinessRules = <T extends Record<string, unknown>>(
  rules: Array<{
    name: string;
    check: (input: T) => boolean;
    message: string;
  }>,
): ValidationFunction<T> => {
  return (input: T): Result<ValidationResult> => {
    const errors: string[] = [];
    const rulesApplied: string[] = [];

    for (const rule of rules) {
      rulesApplied.push(rule.name);
      if (!rule.check(input)) {
        errors.push(rule.message);
      }
    }

    const result: ValidationResult = {
      isValid: errors.length === 0,
      errors,
      warnings: [],
      confidence: errors.length === 0 ? 1.0 : 0.0,
      metadata: {
        rulesApplied,
        validationTime: Date.now(),
      },
    };

    return Success(result);
  };
};
