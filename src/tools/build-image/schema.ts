/**
 * Schema definition for build-image tool
 */

import { z } from 'zod';
import { imageName, tags, buildArgs, platform } from '../shared/schemas';

export const buildImageSchema = z.object({
  path: z
    .string()
    .optional()
    .describe('Build context path (use forward slashes: /path/to/context)'),
  dockerfile: z.string().optional().describe('Dockerfile name (relative to context)'),
  dockerfilePath: z
    .string()
    .optional()
    .describe('Path to Dockerfile (use forward slashes: /path/to/Dockerfile)'),
  imageName: imageName.optional(),
  tags: tags.optional(),
  buildArgs: buildArgs.optional(),
  platform,
  strictPlatformValidation: z
    .boolean()
    .optional()
    .default(false)
    .describe('Deprecated: No longer enforces platform flags. Reserved for future use.'),
});

export type BuildImageParams = z.infer<typeof buildImageSchema>;
