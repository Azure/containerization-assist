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

// ─── Category type ────────────────────────────────────────────────────────────

type WorkflowCategory = 'auth' | 'build' | 'deploy' | 'bestPractices';

// ─── Rules type ───────────────────────────────────────────────────────────────

interface WorkflowRules {
  /** Whether the deploy job needs azure/k8s-bake (helm or kustomize manifests) */
  includeBakeStep: boolean;
  /** Render type to pass to azure/k8s-bake */
  renderType: 'manifests' | 'helm' | 'kustomize';
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
  name: 'generate-github-workflow',

  query: {
    topic: TOPICS.GITHUB_WORKFLOW,
    category: CATEGORY.CICD,
    maxChars: 6000,
    maxSnippets: 15,
    extractFilters: (input) => ({
      environment: input.environment ?? 'production',
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
        renderType:
          fmt === 'helm' ? 'helm' : fmt === 'kustomize' ? 'kustomize' : 'manifests',
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
        environment = 'production',
        branches = ['main'],
        manifestPath = 'k8s/',
        manifestFormat = 'k8s',
      } = input;

      // Derive image name from the last segment of the repository path when not provided
      const imageName =
        input.imageName ??
        (repositoryPath ? repositoryPath.replace(/\\/g, '/').split('/').pop() ?? 'app' : 'app');

      const imageRef = `${registry}/${imageName}:\${{ github.sha }}`;
      const workflowFilePath = '.github/workflows/deploy.yml';

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

      // ── Build bake/deploy instruction lines ─────────────────────────────────

      const deploySteps: string[] = [];

      if (rules.includeBakeStep) {
        deploySteps.push(
          `Use azure/k8s-bake@v1 with renderType: ${rules.renderType} and manifests: ${manifestPath} (id: bake)`,
          `Use azure/k8s-deploy@v5 with namespace: ${namespace}, manifests: \${{ steps.bake.outputs.manifestsBundle }}, images: ${imageRef}`,
        );
      } else {
        deploySteps.push(
          `Use azure/k8s-deploy@v5 with namespace: ${namespace}, manifests: ${manifestPath}, images: ${imageRef}`,
        );
      }

      // ── nextAction instruction ───────────────────────────────────────────────

      const branchList = branches.join(', ');
      const instruction = [
        `Create a new GitHub Actions workflow at ${workflowFilePath}.`,
        '',
        '## Workflow structure',
        '',
        `Trigger: push to branches [${branchList}] and workflow_dispatch.`,
        '',
        'Workflow-level settings:',
        '  - concurrency group: ${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: true',
        '  - permissions: id-token: write, contents: read',
        '',
        '## Job 1 — build-and-push',
        `  runs-on: ${rules.runsOn}`,
        '  steps:',
        '    1. actions/checkout@v4',
        `    2. azure/login@v2 with secrets: AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID`,
        `    3. azure/docker-login@v2 with login-server: ${registry}`,
        `    4. docker/build-push-action@v5 with context: ., push: true, tags: ${imageRef}, cache-from: type=gha, cache-to: type=gha,mode=max`,
        '',
        '## Job 2 — deploy',
        `  needs: build-and-push`,
        `  runs-on: ${rules.runsOn}`,
        `  environment: ${environment}`,
        '  steps:',
        '    1. actions/checkout@v4',
        `    2. azure/login@v2 with secrets: AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID`,
        '    3. azure/setup-kubectl@v4',
        `    4. run: az aks get-credentials --resource-group ${resourceGroup} --name ${clusterName} --overwrite-existing`,
        ...deploySteps.map((s, i) => `    ${5 + i}. ${s}`),
        '',
        '## Required GitHub repository SECRETS',
        '  AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID',
        '',
        '## Required GitHub repository VARIABLES',
        `  REGISTRY=${registry}, CLUSTER_NAME=${clusterName}, RESOURCE_GROUP=${resourceGroup}, K8S_NAMESPACE=${namespace}, IMAGE_NAME=${imageName}`,
        '',
        '## Manifest format',
        `  ${manifestFormat}${rules.includeBakeStep ? ` — include azure/k8s-bake step with renderType: ${rules.renderType}` : ' — deploy manifests directly'}`,
        knowledgeSection,
      ]
        .filter((line) => line !== undefined)
        .join('\n');

      // ── Job descriptions (for structured output) ────────────────────────────

      const workflowJobs: WorkflowJobDescription[] = [
        {
          name: 'build-and-push',
          runsOn: rules.runsOn,
          steps: [
            'actions/checkout@v4',
            'azure/login@v2 (OIDC)',
            `azure/docker-login@v2 → ${registry}`,
            `docker/build-push-action@v5 → ${imageRef} (with GHA layer cache)`,
          ],
        },
        {
          name: 'deploy',
          runsOn: rules.runsOn,
          environment,
          steps: [
            'actions/checkout@v4',
            'azure/login@v2 (OIDC)',
            'azure/setup-kubectl@v4',
            `az aks get-credentials → ${clusterName}`,
            ...(rules.includeBakeStep
              ? [
                  `azure/k8s-bake@v1 (renderType: ${rules.renderType}, manifests: ${manifestPath})`,
                  `azure/k8s-deploy@v5 → namespace: ${namespace}`,
                ]
              : [`azure/k8s-deploy@v5 → namespace: ${namespace}`]),
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
        `Environment: ${environment}\n` +
        `Manifest format: ${manifestFormat}${rules.includeBakeStep ? ` (bake step: ${rules.renderType})` : ''}\n` +
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
        variablesRequired: ['REGISTRY', 'CLUSTER_NAME', 'RESOURCE_GROUP', 'K8S_NAMESPACE', 'IMAGE_NAME'],
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
