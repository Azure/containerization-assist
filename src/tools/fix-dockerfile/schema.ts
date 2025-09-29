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

  // New mode flags for enhanced validation and fixing
  mode: z
    .enum(['lint', 'autofix', 'format', 'full'])
    .default('full')
    .describe(
      'Validation/fix mode: lint=validate only, autofix=apply fixes without AI, format=format only, full=complete pipeline',
    ),
  enableExternalLinter: z
    .boolean()
    .default(true)
    .describe('Enable external dockerfilelint integration for enhanced rule coverage'),
  returnDiff: z
    .boolean()
    .default(false)
    .describe('Return unified diff between original and fixed content'),
  outputFormat: z
    .enum(['json', 'text', 'sarif'])
    .default('json')
    .describe('Output format for validation results'),

  ...samplingOptions,
});
