/**
 * Utilities for working with Result<T> types to reduce boilerplate
 */

import { type Result, Failure, Success } from '@types';

/**
 * Propagates a failure result or continues with success.
 * Reduces boilerplate in result chain handling.
 * Consolidates the pattern: if (!result.ok) return Failure(result.error)
 *
 * @param result - The result to check
 * @param onSuccess - Function to call with the success value
 * @returns The result of onSuccess or the propagated failure
 */
export function propagateFailure<T, U>(
  result: Result<T>,
  onSuccess: (value: T) => Result<U>,
): Result<U> {
  if (!result.ok) {
    return Failure(result.error);
  }
  return onSuccess(result.value);
}

/**
 * Maps over a successful result, propagating failures.
 * Similar to Array.map but for Result types.
 *
 * @param result - The result to map over
 * @param mapper - Function to transform the success value
 * @returns Mapped result or propagated failure
 */
export function mapResult<T, U>(result: Result<T>, mapper: (value: T) => U): Result<U> {
  if (!result.ok) {
    return Failure(result.error);
  }
  return Success(mapper(result.value));
}

/**
 * Chains multiple Result-returning operations.
 * Similar to Promise.then but for Result types.
 *
 * @param result - The initial result
 * @param operations - Chain of operations to apply
 * @returns Final result or first failure encountered
 */
export function chainResults<T>(
  result: Result<T>,
  ...operations: Array<(value: any) => Result<any>>
): Result<any> {
  let current: Result<any> = result;

  for (const operation of operations) {
    if (!current.ok) {
      return current;
    }
    current = operation(current.value);
  }

  return current;
}

/**
 * Combines multiple results into a single result.
 * All must succeed for the combined result to succeed.
 *
 * @param results - Array of results to combine
 * @returns Combined result with array of values or first error
 */
export function combineResults<T>(results: Result<T>[]): Result<T[]> {
  const values: T[] = [];

  for (const result of results) {
    if (!result.ok) {
      return Failure(result.error);
    }
    values.push(result.value);
  }

  return Success(values);
}

/**
 * Try to execute a function and wrap the result in Result<T>.
 * Catches exceptions and converts them to Failure.
 *
 * @param fn - Function to execute
 * @returns Success with return value or Failure with error message
 */
export function tryExecute<T>(fn: () => T): Result<T> {
  try {
    return Success(fn());
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(message);
  }
}

/**
 * Try to execute an async function and wrap the result in Result<T>.
 * Catches exceptions and promise rejections.
 *
 * @param fn - Async function to execute
 * @returns Success with return value or Failure with error message
 */
export async function tryExecuteAsync<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    const value = await fn();
    return Success(value);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(message);
  }
}

/**
 * Unwrap a Result or throw an error.
 * Use sparingly - prefer proper Result handling.
 *
 * @param result - Result to unwrap
 * @param errorPrefix - Optional prefix for error message
 * @returns The success value
 * @throws Error if result is a failure
 */
export function unwrapOrThrow<T>(result: Result<T>, errorPrefix?: string): T {
  if (!result.ok) {
    const message = errorPrefix ? `${errorPrefix}: ${result.error}` : result.error;
    throw new Error(message);
  }
  return result.value;
}

/**
 * Get the value from a Result or return a default.
 *
 * @param result - Result to extract from
 * @param defaultValue - Value to return if result is failure
 * @returns Success value or default
 */
export function unwrapOr<T>(result: Result<T>, defaultValue: T): T {
  return result.ok ? result.value : defaultValue;
}

/**
 * Check if a Result is successful and narrow the type.
 * TypeScript type guard for Result types.
 *
 * @param result - Result to check
 * @returns True if result is successful
 */
export function isSuccess<T>(result: Result<T>): result is { ok: true; value: T } {
  return result.ok;
}

/**
 * Check if a Result is a failure and narrow the type.
 * TypeScript type guard for Result types.
 *
 * @param result - Result to check
 * @returns True if result is a failure
 */
export function isFailure<T>(result: Result<T>): result is { ok: false; error: string } {
  return !result.ok;
}

/**
 * Transform a failure message while preserving success.
 *
 * @param result - Result to potentially transform
 * @param transformer - Function to transform error message
 * @returns Result with transformed error or original success
 */
export function mapError<T>(result: Result<T>, transformer: (error: string) => string): Result<T> {
  if (!result.ok) {
    return Failure(transformer(result.error));
  }
  return result;
}

/**
 * Add context to an error message.
 * Common pattern for adding context to failures.
 *
 * @param result - Result to add context to
 * @param context - Context string to prepend to error
 * @returns Result with contextualized error
 */
export function withErrorContext<T>(result: Result<T>, context: string): Result<T> {
  return mapError(result, (error) => `${context}: ${error}`);
}
