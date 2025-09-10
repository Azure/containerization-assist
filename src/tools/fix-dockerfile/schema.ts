import { z } from 'zod';

export const fixDockerfileSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  dockerfile: z.string().optional().describe('Dockerfile content to fix'),
  error: z.string().optional().describe('Build error message to address'),
  issues: z.array(z.string()).optional().describe('Specific issues to fix'),
});

export type FixDockerfileParams = z.infer<typeof fixDockerfileSchema>;
