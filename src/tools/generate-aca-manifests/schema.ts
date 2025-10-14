import { z } from 'zod';
import { environmentSchema } from '@/config/environment';

export const generateAcaManifestsSchema = z.object({
  // Core fields - now optional with session fallback
  appName: z
    .string()
    .optional()
    .describe('Application name. If not provided, uses name from analyze-repo session data.'),
  imageId: z
    .string()
    .optional()
    .describe(
      'Container image to deploy. If not provided, uses image from build-image session data.',
    ),

  path: z
    .string()
    .optional()
    .describe(
      'Path where the aca folder should be created. If not provided, uses path from session.',
    ),

  // Basic ACA configuration
  cpu: z
    .number()
    .optional()
    .default(0.5)
    .describe('CPU cores (0.25, 0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0)'),
  memory: z
    .string()
    .optional()
    .default('1Gi')
    .describe('Memory allocation (0.5Gi, 1Gi, 1.5Gi, 2Gi, 2.5Gi, 3Gi, 3.5Gi, 4Gi)'),
  minReplicas: z.number().optional().default(0).describe('Minimum instances (0 for serverless)'),
  maxReplicas: z.number().optional().default(10).describe('Maximum instances'),

  // Ingress configuration
  ingressEnabled: z.boolean().optional().describe('Enable ingress for external access'),
  targetPort: z.number().optional().default(8080).describe('Container port to expose'),
  external: z.boolean().optional().default(true).describe('Enable external vs internal ingress'),

  // Environment config (simplified)
  envVars: z
    .array(
      z.object({
        name: z.string(),
        value: z.string().optional(),
        secretRef: z.string().optional(),
      }),
    )
    .optional()
    .describe('Environment variables configuration'),

  // Standard deployment environment
  environment: environmentSchema.optional(),

  // Location for ACA
  location: z.string().optional().default('eastus').describe('Azure region'),
  resourceGroup: z.string().optional().describe('Azure resource group name'),
  environmentName: z.string().optional().describe('Container Apps environment name'),
});

export type GenerateAcaManifestsParams = z.infer<typeof generateAcaManifestsSchema>;
