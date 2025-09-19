/**
 * Consolidated Scoring Functions
 *
 * All scoring helper functions in one place.
 * This file exports functions from internal modules for backward compatibility.
 */

// Re-export all functions from internal/scoring-functions
export {
  countChainedRuns,
  hasDependencyCaching,
  countCleanupOperations,
  countParallelOperations,
  hasVersionedImages,
  hasConsistentIndentation,
  calculateContentUniqueness,
  hasDocumentation,
  countEfficientPatterns,
  hasNonRootUser,
  hasNoSecretPatterns,
  hasMinimumLines,
  hasConsistentPatterns,
  hasProperFormatting,
  hasNoGenericSecrets,
  hasNoInsecureFlags,
  hasOptimalDensity,
  hasHighUniqueness,
  hasEfficientPatterns,
  hasDescriptiveNaming,
  hasExcessivelyLongLines,
  SCORING_FUNCTIONS,
  type ScoringFunctionName,
} from './internal/scoring-functions';
