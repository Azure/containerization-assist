/**
 * Push image tool parameter validation schemas.
 * Defines the structure and validation rules for push operations.
 */

import { z } from 'zod';

export const pushImageSchema = z.object({
  imageId: z.string().min(1).describe('Docker image ID to push'), // Made required
  registry: z.string().url().describe('Target registry URL'), // Made required with URL validation
  credentials: z
    .object({
      username: z.string(),
      password: z.string(),
    })
    .optional()
    .describe('Registry credentials'),
});
