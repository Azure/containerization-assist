/**
 * aks-remote-dev-loop prompt
 *
 * Returns a seeded user message that drives a full AKS remote cluster
 * deployment iteration loop using the containerization-assist MCP tools.
 */

import type { AksRemoteDevLoopArgs } from './schema';

/**
 * Build the prompt text for an AKS remote development loop.
 *
 * The returned string is a comprehensive, step-by-step workflow instruction
 * that references the MCP tools available in containerization-assist.
 */
export function buildAksRemoteDevLoopPrompt(args: AksRemoteDevLoopArgs): string {
  const repoPath = args.repositoryPath || '.';
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

  return `You are driving an **AKS remote cluster deployment iteration loop** using the containerization-assist MCP server tools.

## Context
- Repository path: \`${repoPath}\`
- Container registry: **${registry}**
- ${nsClause}
- ${imageClause}
- ${rgClause}
- ${clusterClause}
- Target environment: **production** (remote AKS cluster with ACR)
- Target platform: **linux/amd64** (standard AKS node architecture).

## Workflow — follow each step in order

### Step 1 — Analyze the repository
Call **analyze-repo** with \`repositoryPath: "${repoPath}"\`.
- Inspect the result for detected language, framework, modules, and existing Dockerfiles.
- If the repository is a monorepo, note all independently deployable modules.

### Step 2 — Generate Dockerfile (if missing)
If no Dockerfile exists for the target module(s):
1. Call **generate-dockerfile** with the repository path and analysis context.
2. Follow the tool's guidance to create the Dockerfile(s) on disk.
3. Retry up to **2 times** if generation fails.

### Step 3 — Build the image
1. Call **build-image-context** with the repository path, image name, and platform \`linux/amd64\`.
2. Execute the returned build command to produce the Docker image.
3. Retry up to **2 times** on failure; call **fix-dockerfile** if the build fails due to Dockerfile issues.

### Step 4 — Scan the image
1. Call **scan-image** with the built image ID.
2. Review vulnerabilities. If critical/high issues are found, call **fix-dockerfile** and rebuild (Step 3). Retry up to **2 times**.

### Step 5 — Tag the image for ACR
1. Call **tag-image** to tag the image for the Azure Container Registry: \`${registry}/<image>:<tag>\`.
2. Retry up to **2 times** on failure.

### Step 6 — Push the image to ACR
1. Call **push-image** to push the tagged image to \`${registry}\`.
2. Retry up to **2 times** on failure.
3. If authentication fails, prompt the user to run \`az acr login --name <registry-name>\`.

### Step 7 — Prepare the AKS cluster
1. Call **prepare-cluster** with:
   - \`environment: "production"\` (assumes existing AKS cluster)
   - \`namespace\`: the namespace from context above
   - \`targetPlatform: "linux/amd64"\`
2. Retry up to **2 times** on failure.
3. If kubeconfig is not set, prompt the user to run \`az aks get-credentials --resource-group <rg> --name <cluster>\`.

### Step 8 — Generate Kubernetes manifests (if missing)
If no K8s manifests exist for the target module(s):
1. Call **generate-k8s-manifests** with the repository path, image name (with ACR registry prefix \`${registry}/\`), namespace, and analysis context.
2. Follow the tool's guidance to create manifest files on disk.
3. Retry up to **2 times** if generation fails.

### Step 9 — Deploy to AKS
1. Apply the generated manifests using \`kubectl apply -f <manifest-folder> --namespace <namespace>\`.
2. Retry up to **2 times** on failure.

### Step 10 — Verify the deployment
1. Call **verify-deploy** with the namespace to check pod status, readiness, and events.
2. If verification fails, inspect pod logs and events, fix issues, and re-deploy. Retry up to **2 times**.
3. Report the external IP / ingress endpoint if a LoadBalancer or Ingress is configured.

## Important rules
- **Retry failed steps at least 2 times** before reporting failure.
- **Follow the chain hints** returned by each tool to determine next steps.
- If a tool suggests calling another tool, follow that suggestion.
- Use **linux/amd64** as the target platform for all builds (standard AKS architecture).
- Keep the user informed of progress at each step.
- If all retry attempts for a step are exhausted, report the failure clearly with diagnostic details.
- For ACR authentication issues, guide the user through \`az acr login\` before retrying.`;
}
