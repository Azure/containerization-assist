/**
 * Parameter defaulting utilities to reduce duplication of params.X || 'default' patterns
 */

/**
 * Applies default values to tool parameters.
 * Type-safe parameter defaulting with explicit schema.
 * Consolidates the pattern: params.field || defaultValue
 *
 * Note: The `Record<string, any>` constraint is intentionally broad to support
 * flexible tool parameter objects with mixed types (strings, numbers, booleans,
 * nested objects, arrays, etc.). The actual type safety comes from the generic
 * parameter `T` which should be the concrete interface of your tool parameters.
 *
 * @param params - Partial parameters from user input
 * @param defaults - Complete default values
 * @returns Merged parameters with defaults applied
 *
 * @example
 * ```typescript
 * interface K8sParams {
 *   namespace: string;
 *   replicas: number;
 *   serviceType: 'ClusterIP' | 'LoadBalancer';
 * }
 *
 * const params: Partial<K8sParams> = { replicas: 3 };
 * const result = withDefaults(params, {
 *   namespace: 'default',
 *   replicas: 1,
 *   serviceType: 'ClusterIP'
 * }); // result: K8sParams with replicas: 3, other defaults applied
 * ```
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

/**
 * Common container/Docker parameter defaults
 */
export const CONTAINER_DEFAULTS = {
  environment: 'production',
  registry: 'docker.io',
  cpu: '0.5',
  memory: '1Gi',
  tag: 'latest',
} as const;

/**
 * Azure Container Apps defaults
 */
export const ACA_DEFAULTS = {
  environment: 'production',
  location: 'eastus',
  cpu: '0.5',
  memory: '1Gi',
  minReplicas: 0,
  maxReplicas: 10,
} as const;

/**
 * Build-related defaults
 */
export const BUILD_DEFAULTS = {
  platform: 'linux/amd64',
  nocache: false,
  push: false,
} as const;

/**
 * Helper to get defaults for a specific tool category
 */
export function getToolDefaults(
  category: 'k8s' | 'container' | 'aca' | 'build',
): Record<string, unknown> {
  switch (category) {
    case 'k8s':
      return K8S_DEFAULTS;
    case 'container':
      return CONTAINER_DEFAULTS;
    case 'aca':
      return ACA_DEFAULTS;
    case 'build':
      return BUILD_DEFAULTS;
    default:
      return {};
  }
}

/**
 * Type-safe parameter builder for tools
 * Ensures all required fields are present after defaulting
 *
 * Note: The `Record<string, any>` constraint is intentionally broad to support
 * flexible tool parameter objects. Type safety is enforced through the generic
 * parameter `T`, which should be your concrete tool parameter interface.
 */
export class ParameterBuilder<T extends Record<string, any>> {
  private params: Partial<T>;
  private defaultValues: Partial<T>;

  constructor(params: Partial<T>) {
    this.params = params;
    this.defaultValues = {};
  }

  /**
   * Set default value for a field
   */
  default<K extends keyof T>(key: K, value: T[K]): this {
    this.defaultValues[key] = value;
    return this;
  }

  /**
   * Set multiple defaults at once
   */
  defaults(defaults: Partial<T>): this {
    Object.assign(this.defaultValues, defaults);
    return this;
  }

  /**
   * Build the final parameters object
   */
  build(): T {
    return withDefaults(this.params, this.defaultValues as T);
  }
}

/**
 * Create a parameter builder for fluent API usage
 *
 * Note: The `Record<string, any>` constraint is intentionally broad to support
 * flexible tool parameter objects. Type safety is enforced through the generic
 * parameter `T`, which should be your concrete tool parameter interface.
 *
 * @example
 * ```typescript
 * const params = buildParams<K8sParams>(inputParams)
 *   .default('namespace', 'default')
 *   .default('replicas', 1)
 *   .defaults(K8S_DEFAULTS)
 *   .build();
 * ```
 */
export function buildParams<T extends Record<string, any>>(
  params: Partial<T>,
): ParameterBuilder<T> {
  return new ParameterBuilder<T>(params);
}
