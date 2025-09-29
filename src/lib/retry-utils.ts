/**
 * Retry Utilities
 *
 * Provides exponential backoff retry logic with jitter for AI services
 * and other operations that may fail due to transient issues.
 */

import { type Result, Failure } from '@/types';

/**
 * Configuration for exponential backoff retry
 */
export interface RetryConfig {
  /** Maximum number of attempts (including initial) */
  readonly maxAttempts: number;
  /** Base delay in milliseconds for first retry */
  readonly baseDelayMs: number;
  /** Maximum delay cap in milliseconds */
  readonly maxDelayMs: number;
  /** Exponential multiplier for each retry */
  readonly exponentialBase: number;
  /** Whether to add random jitter to delays */
  readonly useJitter?: boolean;
  /** Custom predicate to determine if error is retryable */
  readonly isRetryable?: (error: unknown) => boolean;
}

/**
 * Retry attempt information
 */
export interface RetryAttempt {
  readonly attemptNumber: number;
  readonly totalAttempts: number;
  readonly delay: number;
  readonly error: unknown;
}

/**
 * Retry result with metadata
 */
export interface RetryResult<T> {
  readonly result: Result<T>;
  readonly attempts: number;
  readonly totalDelay: number;
  readonly lastError?: unknown;
}

/**
 * Default retry configuration
 */
export const DEFAULT_RETRY_CONFIG: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  maxDelayMs: 8000,
  exponentialBase: 2,
  useJitter: true,
  isRetryable: () => true,
} as const;

/**
 * Performs operation with exponential backoff retry logic
 *
 * @param operation - Async operation that returns a Result
 * @param config - Retry configuration
 * @returns Promise<RetryResult<T>>
 */
export async function withExponentialBackoff<T>(
  operation: () => Promise<Result<T>>,
  config: Partial<RetryConfig> = {},
): Promise<RetryResult<T>> {
  const finalConfig: RetryConfig = { ...DEFAULT_RETRY_CONFIG, ...config };

  let lastError: unknown;
  let totalDelay = 0;

  for (let attempt = 1; attempt <= finalConfig.maxAttempts; attempt++) {
    try {
      const result = await operation();

      if (result.ok) {
        return {
          result,
          attempts: attempt,
          totalDelay,
          lastError,
        };
      }

      lastError = result.error;

      // Don't retry on last attempt
      if (attempt === finalConfig.maxAttempts) {
        break;
      }

      // Check if error is retryable
      if (finalConfig.isRetryable && !finalConfig.isRetryable(result.error)) {
        return {
          result,
          attempts: attempt,
          totalDelay,
          lastError: result.error,
        };
      }

      // Calculate delay with exponential backoff
      const delay = calculateDelay(attempt - 1, finalConfig);
      totalDelay += delay;

      // Wait before next attempt
      await sleep(delay);
    } catch (error) {
      lastError = error;

      // Don't retry on last attempt
      if (attempt === finalConfig.maxAttempts) {
        break;
      }

      // Check if error is retryable
      if (finalConfig.isRetryable && !finalConfig.isRetryable(error)) {
        return {
          result: Failure(`Non-retryable error: ${String(error)}`),
          attempts: attempt,
          totalDelay,
          lastError: error,
        };
      }

      // Calculate delay with exponential backoff
      const delay = calculateDelay(attempt - 1, finalConfig);
      totalDelay += delay;

      // Wait before next attempt
      await sleep(delay);
    }
  }

  // All attempts exhausted
  return {
    result: Failure(
      `All ${finalConfig.maxAttempts} retry attempts failed. Last error: ${String(lastError)}`,
    ),
    attempts: finalConfig.maxAttempts,
    totalDelay,
    lastError,
  };
}

/**
 * Specialized retry for AI operations with common retry conditions
 */
export async function withAIRetry<T>(
  operation: () => Promise<Result<T>>,
  config: Partial<RetryConfig> = {},
): Promise<RetryResult<T>> {
  const aiRetryConfig: RetryConfig = {
    ...DEFAULT_RETRY_CONFIG,
    isRetryable: isAIErrorRetryable,
    ...config,
  };

  return withExponentialBackoff(operation, aiRetryConfig);
}

/**
 * Determines if an AI-related error is worth retrying
 */
export function isAIErrorRetryable(error: unknown): boolean {
  const errorStr = String(error).toLowerCase();

  // Retry on transient network/service errors
  if (
    errorStr.includes('timeout') ||
    errorStr.includes('connection') ||
    errorStr.includes('network') ||
    errorStr.includes('service unavailable') ||
    errorStr.includes('rate limit') ||
    errorStr.includes('502') ||
    errorStr.includes('503') ||
    errorStr.includes('504')
  ) {
    return true;
  }

  // Don't retry on authentication or authorization errors
  if (
    errorStr.includes('unauthorized') ||
    errorStr.includes('forbidden') ||
    errorStr.includes('401') ||
    errorStr.includes('403')
  ) {
    return false;
  }

  // Don't retry on validation or parsing errors
  if (
    errorStr.includes('validation') ||
    errorStr.includes('parse') ||
    errorStr.includes('invalid json') ||
    errorStr.includes('schema')
  ) {
    return false;
  }

  // Default to retrying unknown errors
  return true;
}

/**
 * Calculate delay for a given retry attempt with exponential backoff and jitter
 */
function calculateDelay(attemptIndex: number, config: RetryConfig): number {
  const exponentialDelay = config.baseDelayMs * Math.pow(config.exponentialBase, attemptIndex);
  const cappedDelay = Math.min(exponentialDelay, config.maxDelayMs);

  if (!config.useJitter) {
    return cappedDelay;
  }

  // Add jitter (±25% of delay)
  const jitterRange = cappedDelay * 0.25;
  const jitter = (Math.random() * 2 - 1) * jitterRange;

  return Math.max(0, cappedDelay + jitter);
}

/**
 * Sleep utility function
 */
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Retry with a simple predicate function (non-Result based)
 * Useful for operations that throw exceptions
 */
export async function retryWithPredicate<T>(
  operation: () => Promise<T>,
  shouldRetry: (error: unknown) => boolean,
  config: Partial<RetryConfig> = {},
): Promise<T> {
  const finalConfig: RetryConfig = {
    ...DEFAULT_RETRY_CONFIG,
    isRetryable: shouldRetry,
    ...config,
  };

  let lastError: unknown;

  for (let attempt = 1; attempt <= finalConfig.maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error;

      // Don't retry on last attempt
      if (attempt === finalConfig.maxAttempts) {
        throw error;
      }

      // Check if error is retryable
      if (!shouldRetry(error)) {
        throw error;
      }

      // Calculate delay and wait
      const delay = calculateDelay(attempt - 1, finalConfig);
      await sleep(delay);
    }
  }

  throw lastError;
}

/**
 * Create a retry-wrapped version of an async function
 */
export function createRetryWrapper<TArgs extends readonly unknown[], TReturn>(
  fn: (...args: TArgs) => Promise<Result<TReturn>>,
  config: Partial<RetryConfig> = {},
): (...args: TArgs) => Promise<RetryResult<TReturn>> {
  return (...args: TArgs) => withExponentialBackoff(() => fn(...args), config);
}

/**
 * Circuit breaker state for preventing cascade failures
 */
interface CircuitBreakerState {
  failures: number;
  lastFailureTime: number;
  state: 'closed' | 'open' | 'half-open';
}

/**
 * Simple circuit breaker configuration
 */
export interface CircuitBreakerConfig {
  /** Number of failures before opening circuit */
  readonly failureThreshold: number;
  /** Time to wait before trying again (ms) */
  readonly recoveryTimeoutMs: number;
  /** Timeout for individual requests (ms) */
  readonly requestTimeoutMs: number;
}

/**
 * Circuit breaker with exponential backoff
 * Prevents cascade failures by stopping requests after repeated failures
 */
export class CircuitBreaker<T> {
  private state: CircuitBreakerState = {
    failures: 0,
    lastFailureTime: 0,
    state: 'closed',
  };

  constructor(
    private readonly config: CircuitBreakerConfig,
    private readonly operation: () => Promise<Result<T>>,
  ) {}

  async execute(): Promise<Result<T>> {
    if (this.state.state === 'open') {
      const now = Date.now();
      if (now - this.state.lastFailureTime < this.config.recoveryTimeoutMs) {
        return Failure('Circuit breaker is open');
      }
      this.state.state = 'half-open';
    }

    try {
      const result = await Promise.race([this.operation(), this.createTimeoutPromise()]);

      if (result.ok) {
        this.onSuccess();
        return result;
      } else {
        this.onFailure();
        return result;
      }
    } catch (error) {
      this.onFailure();
      return Failure(`Circuit breaker error: ${String(error)}`);
    }
  }

  private async createTimeoutPromise(): Promise<Result<T>> {
    return new Promise((_, reject) => {
      setTimeout(() => {
        reject(new Error(`Operation timeout after ${this.config.requestTimeoutMs}ms`));
      }, this.config.requestTimeoutMs);
    });
  }

  private onSuccess(): void {
    this.state.failures = 0;
    this.state.state = 'closed';
  }

  private onFailure(): void {
    this.state.failures++;
    this.state.lastFailureTime = Date.now();

    if (this.state.failures >= this.config.failureThreshold) {
      this.state.state = 'open';
    }
  }

  getState(): CircuitBreakerState['state'] {
    return this.state.state;
  }
}
