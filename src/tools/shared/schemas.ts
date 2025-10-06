/**
 * Shared Zod schemas for tool parameters
 * Common building blocks to reduce duplication across tools
 */

import { z } from 'zod';
import { environmentSchema } from '@/config/environment';

// Session management
export const sessionId = z.string().describe('Session identifier for tracking operations');

// Paths
export const path = z
  .string()
  .describe('Path to the repository or file (use forward slashes: /path/to/repo)');

// Kubernetes common fields
export const namespace = z.string().default('default').describe('Kubernetes namespace');

export const namespaceOptional = z.string().optional().describe('Kubernetes namespace');

// Unified environment schema - single source of truth
export const environment = environmentSchema.optional();

// Docker image fields
export const imageId = z.string().describe('Docker image identifier');
export const imageName = z.string().describe('Name for the Docker image');
export const tags = z.array(z.string()).describe('Tags to apply to the image');
export const buildArgs = z.record(z.string(), z.string()).describe('Build arguments');

// Application basics
export const appName = z.string().min(1).describe('Application name (required)');
export const replicas = z.number().optional().describe('Number of replicas');
export const port = z.number().optional().describe('Application port');

// Kubernetes resources
export const resourceObject = z.object({
  memory: z.string(),
  cpu: z.string(),
});

export const resourceObjectWithDefaults = z.object({
  memory: z.string().optional(),
  cpu: z.string().optional(),
});

export const resources = z
  .object({
    requests: resourceObject.optional(),
    limits: resourceObject.optional(),
  })
  .optional()
  .describe('Resource requests and limits');

export const resourcesWithDefaults = z
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
        cpu: z.string().optional().default('512m'),
      })
      .optional(),
  })
  .optional()
  .describe('Resource requests and limits');

// Environment variables
export const envVar = z.object({
  name: z.string(),
  value: z.string(),
});

export const envVars = z.array(envVar).optional().describe('Environment variables to set');

// ConfigMap
export const configMapData = z.record(z.string(), z.string()).optional().describe('ConfigMap data');

// Service types
export const serviceType = z
  .enum(['ClusterIP', 'NodePort', 'LoadBalancer'])
  .optional()
  .describe('Service type for external access');

// Ingress
export const ingressEnabled = z.boolean().optional().describe('Enable ingress controller');
export const ingressHost = z.string().optional().describe('Hostname for ingress routing');

// Health checks
export const healthCheck = z
  .object({
    enabled: z.boolean(),
    path: z.string().optional(),
    port: z.number().optional(),
    initialDelaySeconds: z.number().optional(),
  })
  .optional()
  .describe('Health check configuration');

// Autoscaling
export const autoscaling = z
  .object({
    enabled: z.boolean(),
    minReplicas: z.number().optional(),
    maxReplicas: z.number().optional(),
    targetCPUUtilizationPercentage: z.number().optional(),
  })
  .optional()
  .describe('Autoscaling configuration');

// Sampling options (used by AI-powered generation tools)
// Note: Now uses deterministic single-candidate sampling
export const samplingOptions = {
  useCache: z.boolean().optional().describe('Use caching for repeated requests'),
};

// Analysis options
export const analysisOptions = {
  depth: z.number().optional().describe('Analysis depth (1-5)'),
  includeTests: z.boolean().optional().describe('Include test files in analysis'),
  securityFocus: z.boolean().optional().describe('Focus on security aspects'),
  performanceFocus: z.boolean().optional().describe('Focus on performance aspects'),
};

// Platform
export const platform = z.string().optional().describe('Target platform (e.g., linux/amd64)');

// Multi-module/monorepo support
export const generateAllModules = z
  .boolean()
  .optional()
  .default(false)
  .describe(
    'Generate artifacts for all modules in a monorepo (requires analyze-repo session data)',
  );
