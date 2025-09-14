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
 * Registry for managing suggestion generators
 */
export class SuggestionRegistry {
  private generators: Map<string, SuggestionGenerator>;
  private logger?: Logger;

  constructor(
    defaults: Record<string, SuggestionGenerator> = DEFAULT_SUGGESTION_GENERATORS,
    logger?: Logger,
  ) {
    this.generators = new Map(Object.entries(defaults));
    this.logger = logger as Logger;
  }

  /**
   * Register a custom suggestion generator
   */
  register(param: string, generator: SuggestionGenerator): void {
    this.generators.set(param, generator);
    this.logger?.debug({ param }, 'Registered custom suggestion generator');
  }

  /**
   * Unregister a suggestion generator
   */
  unregister(param: string): boolean {
    return this.generators.delete(param);
  }

  /**
   * Check if a generator exists for a parameter
   */
  has(param: string): boolean {
    return this.generators.has(param);
  }

  /**
   * Generate a suggestion for a single parameter
   */
  generate(
    param: string,
    currentParams: Record<string, unknown>,
    context?: Record<string, unknown>,
  ): unknown {
    const generator = this.generators.get(param);
    if (!generator) {
      this.logger?.trace({ param }, 'No generator found for parameter');
      return undefined;
    }

    try {
      const value = generator(currentParams, context);
      this.logger?.trace({ param, value }, 'Generated suggestion');
      return value;
    } catch (error) {
      this.logger?.warn({ param, error }, 'Generator failed for parameter');
      return undefined;
    }
  }

  /**
   * Generate suggestions for multiple parameters
   */
  generateAll(
    missingParams: string[],
    currentParams: Record<string, unknown>,
    context?: Record<string, unknown>,
  ): Record<string, unknown> {
    const suggestions: Record<string, unknown> = {};

    for (const param of missingParams) {
      // Skip if parameter already has a value
      if (param in currentParams) {
        continue;
      }

      const value = this.generate(param, currentParams, context);
      if (value !== undefined) {
        suggestions[param] = value;
      }
    }

    this.logger?.debug(
      { count: Object.keys(suggestions).length, total: missingParams.length },
      'Generated parameter suggestions',
    );

    return suggestions;
  }

  /**
   * Get all registered parameter names
   */
  getRegisteredParams(): string[] {
    return Array.from(this.generators.keys());
  }

  /**
   * Clear all generators
   */
  clear(): void {
    this.generators.clear();
  }

  /**
   * Reset to default generators
   */
  reset(): void {
    this.clear();
    for (const [param, generator] of Object.entries(DEFAULT_SUGGESTION_GENERATORS)) {
      this.generators.set(param, generator);
    }
  }

  /**
   * Create a new registry with additional generators
   */
  extend(additionalGenerators: Record<string, SuggestionGenerator>): SuggestionRegistry {
    const combined = {
      ...Object.fromEntries(this.generators),
      ...additionalGenerators,
    };
    return new SuggestionRegistry(combined, this.logger);
  }
}

/**
 * Factory function for creating a suggestion registry
 */
export function createSuggestionRegistry(
  customGenerators?: Record<string, SuggestionGenerator>,
  logger?: Logger,
): SuggestionRegistry {
  const generators = customGenerators
    ? { ...DEFAULT_SUGGESTION_GENERATORS, ...customGenerators }
    : DEFAULT_SUGGESTION_GENERATORS;

  return new SuggestionRegistry(generators, logger);
}
