import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'generate_azure_container_apps_manifests',
  title: 'Generate Azure Container Apps Manifests',
  description: 'Generate Azure Container Apps deployment manifests (Bicep or ARM templates)',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    resource_group: z.string().optional().describe('Azure resource group name'),
    location: z.string().optional().describe('Azure region (e.g., eastus, westeurope)'),
    environment_name: z.string().optional().describe('Container Apps Environment name'),
    output_format: z.enum(['bicep', 'arm']).optional().default('bicep')
      .describe('Output format for manifests'),
    registry_url: z.string().optional().describe('Container registry URL (e.g., myregistry.azurecr.io)'),
    enable_dapr: z.boolean().optional().describe('Enable Dapr integration'),
    dapr_app_id: z.string().optional().describe('Dapr application ID'),
    dapr_app_port: z.number().optional().describe('Dapr application port'),
    custom_domain: z.string().optional().describe('Custom domain for the app'),
    min_replicas: z.number().optional().default(1).describe('Minimum replicas'),
    max_replicas: z.number().optional().default(10).describe('Maximum replicas'),
    cpu: z.number().optional().default(0.5).describe('CPU cores (e.g., 0.5, 1.0)'),
    memory: z.string().optional().default('1.0Gi').describe('Memory (e.g., 1.0Gi, 2.0Gi)'),
    environment_variables: z.record(z.string()).optional()
      .describe('Environment variables for the container'),
    secrets: z.record(z.string()).optional()
      .describe('Secrets to be stored in Azure Key Vault'),
    managed_identity: z.boolean().optional()
      .describe('Enable managed identity for the app')
  }
});