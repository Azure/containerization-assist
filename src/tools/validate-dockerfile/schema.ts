import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const validateImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  path: z.string().optional().describe('Path to Dockerfile to validate'),
  dockerfile: z
    .string()
    .optional()
    .describe('Dockerfile content to validate (alternative to path)'),
  strictMode: z
    .boolean()
    .default(false)
    .describe('If true, requires at least one allowlist match when allowlist is configured'),
});

export type ValidateImageParams = z.infer<typeof validateImageSchema>;

export interface ValidateImageResult {
  success: boolean;
  sessionId?: string | undefined;
  passed: boolean;
  baseImages: Array<{
    image: string;
    line: number;
    allowed: boolean;
    denied: boolean;
    matchedAllowRule?: string | undefined;
    matchedDenyRule?: string | undefined;
  }>;
  summary: {
    totalImages: number;
    allowedImages: number;
    deniedImages: number;
    unknownImages: number;
  };
  violations: string[];
  workflowHints?: {
    nextStep: string;
    message: string;
  };
}
