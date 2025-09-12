import { z } from 'zod';

export const generateHelmChartsSchema = z.object({
  // Core required fields
  chartName: z.string().min(1).describe('Helm chart name (required)'),
  appName: z.string().min(1).describe('Application name (required)'),
  imageId: z.string().min(1).describe('Container image to deploy (required)'),

  // Session tracking (standard pattern)
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),

  // Basic Helm configuration
  chartVersion: z.string().optional().default('0.1.0').describe('Chart version'),
  appVersion: z.string().optional().default('1.0.0').describe('Application version'),
  description: z.string().optional().describe('Chart description'),

  // Deployment configuration (reuse from K8s)
  replicas: z.number().optional().default(1).describe('Number of replicas'),
  port: z.number().optional().default(8080).describe('Application port'),
  serviceType: z
    .enum(['ClusterIP', 'NodePort', 'LoadBalancer'])
    .optional()
    .default('ClusterIP')
    .describe('Service type'),

  // Ingress
  ingressEnabled: z.boolean().optional().describe('Enable ingress'),
  ingressHost: z.string().optional().describe('Ingress hostname'),
  ingressClass: z.string().optional().default('nginx').describe('Ingress class'),

  // Resources
  resources: z
    .object({
      requests: z
        .object({
          memory: z.string().optional().default('128Mi'),
          cpu: z.string().optional().default('100m'),
        })
        .optional(),
      limits: z
        .object({
          memory: z.string().optional().default('256Mi'),
          cpu: z.string().optional().default('200m'),
        })
        .optional(),
    })
    .optional()
    .describe('Resource requests and limits'),

  // Health checks
  healthCheck: z
    .object({
      enabled: z.boolean().optional().default(true),
      path: z.string().optional().default('/health'),
      port: z.number().optional(),
      initialDelaySeconds: z.number().optional().default(30),
    })
    .optional()
    .describe('Health check configuration'),

  // Autoscaling
  autoscaling: z
    .object({
      enabled: z.boolean().optional().default(false),
      minReplicas: z.number().optional().default(1),
      maxReplicas: z.number().optional().default(10),
      targetCPUUtilizationPercentage: z.number().optional().default(70),
    })
    .optional()
    .describe('HPA configuration'),

  // Environment
  environment: z
    .enum(['development', 'staging', 'production'])
    .optional()
    .describe('Target environment'),

  // Validation options
  runValidation: z.boolean().optional().default(true).describe('Run helm lint validation'),
  strictValidation: z.boolean().optional().default(false).describe('Fail on warnings'),

  // Sampling options (reuse existing pattern)
  disableSampling: z.boolean().optional().describe('Disable multi-candidate sampling'),
  maxCandidates: z.number().min(1).max(10).optional().describe('Maximum candidates'),
  earlyStopThreshold: z.number().min(0).max(100).optional().describe('Early stop threshold'),
  includeScoreBreakdown: z.boolean().optional().describe('Include score breakdown'),
  returnAllCandidates: z.boolean().optional().describe('Return all candidates'),
  useCache: z.boolean().optional().describe('Use caching'),
});

export type GenerateHelmChartsParams = z.infer<typeof generateHelmChartsSchema>;
