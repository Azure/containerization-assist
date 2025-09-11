/**
 * YAML Prompt Loader
 *
 * Loads prompt definitions from external YAML files organized by category.
 * Replaces the complex prompt registry with a simple file-based system.
 */

import { readFile, readdir, stat } from 'fs/promises';
import { join, extname } from 'path';
import { load } from 'js-yaml';
import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';

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
 * Prompt metadata extracted from YAML template files
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
 * Complete prompt template definition loaded from YAML files
 *
 * Represents a fully parsed prompt template including metadata, content template,
 * and parameter specifications. Ready for rendering with user-provided arguments.
 */
export interface PromptFile {
  metadata: PromptMetadata;
  template: string;
}

/**
 * Prompt loader for YAML-based prompt files
 */
export class PromptLoader {
  private prompts = new Map<string, PromptFile>();
  private logger: Logger;
  private initialized = false;

  constructor(logger: Logger) {
    this.logger = logger.child({ component: 'PromptLoader' });
  }

  /**
   * Load all prompts from a directory structure
   */
  async loadFromDirectory(directory: string): Promise<Result<void>> {
    try {
      this.logger.info({ directory }, 'Loading prompts from directory');

      const categories = await this.getDirectoryCategories(directory);
      let totalLoaded = 0;

      for (const category of categories) {
        const categoryPath = join(directory, category);
        const files = await this.getPromptFiles(categoryPath);

        this.logger.debug({ category, fileCount: files.length }, 'Loading category');

        for (const file of files) {
          const filePath = join(categoryPath, file);
          const loadResult = await this.loadPromptFile(filePath);

          if (loadResult.ok) {
            const prompt = loadResult.value;
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
          } else {
            this.logger.warn(
              { file: filePath, error: loadResult.error },
              'Failed to load prompt file',
            );
          }
        }
      }

      this.initialized = true;
      this.logger.info({ totalLoaded }, 'Prompt loading completed');

      return Success(undefined);
    } catch (error) {
      const message = `Failed to load prompts: ${error instanceof Error ? error.message : 'Unknown error'}`;
      this.logger.error({ error, directory }, message);
      return Failure(message);
    }
  }

  /**
   * Get a prompt by name
   */
  getPrompt(name: string): PromptFile | undefined {
    if (!this.initialized) {
      this.logger.warn('Loader not initialized, call loadFromDirectory first');
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

  /**
   * Get directory categories (subdirectories)
   */
  private async getDirectoryCategories(directory: string): Promise<string[]> {
    const entries = await readdir(directory);
    const categories: string[] = [];

    for (const entry of entries) {
      const entryPath = join(directory, entry);
      const stats = await stat(entryPath);

      if (stats.isDirectory()) {
        categories.push(entry);
      }
    }

    return categories;
  }

  /**
   * Get YAML prompt files from a category directory
   */
  private async getPromptFiles(categoryPath: string): Promise<string[]> {
    try {
      const files = await readdir(categoryPath);
      return files.filter(
        (file) => extname(file).toLowerCase() === '.yaml' || extname(file).toLowerCase() === '.yml',
      );
    } catch (error) {
      this.logger.debug({ categoryPath, error }, 'Could not read category directory');
      return [];
    }
  }

  /**
   * Load and parse a single prompt file
   */
  private async loadPromptFile(filePath: string): Promise<Result<PromptFile>> {
    try {
      const content = await readFile(filePath, 'utf8');
      const parsed = load(content) as {
        metadata?: {
          name?: string;
          category?: string;
          description?: string;
          version?: string;
          parameters?: ParameterSpec[];
        };
        template?: string;
      };

      // Validate structure
      if (!parsed?.metadata || !parsed.template) {
        return Failure(`Invalid prompt file structure: missing metadata or template`);
      }

      const { metadata, template } = parsed;

      // Validate metadata
      if (!metadata.name || !metadata.category || !metadata.description) {
        return Failure(`Invalid metadata: missing required fields (name, category, description)`);
      }

      // Ensure parameters array exists
      metadata.parameters = metadata.parameters || [];

      const promptFile: PromptFile = {
        metadata: {
          name: metadata.name,
          category: metadata.category,
          description: metadata.description,
          version: metadata.version || '1.0',
          parameters: metadata.parameters,
        },
        template: String(template),
      };

      return Success(promptFile);
    } catch (error) {
      return Failure(
        `Failed to parse prompt file: ${error instanceof Error ? error.message : 'Unknown error'}`,
      );
    }
  }
}
