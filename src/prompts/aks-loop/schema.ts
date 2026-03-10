/**
 * Zod schema for the aks-loop prompt arguments.
 *
 * MCP prompt arguments are always strings at the protocol level.
 * Optional fields default to sensible values in the prompt builder;
 * required fields (registry, resourceGroup, clusterName) must be provided.
 */

import { z } from 'zod';
import { sharedLoopSchema } from '../shared/schema';

export const aksRemoteDevLoopSchema = {
  ...sharedLoopSchema,
  registry: z
    .string()
    .describe(
      'Container registry URL (e.g., myregistry.azurecr.io). Required for pushing images to ACR',
    ),
  resourceGroup: z.string().describe('Azure resource group containing the AKS cluster'),
  clusterName: z.string().describe('AKS cluster name to deploy to'),
} as const;

export type AksRemoteDevLoopArgs = {
  namespace?: string | undefined;
  imageName?: string | undefined;
  registry: string;
  resourceGroup: string;
  clusterName: string;
};
