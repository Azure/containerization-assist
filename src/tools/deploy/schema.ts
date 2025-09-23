/**
 * Schema definition for deploy tool
 */

import { z } from 'zod';
import { sessionId, namespaceOptional, replicas, port, environmentFull } from '../shared/schemas';

export const deployApplicationSchema = z.object({
  sessionId: sessionId.optional(),
  imageId: z.string().optional().describe('Docker image to deploy'),
  namespace: namespaceOptional,
  replicas,
  port,
  environment: environmentFull,
});

export type DeployApplicationParams = z.infer<typeof deployApplicationSchema>;
