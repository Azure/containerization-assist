import { generateGithubWorkflowSchema } from './schema';
import { TOOL_NAME, type IToolDefinition } from '../shared/toolDefinition';

export const generateGithubWorkflowToolDefinition = {
  name: TOOL_NAME.GENERATE_GITHUB_WORKFLOW,
  description:
    'Gather insights from the knowledge base and return requirements for generating a GitHub Actions CI/CD workflow that builds a Docker image, pushes it to Azure Container Registry, and deploys it to AKS using Azure OIDC federated credentials.',
  category: 'docker' as const,
  version: '1.0.0',
  schema: generateGithubWorkflowSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'GitHub Actions workflow plan generated. Next: commit .github/workflows/deploy.yml to your repository, then configure AZURE_CLIENT_ID, AZURE_TENANT_ID, and AZURE_SUBSCRIPTION_ID as GitHub repository secrets. Set up an OIDC federated credential in Azure Entra ID pointing to your GitHub repository and branch.',
    failure:
      'Failed to generate GitHub Actions workflow plan. Ensure registry, clusterName, and resourceGroup are provided.',
  },
} satisfies IToolDefinition;
