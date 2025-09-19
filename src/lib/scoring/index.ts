/**
 * Public Scoring API
 *
 * Single entry point for all scoring functionality.
 * Internal implementations are kept in ./internal/ directory.
 */

export {
  // Main scoring functions
  scoreConfigCandidates,
  getConfigStrategies,
  getFormattedConfigStrategy,

  // Configuration initialization (now stateless)
  createScoringEngine,

  // Types
  type ScoringResult,
  type ScoringEngine,
  type SamplingCandidate,
} from './integrated';

// Re-export scoring helper functions for backward compatibility
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
} from './functions';
