/**
 * Sampling Configuration
 *
 * AI sampling strategies, candidate counts, and operation limits.
 */

export const SAMPLING_CONFIG = {
  CANDIDATES: {
    /** Single candidate for fast operations */
    FAST: 1,
    /** Balanced sampling - 3 candidates */
    BALANCED: 3,
    /** Thorough sampling - 5 candidates */
    THOROUGH: 5,
    /** Exhaustive sampling - 8 candidates */
    EXHAUSTIVE: 8,
  },
  /** Default parameters for different sampling strategies */
  DEFAULTS: {
    FAST: {
      candidates: 1,
      stopAt: 75,
      maxTokens: 4096,
    },
    BALANCED: {
      candidates: 3,
      stopAt: 85,
      maxTokens: 4096,
    },
    THOROUGH: {
      candidates: 5,
      stopAt: 88,
      maxTokens: 6144,
    },
  },
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
