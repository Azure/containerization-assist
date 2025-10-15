/**
 * Schema definition for scan tool
 */

import { z } from 'zod';

export const scanImageSchema = z.object({
  imageId: z.string().describe('Docker image ID or name to scan'),
  severity: z
    .union([
      z.enum(['LOW', 'MEDIUM', 'HIGH', 'CRITICAL']),
      z.enum(['low', 'medium', 'high', 'critical']),
    ])
    .optional()
    .describe('Minimum severity to report'),
  scanType: z
    .enum(['vulnerability', 'config', 'all'])
    .default('vulnerability') // Added default
    .describe('Type of scan to perform'),
  scanner: z
    .enum(['trivy', 'snyk', 'grype'])
    .default('trivy') // Added default
    .describe('Scanner to use for vulnerability detection'),
  enableAISuggestions: z
    .boolean()
    .default(true)
    .describe('Enable AI-powered suggestions for vulnerability remediation'),
  aiEnhancementOptions: z
    .object({
      mode: z.enum(['suggestions', 'fixes', 'analysis']).default('suggestions'),
      focus: z.enum(['security', 'performance', 'best-practices', 'all']).default('security'),
      confidence: z.number().min(0).max(1).default(0.8),
      maxSuggestions: z.number().min(1).max(10).default(5),
      includeExamples: z.boolean().default(true),
    })
    .optional()
    .describe('Configuration for AI-powered enhancement'),
});

export type ScanImageParams = z.infer<typeof scanImageSchema>;
