/**
 * Tool Registration Helpers
 *
 * Utilities for registering Container Assist tools with MCP servers
 * while maintaining full control over telemetry and error handling.
 */

import type { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import type { RequestHandlerExtra } from '@modelcontextprotocol/sdk/shared/protocol.js';
import {
  McpError,
  ErrorCode,
  type ServerRequest,
  type ServerNotification,
} from '@modelcontextprotocol/sdk/types.js';

import type { AppRuntime, ExecutionMetadata, ToolInputMap, ToolResultMap } from '@/types/runtime';
import type { Tool } from '@/types/tool';
import type { ToolName } from '@/tools';
import { extractErrorMessage } from './errors';
import { formatOutput, OUTPUTFORMAT, type OutputFormat } from '@/mcp/mcp-server';
import { CHAINHINTSMODE, type ChainHintsMode } from '@/app/orchestrator-types';

/**
 * Tool handler wrapper options with type-safe callbacks
 *
 * When TName is a specific ToolName literal (e.g., 'build-image'), the callbacks
 * receive strongly-typed params and results. When TName is the broader ToolName union,
 * types are looser but still constrained to valid tool inputs/outputs.
 *
 * @template TName - The tool name literal type for type-safe params and results
 *
 * @example
 * ```typescript
 * // With literal tool name - fully typed
 * const handler = createToolHandler(app, 'build-image', {
 *   onSuccess: (result, toolName, params) => {
 *     // result: BuildImageResult
 *     // params: BuildImageInput
 *     console.log(params.imageName, result.imageId);
 *   }
 * });
 *
 * // With union type - broader types
 * const handler = createToolHandler(app, someDynamicToolName, {
 *   onSuccess: (result, toolName, params) => {
 *     // result: union of all tool results
 *     // params: union of all tool inputs
 *   }
 * });
 * ```
 */
export interface ToolHandlerOptions<TName extends ToolName = ToolName> {
  /** Custom transport label for logging (default: 'external') */
  transport?: string;

  /** Output format for tool results (default: 'natural-language') */
  outputFormat?: OutputFormat;

  /** Chain hints mode for controlling next steps in output (default: 'enabled') */
  chainHintsMode?: ChainHintsMode;

  /** Custom error handler - called before throwing McpError */
  onError?: (error: unknown, toolName: TName, params: ToolInputMap[TName]) => void;

  /** Custom success handler - called after successful execution */
  onSuccess?: (result: ToolResultMap[TName], toolName: TName, params: ToolInputMap[TName]) => void;
}

/**
 * Creates a tool handler function that delegates to app.execute()
 *
 * This allows you to use server.tool() for full control over telemetry
 * while letting Container Assist handle all context creation internally.
 *
 * When called with a literal tool name (e.g., 'build-image'), TypeScript
 * automatically infers the specific type, giving you fully-typed callbacks.
 *
 * @template TName - The tool name type (automatically inferred from literal strings)
 * @param app - AppRuntime instance (from createApp())
 * @param toolName - Name of the tool to execute
 * @param options - Optional configuration for the handler
 * @returns Tool handler function compatible with MCP SDK
 *
 * @example
 * ```typescript
 * import { createApp, ALL_TOOLS, createToolHandler } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * // Fully typed - TypeScript infers TName = 'build-image'
 * server.tool(
 *   'build-image',
 *   buildImageTool.description,
 *   buildImageTool.inputSchema,
 *   createToolHandler(app, 'build-image', {
 *     transport: 'my-integration',
 *     outputFormat: 'markdown', // Use markdown format instead of JSON
 *     onSuccess: (result, toolName, params) => {
 *       // result: BuildImageResult (fully typed!)
 *       // params: BuildImageInput (fully typed!)
 *       console.log(params.imageName, result.imageId);
 *       myTelemetry.trackToolExecution(toolName, true);
 *     },
 *     onError: (error, toolName, params) => {
 *       // params: BuildImageInput (fully typed!)
 *       console.log(params.dockerfile);
 *       myTelemetry.trackToolExecution(toolName, false);
 *     },
 *   })
 * );
 * ```
 */
export function createToolHandler<TName extends ToolName>(
  app: AppRuntime,
  toolName: TName,
  options: ToolHandlerOptions<TName> = {},
) {
  const {
    transport = 'external',
    outputFormat = OUTPUTFORMAT.NATURAL_LANGUAGE,
    chainHintsMode = CHAINHINTSMODE.ENABLED,
    onError,
    onSuccess,
  } = options;

  return async (
    rawParams: Record<string, unknown> | undefined,
    extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
  ) => {
    const params = rawParams ?? {};

    try {
      // Extract _meta if present
      const meta =
        params._meta && typeof params._meta === 'object'
          ? (params._meta as Record<string, unknown>)
          : {};

      // Extract metadata from MCP request
      const metadata: ExecutionMetadata = {
        transport,
        signal: extra.signal,
        ...meta,
      };

      // Add sendNotification only if available
      if (extra.sendNotification) {
        const sendNotification = extra.sendNotification;
        metadata.sendNotification = async (notification: unknown) => {
          await sendNotification(notification as ServerNotification);
        };
      }

      // Remove _meta from params before execution
      const { _meta, ...sanitizedParams } = params;

      // Execute via app.execute() - this handles all context creation
      // Type assertions are safe here because:
      // 1. The orchestrator validates toolName exists in the registry
      // 2. Each tool's Zod schema validates params at runtime
      // 3. Invalid toolName or params will return a proper Result.Failure
      const result = await app.execute(toolName as never, sanitizedParams as never, metadata);

      if (!result.ok) {
        // Call error handler if provided
        if (onError) {
          // Type assertion is safe because:
          // 1. toolName is validated by the orchestrator at runtime
          // 2. params are validated by the tool's Zod schema
          // 3. The generic TName ensures type consistency between handler and callbacks
          onError(result.error, toolName, sanitizedParams as ToolInputMap[TName]);
        }

        // Format error with guidance
        let errorMessage = result.error || 'Tool execution failed';
        if (result.guidance) {
          const parts = [errorMessage];
          if (result.guidance.hint) {
            parts.push(`💡 ${result.guidance.hint}`);
          }
          parts.push(
            `🔧 Resolution:`,
            result.guidance.resolution || 'Check logs for more information',
          );
          errorMessage = parts.join('\n\n');
        }

        throw new McpError(ErrorCode.InternalError, errorMessage);
      }

      // Call success handler if provided
      if (onSuccess) {
        // Type assertion is safe for the same reasons as above
        onSuccess(
          result.value as ToolResultMap[TName],
          toolName,
          sanitizedParams as ToolInputMap[TName],
        );
      }

      // Return formatted result
      return {
        content: [
          {
            type: 'text' as const,
            text: formatOutput(result.value, outputFormat, chainHintsMode),
          },
        ],
      };
    } catch (error) {
      if (error instanceof McpError) {
        throw error;
      }

      if (onError) {
        // Type assertion is safe (params include _meta but that's acceptable for error reporting)
        onError(error, toolName, params as unknown as ToolInputMap[TName]);
      }

      throw new McpError(ErrorCode.InternalError, extractErrorMessage(error));
    }
  };
}

/**
 * Register a single Container Assist tool with an MCP server
 *
 * Convenience wrapper around createToolHandler() for registering individual tools.
 * For full type safety, use createToolHandler() directly with literal tool names instead.
 *
 * @param server - MCP server instance
 * @param app - AppRuntime instance
 * @param tool - Tool definition
 * @param options - Optional handler configuration (params/results will be union types)
 *
 * @example
 * ```typescript
 * import { createApp, analyzeRepoTool, registerTool } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * // Simple registration (params/result are union types)
 * registerTool(server, app, analyzeRepoTool);
 *
 * // For full type safety, use createToolHandler instead:
 * server.tool(
 *   'analyze-repo',
 *   analyzeRepoTool.description,
 *   analyzeRepoTool.inputSchema,
 *   createToolHandler(app, 'analyze-repo', {
 *     onSuccess: (result, toolName, params) => {
 *       // Fully typed as AnalyzeRepoResult and AnalyzeRepoInput!
 *       myTelemetry.track('analyze-repo-success', params.path);
 *     },
 *   })
 * );
 * ```
 */
export function registerTool(
  server: McpServer,
  app: AppRuntime,
  tool: Tool,
  options?: ToolHandlerOptions,
): void {
  server.tool(
    tool.name,
    tool.description,
    tool.inputSchema,
    createToolHandler(app, tool.name, options),
  );
}

/**
 * Register all Container Assist tools with an MCP server
 *
 * Convenience function to register multiple tools at once. For full type safety,
 * register tools individually using createToolHandler() with literal tool names.
 *
 * @param server - MCP server instance
 * @param app - AppRuntime instance
 * @param tools - Array of tools to register
 * @param options - Optional handler configuration (applied to all tools with union types)
 *
 * @example
 * ```typescript
 * import { createApp, ALL_TOOLS, registerTools } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * // Bulk registration - convenient but broader types
 * registerTools(server, app, ALL_TOOLS, {
 *   transport: 'my-integration',
 *   onSuccess: (result, toolName, params) => {
 *     // result and params are unions of all tool types
 *     myTelemetry.trackToolExecution(toolName, true);
 *   },
 * });
 *
 * // For type-safe registration, use createToolHandler directly:
 * server.tool(
 *   'build-image',
 *   buildImageTool.description,
 *   buildImageTool.inputSchema,
 *   createToolHandler(app, 'build-image', {
 *     onSuccess: (result, toolName, params) => {
 *       // Fully typed! result: BuildImageResult, params: BuildImageInput
 *     }
 *   })
 * );
 * ```
 */
export function registerTools(
  server: McpServer,
  app: AppRuntime,
  tools: readonly Tool[],
  options?: ToolHandlerOptions,
): void {
  for (const tool of tools) {
    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      createToolHandler(app, tool.name, options),
    );
  }
}
