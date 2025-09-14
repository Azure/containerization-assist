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
import { getSystemStatus, type Dependencies } from '../container';
import { createMCPToolContext, type ToolContext } from './context';
import { extractErrorMessage } from '../lib/error-utils';
import { ToolRouter } from './tool-router';

// Single unified tool definition structure
interface ToolDefinition {
  name: string;
  description: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  schema: any; // Zod schema object (needs to be any for .shape property)
  handler: (params: Record<string, unknown>, context: ToolContext) => Promise<Result<unknown>>;
}

// Import all tools with their schemas and handlers in one place
import { analyzeRepo } from '../tools/analyze-repo';
import { analyzeRepoSchema } from '../tools/analyze-repo/schema';
import type { Result } from '../types';
import { generateDockerfile } from '../tools/generate-dockerfile';
import { generateDockerfileSchema } from '../tools/generate-dockerfile/schema';
import { buildImage } from '../tools/build-image';
import { buildImageSchema } from '../tools/build-image/schema';
import { scanImage } from '../tools/scan';
import { scanImageSchema } from '../tools/scan/schema';
import { deployApplication } from '../tools/deploy';
import { deployApplicationSchema } from '../tools/deploy/schema';
import { pushImage } from '../tools/push-image';
import { pushImageSchema } from '../tools/push-image/schema';
import { tagImage } from '../tools/tag-image';
import { tagImageSchema } from '../tools/tag-image/schema';
import { fixDockerfile } from '../tools/fix-dockerfile';
import { fixDockerfileSchema } from '../tools/fix-dockerfile/schema';
import { resolveBaseImages } from '../tools/resolve-base-images';
import { resolveBaseImagesSchema } from '../tools/resolve-base-images/schema';
import { prepareCluster } from '../tools/prepare-cluster';
import { prepareClusterSchema } from '../tools/prepare-cluster/schema';
import { opsTool } from '../tools/ops';
import { opsToolSchema } from '../tools/ops/schema';
import { generateK8sManifests } from '../tools/generate-k8s-manifests';
import { generateK8sManifestsSchema } from '../tools/generate-k8s-manifests/schema';
import { verifyDeployment } from '../tools/verify-deployment';
import { verifyDeploymentSchema } from '../tools/verify-deployment/schema';
import { generateHelmCharts } from '../tools/generate-helm-charts';
import { generateHelmChartsSchema } from '../tools/generate-helm-charts/schema';
import { generateAcaManifests } from '../tools/generate-aca-manifests';
import { generateAcaManifestsSchema } from '../tools/generate-aca-manifests/schema';
import { convertAcaToK8s } from '../tools/convert-aca-to-k8s';
import { convertAcaToK8sSchema } from '../tools/convert-aca-to-k8s/schema';
import { inspectSession } from '../tools/inspect-session';
import { InspectSessionParamsSchema as inspectSessionSchema } from '../tools/inspect-session/schema';

// Unified tool definitions
const TOOLS: ToolDefinition[] = [
  {
    name: 'analyze-repo',
    description: 'Analyze repository structure',
    schema: analyzeRepoSchema,
    handler: analyzeRepo as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate-dockerfile',
    description: 'Generate optimized Dockerfile',
    schema: generateDockerfileSchema,
    handler: generateDockerfile as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'build-image',
    description: 'Build Docker image',
    schema: buildImageSchema,
    handler: buildImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'scan-image',
    description: 'Scan image for vulnerabilities',
    schema: scanImageSchema,
    handler: scanImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'deploy',
    description: 'Deploy application',
    schema: deployApplicationSchema,
    handler: deployApplication as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'push-image',
    description: 'Push image to registry',
    schema: pushImageSchema,
    handler: pushImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'tag-image',
    description: 'Tag Docker image',
    schema: tagImageSchema,
    handler: tagImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'fix-dockerfile',
    description: 'Fix Dockerfile issues',
    schema: fixDockerfileSchema,
    handler: fixDockerfile as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'resolve-base-images',
    description: 'Resolve optimal base images',
    schema: resolveBaseImagesSchema,
    handler: resolveBaseImages as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'prepare-cluster',
    description: 'Prepare Kubernetes cluster',
    schema: prepareClusterSchema,
    handler: prepareCluster as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'ops',
    description: 'Operational tools',
    schema: opsToolSchema,
    handler: opsTool as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate-k8s-manifests',
    description: 'Generate K8s manifests',
    schema: generateK8sManifestsSchema,
    handler: generateK8sManifests as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'verify-deployment',
    description: 'Verify deployment status',
    schema: verifyDeploymentSchema,
    handler: verifyDeployment as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate-helm-charts',
    description: 'Generate Helm charts for Kubernetes deployment',
    schema: generateHelmChartsSchema,
    handler: generateHelmCharts as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate-aca-manifests',
    description: 'Generate Azure Container Apps manifests',
    schema: generateAcaManifestsSchema,
    handler: generateAcaManifests as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'convert-aca-to-k8s',
    description: 'Convert Azure Container Apps to Kubernetes manifests',
    schema: convertAcaToK8sSchema,
    handler: convertAcaToK8s as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'inspect-session',
    description: 'Inspect current session state',
    schema: inspectSessionSchema,
    handler: inspectSession as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
];

/**
 * Direct MCP Server - uses SDK patterns without unnecessary wrappers
 */
export class DirectMCPServer {
  private server: McpServer;
  private transport: StdioServerTransport;
  private isRunning = false;
  private router?: ToolRouter;
  private toolMap = new Map<string, ToolDefinition>();

  constructor(private deps: Dependencies) {
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
    // Initialize router with tools
    this.initializeRouter();

    // Register tools using direct SDK pattern
    for (const tool of TOOLS) {
      this.toolMap.set(tool.name, tool);
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
            text: JSON.stringify(getSystemStatus(this.deps, this.isRunning), null, 2),
          },
        ],
      }),
    );

    // Register prompts from registry
    await this.registerPrompts();

    this.deps.logger.info(`Registered ${TOOLS.length} tools via SDK`);
  }

  /**
   * Initialize the tool router
   */
  private initializeRouter(): void {
    // Create tools map for router
    const tools = new Map<string, import('./tool-router').RouterTool>();
    for (const tool of TOOLS) {
      tools.set(tool.name, {
        name: tool.name,
        handler: tool.handler,
        schema: tool.schema,
      });
    }

    // Initialize router with dependencies
    this.router = new ToolRouter({
      sessionManager: this.deps.sessionManager,
      logger: this.deps.logger,
      tools,
    });
  }

  /**
   * Create a standardized tool handler
   */
  private createToolHandler(tool: ToolDefinition) {
    return async (params: Record<string, unknown>) => {
      this.deps.logger.info({ tool: tool.name }, 'Executing tool');

      try {
        // Ensure sessionId
        const sessionId =
          params && typeof params === 'object' && params.sessionId
            ? String(params.sessionId)
            : randomUUID();

        if (params && typeof params === 'object') {
          params.sessionId = sessionId;
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

        // Use router if available for intelligent routing
        if (this.router) {
          const forceFlag = params.force === true;
          const routeResult = await this.router.route({
            toolName: tool.name,
            params,
            sessionId,
            context,
            ...(forceFlag && { force: true }),
          });

          // Log executed tools for debugging
          if (routeResult.executedTools.length > 0) {
            this.deps.logger.info(
              { executedTools: routeResult.executedTools },
              'Router executed tools in sequence',
            );
          }

          const result = routeResult.result;

          // Handle Result pattern
          if (result && typeof result === 'object' && 'ok' in result) {
            const typedResult = result;
            if (typedResult.ok) {
              return {
                content: [
                  {
                    type: 'text' as const,
                    text: JSON.stringify(typedResult.value, null, 2),
                  },
                ],
              };
            } else {
              throw new McpError(ErrorCode.InternalError, typedResult.error);
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
        }

        // Fallback to direct execution if no router
        const result = await tool.handler(params, context);

        // Handle Result pattern
        if (result && typeof result === 'object' && 'ok' in result) {
          const typedResult = result;
          if (typedResult.ok) {
            return {
              content: [
                {
                  type: 'text' as const,
                  text: JSON.stringify(typedResult.value, null, 2),
                },
              ],
            };
          } else {
            throw new McpError(ErrorCode.InternalError, typedResult.error);
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

        throw new McpError(ErrorCode.InternalError, extractErrorMessage(error));
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
          throw new McpError(ErrorCode.MethodNotFound, extractErrorMessage(error));
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
  getServer(): unknown {
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
      workflows: 0,
    };
  }

  /**
   * Get available tools for CLI listing
   */
  getTools(): Array<{ name: string; description: string }> {
    return TOOLS.map((tool) => ({
      name: tool.name,
      description: tool.description,
    }));
  }

  /**
   * Get available workflows for CLI listing
   * @deprecated Workflows are deprecated in favor of intelligent tool routing
   */
  getWorkflows(): Array<{ name: string; description: string }> {
    return [];
  }
}
