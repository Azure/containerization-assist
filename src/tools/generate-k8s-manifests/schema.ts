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
    .describe('Docker image to deploy. If not provided, uses image from build-image session data.'),
  appName: appName
    .optional()
    .describe('Application name. If not provided, uses name from analyze-repo session data.'),
  moduleName: z
    .string()
    .optional()
    .describe(
      'Specific module name to generate manifests for (use with monorepo/multi-module projects). If not specified and modules are detected in session, generates for the first module or prompts for selection.',
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
