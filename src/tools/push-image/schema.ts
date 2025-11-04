/**
 * Push image tool parameter validation schemas.
 * Defines the structure and validation rules for push operations.
 */

import { z } from 'zod';
import { platform } from '../shared/schemas';

export const pushImageSchema = z.object({
  imageId: z.string().min(1).describe('Docker image ID or name to push including the registry and tag (e.g., myregistry.azurecr.io/myapp:latest)'),
  platform,
});
