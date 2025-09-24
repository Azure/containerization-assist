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
  environmentFull,
  samplingOptions,
} from '../shared/schemas';

export const generateK8sManifestsSchema = z.object({
  sessionId: sessionId.optional(),
  imageId: imageId.min(1).describe('Docker image to deploy (required)'),
  appName,
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
  environment: environmentFull,
  ...samplingOptions,
});

export type GenerateK8sManifestsParams = z.infer<typeof generateK8sManifestsSchema>;
