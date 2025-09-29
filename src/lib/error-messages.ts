/**
 * Centralized Error Messages
 *
 * Provides consistent error message templates across the application.
 * Uses template functions for parameterized messages.
 */

export const ERROR_MESSAGES = {
  // Tool-related errors
  TOOL_NOT_FOUND: (name: string) => `Tool not found: ${name}`,
  VALIDATION_FAILED: (issues: string) => `Validation failed: ${issues}`,

  // Session-related errors
  SESSION_GET_FAILED: (error: string) => `Failed to get session: ${error}`,
  SESSION_CREATE_FAILED: (error: string) => `Failed to create session: ${error}`,
  SESSION_UPDATE_FAILED: (error: string) => `Failed to update session: ${error}`,
  SESSION_EXPIRED: (id: string) => `Session expired: ${id}`,

  // Policy-related errors
  POLICY_BLOCKED: (rules: string[]) => `Blocked by policies: ${rules.join(', ')}`,
  POLICY_VALIDATION_FAILED: (issues: string) => `Policy validation failed: ${issues}`,
  POLICY_LOAD_FAILED: (error: string) => `Failed to load policy: ${error}`,

  // Infrastructure-related errors
  DOCKER_OPERATION_FAILED: (operation: string, error: string) =>
    `Docker ${operation} failed: ${error}`,
  K8S_OPERATION_FAILED: (operation: string, error: string) =>
    `Kubernetes ${operation} failed: ${error}`,
  K8S_APPLY_FAILED: (kind: string, name: string, error: string) =>
    `Failed to apply ${kind}/${name}: ${error}`,

  // Retry and execution errors
  RETRY_EXHAUSTED: (attempts: number, error: string) =>
    `Failed after ${attempts} attempts: ${error}`,

  // Generic templates
  OPERATION_FAILED: (operation: string, error: string) => `${operation} failed: ${error}`,
  RESOURCE_NOT_FOUND: (type: string, id: string) => `${type} not found: ${id}`,
} as const;

/**
 * Type-safe error message creator
 */
export type ErrorMessageKey = keyof typeof ERROR_MESSAGES;

/**
 * Helper function to create error messages with type safety
 */
export function createErrorMessage<K extends ErrorMessageKey>(
  key: K,
  ...args: Parameters<(typeof ERROR_MESSAGES)[K]>
): string {
  const template = ERROR_MESSAGES[key];
  return (template as any)(...args);
}
