/**
 * MCP Server Implementation
 * Register tools against the orchestrator executor and manage transports.
 *
 * @see {@link ../../docs/adr/005-mcp-integration.md ADR-005: MCP Protocol Integration}
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  McpError,
  ErrorCode,
  type ServerRequest,
  type ServerNotification,
} from '@modelcontextprotocol/sdk/types.js';
import type { RequestHandlerExtra } from '@modelcontextprotocol/sdk/shared/protocol.js';
import { extractErrorMessage } from '@/lib/errors';
import { createLogger, type Logger } from '@/lib/logger';
import type { Tool } from '@/types/tool';
import {
  type ExecuteRequest,
  type ExecuteMetadata,
  type ChainHintsMode,
  CHAINHINTSMODE,
} from '@/app/orchestrator-types';
import type { Result, ErrorGuidance } from '@/types';
import type { ScanImageResult } from '@/tools/scan-image/tool';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { BuildImageResult } from '@/tools/build-image/tool';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { VerifyDeploymentResult } from '@/tools/verify-deploy/tool';
import type { DockerfileFixPlan } from '@/tools/fix-dockerfile/schema';
import type { ManifestPlan } from '@/tools/generate-k8s-manifests/schema';
import type { PushImageResult } from '@/tools/push-image/tool';
import type { TagImageResult } from '@/tools/tag-image/tool';
import type { PrepareClusterResult } from '@/tools/prepare-cluster/tool';
import type { PingResult, ServerStatusResult } from '@/tools/ops/tool';
import {
  formatScanImageNarrative,
  formatDockerfilePlanNarrative,
  formatBuildImageNarrative,
  formatAnalyzeRepoNarrative,
  formatVerifyDeployNarrative,
  formatFixDockerfileNarrative,
  formatGenerateK8sManifestsNarrative,
  formatPushImageNarrative,
  formatTagImageNarrative,
  formatPrepareClusterNarrative,
  formatOpsPingNarrative,
  formatOpsStatusNarrative,
} from '@/mcp/formatters/natural-language-formatters';

/**
 * Constants
 */
const RESOURCE_URI = {
  STATUS: 'containerization://status',
} as const;

const ERROR_FORMAT = {
  HINT_PREFIX: '💡',
  RESOLUTION_PREFIX: '🔧',
  DEFAULT_RESOLUTION: 'Check logs for more information',
} as const;

/**
 * Type definitions for metadata extraction
 */
interface MetaParams {
  requestId?: string;
  invocationId?: string;
  [key: string]: unknown;
}

/**
 * Type guard to check if a value is valid metadata params
 */
function isMetaParams(value: unknown): value is MetaParams {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/**
 * Server options
 */
export interface ServerOptions {
  logger?: Logger;
  transport?: 'stdio';
  name?: string;
  version?: string;
  outputFormat?: OutputFormat;
  chainHintsMode?: ChainHintsMode;
}

/**
 * MCP Server interface
 */
export interface MCPServer {
  start(): Promise<void>;
  stop(): Promise<void>;
  getServer(): Server;
  getTools(): Array<{ name: string; description: string }>;
}

/**
 * Output format options for tool results
 *
 * @property JSON - Full structured JSON output (for APIs and programmatic access)
 * @property TEXT - Concise summary text (for logs and quick display)
 * @property MARKDOWN - Summary with collapsible JSON details (for documentation)
 * @property NATURAL_LANGUAGE - Rich narrative with sections (for user interfaces)
 */
export const OUTPUTFORMAT = {
  MARKDOWN: 'markdown',
  JSON: 'json',
  TEXT: 'text',
  NATURAL_LANGUAGE: 'natural-language',
} as const;
export type OutputFormat = (typeof OUTPUTFORMAT)[keyof typeof OUTPUTFORMAT];

export interface RegisterOptions<TTool extends Tool = Tool> {
  outputFormat: OutputFormat;
  chainHintsMode?: ChainHintsMode;
  server: McpServer;
  tools: readonly TTool[];
  logger: Logger;
  transport: string;
  execute: ToolExecutor;
}

type ToolExecutor = (request: ExecuteRequest) => Promise<Result<unknown>>;

/**
 * Format error message with guidance for better user experience
 * @param error - The error message
 * @param guidance - Optional error guidance with hints and resolution steps
 * @returns Formatted error message with guidance
 */
function formatErrorWithGuidance(error: string, guidance?: ErrorGuidance): string {
  if (!guidance) {
    return error || 'Tool execution failed';
  }

  const parts = [error];

  if (guidance.hint) {
    parts.push(`${ERROR_FORMAT.HINT_PREFIX} ${guidance.hint}`);
  }

  parts.push(
    `${ERROR_FORMAT.RESOLUTION_PREFIX} Resolution:`,
    guidance.resolution || ERROR_FORMAT.DEFAULT_RESOLUTION,
  );

  return parts.join('\n\n');
}

/**
 * Create an MCP server that delegates execution to the orchestrator
 * @param tools - Array of MCP tools to register with the server
 * @param options - Server configuration options
 * @param execute - Tool executor function that handles tool execution requests
 * @returns MCPServer interface for managing the server lifecycle
 */
export function createMCPServer<TTool extends Tool>(
  tools: Array<TTool>,
  options: ServerOptions = {},
  execute: ToolExecutor,
): MCPServer {
  const logger = options.logger || createLogger({ name: 'mcp-server' });
  const serverOptions = {
    name: options.name || 'containerization-assist',
    version: options.version || '1.0.0',
  };

  const server = new McpServer(serverOptions);
  const transportType = options.transport ?? 'stdio';
  const outputFormat = options.outputFormat ?? OUTPUTFORMAT.NATURAL_LANGUAGE;
  const chainHintsMode = options.chainHintsMode ?? CHAINHINTSMODE.ENABLED;
  let transportInstance: StdioServerTransport | null = null;
  let isRunning = false;

  registerToolsWithServer({
    outputFormat,
    chainHintsMode,
    server,
    tools,
    logger,
    transport: transportType,
    execute,
  });

  server.resource(
    'status',
    RESOURCE_URI.STATUS,
    {
      title: 'Container Status',
      description: 'Current status of the containerization system',
    },
    async () => ({
      contents: [
        {
          uri: RESOURCE_URI.STATUS,
          mimeType: 'application/json',
          text: JSON.stringify(
            {
              running: isRunning,
              tools: tools.length,
              transport: transportType,
              timestamp: new Date().toISOString(),
            },
            null,
            2,
          ),
        },
      ],
    }),
  );

  return {
    async start(): Promise<void> {
      if (isRunning) {
        throw new Error('Server is already running');
      }

      if (transportType !== 'stdio') {
        throw new Error(`Only 'stdio' transport is supported. Requested: '${transportType}'`);
      }

      transportInstance = new StdioServerTransport();
      await server.connect(transportInstance);

      isRunning = true;
      logger.info(
        {
          version: serverOptions.version,
          transport: transportType,
          toolCount: tools.length,
        },
        'MCP server started',
      );
    },

    async stop(): Promise<void> {
      if (!isRunning) {
        return;
      }

      await server.close();
      transportInstance = null;
      isRunning = false;
      logger.info({ transport: transportType }, 'MCP server stopped');
    },

    getServer(): Server {
      return server.server;
    },

    getTools(): Array<{ name: string; description: string }> {
      return tools.map((t) => ({
        name: t.name,
        description: t.description,
      }));
    },
  };
}

/**
 * Create tool handler function with proper typing to avoid deep type instantiation
 */
function getHandler(
  toolName: string,
  transport: string,
  outputFormat: OutputFormat,
  chainHintsMode: ChainHintsMode,
  execute: ToolExecutor,
) {
  return async (
    rawParams: Record<string, unknown> | undefined,
    extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
  ) => {
    const params = rawParams ?? {};

    try {
      const { sanitizedParams, metadata } = prepareExecutionPayload(
        toolName,
        params,
        transport,
        extra,
      );

      const result = await execute({
        toolName,
        params: sanitizedParams,
        metadata,
      });

      if (!result.ok) {
        // Format error with guidance if available
        const errorMessage = formatErrorWithGuidance(result.error, result.guidance);
        throw new McpError(ErrorCode.InternalError, errorMessage);
      }

      return {
        content: [
          {
            type: 'text' as const,
            text: formatOutput(result.value, outputFormat, chainHintsMode),
          },
        ],
      };
    } catch (error) {
      throw error instanceof McpError
        ? error
        : new McpError(ErrorCode.InternalError, extractErrorMessage(error));
    }
  };
}

/**
 * Register tools against an MCP server instance, delegating to the orchestrator executor.
 * Each tool is registered with its name, description, and input schema. Tool execution is
 * delegated to the orchestrator's execute function.
 * @param options - Registration options including server, tools, and executor
 */
export function registerToolsWithServer<TTool extends Tool>(options: RegisterOptions<TTool>): void {
  const {
    server,
    tools,
    transport,
    execute,
    outputFormat,
    chainHintsMode = CHAINHINTSMODE.ENABLED,
  } = options;

  for (const tool of tools) {
    const handler = getHandler(tool.name, transport, outputFormat, chainHintsMode, execute);

    // Type assertion to avoid deep type instantiation issues with MCP SDK
    // The MCP SDK's complex generic constraints on tool() cause TS2589 errors
    // Runtime safety is preserved by Zod schema validation
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (server as McpServer & { tool: any }).tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      handler,
    );
  }
}

/**
 * Creates logger context from tool name, transport, and metadata
 * @param toolName - Name of the tool being executed
 * @param transport - Transport type (e.g., 'stdio')
 * @param meta - Optional metadata parameters
 * @returns Logger context object
 */
function createLoggerContext(
  toolName: string,
  transport: string,
  meta?: MetaParams,
): Record<string, unknown> {
  return {
    transport,
    tool: toolName,
    ...(meta?.requestId && typeof meta.requestId === 'string' && { requestId: meta.requestId }),
    ...(meta?.invocationId &&
      typeof meta.invocationId === 'string' && {
        invocationId: meta.invocationId,
      }),
  };
}

/**
 * Creates a type-safe notification adapter that wraps the MCP SDK sendNotification
 * @param mcpSendNotification - The MCP SDK sendNotification function
 * @returns Type-safe notification sender
 */
function createNotificationAdapter(
  mcpSendNotification: (notification: ServerNotification) => Promise<void>,
): (notification: unknown) => Promise<void> {
  return async (notification: unknown) => {
    // Assume the caller provides the correct type, or add runtime validation here if needed
    return mcpSendNotification(notification as ServerNotification);
  };
}

/**
 * Creates execution metadata from parameters and request context
 * @param toolName - Name of the tool being executed
 * @param params - Tool parameters
 * @param transport - Transport type
 * @param extra - Request handler extras from MCP SDK
 * @returns ExecuteMetadata object
 */
function createExecuteMetadata(
  toolName: string,
  params: Record<string, unknown>,
  transport: string,
  extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
): ExecuteMetadata {
  const meta = extractMeta(params);

  return {
    progress: params,
    loggerContext: createLoggerContext(toolName, transport, meta),
    ...(extra.sendNotification && {
      sendNotification: createNotificationAdapter(extra.sendNotification),
    }),
  };
}

/**
 * Prepares execution payload by sanitizing params and creating metadata
 * @param toolName - Name of the tool being executed
 * @param params - Raw tool parameters
 * @param transport - Transport type
 * @param extra - Request handler extras from MCP SDK
 * @returns Object containing sanitized params and execution metadata
 */
function prepareExecutionPayload(
  toolName: string,
  params: Record<string, unknown>,
  transport: string,
  extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
): {
  sanitizedParams: Record<string, unknown>;
  metadata: ExecuteMetadata;
} {
  return {
    sanitizedParams: sanitizeParams(params),
    metadata: createExecuteMetadata(toolName, params, transport, extra),
  };
}

/**
 * Extracts metadata from tool parameters
 * @param params - Raw tool parameters
 * @returns Metadata object or undefined if not present/invalid
 */
function extractMeta(params: Record<string, unknown>): MetaParams | undefined {
  const meta = params._meta;
  return isMetaParams(meta) ? meta : undefined;
}

/**
 * Removes internal metadata fields from parameters
 * @param params - Raw parameters with potential metadata
 * @returns Sanitized parameters without _meta field
 */
function sanitizeParams(params: Record<string, unknown>): Record<string, unknown> {
  const entries = Object.entries(params).filter(([key]) => key !== '_meta');
  return Object.fromEntries(entries);
}

/**
 * Format tool output based on requested format
 *
 * @param output - The tool result to format (typically includes a summary field)
 * @param format - Output format (JSON, TEXT, MARKDOWN, or NATURAL_LANGUAGE)
 * @param chainHintsMode - Whether to include "Next Steps" sections in natural language output (default: 'enabled')
 * @returns Formatted string representation of the output
 *
 * @description
 * Transforms tool results into user-friendly formats:
 * - JSON: Full structured data (default, for APIs)
 * - TEXT: Summary field only (for logs/console)
 * - MARKDOWN: Summary + collapsible JSON (for documentation)
 * - NATURAL_LANGUAGE: Rich narrative (for user interfaces)
 *
 * All tool results include a `summary` field for human-readable display.
 * The NATURAL_LANGUAGE format uses type detection to provide tool-specific
 * rich narratives with sections, formatting, and next steps.
 */
export function formatOutput(
  output: unknown,
  format: OutputFormat,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  switch (format) {
    case OUTPUTFORMAT.JSON:
      return JSON.stringify(output, null, 2);

    case OUTPUTFORMAT.NATURAL_LANGUAGE:
      // Rich narrative formatting - delegates to tool-specific formatters
      return formatAsNaturalLanguage(output, chainHintsMode);

    case OUTPUTFORMAT.MARKDOWN:
      // Check if output has a summary field
      if (typeof output === 'object' && output !== null && 'summary' in output) {
        const { summary, ...rest } = output as { summary: string; [key: string]: unknown };

        // Display summary prominently, with structured data collapsed
        return `${summary}\n\n<details>\n<summary>View detailed output</summary>\n\n\`\`\`json\n${JSON.stringify(rest, null, 2)}\n\`\`\`\n</details>`;
      }

      // Fallback to JSON code block
      return `\`\`\`json\n${JSON.stringify(output, null, 2)}\n\`\`\``;

    case OUTPUTFORMAT.TEXT:
      // Prioritize summary in plain text mode
      if (typeof output === 'object' && output !== null && 'summary' in output) {
        const { summary } = output as { summary: string };
        return summary;
      }

      // Fallback to JSON
      if (typeof output === 'object' && output !== null) {
        return JSON.stringify(output, null, 2);
      }
      return String(output);

    default:
      return JSON.stringify(output, null, 2);
  }
}

/**
 * Format output as natural language narrative
 *
 * @param output - Tool result to format as narrative
 * @param chainHintsMode - Whether to include "Next Steps" sections (default: 'enabled')
 * @returns Rich narrative with formatting, or summary/JSON fallback
 *
 * @description
 * Delegates to tool-specific formatters for rich narratives with:
 * - Section headers and formatting
 * - Bullet points and structured lists
 * - Context-aware next steps (when chainHintsMode is 'enabled')
 * - Proper handling of optional fields
 *
 * Supported tool types with dedicated formatters:
 * - scan-image: Security scan results with severity breakdown
 * - generate-dockerfile: Planning with base images and recommendations
 * - deploy: Deployment status with endpoints and conditions
 * - build-image: Build results with metrics
 * - analyze-repo: Repository analysis with module detection
 *
 * Falls back to summary field or JSON for other tool types.
 */
function formatAsNaturalLanguage(
  output: unknown,
  chainHintsMode: ChainHintsMode = CHAINHINTSMODE.ENABLED,
): string {
  if (!output || typeof output !== 'object') {
    return String(output);
  }

  // Type detection and delegation
  // Each tool result type gets its own formatter
  if (isScanImageResult(output)) {
    return formatScanImageNarrative(output, chainHintsMode);
  }
  if (isDockerfilePlan(output)) {
    return formatDockerfilePlanNarrative(output, chainHintsMode);
  }
  if (isBuildImageResult(output)) {
    return formatBuildImageNarrative(output, chainHintsMode);
  }
  if (isAnalyzeRepoResult(output)) {
    return formatAnalyzeRepoNarrative(output, chainHintsMode);
  }

  // NEW FORMATTERS - Add in order of specificity (most specific first)
  if (isVerifyDeployResult(output)) {
    return formatVerifyDeployNarrative(output, chainHintsMode);
  }
  if (isFixDockerfileResult(output)) {
    return formatFixDockerfileNarrative(output, chainHintsMode);
  }
  if (isGenerateK8sManifestsResult(output)) {
    return formatGenerateK8sManifestsNarrative(output, chainHintsMode);
  }
  if (isPushImageResult(output)) {
    return formatPushImageNarrative(output, chainHintsMode);
  }
  if (isTagImageResult(output)) {
    return formatTagImageNarrative(output, chainHintsMode);
  }
  if (isPrepareClusterResult(output)) {
    return formatPrepareClusterNarrative(output, chainHintsMode);
  }
  // Check ops results (check status before ping due to field overlap)
  if (isServerStatusResult(output)) {
    return formatOpsStatusNarrative(output);
  }
  if (isPingResult(output)) {
    return formatOpsPingNarrative(output);
  }

  // Additional tool result types can be added as needed

  // Fallback: use summary if available, otherwise JSON
  if ('summary' in output) {
    const { summary } = output as { summary: string };
    return summary;
  }

  return JSON.stringify(output, null, 2);
}

/**
 * Type guards for result types
 * These enable proper type detection for natural language formatting
 */

function isScanImageResult(output: object): output is ScanImageResult {
  return 'vulnerabilities' in output && 'scanTime' in output && 'passed' in output;
}

function isDockerfilePlan(output: object): output is DockerfilePlan {
  if (!('recommendations' in output && 'repositoryInfo' in output)) {
    return false;
  }
  if ('manifestType' in output) {
    return false;
  }

  const recommendations = (output as { recommendations: unknown }).recommendations;
  return (
    typeof recommendations === 'object' &&
    recommendations !== null &&
    'buildStrategy' in recommendations
  );
}

function isBuildImageResult(output: object): output is BuildImageResult {
  return 'imageId' in output && 'buildTime' in output;
}

function isAnalyzeRepoResult(output: object): output is RepositoryAnalysis {
  return 'modules' in output && 'isMonorepo' in output;
}

function isVerifyDeployResult(output: object): output is VerifyDeploymentResult {
  return 'deploymentName' in output && 'pods' in output && 'healthCheck' in output;
}

function isFixDockerfileResult(output: object): output is DockerfileFixPlan {
  return 'currentIssues' in output && 'fixes' in output && 'validationScore' in output;
}

function isGenerateK8sManifestsResult(output: object): output is ManifestPlan {
  return 'manifestType' in output && 'recommendations' in output && 'knowledgeMatches' in output;
}

function isTagImageResult(output: object): output is TagImageResult {
  return 'tags' in output && 'imageId' in output && Array.isArray((output as TagImageResult).tags);
}

function isPushImageResult(output: object): output is PushImageResult {
  return 'registry' in output && 'digest' in output && 'pushedTag' in output;
}

function isPrepareClusterResult(output: object): output is PrepareClusterResult {
  return 'clusterReady' in output && 'cluster' in output && 'checks' in output;
}

function isPingResult(output: object): output is PingResult {
  return 'message' in output && 'server' in output && 'capabilities' in output;
}

function isServerStatusResult(output: object): output is ServerStatusResult {
  return 'memory' in output && 'cpu' in output && 'tools' in output && 'version' in output;
}
