/**
 * Schema definition for deploy tool
 */

import { z } from 'zod';
import { sessionId, namespaceOptional, replicas, port, environment } from '../shared/schemas';

export const deployApplicationSchema = z.object({
  sessionId: sessionId.optional(),
  manifestsPath: z
    .string()
    .describe('Path to Kubernetes manifests directory or YAML content (required)'),
  namespace: namespaceOptional,
  replicas,
  port,
  environment,
});

export type DeployApplicationParams = z.infer<typeof deployApplicationSchema>;
