import { createPolicyValidatorPrimitive } from '../policy-validator';
import { validateK8sManifestToolDefinition } from './types';

export default createPolicyValidatorPrimitive(
  validateK8sManifestToolDefinition,
  'validate-k8s-manifest failed',
);
