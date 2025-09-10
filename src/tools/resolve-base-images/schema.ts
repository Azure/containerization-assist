import { z } from 'zod';

export const resolveBaseImagesSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  technology: z.string().optional().describe('Technology stack to resolve'),
  requirements: z.record(z.unknown()).optional().describe('Requirements for base image'),
});

export type ResolveBaseImagesParams = z.infer<typeof resolveBaseImagesSchema>;
