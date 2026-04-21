/**
 * Tag image tool parameter validation schemas.
 * Defines the structure and validation rules for tagging operations.
 */

import { z } from 'zod';
import { workspacePath } from '../shared/schemas';

export const tagImageSchema = z.object({
  imageId: z.string().min(1).describe('Docker image ID to tag'), // Made required
  tag: z.string().min(1).describe('New tag to apply'), // Made required
  workspacePath: workspacePath.optional(),
});

export type TagImageParams = z.infer<typeof tagImageSchema>;
