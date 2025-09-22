/**
 * Common types for simplified AI-delegate tools
 */

/**
 * Generic AI response type for simplified tools
 * The actual structure depends on the AI's response
 */
export type AIResponse = Record<string, unknown>;

/**
 * Prompt parameters type for template functions
 * Using Record to allow flexible parameters
 */
export type PromptParams = Record<string, unknown>;
