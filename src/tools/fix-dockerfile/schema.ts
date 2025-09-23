import { z } from 'zod';
import { sessionId, environmentFull, samplingOptions } from '../shared/schemas';

export const fixDockerfileSchema = z.object({
  sessionId: sessionId.optional(),
  dockerfile: z.string().optional().describe('Dockerfile content to fix'),
  error: z.string().optional().describe('Build error message to address'),
  issues: z.array(z.string()).optional().describe('Specific issues to fix'),
  targetEnvironment: environmentFull.describe('Target environment'),
  ...samplingOptions,
});

export type FixDockerfileParams = z.infer<typeof fixDockerfileSchema>;
