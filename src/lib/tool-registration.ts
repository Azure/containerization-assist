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

import type { AppRuntime, ExecutionMetadata } from '@/types/runtime';
import type { Tool } from '@/types/tool';
import { extractErrorMessage } from './errors';

/**
 * Tool handler wrapper options
 */
export interface ToolHandlerOptions {
  /** Custom transport label for logging (default: 'external') */
  transport?: string;

  /** Custom error handler - called before throwing McpError */
  onError?: (error: unknown, toolName: string, params: Record<string, unknown>) => void;

  /** Custom success handler - called after successful execution */
  onSuccess?: (result: unknown, toolName: string, params: Record<string, unknown>) => void;
}

/**
 * Creates a tool handler function that delegates to app.execute()
 *
 * This allows you to use server.tool() for full control over telemetry
 * while letting Container Assist handle all context creation internally.
 *
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
 * for (const tool of ALL_TOOLS) {
 *   server.tool(
 *     tool.name,
 *     tool.description,
 *     tool.inputSchema,
 *     createToolHandler(app, tool.name, {
 *       transport: 'my-integration',
 *       onSuccess: (result, toolName, params) => {
 *         myTelemetry.trackToolExecution(toolName, true);
 *       },
 *       onError: (error, toolName, params) => {
 *         myTelemetry.trackToolExecution(toolName, false);
 *       },
 *     })
 *   );
 * }
 * ```
 */
export function createToolHandler(
  app: AppRuntime,
  toolName: string,
  options: ToolHandlerOptions = {},
) {
  const { transport = 'external', onError, onSuccess } = options;

  return async (
    rawParams: Record<string, unknown> | undefined,
    extra: RequestHandlerExtra<ServerRequest, ServerNotification>,
  ) => {
    const params = rawParams ?? {};

    try {
      // Extract _meta if present
      const meta = params._meta && typeof params._meta === 'object'
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
      const result = await app.execute(toolName as never, sanitizedParams as never, metadata);

      if (!result.ok) {
        // Call error handler if provided
        if (onError) {
          onError(result.error, toolName, sanitizedParams);
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
        onSuccess(result.value, toolName, sanitizedParams);
      }

      // Return formatted result
      // Note: formatting is handled by the MCP server's output format setting
      return {
        content: [
          {
            type: 'text' as const,
            text: JSON.stringify(result.value, null, 2),
          },
        ],
      };
    } catch (error) {
      if (error instanceof McpError) {
        throw error;
      }

      if (onError) {
        onError(error, toolName, params);
      }

      throw new McpError(ErrorCode.InternalError, extractErrorMessage(error));
    }
  };
}

/**
 * Register a single Container Assist tool with an MCP server
 *
 * Convenience wrapper around createToolHandler() for registering individual tools.
 *
 * @param server - MCP server instance
 * @param app - AppRuntime instance
 * @param tool - Tool definition
 * @param options - Optional handler configuration
 *
 * @example
 * ```typescript
 * import { createApp, analyzeRepoTool, registerTool } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * registerTool(server, app, analyzeRepoTool, {
 *   onSuccess: () => myTelemetry.track('analyze-repo-success'),
 * });
 * ```
 */
export function registerTool(
  server: McpServer,
  app: AppRuntime,
  tool: Tool,
  options?: ToolHandlerOptions,
): void {
  server.tool(tool.name, tool.description, tool.inputSchema, createToolHandler(app, tool.name, options));
}

/**
 * Register all Container Assist tools with an MCP server
 *
 * Convenience function to register multiple tools at once.
 *
 * @param server - MCP server instance
 * @param app - AppRuntime instance
 * @param tools - Array of tools to register
 * @param options - Optional handler configuration (applied to all tools)
 *
 * @example
 * ```typescript
 * import { createApp, ALL_TOOLS, registerTools } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * registerTools(server, app, ALL_TOOLS, {
 *   transport: 'my-integration',
 *   onSuccess: (result, toolName) => {
 *     myTelemetry.trackToolExecution(toolName, true);
 *   },
 * });
 * ```
 */
export function registerTools(
  server: McpServer,
  app: AppRuntime,
  tools: readonly Tool[],
  options?: ToolHandlerOptions,
): void {
  for (const tool of tools) {
    registerTool(server, app, tool, options);
  }
}
