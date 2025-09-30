/**
 * Sampling Configuration
 *
 * AI sampling limits and operation parameters for deterministic single-candidate sampling.
 */

export const SAMPLING_CONFIG = {
  LIMITS: {
    /** Maximum number of suggestions to return */
    MAX_SUGGESTIONS: 10,
    /** Maximum number of enhancement areas */
    MAX_ENHANCEMENT_AREAS: 5,
    /** Maximum number of knowledge sources */
    MAX_KNOWLEDGE_SOURCES: 8,
    /** Knowledge budget multiplier for token calculation */
    KNOWLEDGE_BUDGET_MULTIPLIER: 750,
    /** Maximum number of best practices */
    MAX_BEST_PRACTICES: 10,
    /** Maximum number of priorities */
    MAX_PRIORITIES: 5,
    /** Maximum number of technical debt items */
    MAX_TECHNICAL_DEBT: 5,
    /** Maximum number of base image recommendations */
    MAX_BASE_IMAGE_RECOMMENDATIONS: 5,
  },
  PRIORITIES: {
    /** Intelligence/quality optimization priority */
    INTELLIGENCE: 0.9,
    /** Cost optimization priority */
    COST: 0.2,
    /** Speed optimization priority (moderate) */
    SPEED: 0.3,
  },
} as const;

export type SamplingConfigKey = keyof typeof SAMPLING_CONFIG;
