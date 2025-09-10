import { z } from 'zod';

export const generateK8sManifestsSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  imageId: z.string().optional().describe('Docker image to deploy'),
  appName: z.string().optional().describe('Application name'),
  namespace: z.string().optional().describe('Kubernetes namespace (defaults to "default")'),
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
    .enum(['development', 'staging', 'production'])
    .optional()
    .describe('Target environment'),
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;
