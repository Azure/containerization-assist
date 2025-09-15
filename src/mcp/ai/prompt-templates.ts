/**
 * Prompt Templates - Standardized templates for AI prompts
 */

/**
 * Template configuration for different prompt types
 */
export const PROMPT_TEMPLATES = {
  parameterSuggestion: {
    instruction: 'Return JSON object with suggested parameter values.',
    example: '{"path": ".", "imageId": "app:latest"}',
    format: 'json' as const,
    maxTokens: 2048,
  },

  contextAnalysis: {
    instruction: 'Analyze the context and suggest appropriate values.',
    format: 'structured' as const,
    maxTokens: 4096,
  },

  errorCorrection: {
    instruction: 'Analyze the error and suggest corrections.',
    example: 'Suggest specific fixes for the identified issues.',
    format: 'markdown' as const,
    maxTokens: 3072,
  },

  workflowGuidance: {
    instruction: 'Provide guidance for the next steps in the workflow.',
    format: 'structured' as const,
    maxTokens: 2048,
  },

  validation: {
    instruction: 'Validate the parameters and identify any issues.',
    format: 'json' as const,
    maxTokens: 1024,
  },
} as const;

/**
 * Type for available prompt templates
 */
export type PromptTemplate = keyof typeof PROMPT_TEMPLATES;

/**
 * Type for prompt format options
 */
export type PromptFormat = 'json' | 'structured' | 'markdown' | 'plain';

/**
 * Configuration for a prompt template
 */
export interface PromptTemplateConfig {
  instruction: string;
  example?: string;
  format: PromptFormat;
  maxTokens: number;
}

/**
 * Helper to get template configuration
 */
export function getTemplate(name: PromptTemplate): PromptTemplateConfig {
  return PROMPT_TEMPLATES[name];
}

/**
 * Helper to format instructions based on template
 */
export function formatInstructions(template: PromptTemplate, additionalContext?: string): string[] {
  const config = PROMPT_TEMPLATES[template];
  const instructions: string[] = [];

  if (additionalContext) {
    instructions.push(additionalContext);
  }

  instructions.push(config.instruction);

  if ('example' in config && config.example) {
    instructions.push(`Example: ${config.example}`);
  }

  if (config.format === 'json') {
    instructions.push('Respond with valid JSON only.');
  } else if (config.format === 'structured') {
    instructions.push('Provide a structured response with clear sections.');
  } else if (config.format === 'markdown') {
    instructions.push('Format response using Markdown.');
  }

  return instructions;
}

/**
 * Prompt modifiers for fine-tuning responses
 */
export const PROMPT_MODIFIERS = {
  concise: 'Be concise and direct.',
  detailed: 'Provide detailed explanations.',
  technical: 'Use technical terminology and be precise.',
  simple: 'Use simple language and avoid jargon.',
  stepByStep: 'Provide step-by-step instructions.',
  examples: 'Include practical examples.',
} as const;

/**
 * Type for available prompt modifiers
 */
export type PromptModifier = keyof typeof PROMPT_MODIFIERS;

/**
 * Apply modifiers to a prompt
 */
export function applyModifiers(instructions: string[], modifiers: PromptModifier[]): string[] {
  const modifierInstructions = modifiers.map((m) => PROMPT_MODIFIERS[m]);
  return [...instructions, ...modifierInstructions];
}

/**
 * Common prompt prefixes for consistency
 */
export const PROMPT_PREFIXES = {
  tool: (name: string) => `Tool: ${name}`,
  context: 'Context:',
  parameters: 'Parameters:',
  missing: 'Missing:',
  error: 'Error:',
  objective: 'Objective:',
  current: 'Current:',
  expected: 'Expected:',
} as const;

/**
 * Build a consistent prompt header
 */
export function buildPromptHeader(sections: Array<{ label: string; value: unknown }>): string[] {
  return sections
    .filter((s) => s.value !== undefined && s.value !== null)
    .map((s) => {
      const formatted = typeof s.value === 'string' ? s.value : JSON.stringify(s.value, null, 2);
      return `${s.label}: ${formatted}`;
    });
}
