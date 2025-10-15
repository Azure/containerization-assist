/**
 * Schema definition for deploy tool
 */

import { z } from 'zod';
import { namespaceOptional, replicas, port, environment } from '../shared/schemas';

export const deployApplicationSchema = z.object({
  manifestsPath: z
    .string()
    .describe('Path to Kubernetes manifests directory or YAML content (required)'),
  namespace: namespaceOptional,
  replicas,
  port,
  environment,
});

export type DeployApplicationParams = z.infer<typeof deployApplicationSchema>;
