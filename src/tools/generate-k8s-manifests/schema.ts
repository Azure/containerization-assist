import { z } from 'zod';
import {
  sessionId,
  imageId,
  appName,
  path,
  namespace,
  replicas,
  port,
  serviceType,
  ingressEnabled,
  ingressHost,
  resources,
  envVars,
  configMapData,
  healthCheck,
  autoscaling,
  environment,
  samplingOptions,
} from '../shared/schemas';

export const generateK8sManifestsSchema = z.object({
  sessionId: sessionId.optional(),
  imageId: imageId
    .optional()
    .describe('Docker image to deploy (e.g., myapp:v1.0.0). Typically from build-image output.'),
  appName: appName
    .optional()
    .describe('Application name for Kubernetes resources. Defaults to image name if not provided.'),
  modules: z
    .array(
      z.object({
        name: z.string(),
        path: z.string(),
        language: z.string().optional(),
        framework: z.string().optional(),
        languageVersion: z.string().optional(),
        frameworkVersion: z.string().optional(),
        dependencies: z.array(z.string()).optional(),
        ports: z.array(z.number()).optional(),
        entryPoint: z.string().optional(),
      }),
    )
    .optional()
    .describe(
      'Array of module information for multi-module/monorepo projects. Pass the modules array from analyze-repo output to generate K8s manifests for all modules, or pass specific modules to generate only for those.',
    ),
  path: path.optional().describe('Path where the k8s folder should be created'),
  namespace,
  replicas,
  port,
  serviceType,
  ingressEnabled,
  ingressHost,
  resources,
  envVars,
  configMapData,
  healthCheck,
  autoscaling,
  environment,
  ...samplingOptions,
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;
