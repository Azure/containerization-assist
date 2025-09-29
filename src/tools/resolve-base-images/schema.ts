import { z } from 'zod';
import { environmentSchema } from '@/config/environment';

export const resolveBaseImagesSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  technology: z.string().optional().describe('Technology stack to resolve'),
  requirements: z
    .record(z.string(), z.unknown())
    .optional()
    .describe('Requirements for base image'),
  targetEnvironment: environmentSchema.optional(),
});

export type ResolveBaseImagesParams = z.infer<typeof resolveBaseImagesSchema>;
