import { z } from 'zod';
import { sessionId, environment, samplingOptions } from '../shared/schemas';

export const fixDockerfileSchema = z.object({
  sessionId: sessionId.optional(),
  dockerfile: z.string().optional().describe('Dockerfile content to validate and fix'),
  path: z.string().optional().describe('Path to Dockerfile file to validate and fix'),
  error: z.string().optional().describe('Build error message to address'),
  issues: z.array(z.string()).optional().describe('Specific issues to fix'),
  requirements: z.string().optional().describe('Additional requirements for optimization'),
  targetEnvironment: environment.describe('Target environment'),
  ...samplingOptions,
});

export type FixDockerfileParams = z.infer<typeof fixDockerfileSchema>;
