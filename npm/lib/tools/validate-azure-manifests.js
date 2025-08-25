import { createTool, z } from './_tool-factory.js';

export default createTool({
  name: 'validate_azure_manifests',
  title: 'Validate Azure Manifests',
  description: 'Validate Azure Container Apps manifests (Bicep or ARM templates)',
  inputSchema: {
    session_id: z.string().describe('Session ID from workflow'),
    manifest_path: z.string().optional()
      .describe('Path to the manifest file to validate'),
    strict_mode: z.boolean().optional().default(false)
      .describe('Enable strict validation with additional checks'),
    check_azure_limits: z.boolean().optional().default(true)
      .describe('Check against Azure service limits'),
    validate_dependencies: z.boolean().optional().default(true)
      .describe('Validate resource dependencies')
  }
});