/**
 * Validation module exports - Functional API with Result<T> pattern
 */

export * from './core-types';
export type { ValidationFunction, Validator } from './pipeline';
export {
  validateDockerfileContent,
  validateDockerfileContentWithKnowledge,
  type DockerfileValidationKnowledgeOptions,
} from './dockerfile-validator';

export {
  createKubernetesValidator,
  validateKubernetes as validateKubernetesManifests,
  validateKubernetesManifestsWithKnowledge,
  type KubernetesValidatorInstance,
  type KubernetesValidationKnowledgeOptions,
} from './kubernetes-validator';

// AI Enhancement exports
export {
  enhanceValidationWithAI,
  summarizeValidationResults,
  type EnhancementOptions,
  type AIEnhancementResult,
} from './ai-enhancement';

// Knowledge Enhancement helper exports
export {
  enhanceContentIfNeeded,
  createKnowledgeEnhancementMetadata,
  createWorkflowHintsWithKnowledge,
  shouldApplyKnowledgeEnhancement,
  mergeKnowledgeSuggestions,
  createKnowledgeEnhancementError,
  safelyEnhanceContent,
  hasKnowledgeEnhancement,
  extractKnowledgeStats,
} from './knowledge-helpers';

import { validateDockerfileContent } from './dockerfile-validator';
export const validateDockerfile = validateDockerfileContent;
export type {
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
  DockerfileValidationRule,
  KubernetesValidationRule,
} from './core-types';
