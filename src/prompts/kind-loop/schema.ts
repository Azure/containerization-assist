/**
 * Zod schema for the kind-loop prompt arguments.
 *
 * MCP prompt arguments are always strings at the protocol level.
 * Optional fields default to sensible values in the prompt builder.
 */

import { z } from 'zod';

export const localKindDevLoopSchema = {
  namespace: z
    .string()
    .optional()
    .describe(
      'Kubernetes namespace for the deployment. If empty, a unique namespace is auto-generated (e.g., dev-<short-hash>)',
    ),
  imageName: z
    .string()
    .optional()
    .describe('Docker image name. If empty, derived from the repository directory name'),
} as const;

export type LocalKindDevLoopArgs = {
  namespace?: string;
  imageName?: string;
};
