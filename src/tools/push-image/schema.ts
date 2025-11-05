/**
 * Push image tool parameter validation schemas.
 * Defines the structure and validation rules for push operations.
 */

import { z } from 'zod';
import { platform } from '../shared/schemas';

export const pushImageSchema = z.object({
  imageId: z.string().min(1).describe('Docker image ID or name to push'),
  registry: z.string().min(1).describe('Target registry hostname (e.g., myregistry.azurecr.io, docker.io)'),
  platform,
  credentials: z
    .object({
      username: z.string(),
      password: z.string(),
    })
    .optional()
    .describe('Registry credentials. If not provided, will attempt to use Docker credential helpers'),
});
