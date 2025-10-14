import { z } from 'zod';
import { environmentSchema } from '@/config/environment';

export const resolveBaseImagesSchema = z.object({
  technology: z.string().describe('Technology stack to resolve (e.g., "node", "python", "java")'),
  requirements: z
    .record(z.string(), z.unknown())
    .optional()
    .describe('Requirements for base image'),
  targetEnvironment: environmentSchema.optional(),
});

export type ResolveBaseImagesParams = z.infer<typeof resolveBaseImagesSchema>;
