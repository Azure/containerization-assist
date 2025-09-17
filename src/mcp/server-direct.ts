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
import { getSystemStatus, type Dependencies } from '@/container';
import { createMCPToolContext, type ToolContext } from './context';
import { extractErrorMessage } from '@/lib/error-utils';
import { createToolRouter, type ToolRouter } from './tool-router';

// Single unified tool definition structure
interface ToolDefinition {
  name: string;
  description: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  schema: any; // Zod schema object (needs to be any for .shape property)
  handler: (params: Record<string, unknown>, context: ToolContext) => Promise<Result<unknown>>;
}

// Import all tools with their schemas and handlers in one place
import { analyzeRepo } from '@/tools/analyze-repo/tool';
import { analyzeRepoSchema } from '@/tools/analyze-repo/schema';
import type { Result } from '@/types';
import { generateDockerfile } from '@/tools/generate-dockerfile/tool';
import { generateDockerfileSchema } from '@/tools/generate-dockerfile/schema';
import { buildImage } from '@/tools/build-image/tool';
import { buildImageSchema } from '@/tools/build-image/schema';
import { scanImage } from '@/tools/scan/tool';
import { scanImageSchema } from '@/tools/scan/schema';
import { deployApplication } from '@/tools/deploy/tool';
import { deployApplicationSchema } from '@/tools/deploy/schema';
import { pushImage } from '@/tools/push-image/tool';
import { pushImageSchema } from '@/tools/push-image/schema';
import { tagImage } from '@/tools/tag-image/tool';
import { tagImageSchema } from '@/tools/tag-image/schema';
import { fixDockerfile } from '@/tools/fix-dockerfile/tool';
import { fixDockerfileSchema } from '@/tools/fix-dockerfile/schema';
import { resolveBaseImages } from '@/tools/resolve-base-images/tool';
import { resolveBaseImagesSchema } from '@/tools/resolve-base-images/schema';
import { prepareCluster } from '@/tools/prepare-cluster/tool';
import { prepareClusterSchema } from '@/tools/prepare-cluster/schema';
import { opsTool } from '@/tools/ops/tool';
import { opsToolSchema } from '@/tools/ops/schema';
import { generateK8sManifests } from '@/tools/generate-k8s-manifests/tool';
import { generateK8sManifestsSchema } from '@/tools/generate-k8s-manifests/schema';
import { verifyDeployment } from '@/tools/verify-deployment/tool';
import { verifyDeploymentSchema } from '@/tools/verify-deployment/schema';
import { generateHelmCharts } from '@/tools/generate-helm-charts/tool';
import { generateHelmChartsSchema } from '@/tools/generate-helm-charts/schema';
import { generateAcaManifests } from '@/tools/generate-aca-manifests/tool';
import { generateAcaManifestsSchema } from '@/tools/generate-aca-manifests/schema';
import { convertAcaToK8s } from '@/tools/convert-aca-to-k8s/tool';
import { convertAcaToK8sSchema } from '@/tools/convert-aca-to-k8s/schema';
import { inspectSession } from '@/tools/inspect-session/tool';
import { InspectSessionParamsSchema as inspectSessionSchema } from '@/tools/inspect-session/schema';

// Unified tool definitions
const TOOLS: ToolDefinition[] = [
  {
    name: 'analyze_repo',
    description: 'Analyze repository structure',
    schema: analyzeRepoSchema,
    handler: analyzeRepo as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate_dockerfile',
    description: 'Generate optimized Dockerfile',
    schema: generateDockerfileSchema,
    handler: generateDockerfile as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'build_image',
    description: 'Build Docker image',
    schema: buildImageSchema,
    handler: buildImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'scan',
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
    name: 'push_image',
    description: 'Push image to registry',
    schema: pushImageSchema,
    handler: pushImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'tag_image',
    description: 'Tag Docker image',
    schema: tagImageSchema,
    handler: tagImage as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'fix_dockerfile',
    description: 'Fix Dockerfile issues',
    schema: fixDockerfileSchema,
    handler: fixDockerfile as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'resolve_base_images',
    description: 'Resolve optimal base images',
    schema: resolveBaseImagesSchema,
    handler: resolveBaseImages as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'prepare_cluster',
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
    name: 'generate_k8s_manifests',
    description: 'Generate K8s manifests',
    schema: generateK8sManifestsSchema,
    handler: generateK8sManifests as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'verify_deployment',
    description: 'Verify deployment status',
    schema: verifyDeploymentSchema,
    handler: verifyDeployment as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate_helm_charts',
    description: 'Generate Helm charts for Kubernetes deployment',
    schema: generateHelmChartsSchema,
    handler: generateHelmCharts as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'generate_aca_manifests',
    description: 'Generate Azure Container Apps manifests',
    schema: generateAcaManifestsSchema,
    handler: generateAcaManifests as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'convert_aca_to_k8s',
    description: 'Convert Azure Container Apps to Kubernetes manifests',
    schema: convertAcaToK8sSchema,
    handler: convertAcaToK8s as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
  {
    name: 'inspect_session',
    description: 'Inspect current session state',
    schema: inspectSessionSchema,
    handler: inspectSession as (
      params: Record<string, unknown>,
      context: ToolContext,
    ) => Promise<Result<unknown>>,
  },
];

/**
 * MCP Server state interface
 */
export interface MCPServerState {
  server: McpServer;
  transport: StdioServerTransport;
  isRunning: boolean;
  router?: ToolRouter;
  toolMap: Map<string, ToolDefinition>;
  deps: Dependencies;
}

/**
 * MCP Server interface for dependency injection compatibility
 */
export interface IDirectMCPServer {
  start(): Promise<void>;
  stop(): Promise<void>;
  getServer(): unknown;
  getStatus(): {
    running: boolean;
    tools: number;
    resources: number;
    prompts: number;
  };
  getTools(): Array<{ name: string; description: string }>;
}

/**
 * Register all tools and resources directly with SDK
 */
export const registerHandlers = async (state: MCPServerState): Promise<void> => {
  // Initialize router with tools
  initializeRouter(state);

  // Register tools using direct SDK pattern
  for (const tool of TOOLS) {
    state.toolMap.set(tool.name, tool);
    state.server.tool(
      tool.name,
      tool.description,
      tool.schema?.shape || {},
      createToolHandler(state, tool),
    );
  }

  // Register status resource directly
  state.server.resource(
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
          text: JSON.stringify(getSystemStatus(state.deps, state.isRunning), null, 2),
        },
      ],
    }),
  );

  // Register prompts from registry
  await registerPrompts(state);

  state.deps.logger.info(`Registered ${TOOLS.length} tools via SDK`);
};

/**
 * Initialize the tool router
 */
export const initializeRouter = (state: MCPServerState): void => {
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
  state.router = createToolRouter({
    sessionManager: state.deps.sessionManager,
    logger: state.deps.logger,
    tools,
  });
};

/**
 * Create a standardized tool handler
 */
export const createToolHandler = (state: MCPServerState, tool: ToolDefinition) => {
  return async (params: Record<string, unknown>) => {
    state.deps.logger.info({ tool: tool.name }, 'Executing tool');

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
        state.server.server,
        {},
        state.deps.logger.child({ tool: tool.name }),
        {
          promptRegistry: state.deps.promptRegistry,
          sessionManager: state.deps.sessionManager,
        },
      );

      // Always use router for intelligent routing and dependency resolution
      if (!state.router) {
        throw new McpError(
          ErrorCode.InternalError,
          'Router not initialized - this should never happen',
        );
      }

      const forceFlag = params.force === true;
      const routeResult = await state.router.route({
        toolName: tool.name,
        params,
        sessionId,
        context,
        ...(forceFlag && { force: true }),
      });

      // Log executed tools for debugging
      if (routeResult.executedTools.length > 0) {
        state.deps.logger.info(
          { executedTools: routeResult.executedTools },
          'Router executed tools in sequence',
        );
      }

      // Log workflow hint if present
      if (routeResult.workflowHint) {
        state.deps.logger.info(
          { hint: routeResult.workflowHint.message },
          'Workflow continuation available',
        );
      }

      const result = routeResult.result;

      // Handle Result pattern
      if (result && typeof result === 'object' && 'ok' in result) {
        const typedResult = result;
        if (typedResult.ok) {
          // Extract sessionId from the result for highlighting
          const resultValue = typedResult.value as any;
          const sessionId = resultValue?.sessionId;

          const content = [
            {
              type: 'text' as const,
              text: JSON.stringify(typedResult.value, null, 2),
            },
          ];

          // Add session continuation reminder if sessionId exists
          if (sessionId) {
            content.push({
              type: 'text' as const,
              text: `\n🔗 **SESSION CONTINUATION:** Use sessionId "${sessionId}" in your next tool call to share analysis data`,
            });
          }

          // Add workflow hint as separate content block if present
          if (routeResult.workflowHint) {
            content.push({
              type: 'text' as const,
              text: `\n---\n${routeResult.workflowHint.markdown}`,
            });
          }

          return { content };
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
      state.deps.logger.error({ tool: tool.name, error }, 'Tool execution failed');

      if (error instanceof McpError) {
        throw error;
      }

      throw new McpError(ErrorCode.InternalError, extractErrorMessage(error));
    }
  };
};

/**
 * Register prompts directly from registry
 */
export const registerPrompts = async (state: MCPServerState): Promise<void> => {
  const promptNames = state.deps.promptRegistry.getPromptNames();

  for (const name of promptNames) {
    const info = state.deps.promptRegistry.getPromptInfo(name);
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
    state.server.prompt(name, info.description, schemaShape, async (params) => {
      try {
        return await state.deps.promptRegistry.getPrompt(name, params);
      } catch (error) {
        throw new McpError(ErrorCode.MethodNotFound, extractErrorMessage(error));
      }
    });
  }

  state.deps.logger.info(`Registered ${promptNames.length} prompts`);
};

/**
 * Start the server
 */
export const startServer = async (state: MCPServerState): Promise<void> => {
  if (state.isRunning) {
    state.deps.logger.warn('Server already running');
    return;
  }

  await registerHandlers(state);
  await state.server.connect(state.transport);
  state.isRunning = true;

  state.deps.logger.info(
    {
      tools: TOOLS.length,
      prompts: state.deps.promptRegistry.getPromptNames().length,
      healthy: true,
    },
    'MCP server started',
  );
};

/**
 * Stop the server
 */
export const stopServer = async (state: MCPServerState): Promise<void> => {
  if (!state.isRunning) {
    return;
  }

  await state.server.close();
  state.isRunning = false;
  state.deps.logger.info('Server stopped');
};

/**
 * Get SDK server instance for sampling
 */
export const getServer = (state: MCPServerState): unknown => {
  return state.server.server;
};

/**
 * Get server status
 */
export const getStatus = (
  state: MCPServerState,
): {
  running: boolean;
  tools: number;
  resources: number;
  prompts: number;
} => {
  return {
    running: state.isRunning,
    tools: TOOLS.length,
    resources: 1,
    prompts: state.deps.promptRegistry.getPromptNames().length,
  };
};

/**
 * Get available tools for CLI listing
 */
export const getTools = (): Array<{ name: string; description: string }> => {
  return TOOLS.map((tool) => ({
    name: tool.name,
    description: tool.description,
  }));
};

/**
 * Factory function to create a DirectMCPServer implementation.
 *
 * Maintains all existing functionality while using functional patterns internally.
 */
export const createDirectMCPServer = (deps: Dependencies): IDirectMCPServer => {
  const state: MCPServerState = {
    server: new McpServer(
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
    ),
    transport: new StdioServerTransport(),
    isRunning: false,
    toolMap: new Map<string, ToolDefinition>(),
    deps,
  };

  return {
    start: () => startServer(state),
    stop: () => stopServer(state),
    getServer: () => getServer(state),
    getStatus: () => getStatus(state),
    getTools,
  };
};
