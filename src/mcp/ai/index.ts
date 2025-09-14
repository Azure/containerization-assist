/**
 * AI Module - Exports for AI assistance functionality
 */

export {
  // Host AI Assistant
  type AIParamRequest,
  type AIParamResponse,
  type HostAIAssistant,
  type AIAssistantConfig,
  DefaultHostAIAssistant,
  createHostAIAssistant,
  mergeWithSuggestions,
} from './host-ai-assist';

export {
  // Prompt Builder
  AIPromptBuilder,
} from './prompt-builder';

export {
  // Default Suggestions
  type SuggestionGenerator,
  SuggestionRegistry,
  createSuggestionRegistry,
  DEFAULT_SUGGESTION_GENERATORS,
} from './default-suggestions';

export {
  // Prompt Templates
  PROMPT_TEMPLATES,
  type PromptTemplate,
  type PromptFormat,
  type PromptTemplateConfig,
  getTemplate,
  formatInstructions,
  PROMPT_MODIFIERS,
  type PromptModifier,
  applyModifiers,
  PROMPT_PREFIXES,
  buildPromptHeader,
} from './prompt-templates';
