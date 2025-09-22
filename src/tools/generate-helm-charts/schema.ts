import { z } from 'zod';
import {
  sessionId,
  imageId,
  appName,
  replicas,
  port,
  serviceType,
  ingressEnabled,
  ingressHost,
  environmentBasic,
  samplingOptions,
} from '../shared/schemas';

export const generateHelmChartsSchema = z.object({
  // Core required fields
  chartName: z.string().min(1).describe('Helm chart name (required)'),
  appName,
  imageId: imageId.min(1).describe('Container image to deploy (required)'),

  // Session tracking (standard pattern)
  sessionId: sessionId.optional(),

  // Basic Helm configuration
  chartVersion: z.string().optional().default('0.1.0').describe('Chart version'),
  appVersion: z.string().optional().default('1.0.0').describe('Application version'),
  description: z.string().optional().describe('Chart description'),

  // Deployment configuration (reuse from K8s)
  replicas: replicas ?? z.number().optional().default(1).describe('Number of replicas'),
  port: port ?? z.number().optional().default(8080).describe('Application port'),
  serviceType:
    serviceType ??
    z
      .enum(['ClusterIP', 'NodePort', 'LoadBalancer'])
      .optional()
      .default('ClusterIP')
      .describe('Service type'),

  // Ingress
  ingressEnabled,
  ingressHost,
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

  // Health checks (using shared but with defaults)
  healthCheck: z
    .object({
      enabled: z.boolean().optional().default(true),
      path: z.string().optional().default('/health'),
      port: z.number().optional(),
      initialDelaySeconds: z.number().optional().default(30),
    })
    .optional()
    .describe('Health check configuration'),

  // Autoscaling (using shared but with defaults)
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
  environment: environmentBasic,

  // Validation options
  runValidation: z.boolean().optional().default(true).describe('Run helm lint validation'),
  strictValidation: z.boolean().optional().default(false).describe('Fail on warnings'),

  // Sampling options
  ...samplingOptions,
});

export type GenerateHelmChartsParams = z.infer<typeof generateHelmChartsSchema>;
