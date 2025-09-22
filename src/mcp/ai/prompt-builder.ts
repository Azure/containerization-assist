/**
 * AI Prompt Builder - Functional utilities for constructing well-formatted prompts
 */

import type { AIParamRequest } from './host-ai-assist';

/**
 * Prompt section data structure
 */
interface PromptSection {
  type: 'section' | 'instruction' | 'separator';
  title?: string;
  content?: unknown;
  instruction?: string;
}

/**
 * Add a titled section to the prompt
 */
export const addSection =
  (title: string, content: unknown) =>
  (sections: PromptSection[]): PromptSection[] => {
    if (content !== undefined && content !== null) {
      return [...sections, { type: 'section', title, content }];
    }
    return sections;
  };

/**
 * Add a plain instruction line to the prompt
 */
export const addInstruction =
  (instruction: string) =>
  (sections: PromptSection[]): PromptSection[] => {
    return [...sections, { type: 'instruction', instruction }];
  };

/**
 * Add a blank line separator
 */
export const addSeparator =
  () =>
  (sections: PromptSection[]): PromptSection[] => {
    return [...sections, { type: 'separator' }];
  };

/**
 * Build the final prompt string from sections
 */
export const buildPrompt = (sections: PromptSection[]): string => {
  return sections
    .map((section) => {
      switch (section.type) {
        case 'section': {
          const formatted =
            typeof section.content === 'string'
              ? section.content
              : JSON.stringify(section.content, null, 2);
          return `${section.title}: ${formatted}`;
        }
        case 'instruction':
          return section.instruction;
        case 'separator':
          return '';
        default:
          return '';
      }
    })
    .join('\n');
};

/**
 * Functional composition helper for building prompts
 */
export const pipe =
  <T>(...fns: Array<(arg: T) => T>) =>
  (initial: T): T => {
    return fns.reduce((acc, fn) => fn(acc), initial);
  };

/**
 * Create a prompt for parameter suggestion (functional)
 */
export const createParameterSuggestionPrompt = (request: AIParamRequest): string => {
  const builder = pipe(
    addSection('Tool', request.toolName),
    addSection('Current', request.currentParams),
    addSection('Missing', request.missingParams.join(', ')),
    addSection('Schema', request.schema),
    addSection('Context', request.sessionContext),
    addSeparator(),
    addInstruction('Return JSON object with suggested parameter values.'),
    addInstruction('Example: {"path": ".", "imageId": "app:latest"}'),
  );

  return buildPrompt(builder([]));
};

/**
 * Create a prompt for context analysis (functional)
 */
export const createContextAnalysisPrompt = (
  context: Record<string, unknown>,
  objective: string,
): string => {
  const builder = pipe(
    addSection('Objective', objective),
    addSection('Context', context),
    addSeparator(),
    addInstruction('Analyze the context and provide insights.'),
  );

  return buildPrompt(builder([]));
};
