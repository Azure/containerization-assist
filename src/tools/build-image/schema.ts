/**
 * Schema definition for build-image tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const buildImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  context: z.string().min(1).describe('Build context path (use forward slashes: /path/to/context)'),
  dockerfile: z.string().optional().describe('Dockerfile name (relative to context)'),
  dockerfilePath: z
    .string()
    .optional()
    .describe('Path to Dockerfile (use forward slashes: /path/to/Dockerfile)'),
  imageName: z.string().min(1).describe('Name for the built image (required)'),
  tags: z.array(z.string()).optional().describe('Tags to apply to the image'),
  buildArgs: z.record(z.string()).optional().describe('Build arguments'),
  platform: z.string().optional().describe('Target platform (e.g., linux/amd64)'),
});

export type BuildImageParams = z.infer<typeof buildImageSchema>;
