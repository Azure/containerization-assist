/**
 * Prompt Registry
 *
 * File-based prompt management system that loads prompts from external YAML files
 * and provides SDK-compatible interface for containerization workflows.
 *
 * Key features:
 * - External YAML prompt files for easy editing
 * - Template rendering with parameter substitution
 * - MCP SDK compatibility
 * - Validation and error handling
 */

import type { Logger } from 'pino';
import { join } from 'path';
import {
  ListPromptsResult,
  GetPromptResult,
  PromptArgument,
  PromptMessage,
  McpError,
  ErrorCode,
} from '@modelcontextprotocol/sdk/types.js';
import { SimplePromptLoader, type PromptFile, type ParameterSpec } from './loader';
import { Result } from '@types';

/**
 * Prompt Registry for managing external YAML-based prompt templates
 *
 * This registry loads prompt templates from YAML files and provides a
 * standardized interface for retrieving and formatting them. Supports
 * parameterized prompts with argument substitution and validation.
 *
 * @example
 * ```typescript
 * const registry = new PromptRegistry(logger);
 * await registry.initialize('./src/prompts');
 *
 * const prompt = await registry.getPrompt('dockerfile-generation', {
 *   language: 'nodejs',
 *   baseImage: 'node:18'
 * });
 * ```
 */
export class PromptRegistry {
  private loader: SimplePromptLoader;
  private logger: Logger;
  private initialized = false;

  constructor(logger: Logger) {
    this.logger = logger.child({ component: 'PromptRegistry' });
    this.loader = new SimplePromptLoader(logger);
  }

  /**
   * Initialize the registry by loading prompts from directory
   */
  async initialize(promptsDirectory?: string): Promise<Result<void>> {
    const directory = promptsDirectory || join(process.cwd(), 'src', 'prompts');

    this.logger.info({ directory }, 'Initializing prompt registry');

    const result = await this.loader.loadFromDirectory(directory);
    if (result.ok) {
      this.initialized = true;
      const promptCount = this.loader.getAllPrompts().length;
      this.logger.info({ promptCount }, 'Registry initialized successfully');
    } else {
      this.logger.error({ error: result.error }, 'Failed to initialize registry');
    }

    return result;
  }

  /**
   * List all available prompts (SDK-compatible)
   *
   * @param category - Optional category filter to limit results
   * @returns Promise containing list of available prompts with metadata
   */
  async listPrompts(category?: string): Promise<ListPromptsResult> {
    this.ensureInitialized();

    const allPrompts = this.loader.getAllPrompts();
    const filteredPrompts = category
      ? allPrompts.filter((p) => p.metadata.category === category)
      : allPrompts;

    const prompts = filteredPrompts.map((prompt) => ({
      name: prompt.metadata.name,
      description: prompt.metadata.description,
      arguments: this.convertParameters(prompt.metadata.parameters),
    }));

    this.logger.debug(
      {
        category,
        totalPrompts: allPrompts.length,
        filteredCount: prompts.length,
      },
      'Listed prompts',
    );

    return { prompts };
  }

  /**
   * Get a specific prompt (SDK-compatible)
   *
   * @param name - Name of the prompt to retrieve
   * @param args - Optional arguments for template parameter substitution
   * @returns Promise containing the prompt with rendered content
   * @throws McpError if prompt is not found
   */
  async getPrompt(name: string, args?: Record<string, unknown>): Promise<GetPromptResult> {
    this.ensureInitialized();

    const prompt = this.loader.getPrompt(name);
    if (!prompt) {
      throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
    }

    // Render template with provided arguments
    const renderedText = this.loader.renderTemplate(prompt.template, args || {});

    // Create SDK-compatible message format
    const messages: PromptMessage[] = [
      {
        role: 'user',
        content: {
          type: 'text',
          text: renderedText,
        },
      },
    ];

    this.logger.debug(
      {
        name,
        argumentCount: prompt.metadata.parameters.length,
        messageCount: messages.length,
        templateLength: prompt.template.length,
      },
      'Generated prompt',
    );

    return {
      name: prompt.metadata.name,
      description: prompt.metadata.description,
      arguments: this.convertParameters(prompt.metadata.parameters),
      messages,
    };
  }

  /**
   * Get prompt with messages in ToolContext-compatible format
   */
  async getPromptWithMessages(
    name: string,
    args?: Record<string, unknown>,
  ): Promise<{
    description: string;
    messages: Array<{ role: 'user' | 'assistant'; content: Array<{ type: 'text'; text: string }> }>;
  }> {
    this.ensureInitialized();

    const prompt = this.loader.getPrompt(name);
    if (!prompt) {
      throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
    }

    // Render template
    const renderedText = this.loader.renderTemplate(prompt.template, args || {});

    // Convert to ToolContext format with content arrays
    const messages = [
      {
        role: 'user' as const,
        content: [{ type: 'text' as const, text: renderedText }],
      },
    ];

    return {
      description: prompt.metadata.description,
      messages,
    };
  }

  /**
   * Get prompts by category
   *
   * @param category - Category name to filter by
   * @returns Array of prompt files matching the category
   */
  getPromptsByCategory(category: string): PromptFile[] {
    this.ensureInitialized();
    return this.loader.getPromptsByCategory(category);
  }

  /**
   * Check if a prompt exists
   *
   * @param name - Name of the prompt to check
   * @returns True if the prompt exists and registry is initialized
   */
  hasPrompt(name: string): boolean {
    return this.initialized && this.loader.hasPrompt(name);
  }

  /**
   * Get all prompt names
   *
   * @returns Array of all registered prompt names
   */
  getPromptNames(): string[] {
    this.ensureInitialized();
    return this.loader.getPromptNames();
  }

  /**
   * Get prompt info without rendering
   */
  getPromptInfo(name: string): { description: string; arguments: PromptArgument[] } | null {
    if (!this.initialized) return null;

    const prompt = this.loader.getPrompt(name);
    return prompt
      ? {
          description: prompt.metadata.description,
          arguments: this.convertParameters(prompt.metadata.parameters),
        }
      : null;
  }

  /**
   * Ensure registry is initialized before operations
   */
  private ensureInitialized(): void {
    if (!this.initialized) {
      throw new Error('Registry not initialized. Call initialize() first.');
    }
  }

  /**
   * Convert our parameter format to SDK PromptArgument format
   */
  private convertParameters(parameters: ParameterSpec[]): PromptArgument[] {
    return parameters.map((param) => ({
      name: param.name,
      description: param.description,
      required: param.required || false,
    }));
  }
}
