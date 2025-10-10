import { z } from 'zod';
import {
  sessionId,
  imageId,
  appName,
  respositoryPathAbsoluteUnix,
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
        modulePath: z.string(),
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
      'Array of module information. To generate manifests for specific modules, pass only those modules in this array.',
    ),
  path: respositoryPathAbsoluteUnix.describe('Path where the k8s folder should be created'),
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
