import { z } from 'zod';

export const generateAcaManifestsSchema = z.object({
  // Core required fields
  appName: z.string().min(1).describe('Application name (required)'),
  imageId: z.string().min(1).describe('Container image to deploy (required)'),

  // Session tracking (standard pattern)
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  path: z.string().describe('Path where the aca folder should be created'),

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

  // Simple ingress
  ingressEnabled: z.boolean().optional().describe('Enable ingress for external access'),
  targetPort: z.number().optional().default(8080).describe('Container port to expose'),
  external: z.boolean().optional().default(true).describe('Enable external vs internal ingress'),

  /* Environment config */
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
  environment: z
    .enum(['development', 'staging', 'production'])
    .optional()
    .describe('Target environment'),

  // Location for ACA
  location: z.string().optional().default('eastus').describe('Azure region'),
  resourceGroup: z.string().optional().describe('Azure resource group name'),
  environmentName: z.string().optional().describe('Container Apps environment name'),

  // Reuse existing sampling options from generate-k8s-manifests
  disableSampling: z.boolean().optional().describe('Disable multi-candidate sampling'),
  maxCandidates: z
    .number()
    .min(1)
    .max(10)
    .optional()
    .describe('Maximum number of candidates to generate'),
  earlyStopThreshold: z
    .number()
    .min(0)
    .max(100)
    .optional()
    .describe('Score threshold for early stopping'),
  includeScoreBreakdown: z.boolean().optional().describe('Include detailed score breakdown'),
  returnAllCandidates: z
    .boolean()
    .optional()
    .describe('Return all candidates instead of just winner'),
  useCache: z.boolean().optional().describe('Use caching for repeated requests'),
});

export type GenerateAcaManifestsParams = z.infer<typeof generateAcaManifestsSchema>;
