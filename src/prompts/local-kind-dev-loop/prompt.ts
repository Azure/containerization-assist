/**
 * local-kind-dev-loop prompt
 *
 * Returns a seeded user message that drives a full local Kind cluster
 * development iteration loop using the containerization-assist MCP tools.
 */

import type { LocalKindDevLoopArgs } from './schema';

/**
 * Build the prompt text for a local Kind development loop.
 *
 * The returned string is a comprehensive, step-by-step workflow instruction
 * that references the MCP tools available in containerization-assist.
 */
export function buildLocalKindDevLoopPrompt(args: LocalKindDevLoopArgs): string {
  const repoPath = args.repositoryPath || '.';
  const nsClause = args.namespace
    ? `Use the namespace **${args.namespace}**.`
    : 'Generate a unique namespace name (e.g., `dev-<short-hash>`) for isolation.';
  const imageClause = args.imageName
    ? `Use the image name **${args.imageName}**.`
    : 'Derive the image name from the repository directory name.';

  return `You are driving a **local Kind cluster development iteration loop** using the containerization-assist MCP server tools.

## Context
- Repository path: \`${repoPath}\`
- ${nsClause}
- ${imageClause}
- Target environment: **development** (local Kind cluster with local image registry)
- Use the **local system architecture** for Kind testing (detect it automatically).

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
1. Call **build-image-context** with the repository path, image name, and the **local system platform** (e.g., \`linux/arm64\` or \`linux/amd64\` — detect automatically).
2. Execute the returned build command to produce the Docker image.
3. Retry up to **2 times** on failure; call **fix-dockerfile** if the build fails due to Dockerfile issues.

### Step 4 — Scan the image
1. Call **scan-image** with the built image ID.
2. Review vulnerabilities. If critical/high issues are found, call **fix-dockerfile** and rebuild (Step 3). Retry up to **2 times**.

### Step 5 — Tag the image for the local registry
1. Call **tag-image** to tag the image for the Kind local registry (typically \`localhost:5001/<image>:<tag>\`).
2. Retry up to **2 times** on failure.

### Step 6 — Prepare the Kind cluster
1. Call **prepare-cluster** with:
   - \`environment: "development"\` (this creates a Kind cluster with a local registry)
   - \`namespace\`: the namespace from context above
   - \`targetPlatform\`: the local system architecture
2. Note the local registry address from the result for pushing.
3. Retry up to **2 times** on failure.

### Step 7 — Push the image to the local registry
1. Call **push-image** to push the tagged image to the Kind local registry.
2. Retry up to **2 times** on failure.

### Step 8 — Generate Kubernetes manifests (if missing)
If no K8s manifests exist for the target module(s):
1. Call **generate-k8s-manifests** with the repository path, image name (with local registry prefix), namespace, and analysis context.
2. Follow the tool's guidance to create manifest files on disk.
3. Retry up to **2 times** if generation fails.

### Step 9 — Deploy to the Kind cluster
1. Apply the generated manifests using \`kubectl apply -f <manifest-folder> --namespace <namespace>\`.
2. Retry up to **2 times** on failure.

### Step 10 — Verify the deployment
1. Call **verify-deploy** with the namespace to check pod status, readiness, and events.
2. If verification fails, inspect pod logs and events, fix issues, and re-deploy. Retry up to **2 times**.

## Important rules
- **Retry failed steps at least 2 times** before reporting failure.
- **Follow the chain hints** returned by each tool to determine next steps.
- If a tool suggests calling another tool, follow that suggestion.
- Use the **local system architecture** for all platform-related parameters.
- Keep the user informed of progress at each step.
- If all retry attempts for a step are exhausted, report the failure clearly with diagnostic details.`;
}
