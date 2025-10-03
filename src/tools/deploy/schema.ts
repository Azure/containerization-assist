/**
 * Schema definition for deploy tool
 */

import { z } from 'zod';
import { sessionId, namespaceOptional, replicas, port, environment } from '../shared/schemas';

export const deployApplicationSchema = z.object({
  sessionId: sessionId.optional(),
  imageId: z.string().optional().describe('Docker image to deploy'),
  manifestsPath: z
    .string()
    .optional()
    .describe('Path to Kubernetes manifests directory or YAML content'),
  namespace: namespaceOptional,
  replicas,
  port,
  environment,
});

export type DeployApplicationParams = z.infer<typeof deployApplicationSchema>;
