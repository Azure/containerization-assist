/**
 * Static TypeScript Prompt Loader
 *
 * Loads prompt definitions from statically imported TypeScript constants.
 * Uses compile-time TypeScript imports with strict typing enforcement.
 */

import type { Logger } from 'pino';
import { Result, Success, Failure } from '../../domain/types';

// Static imports for all prompt files
import { enhanceRepoAnalysis } from '../../prompts/analysis/enhance-repo-analysis';
import { dockerfileGeneration } from '../../prompts/containerization/dockerfile-generation';
import { fixDockerfile } from '../../prompts/containerization/fix-dockerfile';
import { generateDockerfile } from '../../prompts/containerization/generate-dockerfile';
import { generateK8sManifests } from '../../prompts/orchestration/generate-k8s-manifests';
import { k8sManifestGeneration } from '../../prompts/orchestration/k8s-manifest-generation';
import { dockerfileSampling } from '../../prompts/sampling/dockerfile-sampling';
import { strategyOptimization } from '../../prompts/sampling/strategy-optimization';
import { securityAnalysis } from '../../prompts/security/security-analysis';
import { parameterSuggestions } from '../../prompts/validation/parameter-suggestions';
import { parameterValidation } from '../../prompts/validation/parameter-validation';

/**
 * Parameter specification for prompt templates
 *
 * Defines the structure and validation rules for parameters that can be
 * substituted into prompt templates during rendering.
 *
 * @example
 * ```typescript
 * const param: ParameterSpec = {
 *   name: 'language',
 *   type: 'string',
 *   required: true,
 *   description: 'Programming language for the project',
 *   default: 'javascript'
 * };
 * ```
 */
export interface ParameterSpec {
  name: string;
  type: 'string' | 'number' | 'boolean' | 'array' | 'object';
  required: boolean;
  description: string;
  default?: string | number | boolean | unknown[] | Record<string, unknown>;
}

/**
 * Prompt metadata extracted from JSON template files
 *
 * Contains descriptive information and parameter definitions for a prompt template.
 * Used for validation, documentation, and dynamic UI generation.
 */
interface PromptMetadata {
  name: string;
  category: string;
  description: string;
  version: string;
  parameters: ParameterSpec[];
}

/**
 * Complete prompt template definition loaded from JSON files
 *
 * Represents a fully parsed prompt template including metadata, content template,
 * and parameter specifications. Ready for rendering with user-provided arguments.
 */
export interface PromptFile {
  metadata: PromptMetadata;
  template: string;
}

/**
 * Static prompt loader for TypeScript-based prompt constants
 */
export class StaticPromptLoader {
  private prompts = new Map<string, PromptFile>();
  private logger: Logger;
  private initialized = false;

  constructor(logger: Logger) {
    this.logger = logger.child({ component: 'StaticPromptLoader' });
  }

  /**
   * Load all prompts from statically imported TypeScript constants
   */
  async initialize(): Promise<Result<void>> {
    try {
      this.logger.info('Loading prompts from static imports');

      // Define all static imports with their TypeScript data
      const promptImports: PromptFile[] = [
        enhanceRepoAnalysis,
        dockerfileGeneration,
        fixDockerfile,
        generateDockerfile,
        generateK8sManifests,
        k8sManifestGeneration,
        dockerfileSampling,
        strategyOptimization,
        securityAnalysis,
        parameterSuggestions,
        parameterValidation,
      ];

      let totalLoaded = 0;

      for (const prompt of promptImports) {
        this.prompts.set(prompt.metadata.name, prompt);
        totalLoaded++;

        this.logger.debug(
          {
            name: prompt.metadata.name,
            category: prompt.metadata.category,
            parameterCount: prompt.metadata.parameters.length,
          },
          'Loaded prompt',
        );
      }

      this.initialized = true;
      this.logger.info({ totalLoaded }, 'Static prompt loading completed');

      return Success(undefined);
    } catch (error) {
      const message = `Failed to load prompts: ${error instanceof Error ? error.message : 'Unknown error'}`;
      this.logger.error({ error }, message);
      return Failure(message);
    }
  }

  /**
   * Get a prompt by name
   */
  getPrompt(name: string): PromptFile | undefined {
    if (!this.initialized) {
      this.logger.warn('Loader not initialized, call initialize first');
      return undefined;
    }

    return this.prompts.get(name);
  }

  /**
   * Get all loaded prompts
   */
  getAllPrompts(): PromptFile[] {
    return Array.from(this.prompts.values());
  }

  /**
   * Get prompts by category
   */
  getPromptsByCategory(category: string): PromptFile[] {
    return this.getAllPrompts().filter((prompt) => prompt.metadata.category === category);
  }

  /**
   * Check if a prompt exists
   */
  hasPrompt(name: string): boolean {
    return this.prompts.has(name);
  }

  /**
   * Get all prompt names
   */
  getPromptNames(): string[] {
    return Array.from(this.prompts.keys());
  }

  /**
   * Get all categories
   */
  getCategories(): string[] {
    const categories = new Set<string>();
    for (const prompt of this.prompts.values()) {
      categories.add(prompt.metadata.category);
    }
    return Array.from(categories);
  }

  /**
   * Simple template rendering with mustache-style variables
   * Supports {{variable}} and {{#condition}}...{{/condition}}
   */
  renderTemplate(template: string, params: Record<string, unknown>): string {
    let rendered = template;

    // Handle conditional blocks {{#var}}...{{/var}}
    rendered = rendered.replace(/\{\{#(\w+)\}\}([\s\S]*?)\{\{\/\1\}\}/g, (_, key, content) => {
      const value = params[key];
      // Include content if variable is truthy (exists and not false/empty)
      return value && value !== false && value !== '' ? content : '';
    });

    // Handle simple variable replacement {{var}}
    rendered = rendered.replace(/\{\{(\w+)\}\}/g, (_, key) => {
      const value = params[key];
      return value !== undefined ? String(value) : '';
    });

    // Clean up extra newlines
    rendered = rendered.replace(/\n{3,}/g, '\n\n').trim();

    return rendered;
  }
}
