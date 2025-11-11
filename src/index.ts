/**
 * Clean, idiomatic TypeScript API for Container Assist
 * Provides strongly-typed runtime with dependency injection support
 */

// Primary API - AppRuntime with type safety and dependency injection

/**
 * Creates a configured application runtime instance with MCP server integration.
 *
 * This is the primary entry point for creating a containerization-assist application.
 * It sets up dependency injection, tool registration, and MCP transport configuration.
 *
 * @returns AppRuntime instance with all tools registered and ready to execute
 *
 * @example
 * ```typescript
 * import { createApp } from 'containerization-assist';
 *
 * const app = await createApp();
 *
 * // Use MCP server
 * await app.mcp.start();
 *
 * // Or execute tools directly
 * const result = await app.executeTool('analyze-repo', { path: './my-app' });
 * if (result.ok) {
 *   console.log('Analysis:', result.value);
 * }
 * ```
 *
 * @public
 */
export { createApp } from './app/index.js';

/**
 * Configuration for MCP transport layer (stdio, SSE, or WebSocket).
 *
 * @public
 */
export type { TransportConfig } from './app/index.js';

/**
 * Application runtime and configuration types.
 *
 * - `AppRuntime`: The main application interface providing tool execution and MCP server access
 * - `AppRuntimeConfig`: Configuration options for creating an app runtime
 * - `ToolInputMap`: Type mapping from tool names to their input parameter schemas
 * - `ToolResultMap`: Type mapping from tool names to their output result types
 * - `ExecutionMetadata`: Metadata about tool execution (timing, errors, etc.)
 * - `CreateAppRuntime`: Factory function signature for creating runtimes
 *
 * @public
 */
export type {
  AppRuntime,
  AppRuntimeConfig,
  ToolInputMap,
  ToolResultMap,
  ExecutionMetadata,
  CreateAppRuntime,
} from './types/runtime.js';

/**
 * Core type definitions for MCP tools and result handling.
 *
 * - `Tool`: The unified tool interface that all tools implement
 * - `Result<T>`: Result type for explicit error handling (Success or Failure)
 * - `Success`: Constructor for successful results
 * - `Failure`: Constructor for failed results with optional error guidance
 * - `ToolContext`: Execution context passed to all tool handlers (logger, progress, etc.)
 *
 * @example
 * ```typescript
 * import { Result, Success, Failure } from 'containerization-assist';
 *
 * function divide(a: number, b: number): Result<number> {
 *   if (b === 0) {
 *     return Failure('Division by zero');
 *   }
 *   return Success(a / b);
 * }
 * ```
 *
 * @public
 */
export type { Tool, Result, Success, Failure, ToolContext } from './types/index.js';

/**
 * Tool helper utilities for creating consistent tool implementations.
 *
 * - `getToolLogger`: Create a scoped logger for a specific tool
 * - `createToolTimer`: Create a timer for tracking tool execution duration
 * - `createStandardizedToolTracker`: Create a combined logger + timer tracker
 *
 * @example
 * ```typescript
 * import { getToolLogger, createToolTimer } from 'containerization-assist';
 *
 * const logger = getToolLogger('my-tool');
 * const timer = createToolTimer(logger);
 *
 * logger.info('Starting operation');
 * // ... do work ...
 * timer.end({ success: true });
 * ```
 *
 * @public
 */
export {
  getToolLogger,
  createToolTimer,
  createStandardizedToolTracker,
} from './lib/tool-helpers.js';

/**
 * Tool registration helpers for integrating with MCP servers.
 *
 * These utilities allow you to register Container Assist tools with MCP servers
 * while maintaining full control over telemetry, error handling, and lifecycle hooks.
 * All context creation (logger, policy, etc.) is handled automatically.
 *
 * - `createToolHandler`: Create a handler function for use with server.tool()
 * - `registerTool`: Register a single tool with an MCP server
 * - `registerTools`: Register multiple tools at once
 *
 * @example
 * ```typescript
 * import { createApp, ALL_TOOLS, createToolHandler } from 'containerization-assist';
 *
 * const app = createApp();
 *
 * // Option 1: Use createToolHandler for full control
 * for (const tool of ALL_TOOLS) {
 *   server.tool(
 *     tool.name,
 *     tool.description,
 *     tool.inputSchema,
 *     createToolHandler(app, tool.name, {
 *       onSuccess: (result, toolName) => {
 *         myTelemetry.trackSuccess(toolName);
 *       },
 *     })
 *   );
 * }
 *
 * // Option 2: Use registerTools for convenience
 * import { registerTools } from 'containerization-assist';
 * registerTools(server, app, ALL_TOOLS);
 * ```
 *
 * @public
 */
export { createToolHandler, registerTool, registerTools } from './lib/tool-registration.js';
export type { ToolHandlerOptions } from './lib/tool-registration.js';

/**
 * Telemetry sanitization utilities for safe logging.
 *
 * Provides functions to hash, obfuscate, and extract safe metrics from tool inputs
 * and outputs. Protects customer workspace information (file paths, code snippets,
 * identifiers) while preserving aggregate metrics for observability.
 *
 * Security Principle: "When in doubt, hash it out"
 *
 * - `hashValue`: Hash a string using SHA256 (first 16 chars)
 * - `sanitizePath`: Replace path segments with hashed values
 * - `sanitizeToolInput`: Remove or hash sensitive fields from tool inputs
 * - `extractSafeTelemetryMetrics`: Extract only aggregate metrics from tool results
 * - `createSafeTelemetryEvent`: Create a complete safe telemetry event
 * - `SafeTelemetryEvent`: Type for safe telemetry events
 *
 * @example
 * ```typescript
 * import { createSafeTelemetryEvent } from 'containerization-assist';
 *
 * // In your telemetry callback
 * onSuccess: (result, toolName, params) => {
 *   const event = createSafeTelemetryEvent(
 *     toolName,
 *     params,
 *     { ok: true, value: result }
 *   );
 *   // Event contains only hashed paths and aggregate metrics
 *   telemetry.track(event);
 * }
 * ```
 *
 * @see docs/TELEMETRY_GUIDELINES.md for detailed security guidelines
 * @public
 */
export {
  hashValue,
  sanitizePath,
  sanitizeToolInput,
  extractSafeTelemetryMetrics,
  createSafeTelemetryEvent,
} from './lib/telemetry-utils.js';
export type { SafeTelemetryEvent } from './lib/telemetry-utils.js';

/**
 * Parameter defaulting utilities for tool inputs.
 *
 * Provides default values for common tool parameters across different categories:
 * - `K8S_DEFAULTS`: Kubernetes deployment defaults (namespace, replicas, etc.)
 * - `CONTAINER_DEFAULTS`: Container image defaults (registry, platform, etc.)
 * - `ACA_DEFAULTS`: Azure Container Apps defaults
 * - `BUILD_DEFAULTS`: Docker build defaults (context, dockerfile path, etc.)
 * - `withDefaults`: Merge user params with defaults
 * - `getToolDefaults`: Get defaults for a specific tool by name
 *
 * @public
 */
export {
  withDefaults,
  K8S_DEFAULTS,
  CONTAINER_DEFAULTS,
  ACA_DEFAULTS,
  BUILD_DEFAULTS,
  getToolDefaults,
} from './lib/param-defaults.js';

/**
 * Helper function to create a tool definition with the unified Tool interface.
 *
 * Ensures tools follow the standard structure with proper typing and schema validation.
 *
 * @example
 * ```typescript
 * import { tool } from 'containerization-assist';
 * import { z } from 'zod';
 *
 * const mySchema = z.object({ input: z.string() });
 *
 * export default tool({
 *   name: 'my-tool',
 *   description: 'My custom tool',
 *   version: '1.0.0',
 *   schema: mySchema,
 *   handler: async (input, ctx) => {
 *     return Success({ result: input.input.toUpperCase() });
 *   }
 * });
 * ```
 *
 * @public
 */
export { tool } from './types/tool.js';

/**
 * All available MCP tools for containerization workflows.
 *
 * Tools are organized by workflow stage:
 * 1. Analysis: `analyzeRepoTool` - Detect language, framework, and dependencies
 * 2. Dockerfile: `generateDockerfileTool`, `fixDockerfileTool`
 * 3. Build: `buildImageTool`, `scanImageTool`, `tagImageTool`, `pushImageTool`
 * 4. Deploy: `generateK8sManifestsTool`, `prepareClusterTool`, `verifyDeployTool`
 * 5. Operations: `opsTool` - Operational utilities
 *
 * @public
 */
export {
  ALL_TOOLS,
  analyzeRepoTool,
  buildImageTool,
  fixDockerfileTool,
  generateDockerfileTool,
  generateK8sManifestsTool,
  opsTool,
  prepareClusterTool,
  pushImageTool,
  scanImageTool,
  tagImageTool,
  verifyDeployTool,
} from './tools/index.js';

/**
 * Utility to extract the shape of a Zod schema for telemetry and type introspection.
 *
 * @public
 */
export { extractSchemaShape } from './lib/zod-utils.js';

/**
 * Zod type for defining object schemas with named properties.
 *
 * @public
 */
export type { ZodRawShape } from 'zod';
