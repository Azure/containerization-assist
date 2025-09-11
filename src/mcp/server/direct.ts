/**
 * Direct MCP Server Implementation
 *
 * Uses SDK-native patterns directly with minimal wrapping.
 * Combines tool registration and execution in a cleaner pattern.
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
import { randomUUID } from 'node:crypto';
import { getContainerStatus, type Deps } from '../../container';
import { createMCPToolContext } from '../context/tool-context-builder';

// Single unified tool definition structure
interface ToolDefinition {
  name: string;
  description: string;
  schema: any; // Zod schema shape
  handler: (params: any, context: any) => Promise<any>;
}

// Import all tools with their schemas and handlers in one place
import { analyzeRepo } from '../../tools/analyze-repo';
import { analyzeRepoSchema } from '../../tools/analyze-repo/schema';
import { generateDockerfile } from '../../tools/generate-dockerfile';
import { generateDockerfileSchema } from '../../tools/generate-dockerfile/schema';
import { buildImage } from '../../tools/build-image';
import { buildImageSchema } from '../../tools/build-image/schema';
import { scanImage } from '../../tools/scan';
import { scanImageSchema } from '../../tools/scan/schema';
import { deployApplication } from '../../tools/deploy';
import { deployApplicationSchema } from '../../tools/deploy/schema';
import { pushImage } from '../../tools/push-image';
import { pushImageSchema } from '../../tools/push-image/schema';
import { tagImage } from '../../tools/tag-image';
import { tagImageSchema } from '../../tools/tag-image/schema';
import { workflow } from '../../tools/workflow';
import { workflowSchema } from '../../tools/workflow/schema';
import { fixDockerfile } from '../../tools/fix-dockerfile';
import { fixDockerfileSchema } from '../../tools/fix-dockerfile/schema';
import { resolveBaseImages } from '../../tools/resolve-base-images';
import { resolveBaseImagesSchema } from '../../tools/resolve-base-images/schema';
import { prepareCluster } from '../../tools/prepare-cluster';
import { prepareClusterSchema } from '../../tools/prepare-cluster/schema';
import { opsTool } from '../../tools/ops';
import { opsToolSchema } from '../../tools/ops/schema';
import { generateK8sManifests } from '../../tools/generate-k8s-manifests';
import { generateK8sManifestsSchema } from '../../tools/generate-k8s-manifests/schema';
import { verifyDeployment } from '../../tools/verify-deployment';
import { verifyDeploymentSchema } from '../../tools/verify-deployment/schema';

// Unified tool definitions
const TOOLS: ToolDefinition[] = [
  {
    name: 'analyze-repo',
    description: 'Analyze repository structure',
    schema: analyzeRepoSchema,
    handler: analyzeRepo,
  },
  {
    name: 'generate-dockerfile',
    description: 'Generate optimized Dockerfile',
    schema: generateDockerfileSchema,
    handler: generateDockerfile,
  },
  {
    name: 'build-image',
    description: 'Build Docker image',
    schema: buildImageSchema,
    handler: buildImage,
  },
  {
    name: 'scan',
    description: 'Scan image for vulnerabilities',
    schema: scanImageSchema,
    handler: scanImage,
  },
  {
    name: 'deploy',
    description: 'Deploy application',
    schema: deployApplicationSchema,
    handler: deployApplication,
  },
  {
    name: 'push-image',
    description: 'Push image to registry',
    schema: pushImageSchema,
    handler: pushImage,
  },
  { name: 'tag-image', description: 'Tag Docker image', schema: tagImageSchema, handler: tagImage },
  { name: 'workflow', description: 'Execute workflow', schema: workflowSchema, handler: workflow },
  {
    name: 'fix-dockerfile',
    description: 'Fix Dockerfile issues',
    schema: fixDockerfileSchema,
    handler: fixDockerfile,
  },
  {
    name: 'resolve-base-images',
    description: 'Resolve optimal base images',
    schema: resolveBaseImagesSchema,
    handler: resolveBaseImages,
  },
  {
    name: 'prepare-cluster',
    description: 'Prepare Kubernetes cluster',
    schema: prepareClusterSchema,
    handler: prepareCluster,
  },
  { name: 'ops', description: 'Operational tools', schema: opsToolSchema, handler: opsTool },
  {
    name: 'generate-k8s-manifests',
    description: 'Generate K8s manifests',
    schema: generateK8sManifestsSchema,
    handler: generateK8sManifests,
  },
  {
    name: 'verify-deployment',
    description: 'Verify deployment status',
    schema: verifyDeploymentSchema,
    handler: verifyDeployment,
  },
];

/**
 * Direct MCP Server - uses SDK patterns without unnecessary wrappers
 */
export class DirectMCPServer {
  private server: McpServer;
  private transport: StdioServerTransport;
  private isRunning = false;

  constructor(private deps: Deps) {
    // Create SDK server directly with capabilities
    this.server = new McpServer(
      {
        name: deps.config.mcp.name,
        version: deps.config.mcp.version,
      },
      {
        capabilities: {
          resources: { subscribe: false, listChanged: false },
          prompts: { listChanged: false },
          tools: { listChanged: false },
        },
      },
    );

    this.transport = new StdioServerTransport();
  }

  /**
   * Register all tools and resources directly with SDK
   */
  private async registerHandlers(): Promise<void> {
    // Register tools using direct SDK pattern
    for (const tool of TOOLS) {
      this.server.tool(
        tool.name,
        tool.description,
        tool.schema?.shape || {},
        this.createToolHandler(tool),
      );
    }

    // Register status resource directly
    this.server.resource(
      'status',
      'containerization://status',
      {
        title: 'Container Status',
        description: 'Current status of the containerization system',
      },
      async () => ({
        contents: [
          {
            uri: 'containerization://status',
            mimeType: 'application/json',
            text: JSON.stringify(getContainerStatus(this.deps, this.isRunning), null, 2),
          },
        ],
      }),
    );

    // Register prompts from registry
    await this.registerPrompts();

    this.deps.logger.info(`Registered ${TOOLS.length} tools via SDK`);
  }

  /**
   * Create a standardized tool handler
   */
  private createToolHandler(tool: ToolDefinition) {
    return async (params: any) => {
      this.deps.logger.info({ tool: tool.name }, 'Executing tool');

      try {
        // Ensure sessionId
        if (!params.sessionId) {
          params.sessionId = randomUUID();
        }

        // Create context with all dependencies
        const context = createMCPToolContext(
          this.server.server,
          {},
          this.deps.logger.child({ tool: tool.name }),
          {
            promptRegistry: this.deps.promptRegistry,
            sessionManager: this.deps.sessionManager,
          },
        );

        // Execute tool
        const result = await tool.handler(params, context);

        // Handle Result pattern
        if ('ok' in result) {
          if (result.ok) {
            return {
              content: [
                {
                  type: 'text' as const,
                  text: JSON.stringify(result.value, null, 2),
                },
              ],
            };
          } else {
            throw new McpError(ErrorCode.InternalError, result.error);
          }
        }

        // Direct return
        return {
          content: [
            {
              type: 'text' as const,
              text: JSON.stringify(result, null, 2),
            },
          ],
        };
      } catch (error) {
        this.deps.logger.error({ tool: tool.name, error }, 'Tool execution failed');

        if (error instanceof McpError) {
          throw error;
        }

        throw new McpError(
          ErrorCode.InternalError,
          error instanceof Error ? error.message : 'Unknown error occurred',
        );
      }
    };
  }

  /**
   * Register prompts directly from registry
   */
  private async registerPrompts(): Promise<void> {
    const promptNames = this.deps.promptRegistry.getPromptNames();

    for (const name of promptNames) {
      const info = this.deps.promptRegistry.getPromptInfo(name);
      if (!info) continue;

      // Build schema from arguments
      const schemaShape: Record<string, any> = {};
      const zod = await import('zod');
      const { z } = zod;
      for (const arg of info.arguments) {
        schemaShape[arg.name] = arg.required
          ? z.string().describe(arg.description || arg.name)
          : z
              .string()
              .optional()
              .describe(arg.description || arg.name);
      }

      // Register directly with SDK
      this.server.prompt(name, info.description, schemaShape, async (params) => {
        try {
          return await this.deps.promptRegistry.getPrompt(name, params);
        } catch (error) {
          throw new McpError(
            ErrorCode.MethodNotFound,
            error instanceof Error ? error.message : `Prompt not found: ${name}`,
          );
        }
      });
    }

    this.deps.logger.info(`Registered ${promptNames.length} prompts`);
  }

  /**
   * Start the server
   */
  async start(): Promise<void> {
    if (this.isRunning) {
      this.deps.logger.warn('Server already running');
      return;
    }

    await this.registerHandlers();
    await this.server.connect(this.transport);
    this.isRunning = true;

    this.deps.logger.info(
      {
        tools: TOOLS.length,
        prompts: this.deps.promptRegistry.getPromptNames().length,
        healthy: true,
      },
      'MCP server started',
    );
  }

  /**
   * Stop the server
   */
  async stop(): Promise<void> {
    if (!this.isRunning) {
      return;
    }

    await this.server.close();
    this.isRunning = false;
    this.deps.logger.info('Server stopped');
  }

  /**
   * Get SDK server instance for sampling
   */
  getServer(): any {
    return this.server.server;
  }

  /**
   * Get server status
   */
  getStatus(): {
    running: boolean;
    tools: number;
    resources: number;
    prompts: number;
    workflows: number;
  } {
    return {
      running: this.isRunning,
      tools: TOOLS.length,
      resources: 1,
      prompts: this.deps.promptRegistry.getPromptNames().length,
      workflows: 2,
    };
  }
}
