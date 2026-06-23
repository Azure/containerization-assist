/**
 * Generate GitHub Workflow Tool
 *
 * Queries the GitHub Actions knowledge base and returns a structured plan
 * for creating a .github/workflows/deploy.yml file that:
 *   1. Builds a Docker image from the repository Dockerfile
 *   2. Pushes it to Azure Container Registry (ACR)
 *   3. Deploys it to AKS using Azure OIDC federated credentials
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behaviour.
 * No AI calls are made — the tool returns a plan for the MCP client AI to use.
 *
 * ⚠️  Do NOT use import.meta in this file — the CJS build forbids it.
 */

import path from 'node:path';
import { type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/core/context';
import { tool } from '@/types/tool';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import { PACKAGE_VERSION } from '@/lib/package-version';
import { TOOL_NAME } from '../shared/toolDefinition';
import {
  generateGithubWorkflowSchema,
  type GenerateGithubWorkflowParams,
  type GithubWorkflowPlan,
  type WorkflowJobDescription,
} from './schema';
import { generateGithubWorkflowToolDefinition } from './types';

const { name } = generateGithubWorkflowToolDefinition;

// ─── Category type ────────────────────────────────────────────────────────────

type WorkflowCategory = 'auth' | 'build' | 'deploy' | 'bestPractices';

// ─── Rules type ───────────────────────────────────────────────────────────────

interface WorkflowRules {
  /** Whether the deploy job needs azure/k8s-bake (helm or kustomize manifests) */
  includeBakeStep: boolean;
  /** Render engine to pass to azure/k8s-bake (only used when includeBakeStep is true) */
  renderEngine: 'helm' | 'kustomize';
  /** Runner image — always ubuntu-latest */
  runsOn: string;
}

// ─── Knowledge-tool pattern ───────────────────────────────────────────────────

const runPattern = createKnowledgeTool<
  GenerateGithubWorkflowParams,
  GithubWorkflowPlan,
  WorkflowCategory,
  WorkflowRules
>({
  name,

  query: {
    topic: TOPICS.GITHUB_WORKFLOW,
    category: CATEGORY.CICD,
    maxChars: 6000,
    maxSnippets: 15,
    extractFilters: (input) => ({
      language: input.language,
      framework: input.framework,
    }),
  },

  categorization: {
    categoryNames: ['auth', 'build', 'deploy', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<WorkflowCategory>({
      auth: (s) =>
        Boolean(s.tags?.includes('azure-oidc') || s.tags?.includes('azure-login')),
      build: (s) =>
        Boolean(
          s.tags?.includes('docker-build') ||
            s.tags?.includes('acr') ||
            s.tags?.includes('registry'),
        ),
      deploy: (s) =>
        Boolean(
          s.tags?.includes('aks') ||
            s.tags?.includes('kubectl') ||
            s.tags?.includes('k8s-deploy') ||
            s.tags?.includes('k8s-bake'),
        ),
      bestPractices: () => true, // catch-all for caching, concurrency, governance
    }),
  },

  rules: {
    applyRules: (input) => {
      const fmt = input.manifestFormat ?? 'k8s';
      return {
        includeBakeStep: fmt === 'helm' || fmt === 'kustomize',
        renderEngine: fmt === 'kustomize' ? 'kustomize' : 'helm',
        runsOn: 'ubuntu-latest',
      };
    },
  },

  plan: {
    buildPlan: (input, knowledge, rules, confidence) => {
      const {
        repositoryPath,
        registry,
        clusterName,
        resourceGroup,
        namespace = 'default',
        branches = ['main'],
        manifestPath = 'k8s/',
        manifestFormat = 'k8s',
        dockerFile = 'Dockerfile',
        buildContextPath = '.',
        acrResourceGroup,
        workflowFileName = 'deploy.yml',
      } = input;

      // Derive image name from the repository directory name when not provided.
      // Use path.basename (project convention) — it ignores trailing slashes, so a
      // path like "/home/user/myapp/" still yields "myapp" rather than an empty string.
      // basename returns '' for slash-only paths, so fall back with || (not ??).
      const imageName =
        input.imageName ?? (path.basename(repositoryPath.replace(/\\/g, '/')) || 'app');

      // ACR registry name (short form, without .azurecr.io) for use in az acr commands
      const registryName = registry.replace(/\.azurecr\.io$/i, '');
      const acrRg = acrResourceGroup ?? resourceGroup;

      // Reduce the workflow file name to a bare basename (project convention) so a value
      // like "../escape.yml" or "nested/path.yml" cannot escape .github/workflows/.
      // basename strips any directory/traversal segments; fall back for slash-only input.
      const workflowFileBaseName = path.basename(workflowFileName.replace(/\\/g, '/')) || 'deploy.yml';
      const workflowFilePath = `.github/workflows/${workflowFileBaseName}`;

      // ── Collect categorised snippets for the instruction ────────────────────

      const authSnippets = knowledge.categories.auth ?? [];
      const buildSnippets = knowledge.categories.build ?? [];
      const deploySnippets = knowledge.categories.deploy ?? [];

      // Exclude snippets already in auth/build/deploy from bestPractices
      const authIds = new Set(authSnippets.map((s) => s.id));
      const buildIds = new Set(buildSnippets.map((s) => s.id));
      const deployIds = new Set(deploySnippets.map((s) => s.id));

      const bestPracticeSnippets = (knowledge.categories.bestPractices ?? []).filter(
        (s) => !authIds.has(s.id) && !buildIds.has(s.id) && !deployIds.has(s.id),
      );

      // ── Format knowledge for instruction ────────────────────────────────────

      const formatSnippets = (
        snippets: Array<{ id: string; text: string }>,
        heading: string,
      ): string => {
        if (snippets.length === 0) return '';
        const items = snippets.map((s) => `  - ${s.text}`).join('\n');
        return `\n### ${heading}\n${items}`;
      };

      const knowledgeSection =
        formatSnippets(authSnippets, 'Authentication (OIDC)') +
        formatSnippets(buildSnippets, 'Build & Push') +
        formatSnippets(deploySnippets, 'Deploy to AKS') +
        formatSnippets(bestPracticeSnippets, 'Best Practices');

      // ── Pinned YAML snippets (drift-prone steps — must be copied verbatim) ──

      const buildStepYaml = [
        `      - name: Log into ACR`,
        `        run: |`,
        `          az acr login -n \${{ env.AZURE_CONTAINER_REGISTRY }}`,
        ``,
        `      - name: Build and push image to ACR`,
        `        run: |`,
        `          az acr build --image \${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/\${{ env.CONTAINER_NAME }}:\${{ github.sha }} --registry \${{ env.AZURE_CONTAINER_REGISTRY }} -g \${{ env.ACR_RESOURCE_GROUP }} -f \${{ env.DOCKER_FILE }} \${{ env.BUILD_CONTEXT_PATH }}`,
      ].join('\n');

      const aksContextYaml = [
        `      - name: Set up kubelogin for non-interactive login`,
        `        uses: azure/use-kubelogin@v1`,
        `        with:`,
        `          kubelogin-version: "v0.0.25"`,
        ``,
        `      - name: Get K8s context`,
        `        uses: azure/aks-set-context@v5`,
        `        with:`,
        `          resource-group: \${{ env.CLUSTER_RESOURCE_GROUP }}`,
        `          cluster-name: \${{ env.CLUSTER_NAME }}`,
        `          admin: "false"`,
        `          use-kubelogin: "true"`,
      ].join('\n');

      const bakePathKey = rules.renderEngine === 'helm' ? 'helmChart' : 'kustomizationPath';

      const deployStepYaml = rules.includeBakeStep
        ? [
            `      - name: Bake manifests`,
            `        uses: azure/k8s-bake@v4`,
            `        with:`,
            `          renderEngine: ${rules.renderEngine}`,
            `          ${bakePathKey}: \${{ env.DEPLOYMENT_MANIFEST_PATH }}`,
            `        id: bake`,
            ``,
            `      - name: Deploys application`,
            `        uses: Azure/k8s-deploy@v6`,
            `        with:`,
            `          action: deploy`,
            `          manifests: \${{ steps.bake.outputs.manifestsBundle }}`,
            `          images: |`,
            `            \${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/\${{ env.CONTAINER_NAME }}:\${{ github.sha }}`,
            `          namespace: \${{ env.NAMESPACE }}`,
          ].join('\n')
        : [
            `      - name: Deploys application`,
            `        uses: Azure/k8s-deploy@v6`,
            `        with:`,
            `          action: deploy`,
            `          manifests: \${{ env.DEPLOYMENT_MANIFEST_PATH }}`,
            `          images: |`,
            `            \${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/\${{ env.CONTAINER_NAME }}:\${{ github.sha }}`,
            `          namespace: \${{ env.NAMESPACE }}`,
          ].join('\n');

      // ── Build deploy step descriptions (for prose plan) ─────────────────────

      const deploySteps: string[] = rules.includeBakeStep
        ? [
            `azure/k8s-bake@v4 with renderEngine: ${rules.renderEngine} and ${bakePathKey}: \${{ env.DEPLOYMENT_MANIFEST_PATH }} (id: bake)`,
            `Azure/k8s-deploy@v6 with action: deploy, manifests: \${{ steps.bake.outputs.manifestsBundle }}, images: \${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/\${{ env.CONTAINER_NAME }}:\${{ github.sha }}, namespace: \${{ env.NAMESPACE }}`,
          ]
        : [
            `Azure/k8s-deploy@v6 with action: deploy, manifests: \${{ env.DEPLOYMENT_MANIFEST_PATH }}, images: \${{ env.AZURE_CONTAINER_REGISTRY }}.azurecr.io/\${{ env.CONTAINER_NAME }}:\${{ github.sha }}, namespace: \${{ env.NAMESPACE }}`,
          ];

      // ── nextAction instruction ───────────────────────────────────────────────

      const branchList = branches.join(', ');
      const instruction = [
        `Create a new GitHub Actions workflow at ${workflowFilePath}.`,
        ``,
        `## ⛔ CRITICAL RULES — these MUST be followed exactly`,
        `  1. Use the literal job keys \`buildImage\` and \`deploy\` — do NOT rename them (e.g. NOT \`build-and-push\`).`,
        `  2. Build the image with \`az acr build\` ONLY — do NOT use \`docker/build-push-action\`, \`docker build\`, \`docker buildx\`, or \`docker/setup-buildx-action\`.`,
        `  3. Do NOT add an \`environment:\` key to ANY job. A job-level \`environment\` changes the GitHub OIDC subject claim from \`repo:OWNER/REPO:ref:refs/heads/BRANCH\` to \`repo:OWNER/REPO:environment:NAME\`, which breaks Azure federated-credential authentication unless a matching environment-scoped credential exists.`,
        ``,
        `## Triggers`,
        `  push to branches [${branchList}] and workflow_dispatch`,
        ``,
        `## Workflow-level env variables`,
        `  ACR_RESOURCE_GROUP: ${acrRg}`,
        `  AZURE_CONTAINER_REGISTRY: ${registryName}`,
        `  CONTAINER_NAME: ${imageName}`,
        `  CLUSTER_NAME: ${clusterName}`,
        `  CLUSTER_RESOURCE_GROUP: ${resourceGroup}`,
        `  DEPLOYMENT_MANIFEST_PATH: ${manifestPath}`,
        `  DOCKER_FILE: ${dockerFile}`,
        `  BUILD_CONTEXT_PATH: ${buildContextPath}`,
        `  NAMESPACE: ${namespace}`,
        ``,
        `## Job 1 — buildImage`,
        `  runs-on: ${rules.runsOn}`,
        `  permissions: contents: read, id-token: write`,
        `  steps:`,
        `    1. actions/checkout@v6`,
        `    2. azure/login@v3 with client-id: \${{ secrets.AZURE_CLIENT_ID }}, tenant-id: \${{ secrets.AZURE_TENANT_ID }}, subscription-id: \${{ secrets.AZURE_SUBSCRIPTION_ID }}`,
        `    3. Log into ACR (see pinned snippet below)`,
        `    4. Build and push image to ACR (see pinned snippet below)`,
        ``,
        `## Job 2 — deploy`,
        `  needs: [buildImage]`,
        `  runs-on: ${rules.runsOn}`,
        `  permissions: actions: read, contents: read, id-token: write`,
        `  steps:`,
        `    1. actions/checkout@v6`,
        `    2. azure/login@v3 with client-id: \${{ secrets.AZURE_CLIENT_ID }}, tenant-id: \${{ secrets.AZURE_TENANT_ID }}, subscription-id: \${{ secrets.AZURE_SUBSCRIPTION_ID }}`,
        `    3. Set up kubelogin + AKS context (see pinned snippet below)`,
        ...deploySteps.map((s, i) => `    ${4 + i}. ${s}`),
        `    ${4 + deploySteps.length}. Annotate deployment (see pinned snippet below)`,
        ``,
        `## ⚠️ PINNED YAML SNIPPETS — copy these blocks verbatim`,
        ``,
        `These steps are drift-prone; the AI client MUST emit them exactly as shown.`,
        `Do NOT substitute \`docker/build-push-action\`, \`docker/setup-buildx-action\`, \`docker/login-action\`, \`docker buildx\`, \`az aks get-credentials\`, or \`azure/setup-kubectl@v4\`.`,
        `Do NOT add an \`environment:\` key to either job — it breaks Azure OIDC authentication (see CRITICAL RULES above).`,
        ``,
        `### buildImage — Log into ACR + build (replaces any docker-* actions)`,
        '```yaml',
        buildStepYaml,
        '```',
        ``,
        `### deploy — kubelogin + aks-set-context (replaces az aks get-credentials)`,
        '```yaml',
        aksContextYaml,
        '```',
        ``,
        `### deploy — k8s-deploy${rules.includeBakeStep ? ` (with bake for ${rules.renderEngine})` : ''}`,
        '```yaml',
        deployStepYaml,
        '```',
        ``,
        `### deploy — annotate deployed resources`,
        '```yaml',
        [
          `      - name: Annotate deployment`,
          `        run: |`,
          `          if kubectl get deployment -n \${{ env.NAMESPACE }} --no-headers 2>/dev/null | grep -q .; then`,
          `            kubectl annotate deployment --all -n \${{ env.NAMESPACE }} \\`,
          `              aks-project/pipeline-repo="\${{ github.repository }}" \\`,
          `              aks-project/pipeline-workflow="\${{ github.workflow }}" \\`,
          `              aks-project/deployed-by="vscode" \\`,
          `              aks-project/pipeline-run-url="\${{ github.server_url }}/\${{ github.repository }}/actions/runs/\${{ github.run_id }}" \\`,
          `              --overwrite`,
          `          fi`,
        ].join('\n'),
        '```',
        ``,
        `## Required GitHub repository SECRETS`,
        `  AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID`,
        ``,
        `## OIDC setup`,
        `  Configure an OIDC federated credential in Azure Entra ID for this repository and branch.`,
        ``,
        `## Manifest format`,
        `  ${manifestFormat}${rules.includeBakeStep ? ` — include azure/k8s-bake step with renderEngine: ${rules.renderEngine}` : ' — deploy manifests directly'}`,
        knowledgeSection,
      ]
        .filter((line) => line !== undefined)
        .join('\n');

      // ── Job descriptions (for structured output) ────────────────────────────

      const workflowJobs: WorkflowJobDescription[] = [
        {
          name: 'buildImage',
          runsOn: rules.runsOn,
          steps: [
            'actions/checkout@v6',
            'azure/login@v3 (OIDC)',
            `az acr login -n ${registryName}`,
            `az acr build → ${registryName}.azurecr.io/${imageName}:\${{ github.sha }}`,
          ],
        },
        {
          name: 'deploy',
          runsOn: rules.runsOn,
          steps: [
            'actions/checkout@v6',
            'azure/login@v3 (OIDC)',
            'azure/use-kubelogin@v1',
            `azure/aks-set-context@v5 → ${clusterName}`,
            ...(rules.includeBakeStep
              ? [
                  `azure/k8s-bake@v4 (renderEngine: ${rules.renderEngine}, path: ${manifestPath})`,
                  `Azure/k8s-deploy@v6 → namespace: ${namespace}`,
                ]
              : [`Azure/k8s-deploy@v6 → namespace: ${namespace}`]),
            `kubectl annotate deployment --all -n ${namespace} (pipeline metadata)`,
          ],
        },
      ];

      // ── Summary ─────────────────────────────────────────────────────────────

      const totalSnippets =
        authSnippets.length +
        buildSnippets.length +
        deploySnippets.length +
        bestPracticeSnippets.length;

      const summary =
        `🔨 ACTION REQUIRED: Create GitHub Actions workflow\n` +
        `Path: ${workflowFilePath}\n` +
        `Registry: ${registry}\n` +
        `Image: ${imageName} (tagged with commit SHA)\n` +
        `Cluster: ${clusterName} (resource group: ${resourceGroup})\n` +
        `Namespace: ${namespace}\n` +
        `Manifest format: ${manifestFormat}${rules.includeBakeStep ? ` (bake step: ${rules.renderEngine})` : ''}\n` +
        `Trigger branches: ${branchList}\n` +
        `Knowledge snippets applied: ${totalSnippets}\n\n` +
        `✅ Ready to create workflow. After committing, configure GitHub secrets:\n` +
        `   AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID\n` +
        `Then set up an OIDC federated credential in Azure Entra ID for this repository.`;

      return {
        nextAction: {
          action: 'create-files',
          instruction,
          files: [
            {
              path: workflowFilePath,
              purpose: 'GitHub Actions CI/CD workflow — build, push to ACR, deploy to AKS',
            },
          ],
        },
        workflowJobs,
        secretsRequired: ['AZURE_CLIENT_ID', 'AZURE_TENANT_ID', 'AZURE_SUBSCRIPTION_ID'],
        variablesRequired: [],
        summary,
        attributionLabels: {
          annotations: {
            'com.azure.containerizationassist/version': PACKAGE_VERSION,
            'com.azure.containerizationassist/workflow-generator': 'generate-github-workflow',
          },
        },
        confidence,
      };
    },
  },
});

// ─── Handler ──────────────────────────────────────────────────────────────────

async function handleGenerateGithubWorkflow(
  input: GenerateGithubWorkflowParams,
  ctx: ToolContext,
): Promise<Result<GithubWorkflowPlan>> {
  return runPattern(input, ctx);
}

// ─── Tool export ──────────────────────────────────────────────────────────────

export default tool({
  name: TOOL_NAME.GENERATE_GITHUB_WORKFLOW,
  description: generateGithubWorkflowToolDefinition.description,
  schema: generateGithubWorkflowSchema,
  metadata: { knowledgeEnhanced: true },
  handler: handleGenerateGithubWorkflow,
  category: 'docker',
  version: '1.0.0',
  chainHints: generateGithubWorkflowToolDefinition.chainHints,
});
