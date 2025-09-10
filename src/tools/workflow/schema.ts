import { z } from 'zod';

export const workflowSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  workflow: z.enum(['containerization', 'deployment', 'full']).describe('Workflow to execute'),
  options: z.record(z.unknown()).optional().describe('Workflow-specific options'),
});

export type WorkflowParams = z.infer<typeof workflowSchema>;
