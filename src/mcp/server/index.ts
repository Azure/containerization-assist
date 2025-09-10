/**
 * MCP Server implementation using the Model Context Protocol SDK.
 * Provides tools, resources, and prompts for containerization workflows.
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { createMCPToolContext } from '../context/tool-context-builder';
import { z } from 'zod';
import { randomUUID } from 'node:crypto';
import { analyzeRepoSchema } from '../../tools/analyze-repo/schema';
import { generateDockerfileSchema } from '../../tools/generate-dockerfile/schema';
import { buildImageSchema } from '../../tools/build-image/schema';
import { scanImageSchema } from '../../tools/scan/schema';
import { deployApplicationSchema } from '../../tools/deploy/schema';
import { pushImageSchema } from '../../tools/push-image/schema';
import { tagImageSchema } from '../../tools/tag-image/schema';
import { workflowSchema } from '../../tools/workflow/schema';
import { fixDockerfileSchema } from '../../tools/fix-dockerfile/schema';
import { resolveBaseImagesSchema } from '../../tools/resolve-base-images/schema';
import { prepareClusterSchema } from '../../tools/prepare-cluster/schema';
import { opsToolSchema } from '../../tools/ops/schema';
import { generateK8sManifestsSchema } from '../../tools/generate-k8s-manifests/schema';
import { verifyDeploymentSchema } from '../../tools/verify-deployment/schema';
import { containerizationWorkflowSchema, deploymentWorkflowSchema } from './schemas';
import { containerizationWorkflow } from '../../workflows/containerization';
import { deploymentWorkflow } from '../../workflows/deployment';
import { getContainerStatus, type Deps } from '../../container';

// Import tool functions
import { analyzeRepo } from '../../tools/analyze-repo';
import { generateDockerfile } from '../../tools/generate-dockerfile';
import { buildImage } from '../../tools/build-image';
import { scanImage } from '../../tools/scan';
import { deployApplication } from '../../tools/deploy';
import { pushImage } from '../../tools/push-image';
import { tagImage } from '../../tools/tag-image';
import { workflow } from '../../tools/workflow';
import { fixDockerfile } from '../../tools/fix-dockerfile';
import { resolveBaseImages } from '../../tools/resolve-base-images';
import { prepareCluster } from '../../tools/prepare-cluster';
import { opsTool } from '../../tools/ops';
import { generateK8sManifests } from '../../tools/generate-k8s-manifests';
import { verifyDeployment } from '../../tools/verify-deployment';

// Tool schemas map
const toolSchemas = {
  'analyze-repo': analyzeRepoSchema,
  'generate-dockerfile': generateDockerfileSchema,
  'build-image': buildImageSchema,
  scan: scanImageSchema,
  deploy: deployApplicationSchema,
  push: pushImageSchema,
  tag: tagImageSchema,
  workflow: workflowSchema,
  'fix-dockerfile': fixDockerfileSchema,
  'resolve-base-images': resolveBaseImagesSchema,
  'prepare-cluster': prepareClusterSchema,
  ops: opsToolSchema,
  'generate-k8s-manifests': generateK8sManifestsSchema,
  'verify-deployment': verifyDeploymentSchema,
  containerization: containerizationWorkflowSchema,
  deployment: deploymentWorkflowSchema,
} as const;

// Tool function map
const toolFunctions = {
  'analyze-repo': analyzeRepo,
  'generate-dockerfile': generateDockerfile,
  'build-image': buildImage,
  scan: scanImage,
  deploy: deployApplication,
  'push-image': pushImage,
  'tag-image': tagImage,
  workflow,
  'fix-dockerfile': fixDockerfile,
  'resolve-base-images': resolveBaseImages,
  'prepare-cluster': prepareCluster,
  ops: opsTool,
  'generate-k8s-manifests': generateK8sManifests,
  'verify-deployment': verifyDeployment,
} as const;

/**
 * MCP Server class that integrates containerization tools with the MCP protocol.
 * Handles tool invocation, resource management, and prompt templates.
 */
export class MCPServer {
  private server: McpServer;
  private transport: StdioServerTransport;
  private deps: Deps;
  private isRunning: boolean = false;
  private registeredToolCount: number = 0;
  private registeredResourceCount: number = 0;

  constructor(
    deps: Deps,
    options?: {
      name?: string;
      version?: string;
    },
  ) {
    this.deps = deps;

    /**
     * Invariant: Server capabilities are immutable after construction
     * Trade-off: Static capabilities vs. dynamic registration for predictable behavior
     */
    this.server = new McpServer(
      {
        name: options?.name ?? deps.config.mcp.name,
        version: options?.version ?? deps.config.mcp.version,
      },
      {
        capabilities: {
          resources: {
            subscribe: false,
            listChanged: false,
          },
          prompts: {
            listChanged: false,
          },
          tools: {
            listChanged: false,
          },
        },
      },
    );

    this.transport = new StdioServerTransport();
    /** Precondition: Handlers must be setup before connecting to prevent race conditions */
  }

  private setupHandlers(): void {
    this.registerAllTools();
    this.registerWorkflowTools();
    this.server.resource(
      'status',
      'containerization://status',
      {
        title: 'Container Status',
        description: 'Current status of the containerization system',
      },
      async () => {
        const status = getContainerStatus(this.deps, this.isRunning);
        return {
          contents: [
            {
              uri: 'containerization://status',
              mimeType: 'application/json',
              text: JSON.stringify(status, null, 2),
            },
          ],
        };
      },
    );

    this.registeredResourceCount = 1;
    this.registerPromptsFromRegistry();

    /** Postcondition: All handlers registered, server ready for connections */

    this.deps.logger.info('SDK-native handlers configured');
  }

  /**
   * Register all tools using McpServer's tool() method
   */
  private registerAllTools(): void {
    for (const [name, schema] of Object.entries(toolSchemas)) {
      /**
       * Invariant: Workflows are registered separately to maintain distinct lifecycle
       * Trade-off: Code duplication vs. cleaner separation of concerns
       */
      if (name === 'containerization' || name === 'deployment') {
        continue;
      }

      /** Rationale: SDK requires Zod shape for automatic validation */
      const schemaShape = schema?.shape || {};

      this.server.tool(name, `${name} tool`, schemaShape, async (params: any) => {
        this.deps.logger.info(
          {
            tool: name,
            hasSharedSessionManager: !!this.deps.sessionManager,
            sessionManagerType: typeof this.deps.sessionManager,
          },
          'Executing tool via McpServer handler',
        );

        try {
          /**
           * Invariant: Every tool execution must have a sessionId
           * Rationale: Enables state persistence and workflow continuity
           */
          if (!params.sessionId) {
            params.sessionId = randomUUID();
            this.deps.logger.debug(
              { sessionId: params.sessionId },
              'Generated new session ID for tool execution',
            );
          }

          const toolFunction = toolFunctions[name as keyof typeof toolFunctions];
          if (!toolFunction) {
            throw new McpError(ErrorCode.MethodNotFound, `Tool function not found: ${name}`);
          }

          const context = createMCPToolContext(
            this.getServer(),
            {},
            this.deps.logger.child({ tool: name }),
            {
              promptRegistry: this.deps.promptRegistry,
              sessionManager: this.deps.sessionManager,
            },
          );

          const result = await toolFunction(params, context);

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

          return {
            content: [
              {
                type: 'text' as const,
                text: JSON.stringify(result, null, 2),
              },
            ],
          };
        } catch (error) {
          this.deps.logger.error({ tool: name, error }, 'Tool execution failed');

          if (error instanceof McpError) {
            throw error;
          }

          throw new McpError(
            ErrorCode.InternalError,
            error instanceof Error ? error.message : 'Unknown error occurred',
          );
        }
      });
    }

    this.registeredToolCount = Object.keys(toolSchemas).length - 2;

    this.deps.logger.info(
      { count: this.registeredToolCount },
      'Tools registered with McpServer SDK',
    );
  }

  /**
   * Register workflow tools using McpServer's tool() method
   */
  private registerWorkflowTools(): void {
    this.server.tool(
      'containerization',
      'Containerization workflow',
      containerizationWorkflowSchema.shape,
      async (params) => {
        this.deps.logger.info(
          { workflow: 'containerization' },
          'Executing containerization workflow',
        );

        /**
         * Invariant: Workflow params must include projectPath and sessionId
         * Rationale: Workflows require context for multi-step operations
         */
        const workflowParams = {
          ...params,
          projectPath: params.repoPath || this.deps.config.workspace.workspaceDir,
          sessionId: params.sessionId || randomUUID(),
        };

        if (!params.sessionId) {
          this.deps.logger.debug(
            { sessionId: workflowParams.sessionId },
            'Generated new session ID for containerization workflow',
          );
        }

        const toolContext = createMCPToolContext(
          this.getServer(),
          {},
          this.deps.logger.child({ workflow: 'containerization' }),
          {
            promptRegistry: this.deps.promptRegistry,
            sessionManager: this.deps.sessionManager,
          },
        );

        /** Trade-off: Extended context maintains backward compatibility while migrating to new pattern */
        const extendedContext = {
          ...toolContext,
          deps: this.deps,
        };

        const result = await containerizationWorkflow.execute(
          workflowParams,
          this.deps.logger,
          extendedContext,
        );

        return {
          content: [
            {
              type: 'text' as const,
              text: JSON.stringify(result, null, 2),
            },
          ],
        };
      },
    );

    this.server.tool(
      'deployment',
      'Deployment workflow',
      deploymentWorkflowSchema.shape,
      async (params) => {
        this.deps.logger.info({ workflow: 'deployment' }, 'Executing deployment workflow');

        const generatedSessionId = params.sessionId || randomUUID();
        if (!params.sessionId) {
          this.deps.logger.debug(
            { sessionId: generatedSessionId },
            'Generated new session ID for deployment workflow',
          );
        }
        const workflowParams = {
          sessionId: generatedSessionId,
          imageId: params.imageId || 'latest',
          clusterConfig: {
            namespace: params.namespace || this.deps.config.kubernetes.namespace,
            context: params.clusterType || 'default',
          },
          deploymentOptions: {
            name: 'deployment',
            replicas: 1,
          },
        };

        // Create ToolContext for the workflow with sessionManager
        const toolContext = createMCPToolContext(
          this.getServer(),
          {}, // empty request object since we don't have access to it here
          this.deps.logger.child({ workflow: 'deployment' }),
          {
            promptRegistry: this.deps.promptRegistry,
            sessionManager: this.deps.sessionManager,
          },
        );

        const result = await deploymentWorkflow.execute(
          workflowParams,
          this.deps.logger,
          toolContext as unknown as Record<string, unknown>,
        );

        return {
          content: [
            {
              type: 'text' as const,
              text: JSON.stringify(result, null, 2),
            },
          ],
        };
      },
    );

    // Add workflow tools to the count
    this.registeredToolCount += 2;

    this.deps.logger.info('Workflow tools registered with McpServer');
  }

  /**
   * Register prompts from the prompt registry dynamically
   */
  private registerPromptsFromRegistry(): void {
    const promptNames = this.deps.promptRegistry.getPromptNames();

    for (const name of promptNames) {
      const promptInfo = this.deps.promptRegistry.getPromptInfo(name);
      if (!promptInfo) continue;

      // Convert PromptArguments to Zod schema shape
      const schemaShape: Record<string, z.ZodType> = {};
      for (const arg of promptInfo.arguments) {
        const description = arg.description || `${arg.name} parameter`;
        if (arg.required) {
          schemaShape[arg.name] = z.string().describe(description);
        } else {
          schemaShape[arg.name] = z.string().optional().describe(description);
        }
      }

      // Register with the MCP server
      this.server.prompt(
        name,
        promptInfo.description || `Prompt: ${name}`,
        schemaShape,
        async (params) => {
          try {
            const result = await this.deps.promptRegistry.getPrompt(name, params);
            return result;
          } catch (error) {
            throw new McpError(
              ErrorCode.MethodNotFound,
              error instanceof Error ? error.message : `Prompt not found: ${name}`,
            );
          }
        },
      );
    }

    this.deps.logger.info({ count: promptNames.length }, 'Prompts registered from registry');
  }

  /**
   * Start the server
   */
  async start(): Promise<void> {
    if (this.isRunning) {
      this.deps.logger.warn('Server is already running');
      return;
    }

    try {
      this.deps.logger.info('Starting MCP server connection...');

      // Setup handlers before connecting
      this.setupHandlers();

      // Connect the transport to the server
      await this.server.connect(this.transport);
      this.isRunning = true;

      this.deps.logger.info('MCP server connection established successfully');

      // Log actual registered counts
      this.deps.logger.info(
        {
          tools: this.registeredToolCount,
          resources: this.registeredResourceCount,
          prompts: this.deps.promptRegistry.getPromptNames().length,
          workflows: 2,
          healthy: true,
        },
        'MCP server started',
      );
    } catch (error) {
      this.deps.logger.error({ error }, 'Failed to start server');
      throw error;
    }
  }

  /**
   * Stop the server
   */
  async stop(): Promise<void> {
    if (!this.isRunning) {
      this.deps.logger.warn('Server is not running');
      return;
    }

    try {
      await this.server.close();
      this.isRunning = false;
      this.deps.logger.info('Server stopped');
    } catch (error) {
      this.deps.logger.error({ error }, 'Failed to stop server');
      throw error;
    }
  }

  /**
   * Get the underlying MCP SDK server instance for sampling/AI requests
   * Used by tools for making sampling and prompt requests
   */
  getServer(): Server {
    return this.server.server;
  }

  /**
   * Get server status
   * Delegates to the container for single source of truth
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
      tools: this.registeredToolCount,
      resources: this.registeredResourceCount,
      prompts: this.deps.promptRegistry.getPromptNames().length,
      workflows: 2,
    };
  }

  /**
   * Get list of available tools with their descriptions
   */
  getTools(): Array<{ name: string; description: string }> {
    const tools = Object.keys(toolSchemas).map((name) => ({
      name,
      description: `${name} tool`,
    }));

    // Add workflow tools
    const workflowTools = this.getWorkflows();

    return [...tools, ...workflowTools];
  }

  /**
   * Get list of workflow tools
   */
  getWorkflows(): Array<{ name: string; description: string }> {
    return [
      {
        name: 'start_workflow',
        description: 'Start a complete containerization workflow',
      },
      {
        name: 'workflow_status',
        description: 'Get the status of a running workflow',
      },
    ];
  }
}
