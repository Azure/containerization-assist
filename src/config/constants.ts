/**
 * Configuration Constants
 *
 * Consolidated constants for sampling, scoring, and token limits.
 * Merges sampling.ts, scoring.ts, and tokens.ts into a single source of truth.
 */

/**
 * AI Sampling Configuration
 * Limits and operation parameters for deterministic single-candidate sampling.
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

/**
 * Scoring Configuration
 * Centralized scoring thresholds, grading boundaries, and severity weights
 * for quality assessment and AI operations.
 */
export const SCORING_CONFIG = {
  /** Scoring scale maximum value for normalization */
  SCALE: 100,
  THRESHOLDS: {
    /** Fast/minimal quality threshold - 75/100 */
    FAST: 75,
    /** Standard quality threshold - 85/100 */
    STANDARD: 85,
    /** High quality threshold - 88/100 */
    HIGH_QUALITY: 88,
    /** Excellent quality threshold - 95/100 */
    EXCELLENT: 95,
    /** Perfect score baseline */
    PERFECT: 100,
  },
  /** Score grading boundaries */
  GRADES: {
    A: 90,
    B: 80,
    C: 70,
    D: 60,
    /** Below 60 is grade F */
  },
  /** Severity weight multipliers for score calculations */
  SEVERITY_WEIGHTS: {
    ERROR: 10,
    WARNING: 3,
    INFO: 1,
  },
  CONFIDENCE: {
    /** Minimum confidence for AI operations */
    MIN: 0.0,
    /** Maximum confidence for AI operations */
    MAX: 1.0,
    /** Default confidence threshold */
    DEFAULT: 0.7,
    /** High confidence threshold */
    HIGH: 0.8,
    /** Very high confidence threshold */
    VERY_HIGH: 0.9,
  },
  QUALITY: {
    /** Minimum substantial analysis length */
    MIN_SUBSTANTIAL_LENGTH: 200,
    /** Preview text truncation length */
    PREVIEW_LENGTH: 160,
    /** Minimum reasonable content length */
    MIN_REASONABLE_LENGTH: 50,
    /** Minimum lines for multi-line structure */
    MIN_MULTILINE_STRUCTURE: 5,
  },
} as const;

/**
 * Token Configuration
 * Token limits for different operation types and contexts.
 */
export const TOKEN_CONFIG = {
  /** Standard token limit - 4096 */
  STANDARD: 4096,
  /** Extended token limit - 6144 */
  EXTENDED: 6144,
  /** Repair operation token limit - 256 */
  REPAIR: 256,
  /** Large operation token limit - 8192 */
  LARGE: 8192,
} as const;

export type SamplingConfigKey = keyof typeof SAMPLING_CONFIG;
export type ScoringConfigKey = keyof typeof SCORING_CONFIG;
export type TokenConfigKey = keyof typeof TOKEN_CONFIG;
