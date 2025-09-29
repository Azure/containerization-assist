/**
 * Inspect session tool parameter validation schemas.
 * Provides session introspection for debugging.
 */

import { z } from 'zod';

export const InspectSessionParamsSchema = z.object({
  sessionId: z
    .string()
    .optional()
    .describe('Session ID to inspect. If not provided, lists all sessions'),
  includeSlices: z.boolean().default(false).describe('Include tool-specific slices in the output'),
  format: z.enum(['json', 'summary']).default('summary').describe('Output format'),
});

export const InspectSessionResultSchema = z.object({
  sessions: z
    .array(
      z.object({
        id: z.string(),
        createdAt: z.date(),
        updatedAt: z.date(),
        ttlRemaining: z.number().describe('TTL remaining in seconds'),
        completedSteps: z.array(z.string()),
        currentStep: z.string().nullable(),
        metadata: z.record(z.string(), z.unknown()),
        toolSlices: z.record(z.string(), z.unknown()).optional(),
        errors: z.record(z.string(), z.string()).optional(),
      }),
    )
    .optional(),
  totalSessions: z.number(),
  maxSessions: z.number(),
  message: z.string(),
});

export type InspectSessionParams = z.infer<typeof InspectSessionParamsSchema>;
export type InspectSessionResult = z.infer<typeof InspectSessionResultSchema>;
