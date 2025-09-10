/**
 * Core Prompts Module
 *
 * Exports the prompt registry and related types for managing
 * statically imported JSON prompt templates.
 */

export { PromptRegistry } from './registry';
export { StaticPromptLoader, type PromptFile, type ParameterSpec } from './static-loader';
