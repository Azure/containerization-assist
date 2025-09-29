/**
 * Scoring Configuration
 *
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

export type ScoringConfigKey = keyof typeof SCORING_CONFIG;
