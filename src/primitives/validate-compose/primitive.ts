import { createPolicyValidatorPrimitive } from '../policy-validator';
import { validateComposeToolDefinition } from './types';

export default createPolicyValidatorPrimitive(
  validateComposeToolDefinition,
  'validate-compose failed',
);
