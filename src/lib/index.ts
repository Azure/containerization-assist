/**
 * Lib Utilities Barrel File
 *
 * Clean, focused exports for pure utility functions.
 * Organized by domain for easy navigation.
 */

// Content processing utilities
export {
  extractJsonFromText,
  sanitizeResponseText,
  estimateTokenCount,
  truncatePreservingStructure,
  extractStructuredContent,
  normalizeWhitespace,
  appearsToBeCode,
} from './content-utils';

// Capability management
export {
  CapabilitySet,
  PredefinedCapabilities,
  createCapabilitySet,
  mergeCapabilities,
  canHandleTask,
  findCapableTools,
  getRecommendedCapabilities,
} from './capabilities';

// Text processing (legacy - prefer content-utils for new code)
export {
  stripFencesAndNoise,
  isValidDockerfileContent,
  extractBaseImage,
  isValidKubernetesContent,
} from './text-processing';

// Core utilities
export { createLogger } from './logger';
export { extractErrorMessage } from './error-utils';

// Async utilities
export { type Cache } from './cache';

// Regular expression patterns
export { DOCKERFILE_FENCE, YAML_FENCE, GENERIC_FENCE, AS_CLAUSE } from './regex-patterns';
