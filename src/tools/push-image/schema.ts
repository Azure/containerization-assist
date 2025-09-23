/**
 * Push image tool parameter validation schemas.
 * Defines the structure and validation rules for push operations.
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const pushImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
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

export type PushImageParams = z.infer<typeof pushImageSchema>;
