/**
 * Validation module exports - Functional API with Result<T> pattern
 */

export * from './core-types';
export type { ValidationFunction, Validator } from './pipeline';

export {
  createKubernetesValidator,
  type KubernetesValidatorInstance,
} from './kubernetes-validator';
export type {
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
  DockerfileValidationRule,
  KubernetesValidationRule,
} from './core-types';
