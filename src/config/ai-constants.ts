/**
 * AI Configuration Constants
 *
 * Centralized configuration for all AI services including scoring thresholds,
 * token limits, retry settings, and sampling parameters.
 */

export const AI_CONFIG = {
  SCORING: {
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
  },
  TOKENS: {
    /** Standard token limit - 4096 */
    STANDARD: 4096,
    /** Extended token limit - 6144 */
    EXTENDED: 6144,
    /** Repair operation token limit - 256 */
    REPAIR: 256,
    /** Large operation token limit - 8192 */
    LARGE: 8192,
  },
  RETRY: {
    /** Maximum retry attempts */
    MAX_ATTEMPTS: 3,
    /** Base delay in milliseconds */
    BASE_DELAY_MS: 1000,
    /** Maximum delay in milliseconds */
    MAX_DELAY_MS: 8000,
    /** Exponential backoff base multiplier */
    EXPONENTIAL_BASE: 2,
  },
  SAMPLING: {
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

export type AIConfigKey = keyof typeof AI_CONFIG;

/**
 * Validation scoring configuration for different severity types
 */
export const VALIDATION_SCORING = {
  /** K8s schema validation scoring multipliers */
  K8S_SCHEMA: {
    ERROR_WEIGHT: 25,
    WARNING_WEIGHT: 8,
    INFO_WEIGHT: 2,
  },
  /** Dockerfile linting scoring multipliers */
  DOCKERFILE: {
    ERROR_WEIGHT: 10,
    WARNING_WEIGHT: 3,
    INFO_WEIGHT: 1,
  },
  /** Default merge report scores */
  DEFAULT_SCORES: {
    PERFECT: 100,
    ZERO: 0,
  },
} as const;

/**
 * User ID constants for containerization
 */
export const USER_IDS = {
  /** Standard non-root user ID */
  STANDARD: 1001,
  /** Alternative user ID for compatibility */
  ALTERNATIVE: 1000,
} as const;

/**
 * Kubernetes defaults
 */
export const K8S_DEFAULTS = {
  PROBES: {
    READINESS: {
      initialDelaySeconds: 5,
      periodSeconds: 5,
    },
    LIVENESS: {
      initialDelaySeconds: 30,
      periodSeconds: 10,
    },
  },
  SECURITY: {
    fsGroup: 1000,
  },
} as const;

/**
 * Scoring component weights for different quality aspects
 */
export const SCORING_WEIGHTS = {
  VALIDATION: {
    FORMAT: 25,
    COMPLETENESS: 20,
    SPECIFICITY: 20,
    ACCURACY: 20,
    RELEVANCE: 15,
  },
  KNOWLEDGE: {
    STRUCTURE: 20,
    ENHANCEMENT: 25,
    KNOWLEDGE: 20,
    COMPLETENESS: 20,
    RELEVANCE: 15,
  },
  ENHANCEMENT: {
    COMPLETENESS: 25,
    RELEVANCE: 25,
    PRACTICALITY: 20,
    CLARITY: 15,
    SPECIFICITY: 15,
  },
} as const;
