/**
 * Zod schema for the local-kind-dev-loop prompt arguments.
 *
 * All fields are plain z.string() because MCP prompt arguments are
 * always strings at the protocol level. Empty string means "not provided"
 * and is handled as a default in the prompt builder.
 */

import { z } from 'zod';

export const localKindDevLoopSchema = {
  repositoryPath: z
    .string()
    .describe(
      'Absolute path to the repository to containerize. Defaults to current directory if omitted',
    ),
  namespace: z
    .string()
    .describe(
      'Kubernetes namespace for the deployment. If empty, a unique namespace is auto-generated (e.g., dev-<short-hash>)',
    ),
  imageName: z
    .string()
    .describe('Docker image name. If empty, derived from the repository directory name'),
} as const;

export type LocalKindDevLoopArgs = {
  repositoryPath: string;
  namespace: string;
  imageName: string;
};
