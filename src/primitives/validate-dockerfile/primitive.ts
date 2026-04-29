import { createPolicyValidatorPrimitive } from '../policy-validator';
import { validateDockerfileToolDefinition } from './types';

export default createPolicyValidatorPrimitive(
  validateDockerfileToolDefinition,
  'validate-dockerfile failed',
);
