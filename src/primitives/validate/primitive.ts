import { createPolicyValidatorPrimitive } from '../policy-validator';
import { validateSchema } from './schema';
import { validateToolDefinition } from './types';

export default createPolicyValidatorPrimitive(
  validateToolDefinition,
  validateSchema,
  'validate failed',
);
