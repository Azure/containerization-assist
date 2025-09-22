/**
 * Parameter defaulting utilities to reduce duplication of params.X || 'default' patterns
 */

/**
 * Applies default values to tool parameters.
 * Type-safe parameter defaulting with explicit schema.
 * Consolidates the pattern: params.field || defaultValue
 *
 * @param params - Partial parameters from user input
 * @param defaults - Complete default values
 * @returns Merged parameters with defaults applied
 */
export function withDefaults<T extends Record<string, any>>(params: Partial<T>, defaults: T): T {
  const result = { ...defaults };

  // Only override defaults with truthy values from params
  for (const key in params) {
    const value = params[key];
    if (value !== undefined && value !== null) {
      result[key] = value;
    }
  }

  return result;
}

/**
 * Common Kubernetes parameter defaults
 */
export const K8S_DEFAULTS = {
  namespace: 'default',
  serviceType: 'ClusterIP',
  replicas: 1,
  port: 8080,
  targetPort: 8080,
} as const;
