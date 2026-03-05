/**
 * Zod schema for the aks-remote-dev-loop prompt arguments.
 *
 * All fields are plain z.string() because MCP prompt arguments are
 * always strings at the protocol level. Empty string means "not provided"
 * and is handled as a default in the prompt builder.
 */

import { z } from 'zod';

export const aksRemoteDevLoopSchema = {
  repositoryPath: z
    .string()
    .describe(
      'Absolute path to the repository to containerize. Defaults to current directory if omitted',
    ),
  namespace: z
    .string()
    .describe(
      'Kubernetes namespace for the deployment. If empty, a unique namespace is auto-generated',
    ),
  imageName: z
    .string()
    .describe('Docker image name. If empty, derived from the repository directory name'),
  registry: z
    .string()
    .describe(
      'Container registry URL (e.g., myregistry.azurecr.io). Required for pushing images to ACR',
    ),
  resourceGroup: z.string().describe('Azure resource group containing the AKS cluster'),
  clusterName: z.string().describe('AKS cluster name to deploy to'),
} as const;

export type AksRemoteDevLoopArgs = {
  repositoryPath: string;
  namespace: string;
  imageName: string;
  registry: string;
  resourceGroup: string;
  clusterName: string;
};
