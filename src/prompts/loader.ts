/**
 * Prompt Loader
 *
 * Bridges the new manifest-based system with the existing prompt registry.
 * Handles YAML parsing and provides the same interface as the old embedded system.
 */

import yaml from 'js-yaml';
import { loadPromptText } from './locator';
import type { PromptEntry } from './prompt-registry';

/**
 * Load and parse a prompt by ID
 */
export async function loadPrompt(id: string): Promise<PromptEntry> {
  const content = await loadPromptText(id);

  try {
    const parsed = yaml.load(content) as any;

    if (!parsed || typeof parsed !== 'object') {
      throw new Error(`Invalid YAML structure in prompt: ${id}`);
    }

    // Ensure required fields exist
    const prompt: PromptEntry = {
      id: parsed.id || id,
      category: parsed.category || 'unknown',
      description: parsed.description || '',
      version: parsed.version || '1.0.0',
      format: parsed.format || 'text',
      parameters: parsed.parameters || [],
      template: parsed.template || parsed.user || content, // Fallback to raw content
      user: parsed.user,
      ttl: parsed.ttl,
      source: parsed.source,
      previousVersion: parsed.previousVersion,
      deprecated: parsed.deprecated,
      extends: parsed.extends,
    };

    return prompt;
  } catch (error) {
    throw new Error(`Failed to parse prompt ${id}: ${error}`);
  }
}

/**
 * Load multiple prompts by IDs
 */
export async function loadPrompts(ids: string[]): Promise<Record<string, PromptEntry>> {
  const results: Record<string, PromptEntry> = {};

  const loadPromises = ids.map(async (id) => {
    try {
      const prompt = await loadPrompt(id);
      results[id] = prompt;
    } catch (error) {
      console.warn(`Failed to load prompt ${id}:`, error);
    }
  });

  await Promise.all(loadPromises);
  return results;
}
