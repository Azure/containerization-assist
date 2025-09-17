/**
 * Prompt Registry - Functional Module
 *
 * File-based prompt management system that loads prompts from external JSON files
 * and provides SDK-compatible interface for containerization workflows.
 *
 * Key features:
 * - External JSON prompt files for easy editing
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
import {
  loadPromptsFromDirectory,
  renderPromptTemplate,
  type PromptEntry,
  type ParameterSpec,
} from './loader';
import { Result, Success, Failure } from '@/types';

// Module-level state
let promptMap: Map<string, PromptEntry> | null = null;
let logger: Logger | null = null;
let initialized = false;

/**
 * Initialize the prompt registry by loading prompts from directory
 *
 * @param promptsDirectory - Directory containing prompt JSON files
 * @param parentLogger - Logger instance for logging
 * @returns Result indicating success or failure
 *
 * @example
 * ```typescript
 * await initializePrompts('./src/prompts', logger);
 * const prompt = await getPrompt('dockerfile-generation', {
 *   language: 'nodejs',
 *   baseImage: 'node:18'
 * });
 * ```
 */
export async function initializePrompts(
  promptsDirectory: string,
  parentLogger: Logger,
): Promise<Result<void>> {
  const directory = promptsDirectory || join(process.cwd(), 'src', 'prompts');

  logger = parentLogger.child({ component: 'PromptRegistry' });

  logger.info({ directory }, 'Initializing prompt registry');

  const result = await loadPromptsFromDirectory(directory, parentLogger);
  if (result.ok) {
    promptMap = result.value;
    initialized = true;
    const promptCount = promptMap.size;
    logger.info({ promptCount }, 'Registry initialized successfully');
    return Success(undefined);
  } else {
    logger.error({ error: result.error }, 'Failed to initialize registry');
    return Failure(result.error);
  }
}

/**
 * List all available prompts (SDK-compatible)
 *
 * @param category - Optional category filter to limit results
 * @returns Promise containing list of available prompts with metadata
 */
export async function listPrompts(category?: string): Promise<ListPromptsResult> {
  ensureInitialized();

  if (!promptMap) throw new Error('Prompt registry not initialized');
  const allPrompts = Array.from(promptMap.values());
  const filteredPrompts = category ? allPrompts.filter((p) => p.category === category) : allPrompts;

  const prompts = filteredPrompts.map((prompt) => ({
    name: prompt.id,
    description: prompt.description,
    arguments: convertParameters(prompt.parameters),
  }));

  if (logger) {
    logger.debug(
      {
        category,
        totalPrompts: allPrompts.length,
        filteredCount: prompts.length,
      },
      'Listed prompts',
    );
  }

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
export async function getPrompt(
  name: string,
  args?: Record<string, unknown>,
): Promise<GetPromptResult> {
  ensureInitialized();

  if (!promptMap) throw new Error('Prompt registry not initialized');
  const prompt = promptMap.get(name);
  if (!prompt) {
    throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
  }

  // Render template with provided arguments
  const renderedText = renderPromptTemplate(prompt.template, args || {});

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

  if (logger) {
    logger.debug(
      {
        name,
        argumentCount: prompt.parameters.length,
        messageCount: messages.length,
        templateLength: prompt.template.length,
      },
      'Generated prompt',
    );
  }

  return {
    name: prompt.id,
    description: prompt.description,
    arguments: convertParameters(prompt.parameters),
    messages,
  };
}

/**
 * Get prompt with messages in ToolContext-compatible format
 */
export async function getPromptWithMessages(
  name: string,
  args?: Record<string, unknown>,
): Promise<{
  description: string;
  messages: Array<{ role: 'user' | 'assistant'; content: Array<{ type: 'text'; text: string }> }>;
}> {
  ensureInitialized();

  if (!promptMap) throw new Error('Prompt registry not initialized');
  const prompt = promptMap.get(name);
  if (!prompt) {
    throw new McpError(ErrorCode.MethodNotFound, `Prompt not found: ${name}`);
  }

  // Render template
  const renderedText = renderPromptTemplate(prompt.template, args || {});

  // Convert to ToolContext format with content arrays
  const messages = [
    {
      role: 'user' as const,
      content: [{ type: 'text' as const, text: renderedText }],
    },
  ];

  return {
    description: prompt.description,
    messages,
  };
}

/**
 * Get prompts by category
 *
 * @param category - Category name to filter by
 * @returns Array of prompt entries matching the category
 */
export function getPromptsByCategory(category: string): PromptEntry[] {
  ensureInitialized();
  if (!promptMap) throw new Error('Prompt registry not initialized');
  return Array.from(promptMap.values()).filter((prompt) => prompt.category === category);
}

/**
 * Check if a prompt exists
 *
 * @param name - Name of the prompt to check
 * @returns True if the prompt exists and registry is initialized
 */
export function hasPrompt(name: string): boolean {
  return initialized && promptMap?.has(name) === true;
}

/**
 * Get all prompt names
 *
 * @returns Array of all registered prompt names
 */
export function getPromptNames(): string[] {
  ensureInitialized();
  if (!promptMap) throw new Error('Prompt registry not initialized');
  return Array.from(promptMap.keys());
}

/**
 * Get prompt info without rendering
 */
export function getPromptInfo(
  name: string,
): { description: string; arguments: PromptArgument[] } | null {
  if (!initialized) return null;

  const prompt = promptMap?.get(name);
  return prompt
    ? {
        description: prompt.description,
        arguments: convertParameters(prompt.parameters),
      }
    : null;
}

/**
 * Clear the prompt cache and reset state
 */
export function clearPromptCache(): void {
  promptMap = null;
  logger = null;
  initialized = false;
}

// Private helper functions

/**
 * Ensure registry is initialized before operations
 */
function ensureInitialized(): void {
  if (!initialized || !promptMap) {
    throw new Error('Prompt registry not initialized. Call initializePrompts() first.');
  }
}

/**
 * Convert our parameter format to SDK PromptArgument format
 */
function convertParameters(parameters: ParameterSpec[]): PromptArgument[] {
  return parameters.map((param) => ({
    name: param.name,
    description: param.description,
    required: param.required || false,
  }));
}
