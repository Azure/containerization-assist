/**
 * Default Suggestions - Configuration-driven parameter suggestion system
 */

import type { Logger } from 'pino';

/**
 * Function type for generating parameter suggestions
 */
export type SuggestionGenerator = (
  params: Record<string, unknown>,
  context?: Record<string, unknown>,
) => unknown;

/**
 * Default suggestion generators for common parameters
 */
export const DEFAULT_SUGGESTION_GENERATORS: Record<string, SuggestionGenerator> = {
  path: () => '.',

  imageId: (params) => {
    const appName = params.appName || params.name || 'app';
    const tag = params.tag || params.version || 'latest';
    return `${appName}:${tag}`;
  },

  imageName: (params) => {
    const appName = params.appName || params.name || 'app';
    const tag = params.tag || params.version || 'latest';
    return `${appName}:${tag}`;
  },

  registry: (_, context) => context?.registry || context?.defaultRegistry,

  namespace: (params) => params.namespace || 'default',

  replicas: () => 1,

  port: (params) => params.port || 8080,

  dockerfile: (params) => {
    const path = String(params.path || '.');
    return `${path}/Dockerfile`;
  },

  contextPath: (params) => params.path || '.',

  buildArgs: () => ({}),

  labels: (params) => ({
    app: String(params.appName || params.name || 'app'),
    version: String(params.version || 'latest'),
  }),

  environment: () => 'development',

  cluster: (_, context) => context?.cluster || 'local',

  timeout: () => 300,

  memory: () => '512Mi',

  cpu: () => '500m',

  volumeMounts: () => [],

  configMaps: () => [],

  secrets: () => [],

  serviceType: () => 'ClusterIP',

  targetPort: (params) => params.port || 8080,

  protocol: () => 'TCP',

  healthCheckPath: () => '/health',

  readinessPath: () => '/ready',

  livenessPath: () => '/health',
};

/**
 * Module-level registry state
 */
const generators = new Map<string, SuggestionGenerator>(
  Object.entries(DEFAULT_SUGGESTION_GENERATORS),
);
let registryLogger: Logger | undefined;

/**
 * Set the logger for the registry
 */
export const setRegistryLogger = (logger: Logger): void => {
  registryLogger = logger;
};

/**
 * Register a custom suggestion generator
 */
export const registerSuggestion = (param: string, generator: SuggestionGenerator): void => {
  generators.set(param, generator);
  registryLogger?.debug({ param }, 'Registered custom suggestion generator');
};

/**
 * Unregister a suggestion generator
 */
export const unregisterSuggestion = (param: string): boolean => {
  return generators.delete(param);
};

/**
 * Check if a generator exists for a parameter
 */
export const hasSuggestion = (param: string): boolean => {
  return generators.has(param);
};

/**
 * Generate a suggestion for a single parameter
 */
export const generateSuggestion = (
  param: string,
  currentParams: Record<string, unknown>,
  context?: Record<string, unknown>,
): unknown => {
  const generator = generators.get(param);
  if (!generator) {
    registryLogger?.trace({ param }, 'No generator found for parameter');
    return undefined;
  }

  try {
    const value = generator(currentParams, context);
    registryLogger?.trace({ param, value }, 'Generated suggestion');
    return value;
  } catch (error) {
    registryLogger?.warn({ param, error }, 'Generator failed for parameter');
    return undefined;
  }
};

/**
 * Generate suggestions for multiple parameters
 */
export const generateAllSuggestions = (
  missingParams: string[],
  currentParams: Record<string, unknown>,
  context?: Record<string, unknown>,
): Record<string, unknown> => {
  const suggestions: Record<string, unknown> = {};

  for (const param of missingParams) {
    // Skip if parameter already has a value
    if (param in currentParams) {
      continue;
    }

    const value = generateSuggestion(param, currentParams, context);
    if (value !== undefined) {
      suggestions[param] = value;
    }
  }

  registryLogger?.debug(
    { count: Object.keys(suggestions).length, total: missingParams.length },
    'Generated parameter suggestions',
  );

  return suggestions;
};

/**
 * Get all registered parameter names
 */
export const getRegisteredParams = (): string[] => {
  return Array.from(generators.keys());
};

/**
 * Clear all generators
 */
export const clearSuggestions = (): void => {
  generators.clear();
};

/**
 * Reset to default generators
 */
export const resetSuggestions = (): void => {
  clearSuggestions();
  for (const [param, generator] of Object.entries(DEFAULT_SUGGESTION_GENERATORS)) {
    generators.set(param, generator);
  }
};

/**
 * Factory function for creating a suggestion registry instance (for backward compatibility)
 */
export function createSuggestionRegistry(
  customGenerators?: Record<string, SuggestionGenerator>,
  logger?: Logger,
): {
  register: typeof registerSuggestion;
  unregister: typeof unregisterSuggestion;
  has: typeof hasSuggestion;
  generate: typeof generateSuggestion;
  generateAll: typeof generateAllSuggestions;
  getRegisteredParams: typeof getRegisteredParams;
  clear: typeof clearSuggestions;
  reset: typeof resetSuggestions;
} {
  // If custom generators provided, add them to the registry
  if (customGenerators) {
    for (const [param, generator] of Object.entries(customGenerators)) {
      registerSuggestion(param, generator);
    }
  }

  // Set logger if provided
  if (logger) {
    setRegistryLogger(logger);
  }

  return {
    register: registerSuggestion,
    unregister: unregisterSuggestion,
    has: hasSuggestion,
    generate: generateSuggestion,
    generateAll: generateAllSuggestions,
    getRegisteredParams,
    clear: clearSuggestions,
    reset: resetSuggestions,
  };
}
