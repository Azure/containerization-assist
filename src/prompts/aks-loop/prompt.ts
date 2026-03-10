/**
 * aks-loop prompt
 *
 * Returns a seeded user message that drives a full AKS remote cluster
 * deployment iteration loop using the containerization-assist MCP tools.
 */

import { TOOL_NAME } from '@/tools';
import { buildLoopPrompt } from '../shared/build-loop-prompt';
import type { AksRemoteDevLoopArgs } from './schema';

/**
 * Build the prompt text for an AKS remote development loop.
 */
export function buildAksRemoteDevLoopPrompt(args: AksRemoteDevLoopArgs): string {
  const registry = args.registry;
  const nsClause = args.namespace
    ? `Use the namespace **${args.namespace}**.`
    : 'Generate a unique namespace name (e.g., `staging-<short-hash>`) for isolation.';
  const imageClause = args.imageName
    ? `Use the image name **${args.imageName}**.`
    : 'Derive the image name from the repository directory name.';
  const rgClause = args.resourceGroup
    ? `Resource group: **${args.resourceGroup}**.`
    : 'Determine the resource group from the current Azure context or ask the user.';
  const clusterClause = args.clusterName
    ? `AKS cluster: **${args.clusterName}**.`
    : 'Determine the AKS cluster name from the current kubeconfig context or ask the user.';

  return buildLoopPrompt(nsClause, imageClause, {
    title: 'AKS remote cluster deployment iteration loop',
    contextLines: [
      `- Container registry: **${registry}**`,
      `- ${rgClause}`,
      `- ${clusterClause}`,
      '- Target environment: **production** (remote AKS cluster with ACR)',
      '- Target platform: **linux/amd64** (standard AKS node architecture).',
    ],
    buildPlatform: 'platform \\`linux/amd64\\`',
    prepareStep: [
      `1. Call **${TOOL_NAME.PREPARE_CLUSTER}** with:`,
      '   - \\`clusterType: "generic"\\` (assumes existing AKS cluster)',
      '   - \\`namespace\\`: the namespace from context above',
      '   - \\`targetPlatform: "linux/amd64"\\`',
      '2. Retry up to **2 times** on failure.',
      '3. If kubeconfig is not set, prompt the user to run \\`az aks get-credentials --resource-group <rg> --name <cluster>\\`.',
    ].join('\n'),
    tagInstruction: `Call **${TOOL_NAME.TAG_IMAGE}** to tag the image for the Azure Container Registry: \\\`${registry}/<image>:<tag>\\\`.`,
    pushInstructions: [
      `1. Call **${TOOL_NAME.PUSH_IMAGE}** to push the tagged image to \\\`${registry}\\\`.`,
      '2. Retry up to **2 times** on failure.',
      '3. If authentication fails, prompt the user to run \\`az acr login --name <registry-name>\\`.',
    ].join('\n'),
    manifestRegistryClause: `Set the image reference in the manifests to use the ACR registry prefix \\\`${registry}/\\\`.`,
    deployTarget: 'AKS',
    verifyExtra:
      '3. Report the external IP / ingress endpoint if a LoadBalancer or Ingress is configured.',
    platformRule:
      'Use **linux/amd64** as the target platform for all builds (standard AKS architecture).',
    extraRules: [
      'For ACR authentication issues, guide the user through \\`az acr login\\` before retrying.',
    ],
  });
}
