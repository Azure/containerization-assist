/**
 * aks-loop prompt
 *
 * Returns a seeded user message that drives a full AKS remote cluster
 * deployment iteration loop using the containerization-assist MCP tools.
 */

import { TOOL_NAME } from '@/tools';
import {
  acrValidationStep,
  aksClusterValidationStep,
  aksRbacPreDeployCheck,
  analyzeStep,
  assemblePrompt,
  azureSubscriptionStep,
  azureTenantStep,
  databaseCheckStep,
  deployStep,
  envVarCheckStep,
  generateDockerfileStep,
  generateGithubWorkflowStep,
  scanStep,
  sharedRules,
  verifyStep,
} from '../shared/steps';
import type { AksRemoteDevLoopArgs } from './schema';

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

  return assemblePrompt(
    'AKS remote cluster deployment iteration loop',
    [
      '- Repository: use the **current working directory** (confirm with the user before proceeding).',
      `- Container registry: **${registry}**`,
      `- ${nsClause}`,
      `- ${imageClause}`,
      `- ${rgClause}`,
      `- ${clusterClause}`,
      '- Target environment: **production** (remote AKS cluster with ACR)',
      '- Target platform: **linux/amd64** (standard AKS node architecture).',
    ],
    [
      // Pre-flight: Azure resource selection & validation
      azureTenantStep(),
      azureSubscriptionStep(),
      aksClusterValidationStep(),
      acrValidationStep(),

      // Core workflow
      analyzeStep(),
      databaseCheckStep(),
      envVarCheckStep(),
      generateDockerfileStep(),
      {
        heading: 'Build the image',
        body: [
          `1. Call **${TOOL_NAME.BUILD_IMAGE_CONTEXT}** with the repository path, image name, and platform \`linux/amd64\`.`,
          '2. Execute the returned build command to produce the Docker image.',
          `3. Retry up to **2 times** on failure; call **${TOOL_NAME.FIX_DOCKERFILE}** if the build fails due to Dockerfile issues.`,
        ].join('\n'),
      },
      scanStep(),
      {
        heading: 'Prepare the AKS cluster',
        body: [
          `1. Call **${TOOL_NAME.PREPARE_CLUSTER}** with:`,
          '   - `clusterType: "generic"` (assumes existing AKS cluster)',
          '   - `namespace`: the namespace from context above',
          '   - `targetPlatform: "linux/amd64"`',
          '2. Retry up to **2 times** on failure.',
          '3. If kubeconfig is missing, run `az aks get-credentials --resource-group <rg> --name <cluster>` first.',
        ].join('\n'),
      },
      {
        heading: 'Tag the image for ACR',
        body: [
          `1. Call **${TOOL_NAME.TAG_IMAGE}** to tag the image for the Azure Container Registry: \`${registry}/<image>:<tag>\`.`,
          '2. Retry up to **2 times** on failure.',
        ].join('\n'),
      },
      {
        heading: 'Push the image to ACR',
        body: [
          `1. Call **${TOOL_NAME.PUSH_IMAGE}** to push the tagged image to \`${registry}\`.`,
          '2. Retry up to **2 times** on failure.',
          '3. If authentication fails, prompt the user to run `az acr login --name <registry-name>`.',
        ].join('\n'),
      },
      {
        heading: 'Generate Kubernetes manifests (if missing)',
        body: [
          'If no K8s manifests exist for the target module(s):',
          `1. Call **${TOOL_NAME.GENERATE_K8S_MANIFESTS}** with the repository path, namespace, and analysis context. Set the image reference in the manifests to use the ACR registry prefix \`${registry}/\`. Pass the \`detectedDatabases\` array from analyze-repo results directly as the \`detectedDatabases\` parameter so the tool can generate a ServiceAccount with workload identity annotations. Pass the \`detectedEnvVars\` array so the tool can generate ConfigMap for config vars and reference Secrets for secret vars.`,
          "2. Follow the tool's guidance to create manifest files on disk.",
          '3. Retry up to **2 times** if generation fails.',
          '',
          '**AKS Automatic hardening** — if `isAutomatic` was detected in the pre-flight cluster validation step, ensure generated manifests include:',
          '- Pod-level: `securityContext.runAsNonRoot: true` and `securityContext.seccompProfile.type: RuntimeDefault`',
          '- Container-level: `securityContext.allowPrivilegeEscalation: false` and `capabilities.drop: ["ALL"]`',
          '- Resource requests and limits on every container (e.g., `cpu: 500m / 1000m`, `memory: 512Mi / 1Gi`)',
        ].join('\n'),
      },

      // Pre-deploy RBAC check (only when Azure RBAC detected)
      aksRbacPreDeployCheck(),

      deployStep('AKS'),
      verifyStep([
        '3. Report the external IP / ingress endpoint if a LoadBalancer or Ingress is configured.',
      ]),
      generateGithubWorkflowStep(registry, args.clusterName, args.resourceGroup),
    ],
    [
      ...sharedRules,
      '- Use **linux/amd64** as the target platform for all builds (standard AKS architecture).',
      '- For ACR authentication issues, guide the user through `az acr login` before retrying.',
      '- If the RBAC pre-deploy check (`Verify effective access`) halts the workflow, follow its embedded instructions: emit the missing-roles diagnostic, wait for the user to grant the roles, and poll until access propagates. Do **NOT** proceed to deploy or verify until probes pass.',
      "- Do **NOT** suggest `az aks command invoke` as an RBAC bypass when `isAzureRbac` is true — it authenticates as the caller's identity and hits the same `Forbidden` responses.",
      '- Do **NOT** suggest `--admin` kubeconfig when `localAccountsDisabled` is true — it will be rejected by AKS.',
      '- If using `vscode_askQuestions` for pickers, always include a free-text input option so the user can type a value manually.',
    ],
  );
}
