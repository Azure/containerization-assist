/**
 * Functional Prompt Loader
 *
 * Pure functions for loading and managing prompt definitions from external JSON files.
 */

import { readFile, readdir, stat } from 'fs/promises';
import { join, extname } from 'path';
import type { Logger } from 'pino';
import { Result, Success, Failure } from '../types';
import { extractErrorMessage } from '../lib/error-utils';

/**
 * Parameter specification for prompt templates
 */
export interface ParameterSpec {
  name: string;
  type: 'string' | 'number' | 'boolean' | 'array' | 'object';
  required: boolean;
  description: string;
  default?: string | number | boolean | unknown[] | Record<string, unknown>;
}

/**
 * Prompt entry structure
 */
export interface PromptEntry {
  id: string;
  category: string;
  description: string;
  version: string;
  parameters: ParameterSpec[];
  template: string;
}

/**
 * Load prompts from a directory structure and return as a Map
 *
 * @param directory - Directory containing prompt JSON files organized by category
 * @param logger - Logger instance for debugging and error reporting
 * @returns Result containing Map of prompt ID to PromptEntry
 */
export async function loadPromptsFromDirectory(
  directory: string,
  logger: Logger,
): Promise<Result<Map<string, PromptEntry>>> {
  const prompts = new Map<string, PromptEntry>();
  const loaderLogger = logger.child({ component: 'PromptLoaderFunctions' });

  try {
    loaderLogger.info({ directory }, 'Loading prompts from directory');

    const categories = await getDirectoryCategories(directory);
    let totalLoaded = 0;

    for (const category of categories) {
      const categoryPath = join(directory, category);
      const files = await getPromptFiles(categoryPath);

      loaderLogger.debug({ category, fileCount: files.length }, 'Loading category');

      for (const file of files) {
        const filePath = join(categoryPath, file);
        const loadResult = await loadPromptFile(filePath);

        if (loadResult.ok) {
          const prompt = loadResult.value;
          prompts.set(prompt.id, prompt);
          totalLoaded++;

          loaderLogger.debug(
            {
              name: prompt.id,
              category: prompt.category,
              parameterCount: prompt.parameters.length,
            },
            'Loaded prompt',
          );
        } else {
          loaderLogger.warn(
            { file: filePath, error: loadResult.error },
            'Failed to load prompt file',
          );
        }
      }
    }

    loaderLogger.info({ totalLoaded }, 'Prompt loading completed');
    return Success(prompts);
  } catch (error) {
    const message = `Failed to load prompts: ${extractErrorMessage(error)}`;
    loaderLogger.error({ error, directory }, message);
    return Failure(message);
  }
}

/**
 * Simple template rendering with mustache-style variables
 * Supports {{variable}} and {{#condition}}...{{/condition}}
 *
 * @param template - Template string with placeholders
 * @param params - Parameters to substitute into template
 * @returns Rendered template string
 */
export function renderPromptTemplate(template: string, params: Record<string, unknown>): string {
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

// Private helper functions

/**
 * Get directory categories (subdirectories)
 */
async function getDirectoryCategories(directory: string): Promise<string[]> {
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
 * Get JSON prompt files from a category directory
 */
async function getPromptFiles(categoryPath: string): Promise<string[]> {
  try {
    const files = await readdir(categoryPath);
    return files.filter((file) => extname(file).toLowerCase() === '.json');
  } catch {
    return [];
  }
}

/**
 * Load and parse a single prompt file
 */
async function loadPromptFile(filePath: string): Promise<Result<PromptEntry>> {
  try {
    const content = await readFile(filePath, 'utf8');
    const parsed = JSON.parse(content) as Partial<PromptEntry>;

    // Validate structure
    if (!parsed.id || !parsed.category || !parsed.description || !parsed.template) {
      return Failure(`Invalid prompt file structure: missing required fields`);
    }

    // Ensure parameters array exists
    parsed.parameters = parsed.parameters || [];

    const promptEntry: PromptEntry = {
      id: parsed.id,
      category: parsed.category,
      description: parsed.description,
      version: parsed.version || '1.0',
      parameters: parsed.parameters,
      template: parsed.template,
    };

    return Success(promptEntry);
  } catch (error) {
    return Failure(`Failed to parse prompt file: ${extractErrorMessage(error)}`);
  }
}
