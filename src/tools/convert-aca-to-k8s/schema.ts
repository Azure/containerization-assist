import { z } from 'zod';

export const convertAcaToK8sSchema = z.object({
  // Input options - one of these is required
  acaManifest: z.string().optional().describe('Azure Container Apps manifest content to convert'),
  acaPath: z.string().optional().describe('Path to Azure Container Apps manifest file'),

  // Session and output
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  outputPath: z.string().optional().describe('Path where to write the K8s manifests'),

  // Configuration
  namespace: z.string().optional().default('default').describe('Target Kubernetes namespace'),
  includeComments: z
    .boolean()
    .optional()
    .default(true)
    .describe('Add helpful comments in the output'),
  convertDapr: z
    .boolean()
    .optional()
    .default(true)
    .describe('Convert Dapr configuration to annotations'),
  ingressClass: z.string().optional().describe('Ingress controller class (nginx, traefik)'),
  storageClass: z.string().optional().describe('Storage class for persistent volumes'),
  targetCluster: z.string().optional().describe('Target cluster type (aks, generic)'),
});

export const convertAcaToK8sOutputSchema = z.object({
  manifests: z.string().describe('Generated Kubernetes YAML manifests'),
  resourceCount: z.number().describe('Number of resources generated'),
  resources: z
    .array(
      z.object({
        kind: z.string(),
        name: z.string(),
        namespace: z.string(),
      }),
    )
    .describe('List of generated resources'),
  warnings: z.array(z.string()).optional().describe('Conversion warnings if any'),
  notes: z.array(z.string()).optional().describe('Important notes about the conversion'),
});

export type ConvertAcaToK8sParams = z.infer<typeof convertAcaToK8sSchema>;
export type ConvertAcaToK8sOutput = z.infer<typeof convertAcaToK8sOutputSchema>;
