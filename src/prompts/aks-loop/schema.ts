/**
 * Zod schema for the aks-loop prompt arguments.
 *
 * MCP prompt arguments are always strings at the protocol level.
 * Optional fields default to sensible values in the prompt builder;
 * required fields (registry, resourceGroup, clusterName) must be provided.
 */

import { z } from 'zod';

export const aksRemoteDevLoopSchema = {
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
  registry: z
    .string()
    .describe(
      'Container registry URL (e.g., myregistry.azurecr.io). Required for pushing images to ACR',
    ),
  resourceGroup: z.string().describe('Azure resource group containing the AKS cluster'),
  clusterName: z.string().describe('AKS cluster name to deploy to'),
} as const;

export type AksRemoteDevLoopArgs = {
  namespace?: string;
  imageName?: string;
  registry: string;
  resourceGroup: string;
  clusterName: string;
};
