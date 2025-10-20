/**
 * Environment Variable Parsing Utilities
 *
 * Standardized utilities for parsing environment variables with type safety
 * and consistent default handling.
 */

/**
 * Parse integer from environment variable with default
 *
 * @param key - Environment variable name
 * @param defaultValue - Default value if not set or invalid
 * @returns Parsed integer or default value
 *
 * @example
 * parseIntEnv('PORT', 3000) // Returns 3000 if PORT not set
 * parseIntEnv('MAX_SIZE', 100) // Returns 100 if MAX_SIZE is invalid number
 */
export function parseIntEnv(key: string, defaultValue: number): number {
  const value = process.env[key];
  if (!value) return defaultValue;
  const parsed = parseInt(value, 10);
  return isNaN(parsed) ? defaultValue : parsed;
}

/**
 * Parse string from environment variable with default
 *
 * @param key - Environment variable name
 * @param defaultValue - Default value if not set
 * @returns Environment variable value or default
 *
 * @example
 * parseStringEnv('LOG_LEVEL', 'info') // Returns 'info' if LOG_LEVEL not set
 */
export function parseStringEnv(key: string, defaultValue: string): string {
  const value = process.env[key];
  return value === undefined ? defaultValue : value;
}

/**
 * Parse boolean from environment variable with default
 *
 * Recognizes common boolean string representations:
 * - true: 'true', '1', 'yes'
 * - false: 'false', '0', 'no'
 *
 * @param key - Environment variable name
 * @param defaultValue - Default value if not set or unrecognized
 * @returns Boolean value
 *
 * @example
 * parseBoolEnv('ENABLE_FEATURE', true) // Returns true if not set
 * parseBoolEnv('ENABLE_FEATURE', true) // Returns false if 'false', '0', or 'no'
 */
export function parseBoolEnv(key: string, defaultValue: boolean): boolean {
  const value = process.env[key];
  if (value === undefined) return defaultValue;
  const lower = value.toLowerCase();
  if (lower === 'false' || lower === '0' || lower === 'no') return false;
  if (lower === 'true' || lower === '1' || lower === 'yes') return true;
  return defaultValue;
}

/**
 * Parse comma-separated list from environment variable
 *
 * Trims whitespace from each item and filters out empty strings.
 *
 * @param key - Environment variable name
 * @param delimiter - Delimiter to split on (default: ',')
 * @returns Array of trimmed, non-empty strings
 *
 * @example
 * parseListEnv('ALLOWED_HOSTS') // ['host1', 'host2'] if ALLOWED_HOSTS='host1, host2'
 * parseListEnv('TAGS', ';') // ['tag1', 'tag2'] if TAGS='tag1;tag2' with semicolon delimiter
 */
export function parseListEnv(key: string, delimiter = ','): string[] {
  const value = process.env[key];
  if (!value) return [];
  return value
    .split(delimiter)
    .map((s) => s.trim())
    .filter(Boolean);
}
