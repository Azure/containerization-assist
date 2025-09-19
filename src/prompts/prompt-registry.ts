/**
 * Unified Prompt Registry
 *
 * Single, simplified prompt management system that:
 * - Unifies prompt-registry.ts and enhanced-registry.ts
 * - Provides Zod validation for prompt schemas
 * - Simple mustache templating (variables only, no conditionals)
 * - In-memory caching with TTL support
 * - ~170 lines total (vs 330+ in dual system)
 */

import { z } from 'zod';
import type { Logger } from 'pino';
import Mustache from 'mustache';
import { Result, Success, Failure } from '@types';
import { getPerformanceProfiler, PerformanceCache } from '@lib/performance';
import { createCacheKey as createStableCacheKey } from '@lib/stable-json';

// Import new prompt loading system
import { listPromptIds, isPromptEmbedded } from './locator';
import { loadPrompt } from './loader';

// Zod schemas for validation
const ParameterSchema = z.object({
  name: z.string(),
  type: z.enum(['string', 'number', 'boolean', 'array', 'object']),
  required: z.boolean(),
  description: z.string(),
  default: z.any().optional(),
});

const _PromptSchema = z.object({
  id: z.string(),
  category: z.string(),
  description: z.string(),
  version: z.string().regex(/^\d+\.\d+\.\d+$/, 'Version must be in semver format (e.g., 1.0.0)'),
  format: z.enum(['text', 'json', 'markdown']),
  parameters: z.array(ParameterSchema),
  template: z.string(),
  user: z.string().optional(), // User prompt that contains the actual request with variables
  ttl: z.number().optional(), // TTL in seconds for knowledge snippets
  source: z.string().optional(), // Source URL or reference
  previousVersion: z.string().optional(), // Previous version for migration
  deprecated: z.boolean().optional(), // Mark as deprecated
  extends: z.string().optional(), // Prompt inheritance
});

export type PromptEntry = z.infer<typeof _PromptSchema>;
export type PromptParameter = z.infer<typeof ParameterSchema>;

// Version resolution options
export interface VersionOptions {
  version?: string; // Specific version (e.g., "1.2.0")
  allowDeprecated?: boolean; // Allow deprecated versions
  preferLatest?: boolean; // Prefer latest version (default: true)
}

// A/B Testing integration
export interface ABTestingOptions {
  testId?: string;
  userId?: string;
  sessionId?: string;
  recordMetrics?: boolean;
}

// Enhanced rendering options with A/B testing
export interface EnhancedRenderOptions extends VersionOptions {
  abTesting?: ABTestingOptions;
}

// A/B testing result
export interface ABTestPromptResult {
  content: string;
  metadata: {
    testId?: string;
    variantId?: string;
    promptId: string;
    promptVersion?: string;
  };
}

// New enhanced API (fully async)
export interface PromptAPI {
  render(
    id: string,
    params?: Record<string, unknown>,
    options?: EnhancedRenderOptions,
  ): Promise<Result<string>>;
  renderWithABTest(
    id: string,
    params?: Record<string, unknown>,
    options?: EnhancedRenderOptions,
  ): Promise<Result<ABTestPromptResult>>;
  getMetadata(id: string, options?: VersionOptions): Promise<PromptEntry | null>;
  list(category?: string): Promise<PromptEntry[]>;
  validate(
    id: string,
    params: Record<string, unknown>,
    options?: VersionOptions,
  ): Promise<Result<boolean>>;
  getVersions(id: string): Promise<string[]>; // Get all available versions of a prompt
  getLatestVersion(id: string): Promise<string | null>; // Get latest version
}

/**
 * Enhanced Prompt API with full Mustache support, versioning, and A/B testing
 */
export const prompts: PromptAPI = {
  async render(
    id: string,
    params?: Record<string, unknown>,
    options?: EnhancedRenderOptions,
  ): Promise<Result<string>> {
    if (options?.abTesting?.testId) {
      const abResult = await this.renderWithABTest(id, params, options);
      return abResult.ok ? Success(abResult.value.content) : abResult;
    }

    const result = await getPromptWithVersion(id, params, options);
    return result.ok ? Success(result.value.content) : result;
  },

  async renderWithABTest(
    id: string,
    params?: Record<string, unknown>,
    options?: EnhancedRenderOptions,
  ): Promise<Result<ABTestPromptResult>> {
    if (!options?.abTesting?.testId) {
      // No A/B test, use regular rendering
      const result = await getPromptWithVersion(id, params, options);
      if (!result.ok) return result;

      return Success({
        content: result.value.content,
        metadata: {
          promptId: result.value.metadata.id,
          promptVersion: result.value.metadata.version,
        },
      });
    }

    try {
      // Import A/B testing manager (dynamic import to avoid circular dependencies)
      const { getABTestManager } = await import('./ab-testing');
      const abTestManager = getABTestManager(state.logger);

      // Get variant from A/B test
      const variantResult = abTestManager.getVariant(options.abTesting.testId, {
        tool: 'prompt-render', // Could be passed in context
        userId: options.abTesting.userId,
        sessionId: options.abTesting.sessionId,
      });

      if (!variantResult.ok) {
        // Fall back to regular rendering if A/B test fails
        state.logger?.debug(
          { error: variantResult.error },
          'A/B test failed, falling back to regular render',
        );
        const result = await getPromptWithVersion(id, params, options);
        if (!result.ok) return result;

        return Success({
          content: result.value.content,
          metadata: {
            promptId: result.value.metadata.id,
            promptVersion: result.value.metadata.version,
          },
        });
      }

      const variant = variantResult.value;

      // Use the variant's prompt and version
      const versionOptions: VersionOptions = {
        ...options,
        version: variant.promptVersion,
      };

      const result = await getPromptWithVersion(variant.promptId, params, versionOptions);
      if (!result.ok) return result;

      return Success({
        content: result.value.content,
        metadata: {
          testId: options.abTesting.testId,
          variantId: variant.variantId,
          promptId: result.value.metadata.id,
          promptVersion: result.value.metadata.version,
        },
      });
    } catch (error) {
      // Fall back to regular rendering on any error
      state.logger?.error({ error }, 'A/B testing failed, falling back to regular render');
      const result = await getPromptWithVersion(id, params, options);
      if (!result.ok) return result;

      return Success({
        content: result.value.content,
        metadata: {
          promptId: result.value.metadata.id,
          promptVersion: result.value.metadata.version,
        },
      });
    }
  },

  async getMetadata(id: string, options?: VersionOptions): Promise<PromptEntry | null> {
    return await getPromptMetadataWithVersion(id, options);
  },

  async list(category?: string): Promise<PromptEntry[]> {
    return await listPrompts(category);
  },

  async validate(
    id: string,
    params: Record<string, unknown>,
    options?: VersionOptions,
  ): Promise<Result<boolean>> {
    const prompt = await getPromptMetadataWithVersion(id, options);
    if (!prompt) {
      return Failure(`Prompt not found: ${id}`);
    }

    // Check required parameters
    for (const param of prompt.parameters) {
      if (param.required && !(param.name in params)) {
        return Failure(`Missing required parameter: ${param.name}`);
      }

      // Type validation
      const value = params[param.name];
      if (value !== undefined && !validateType(value, param.type)) {
        return Failure(
          `Parameter ${param.name} has wrong type. Expected ${param.type}, got ${typeof value}`,
        );
      }
    }

    return Success(true);
  },

  async getVersions(id: string): Promise<string[]> {
    if (!state.initialized) return [];

    // For now, since we only support single versions, just return the current version
    const prompt = await getPromptMetadata(id);
    return prompt ? [prompt.version] : [];
  },

  async getLatestVersion(id: string): Promise<string | null> {
    const prompt = await getPromptMetadata(id);
    return prompt ? prompt.version : null;
  },
};

/**
 * Validate parameter type
 */
function validateType(value: unknown, type: string): boolean {
  switch (type) {
    case 'string':
      return typeof value === 'string';
    case 'number':
      return typeof value === 'number';
    case 'boolean':
      return typeof value === 'boolean';
    case 'array':
      return Array.isArray(value);
    case 'object':
      return typeof value === 'object' && value !== null && !Array.isArray(value);
    default:
      return true;
  }
}

// Registry state with performance optimization
interface RegistryState {
  prompts: Map<string, PromptEntry>;
  logger?: Logger;
  initialized: boolean;
  cache: Map<string, { content: string; timestamp: number }>;
  performanceCache: PerformanceCache<
    string,
    { content: string; metadata: Omit<PromptEntry, 'template'> }
  >;
  templateCache: PerformanceCache<string, string>;
  lazyLoaded: Set<string>; // Track lazily loaded categories
  profiler: ReturnType<typeof getPerformanceProfiler>;
}

const state: RegistryState = {
  prompts: new Map(),
  initialized: false,
  cache: new Map(),
  performanceCache: new PerformanceCache(500, 300000), // 500 entries, 5min TTL
  templateCache: new PerformanceCache(1000, 600000), // 1000 templates, 10min TTL
  lazyLoaded: new Set(),
  profiler: getPerformanceProfiler(),
};

/**
 * Initialize registry using new manifest-based system
 */
export async function initializeRegistry(
  _directory: string, // Kept for API compatibility but ignored
  logger?: Logger,
): Promise<Result<void>> {
  try {
    if (logger) {
      state.logger = logger.child({ component: 'UnifiedPromptRegistry' });
    }
    state.logger?.info('Initializing unified prompt registry with manifest system');

    // Get all available prompt IDs from manifest or embedded
    const promptIds = await listPromptIds();

    // Load only embedded (critical) prompts on startup for performance
    const criticalPrompts = new Map<string, PromptEntry>();

    for (const id of promptIds) {
      if (await isPromptEmbedded(id)) {
        try {
          const prompt = await loadPrompt(id);
          criticalPrompts.set(id, prompt);
        } catch (error) {
          state.logger?.warn({ promptId: id, error }, 'Failed to load critical prompt');
        }
      }
    }

    state.prompts = criticalPrompts;
    state.initialized = true;

    state.logger?.info(
      {
        totalPrompts: promptIds.length,
        loadedPrompts: criticalPrompts.size,
      },
      'Registry initialized with manifest system',
    );

    return Success(undefined);
  } catch (error) {
    const message = `Failed to initialize registry: ${error}`;
    state.logger?.error({ error }, message);
    return Failure(message);
  }
}

/**
 * Get a prompt by ID and render with parameters (async with lazy loading)
 */
export async function getPrompt(
  id: string,
  params?: Record<string, unknown>,
): Promise<Result<{ content: string; metadata: Omit<PromptEntry, 'template'> }>> {
  if (!state.initialized) {
    return Failure('Registry not initialized');
  }

  // Check if already loaded
  let prompt = state.prompts.get(id);

  // If not loaded, try to lazy load it
  if (!prompt) {
    try {
      prompt = await loadPrompt(id);
      state.prompts.set(id, prompt);
      state.logger?.debug({ promptId: id }, 'Lazy loaded prompt');
    } catch (error) {
      return Failure(`Failed to load prompt: ${id} - ${error}`);
    }
  }

  // Validate required parameters
  const missingParams = prompt.parameters
    .filter((p) => p.required && !(p.name in (params || {})))
    .map((p) => p.name);

  if (missingParams.length > 0) {
    return Failure(`Missing required parameters: ${missingParams.join(', ')}`);
  }

  // Apply defaults for optional parameters
  const finalParams = { ...params };
  for (const param of prompt.parameters) {
    if (param.default !== undefined && !(param.name in finalParams)) {
      finalParams[param.name] = param.default;
    }
  }

  // Render template using Mustache library
  // Disable HTML escaping since we're working with plain text/JSON templates
  Mustache.escape = (text: string) => text;
  const processedParams = processParamsForMustache(finalParams);
  const content = Mustache.render(prompt.template, processedParams);

  const metadata = {
    id: prompt.id,
    category: prompt.category,
    description: prompt.description,
    version: prompt.version,
    format: prompt.format,
    parameters: prompt.parameters,
    ttl: prompt.ttl,
    source: prompt.source,
  };

  return Success({ content, metadata });
}

/**
 * List all available prompts with optional category filter (async with lazy loading)
 */
export async function listPrompts(category?: string): Promise<PromptEntry[]> {
  if (!state.initialized) return [];

  // Get all prompt IDs from manifest
  const allIds = await listPromptIds();

  // Load all prompts lazily
  const allPrompts: PromptEntry[] = [];
  for (const id of allIds) {
    try {
      let prompt = state.prompts.get(id);
      if (!prompt) {
        prompt = await loadPrompt(id);
        state.prompts.set(id, prompt);
      }
      allPrompts.push(prompt);
    } catch (error) {
      state.logger?.warn({ promptId: id, error }, 'Failed to load prompt during listing');
    }
  }

  return category ? allPrompts.filter((p) => p.category === category) : allPrompts;
}

/**
 * Get prompt metadata without rendering (async with lazy loading)
 */
export async function getPromptMetadata(id: string): Promise<PromptEntry | null> {
  // Check if already loaded
  let prompt = state.prompts.get(id);

  // If not loaded, try to lazy load it
  if (!prompt) {
    try {
      prompt = await loadPrompt(id);
      state.prompts.set(id, prompt);
    } catch {
      return null;
    }
  }

  return prompt;
}

/**
 * Get prompt with version support and performance caching
 */
async function getPromptWithVersion(
  id: string,
  params?: Record<string, unknown>,
  options?: VersionOptions,
): Promise<Result<{ content: string; metadata: Omit<PromptEntry, 'template'> }>> {
  const profileResult = await state.profiler.profile<
    Result<{ content: string; metadata: Omit<PromptEntry, 'template'> }>
  >('prompt-render', async () => {
    if (!state.initialized) {
      return Failure('Registry not initialized');
    }

    // Create cache key based on ID, params, and options
    const cacheKey = createCacheKey(id, params, options);

    // Check performance cache first
    const cached = await state.performanceCache.get(cacheKey);
    if (cached) {
      state.logger?.debug({ promptId: id, cacheKey }, 'Prompt rendered from cache');
      return Success(cached);
    }

    let prompt = resolvePromptVersion(id, options);

    // If not found in loaded prompts, try lazy loading
    if (!prompt) {
      try {
        const loadedPrompt = await loadPrompt(id);
        state.prompts.set(id, loadedPrompt);
        prompt = resolvePromptVersion(id, options);
        state.logger?.debug({ promptId: id }, 'Lazy loaded prompt for version resolution');
      } catch {
        return Failure(
          `Prompt not found: ${id}${options?.version ? ` (version ${options.version})` : ''}`,
        );
      }
    }

    if (!prompt) {
      return Failure(
        `Prompt not found: ${id}${options?.version ? ` (version ${options.version})` : ''}`,
      );
    }

    // Check if deprecated and not explicitly allowed
    if (prompt.deprecated && !options?.allowDeprecated) {
      return Failure(`Prompt ${id} version ${prompt.version} is deprecated`);
    }

    // Handle prompt inheritance
    let finalPrompt = prompt;
    if (prompt.extends) {
      const extendedResult = await handlePromptInheritance(prompt);
      if (!extendedResult.ok) {
        return extendedResult;
      }
      finalPrompt = extendedResult.value;
    }

    // Validate required parameters
    const missingParams = finalPrompt.parameters
      .filter((p) => p.required && !(p.name in (params || {})))
      .map((p) => p.name);

    if (missingParams.length > 0) {
      return Failure(`Missing required parameters: ${missingParams.join(', ')}`);
    }

    // Apply defaults for optional parameters
    const finalParams = { ...params };
    for (const param of finalPrompt.parameters) {
      if (param.default !== undefined && !(param.name in finalParams)) {
        finalParams[param.name] = param.default;
      }
    }

    // Render template using Mustache library with caching
    const content = await renderTemplateWithCache(finalPrompt.template, finalParams);

    const metadata = {
      id: finalPrompt.id,
      category: finalPrompt.category,
      description: finalPrompt.description,
      version: finalPrompt.version,
      format: finalPrompt.format,
      parameters: finalPrompt.parameters,
      ttl: finalPrompt.ttl,
      source: finalPrompt.source,
      previousVersion: finalPrompt.previousVersion,
      deprecated: finalPrompt.deprecated,
      extends: finalPrompt.extends,
    };

    const result = { content, metadata };

    // Cache the result for future use
    await state.performanceCache.set(cacheKey, result);

    return Success(result);
  });

  return profileResult.result;
}

/**
 * Get prompt metadata with version support (async)
 */
async function getPromptMetadataWithVersion(
  id: string,
  options?: VersionOptions,
): Promise<PromptEntry | null> {
  return await resolvePromptVersionAsync(id, options);
}

/**
 * Resolve prompt version based on options (async)
 */
async function resolvePromptVersionAsync(
  id: string,
  options?: VersionOptions,
): Promise<PromptEntry | null> {
  if (!state.initialized) return null;

  // Try to get from loaded prompts first
  let prompt = state.prompts.get(id);

  // If not loaded, try to lazy load it
  if (!prompt) {
    try {
      prompt = await loadPrompt(id);
      state.prompts.set(id, prompt);
    } catch {
      return null;
    }
  }

  // If specific version requested, check if it matches
  if (options?.version && prompt.version !== options.version) {
    return null;
  }

  // Check if deprecated and not explicitly allowed
  if (prompt.deprecated && !options?.allowDeprecated) {
    return null;
  }

  return prompt;
}

/**
 * Resolve prompt version based on options (sync - for backwards compatibility)
 */
function resolvePromptVersion(id: string, options?: VersionOptions): PromptEntry | null {
  if (!state.initialized) return null;

  // Only check loaded prompts in sync version
  const prompt = state.prompts.get(id);
  if (!prompt) {
    return null;
  }

  // If specific version requested, check if it matches
  if (options?.version && prompt.version !== options.version) {
    return null;
  }

  // Check if deprecated and not explicitly allowed
  if (prompt.deprecated && !options?.allowDeprecated) {
    return null;
  }

  return prompt;
}

/**
 * Handle prompt inheritance by merging parent and child prompts (async version)
 */
async function handlePromptInheritance(prompt: PromptEntry): Promise<Result<PromptEntry>> {
  if (!prompt.extends) {
    return Success(prompt);
  }

  const parentPrompt = state.prompts.get(prompt.extends);
  if (!parentPrompt) {
    return Failure(`Parent prompt not found: ${prompt.extends}`);
  }

  // Recursively handle inheritance chain
  const parentResult = await handlePromptInheritance(parentPrompt);
  if (!parentResult.ok) {
    return parentResult;
  }

  const parent = parentResult.value;

  // Merge parameters (child overrides parent)
  const mergedParameters = [...parent.parameters];
  for (const childParam of prompt.parameters) {
    const existingIndex = mergedParameters.findIndex((p) => p.name === childParam.name);
    if (existingIndex >= 0) {
      mergedParameters[existingIndex] = childParam;
    } else {
      mergedParameters.push(childParam);
    }
  }

  // Merge templates (child template takes precedence, but can reference parent)
  let mergedTemplate = prompt.template;
  if (mergedTemplate.includes('{{>parent}}')) {
    mergedTemplate = mergedTemplate.replace('{{>parent}}', parent.template);
  }

  return Success({
    ...prompt,
    parameters: mergedParameters,
    template: mergedTemplate,
  });
}

// Compatibility wrappers for legacy API

/**
 * Initialize prompts (compatibility wrapper)
 */
export async function initializePrompts(directory: string, logger?: Logger): Promise<Result<void>> {
  return initializeRegistry(directory, logger);
}

/**
 * Get prompt with messages (compatibility wrapper)
 */
export async function getPromptWithMessages(
  name: string,
  args?: Record<string, unknown>,
): Promise<{
  description: string;
  messages: Array<{ role: 'user' | 'assistant'; content: Array<{ type: 'text'; text: string }> }>;
}> {
  const result = await getPrompt(name, args);
  if (!result.ok) {
    throw new Error(result.error);
  }

  return {
    description: result.value.metadata.description,
    messages: [
      {
        role: 'user' as const,
        content: [{ type: 'text' as const, text: result.value.content }],
      },
    ],
  };
}

// Private helpers

/**
 * Process params to ensure compatibility with Mustache
 * Converts objects to JSON strings when needed for display
 */
function processParamsForMustache(params: Record<string, unknown>): Record<string, unknown> {
  const processed: Record<string, unknown> = {};

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) {
      processed[key] = '';
    } else if (typeof value === 'object' && !Array.isArray(value)) {
      // For non-array objects, convert to JSON string for display in templates
      // This matches the original behavior where objects were stringified
      processed[key] = JSON.stringify(value, null, 2);
    } else if (Array.isArray(value)) {
      // For arrays, check if they contain objects that need stringification
      processed[key] = value.map((item) =>
        typeof item === 'object' && item !== null && !Array.isArray(item)
          ? JSON.stringify(item, null, 2)
          : item,
      );
    } else {
      processed[key] = value;
    }
  }

  return processed;
}

/**
 * Create stable cache key for prompt rendering
 * Uses stable JSON serialization to ensure consistent keys regardless of parameter order
 */
function createCacheKey(
  id: string,
  params?: Record<string, unknown>,
  options?: VersionOptions,
): string {
  // Combine params and options into a single object for stable hashing
  const combined = {
    params: params || {},
    options: options || {},
  };
  return createStableCacheKey(id, combined);
}

/**
 * Render template with caching using stable cache keys
 */
async function renderTemplateWithCache(
  template: string,
  params: Record<string, unknown>,
): Promise<string> {
  // Create stable cache key for template and params
  const templateId = template.substring(0, 100); // Use first 100 chars as template identifier
  const templateKey = createStableCacheKey(templateId, params);

  // Check template cache
  const cached = await state.templateCache.get(templateKey);
  if (cached) {
    return cached;
  }

  // Process params for Mustache compatibility and render
  // Disable HTML escaping since we're working with plain text/JSON templates
  Mustache.escape = (text: string) => text;
  const processedParams = processParamsForMustache(params);
  const rendered = Mustache.render(template, processedParams);
  await state.templateCache.set(templateKey, rendered);

  return rendered;
}

// Uses embedded prompts only
