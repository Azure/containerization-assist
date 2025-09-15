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
 * State for the parameter builder
 */
export interface BuilderState<T> {
  params: Partial<T>;
  defaultValues: Partial<T>;
}

/**
 * Add a default value to the builder state
 */
export const addDefault = <T extends Record<string, any>, K extends keyof T>(
  state: BuilderState<T>,
  key: K,
  value: T[K],
): BuilderState<T> => ({
  ...state,
  defaultValues: { ...state.defaultValues, [key]: value },
});

/**
 * Add multiple defaults to the builder state
 */
export const addDefaults = <T extends Record<string, any>>(
  state: BuilderState<T>,
  defaults: Partial<T>,
): BuilderState<T> => ({
  ...state,
  defaultValues: { ...state.defaultValues, ...defaults },
});

/**
 * Build the final parameters from the builder state
 */
export const buildParameters = <T extends Record<string, any>>(state: BuilderState<T>): T => {
  return withDefaults(state.params, state.defaultValues as T);
};

/**
 * Interface for the parameter builder
 */
export interface ParameterBuilderInterface<T extends Record<string, any>> {
  default<K extends keyof T>(key: K, value: T[K]): ParameterBuilderInterface<T>;
  defaults(defaults: Partial<T>): ParameterBuilderInterface<T>;
  build(): T;
}

/**
 * Type-safe parameter builder factory for tools
 * Ensures all required fields are present after defaulting
 */
export const createParameterBuilder = <T extends Record<string, any>>(
  params: Partial<T>,
): ParameterBuilderInterface<T> => {
  let state: BuilderState<T> = {
    params,
    defaultValues: {},
  };

  const builder: ParameterBuilderInterface<T> = {
    default: (key, value) => {
      state = addDefault(state, key, value);
      return builder;
    },
    defaults: (defaults) => {
      state = addDefaults(state, defaults);
      return builder;
    },
    build: () => buildParameters(state),
  };

  return builder;
};

/**
 * Create a parameter builder for fluent API usage
 * Backward compatibility alias for createParameterBuilder
 *
 * Example:
 * const params = buildParams(inputParams)
 *   .default('namespace', 'default')
 *   .default('replicas', 1)
 *   .defaults(K8S_DEFAULTS)
 *   .build();
 */
export const buildParams = createParameterBuilder;

/**
 * Export ParameterBuilder type for backward compatibility
 * (For type references only - not a class anymore)
 */
export type ParameterBuilder<T extends Record<string, any>> = ParameterBuilderInterface<T>;
