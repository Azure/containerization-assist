/**
 * Enhanced prompt registry with simple composition support (Functional)
 *
 * Invariant: Prompt templates maintain consistent variable interpolation format
 * Trade-off: Runtime template rendering over compile-time validation for flexibility
 */
import type { Logger } from 'pino';
import { renderPromptTemplate, type PromptEntry } from './loader';
import { Result, Success, Failure } from '@types';

/**
 * Enhancement options for prompt rendering
 */
export interface PromptEnhancements {
  prefix?: string;
  suffix?: string;
  knowledge?: Array<{ recommendation: string; example?: string }>;
}

/**
 * Registry state for prompt management
 */
export interface PromptRegistryState {
  prompts: Map<string, PromptEntry>;
  logger: Logger | undefined;
}

/**
 * Format knowledge recommendations into markdown
 */
const formatKnowledge = (
  knowledge: Array<{ recommendation: string; example?: string }>,
): string => {
  const lines = ['### Knowledge-Based Recommendations:'];

  for (const item of knowledge.slice(0, 5)) {
    // Limit to 5 recommendations
    lines.push(`- ${item.recommendation}`);
    if (item.example) {
      lines.push(`  Example: ${item.example}`);
    }
  }

  return lines.join('\n');
};

/**
 * Get and render a prompt with optional enhancements
 */
export const getEnhancedPrompt = (
  state: PromptRegistryState,
  promptId: string,
  args: Record<string, unknown>,
  enhancements?: PromptEnhancements,
): Result<string> => {
  try {
    const prompt = state.prompts.get(promptId);
    if (!prompt) {
      return Failure(`Prompt not found: ${promptId}`);
    }

    // Render base prompt
    let rendered = renderPromptTemplate(prompt.template, args);

    // Add enhancements
    if (enhancements) {
      const parts: string[] = [];

      if (enhancements.prefix) {
        parts.push(enhancements.prefix);
      }

      parts.push(rendered);

      if (enhancements.knowledge && enhancements.knowledge.length > 0) {
        const knowledgeSection = formatKnowledge(enhancements.knowledge);
        parts.push(knowledgeSection);
      }

      if (enhancements.suffix) {
        parts.push(enhancements.suffix);
      }

      rendered = parts.join('\n\n');
    }

    return Success(rendered);
  } catch (error) {
    state.logger?.error({ error, promptId }, 'Failed to get enhanced prompt');
    return Failure(`Failed to render prompt: ${error}`);
  }
};

/**
 * Compose multiple prompts into one
 */
export const composePrompts = (
  state: PromptRegistryState,
  promptIds: string[],
  args: Record<string, unknown>,
  separator: string = '\n\n---\n\n',
): Result<string> => {
  try {
    const sections: string[] = [];

    for (const promptId of promptIds) {
      const prompt = state.prompts.get(promptId);
      if (!prompt) {
        state.logger?.warn({ promptId }, 'Prompt not found, skipping');
        continue;
      }

      const rendered = renderPromptTemplate(prompt.template, args);
      sections.push(rendered);
    }

    if (sections.length === 0) {
      return Failure('No valid prompts found');
    }

    return Success(sections.join(separator));
  } catch (error) {
    return Failure(`Failed to compose prompts: ${error}`);
  }
};

/**
 * Create an enhanced prompt registry factory
 */
export const createEnhancedPromptRegistry = (
  prompts: Map<string, PromptEntry>,
  logger?: Logger,
): {
  getEnhancedPrompt: (
    promptId: string,
    args: Record<string, unknown>,
    enhancements?: PromptEnhancements,
  ) => Result<string>;
  composePrompts: (
    promptIds: string[],
    args: Record<string, unknown>,
    separator?: string,
  ) => Result<string>;
  getPrompts: () => Map<string, PromptEntry>;
} => {
  const state: PromptRegistryState = {
    prompts,
    logger: logger ? logger.child({ component: 'EnhancedPromptRegistry' }) : undefined,
  };

  return {
    getEnhancedPrompt: (
      promptId: string,
      args: Record<string, unknown>,
      enhancements?: PromptEnhancements,
    ) => getEnhancedPrompt(state, promptId, args, enhancements),
    composePrompts: (promptIds: string[], args: Record<string, unknown>, separator?: string) =>
      composePrompts(state, promptIds, args, separator),
    getPrompts: () => new Map(state.prompts),
  };
};
