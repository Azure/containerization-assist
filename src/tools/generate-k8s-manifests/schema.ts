import { z } from 'zod';

export const generateK8sManifestsSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  imageId: z.string().min(1).describe('Docker image to deploy (required)'),
  appName: z.string().min(1).describe('Application name (required)'),
  path: z.string().describe('Path where the k8s folder should be created'),
  namespace: z.string().default('default').describe('Kubernetes namespace'),
  replicas: z.number().optional().describe('Number of replicas'),
  port: z.number().optional().describe('Application port'),
  serviceType: z
    .enum(['ClusterIP', 'NodePort', 'LoadBalancer'])
    .optional()
    .describe('Service type for external access'),
  ingressEnabled: z.boolean().optional().describe('Enable ingress controller'),
  ingressHost: z.string().optional().describe('Hostname for ingress routing'),
  resources: z
    .object({
      requests: z
        .object({
          memory: z.string(),
          cpu: z.string(),
        })
        .optional(),
      limits: z
        .object({
          memory: z.string(),
          cpu: z.string(),
        })
        .optional(),
    })
    .optional()
    .describe('Resource requests and limits'),
  envVars: z
    .array(
      z.object({
        name: z.string(),
        value: z.string(),
      }),
    )
    .optional()
    .describe('Environment variables to set'),
  configMapData: z.record(z.string()).optional().describe('ConfigMap data'),
  healthCheck: z
    .object({
      enabled: z.boolean(),
      path: z.string().optional(),
      port: z.number().optional(),
      initialDelaySeconds: z.number().optional(),
    })
    .optional()
    .describe('Health check configuration'),
  autoscaling: z
    .object({
      enabled: z.boolean(),
      minReplicas: z.number().optional(),
      maxReplicas: z.number().optional(),
      targetCPUUtilizationPercentage: z.number().optional(),
    })
    .optional()
    .describe('Autoscaling configuration'),
  environment: z
    .enum(['development', 'staging', 'production', 'testing'])
    .optional()
    .describe('Target environment'),

  // Sampling options
  disableSampling: z
    .boolean()
    .optional()
    .describe('Disable multi-candidate sampling (sampling is enabled by default)'),
  maxCandidates: z
    .number()
    .min(1)
    .max(10)
    .optional()
    .describe('Maximum number of candidates to generate (1-10)'),
  earlyStopThreshold: z
    .number()
    .min(0)
    .max(100)
    .optional()
    .describe('Score threshold for early stopping (0-100)'),
  includeScoreBreakdown: z
    .boolean()
    .optional()
    .describe('Include detailed score breakdown in response'),
  returnAllCandidates: z
    .boolean()
    .optional()
    .describe('Return all candidates instead of just the winner'),
  useCache: z.boolean().optional().describe('Use caching for repeated requests'),
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;
