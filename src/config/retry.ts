/**
 * Retry Configuration
 *
 * Retry logic parameters including attempts, delays, and backoff settings.
 */

export const RETRY_CONFIG = {
  /** Maximum retry attempts */
  MAX_ATTEMPTS: 3,
  /** Base delay in milliseconds */
  BASE_DELAY_MS: 1000,
  /** Maximum delay in milliseconds */
  MAX_DELAY_MS: 8000,
  /** Exponential backoff base multiplier */
  EXPONENTIAL_BASE: 2,
} as const;

export type RetryConfigKey = keyof typeof RETRY_CONFIG;
