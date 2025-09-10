/**
 * Schema definition for build-image tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const buildImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  context: z.string().optional().describe('Build context path'),
  dockerfile: z.string().optional().describe('Dockerfile name'),
  dockerfilePath: z.string().optional().describe('Path to Dockerfile'),
  imageName: z.string().optional().describe('Name for the built image'),
  tags: z.array(z.string()).optional().describe('Tags to apply to the image'),
  buildArgs: z.record(z.string()).optional().describe('Build arguments'),
  platform: z.string().optional().describe('Target platform (e.g., linux/amd64)'),
});

export type BuildImageParams = z.infer<typeof buildImageSchema>;
