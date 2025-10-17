import { z } from 'zod';

export const validateImageSchema = z.object({
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

export interface ValidateImageResult {
  success: boolean;
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
}
