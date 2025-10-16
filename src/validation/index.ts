/**
 * Validation module exports - Functional API with Result<T> pattern
 */

export * from './core-types';
export type { ValidationFunction, Validator } from './pipeline';
export { validateDockerfileContent } from './dockerfile-validator';

export {
  createKubernetesValidator,
  validateKubernetes as validateKubernetesManifests,
  type KubernetesValidatorInstance,
} from './kubernetes-validator';

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
