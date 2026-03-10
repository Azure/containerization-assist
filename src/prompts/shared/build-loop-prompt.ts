/**
 * Shared loop prompt builder.
 *
 * Both kind-loop and aks-loop follow the same 10-step workflow.
 * Each loop injects its own config for the parts that differ
 * (context, platform, cluster prep, registry, extra rules).
 */

import { TOOL_NAME } from '@/tools';

/** Configuration slots that vary between loop prompts. */
export interface LoopPromptConfig {
  /** e.g. "local Kind cluster development iteration" */
  title: string;
  /** Extra context bullet lines (after repo/namespace/image) */
  contextLines: string[];
  /** Platform clause for Step 3 build (e.g. "the **local system platform** …" or "`linux/amd64`") */
  buildPlatform: string;
  /** Full bullet list for Step 5 — Prepare cluster (multi-line, indented) */
  prepareStep: string;
  /** Full instruction for Step 6 — Tag (line 1 only; "Retry …" is appended automatically) */
  tagInstruction: string;
  /** Full instruction(s) for Step 7 — Push (may be multi-line for auth hints) */
  pushInstructions: string;
  /** Clause describing the registry prefix for manifests in Step 8 */
  manifestRegistryClause: string;
  /** Label for Step 9 heading, e.g. "the Kind cluster" or "AKS" */
  deployTarget: string;
  /** Optional extra lines appended to Step 10 (e.g. report external IP) */
  verifyExtra?: string;
  /** Platform rule for the Important rules section */
  platformRule: string;
  /** Any additional Important rules lines */
  extraRules: string[];
}

/**
 * Build a loop prompt from shared structure + config slots.
 *
 * @param nsClause   - Pre-built namespace context sentence
 * @param imageClause - Pre-built image name context sentence
 * @param config     - Loop-specific configuration
 */
export function buildLoopPrompt(
  nsClause: string,
  imageClause: string,
  config: LoopPromptConfig,
): string {
  const contextBlock = [
    '- Repository: use the **current working directory** (confirm with the user before proceeding).',
    `- ${nsClause}`,
    `- ${imageClause}`,
    ...config.contextLines,
  ].join('\n');

  const extraRulesBlock = config.extraRules.length
    ? `\n${config.extraRules.map((r) => `- ${r}`).join('\n')}`
    : '';

  const verifyExtraBlock = config.verifyExtra ? `\n${config.verifyExtra}` : '';

  return `You are driving a **${config.title}** using the containerization-assist MCP server tools.

## Context
${contextBlock}

## Workflow — follow each step in order

### Step 1 — Analyze the repository
Call **${TOOL_NAME.ANALYZE_REPO}** using the current working directory as the repository path.
- Confirm the detected repository, language, framework, modules, and existing Dockerfiles with the user before proceeding.
- If the repository is a monorepo, list all independently deployable modules and ask the user which ones to target.

### Step 2 — Generate Dockerfile (if missing)
If no Dockerfile exists for the target module(s):
1. Call **${TOOL_NAME.GENERATE_DOCKERFILE}** with the repository path and analysis context.
2. Follow the tool's guidance to create the Dockerfile(s) on disk.
3. Retry up to **2 times** if generation fails.

### Step 3 — Build the image
1. Call **${TOOL_NAME.BUILD_IMAGE_CONTEXT}** with the repository path, image name, and ${config.buildPlatform}.
2. Execute the returned build command to produce the Docker image.
3. Retry up to **2 times** on failure; call **${TOOL_NAME.FIX_DOCKERFILE}** if the build fails due to Dockerfile issues.

### Step 4 — Scan the image
1. Call **${TOOL_NAME.SCAN_IMAGE}** with the built image ID.
2. Review vulnerabilities. If critical/high issues are found, call **${TOOL_NAME.FIX_DOCKERFILE}** and rebuild (Step 3). Retry up to **2 times**.

### Step 5 — Prepare the cluster
${config.prepareStep}

### Step 6 — Tag the image
1. ${config.tagInstruction}
2. Retry up to **2 times** on failure.

### Step 7 — Push the image
${config.pushInstructions}

### Step 8 — Generate Kubernetes manifests (if missing)
If no K8s manifests exist for the target module(s):
1. Call **${TOOL_NAME.GENERATE_K8S_MANIFESTS}** with the repository path, namespace, and analysis context. ${config.manifestRegistryClause}
2. Follow the tool's guidance to create manifest files on disk.
3. Retry up to **2 times** if generation fails.

### Step 9 — Deploy to ${config.deployTarget}
1. Apply the generated manifests using \`kubectl apply -f <manifest-folder> --namespace <namespace>\`.
2. Retry up to **2 times** on failure.

### Step 10 — Verify the deployment
1. Call **${TOOL_NAME.VERIFY_DEPLOY}** with the namespace to check pod status, readiness, and events.
2. If verification fails, inspect pod logs and events, fix issues, and re-deploy. Retry up to **2 times**.${verifyExtraBlock}

## Important rules
- **Retry failed steps at least 2 times** before reporting failure.
- **Follow the chain hints** returned by each tool to determine next steps.
- If a tool suggests calling another tool, follow that suggestion.
- ${config.platformRule}
- Keep the user informed of progress at each step.
- If all retry attempts for a step are exhausted, report the failure clearly with diagnostic details.${extraRulesBlock}`;
}
