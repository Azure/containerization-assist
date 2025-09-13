/**
 * Enhanced prompt registry with simple composition support
 */
import type { Logger } from 'pino';
import { renderPromptTemplate, type PromptEntry } from './loader';
import { Result, Success, Failure } from '../types';

/**
 * Enhanced prompt registry with composition capabilities
 */
export class EnhancedPromptRegistry {
  private prompts: Map<string, PromptEntry>;
  private logger: Logger | undefined;

  constructor(prompts: Map<string, PromptEntry>, logger?: Logger) {
    this.prompts = prompts;
    this.logger = logger ? logger.child({ component: 'EnhancedPromptRegistry' }) : undefined;
  }

  /**
   * Get and render a prompt with optional enhancements
   */
  getEnhancedPrompt(
    promptId: string,
    args: Record<string, unknown>,
    enhancements?: {
      prefix?: string;
      suffix?: string;
      knowledge?: Array<{ recommendation: string; example?: string }>;
    },
  ): Result<string> {
    try {
      const prompt = this.prompts.get(promptId);
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
          const knowledgeSection = this.formatKnowledge(enhancements.knowledge);
          parts.push(knowledgeSection);
        }

        if (enhancements.suffix) {
          parts.push(enhancements.suffix);
        }

        rendered = parts.join('\n\n');
      }

      return Success(rendered);
    } catch (error) {
      this.logger?.error({ error, promptId }, 'Failed to get enhanced prompt');
      return Failure(`Failed to render prompt: ${error}`);
    }
  }

  /**
   * Compose multiple prompts into one
   */
  composePrompts(
    promptIds: string[],
    args: Record<string, unknown>,
    separator: string = '\n\n---\n\n',
  ): Result<string> {
    try {
      const sections: string[] = [];

      for (const promptId of promptIds) {
        const prompt = this.prompts.get(promptId);
        if (!prompt) {
          this.logger?.warn({ promptId }, 'Prompt not found, skipping');
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
  }

  /**
   * Format knowledge recommendations
   */
  private formatKnowledge(knowledge: Array<{ recommendation: string; example?: string }>): string {
    const lines = ['### Knowledge-Based Recommendations:'];

    for (const item of knowledge.slice(0, 5)) {
      // Limit to 5 recommendations
      lines.push(`- ${item.recommendation}`);
      if (item.example) {
        lines.push(`  Example: ${item.example}`);
      }
    }

    return lines.join('\n');
  }
}
