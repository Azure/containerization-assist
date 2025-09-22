/**
 * Schema definition for scan tool
 */

import { z } from 'zod';

const sessionIdSchema = z.string().describe('Session identifier for tracking operations');

export const scanImageSchema = z.object({
  sessionId: sessionIdSchema.optional(),
  imageId: z.string().optional().describe('Docker image ID or name to scan'),
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
});

export type ScanImageParams = z.infer<typeof scanImageSchema>;
