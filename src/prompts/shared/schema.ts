/**
 * Shared Zod schema fragments for loop prompt arguments.
 *
 * MCP prompt arguments are always strings at the protocol level.
 * Optional fields default to sensible values in the prompt builder.
 */

import { z } from 'zod';

/** Schema fields shared by all dev-loop prompts. */
export const sharedLoopSchema = {
  namespace: z
    .string()
    .optional()
    .describe(
      'Kubernetes namespace for the deployment. If empty, a unique namespace is auto-generated',
    ),
  imageName: z
    .string()
    .optional()
    .describe('Docker image name. If empty, derived from the repository directory name'),
} as const;
