/**
 * Schema definition for generate-github-workflow tool
 */

import { z } from 'zod';
import { repositoryPath, workspacePath, type ToolNextAction } from '../shared/schemas';

export const generateGithubWorkflowSchema = z.object({
  repositoryPath: repositoryPath.describe(
    'Repository path (automatically normalized to forward slashes on all platforms).',
  ),
  workspacePath: workspacePath.optional(),

  registry: z
    .string()
    .min(1, 'Registry cannot be empty')
    .describe(
      'Azure Container Registry login server (e.g. myregistry.azurecr.io). Used as the image push destination.',
    ),

  clusterName: z
    .string()
    .min(1, 'Cluster name cannot be empty')
    .describe('AKS cluster name to deploy to.'),

  resourceGroup: z
    .string()
    .min(1, 'Resource group cannot be empty')
    .describe('Azure resource group containing the AKS cluster.'),

  imageName: z
    .string()
    .optional()
    .describe(
      'Docker image name (e.g. myapp). Defaults to the repository directory name if not provided.',
    ),

  namespace: z
    .string()
    .optional()
    .default('default')
    .describe('Kubernetes namespace to deploy into. Defaults to "default".'),

  manifestFormat: z
    .enum(['k8s', 'helm', 'kustomize'])
    .optional()
    .default('k8s')
    .describe(
      'Manifest format used by the project. Determines whether azure/k8s-bake is needed and which renderEngine to use.',
    ),

  manifestPath: z
    .string()
    .optional()
    .default('k8s/')
    .describe(
      'Relative path to the Kubernetes manifests directory (e.g. k8s/). Defaults to k8s/ if not provided.',
    ),

  dockerFile: z
    .string()
    .optional()
    .default('Dockerfile')
    .describe('Path to the Dockerfile relative to the build context. Defaults to "Dockerfile".'),

  buildContextPath: z
    .string()
    .optional()
    .default('.')
    .describe('Docker build context path. Defaults to "." (repository root).'),

  acrResourceGroup: z
    .string()
    .optional()
    .describe(
      'Azure resource group containing the ACR. Defaults to resourceGroup if not provided.',
    ),

  workflowFileName: z
    .string()
    .optional()
    .default('deploy.yml')
    .describe(
      'Name of the workflow file created under .github/workflows/. Defaults to "deploy.yml".',
    ),

  branches: z
    .array(z.string())
    .optional()
    .default(['main'])
    .describe('Branch names that trigger the workflow on push. Defaults to ["main"].'),

  language: z
    .string()
    .optional()
    .describe('Primary programming language from analyze-repo. Used to tailor caching hints.'),

  framework: z
    .string()
    .optional()
    .describe('Framework from analyze-repo. Used to tailor caching hints.'),
});

export type GenerateGithubWorkflowParams = z.infer<typeof generateGithubWorkflowSchema>;

/**
 * Description of a single GitHub Actions job in the generated workflow
 */
export interface WorkflowJobDescription {
  /** Job identifier (e.g. "buildImage", "deploy") */
  name: string;
  /** Ordered list of step descriptions */
  steps: string[];
  /** Runner to use (always ubuntu-latest) */
  runsOn: string;

}

/**
 * Output plan returned by the generate-github-workflow tool.
 * Contains a nextAction directive that tells the AI client what file to create
 * and a detailed instruction covering all knowledge-based requirements.
 */
export interface GithubWorkflowPlan {
  /**
   * File creation directive for the AI client.
   * action is always 'create-files'; files contains the workflow path.
   */
  nextAction: ToolNextAction;

  /**
   * Structured descriptions of each job in the workflow.
   * Used for display and post-processing.
   */
  workflowJobs: WorkflowJobDescription[];

  /**
   * GitHub repository secrets the user must configure before running the workflow.
   * Always includes the three Azure OIDC secrets.
   */
  secretsRequired: string[];

  /**
   * GitHub repository variables (non-sensitive) the user should configure.
   */
  variablesRequired: string[];

  /**
   * Human-readable summary with action-oriented messaging.
   */
  summary: string;

  /**
   * Attribution annotations to include in generated Kubernetes manifests.
   * Tracks which version of containerization-assist produced this workflow.
   */
  attributionLabels: {
    annotations: Record<string, string>;
  };

  /** Confidence score (0-1) based on knowledge base match quality */
  confidence?: number;
}
