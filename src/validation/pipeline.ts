/**
 * Validation Pipeline Utilities - Functional validation composition
 */

import { Result } from '@/types';
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

/**
 * Combine multiple validation functions - collect all results
 */

/**
 * Create a validation function from a simple predicate
 */

/**
 * Create a validator that checks for required fields
 */

/**
 * Create a validator that checks parameter types
 */

/**
 * Create a validator that normalizes parameter values
 */

/**
 * Create a validator from a Zod schema
 */

/**
 * Create a conditional validator
 */

/**
 * Create a validator that applies business rules
 */
