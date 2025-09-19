/**
 * Tag image tool parameter validation schemas.
 * Defines the structure and validation rules for tagging operations.
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const tagImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  imageId: z.string().optional().describe('Docker image ID to tag'),
  tag: z.string().describe('New tag to apply'),
});

export type TagImageParams = z.infer<typeof tagImageSchema>;
