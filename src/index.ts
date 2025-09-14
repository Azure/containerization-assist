/**
 * Main export file for external tool consumption
 * Provides tools, helpers, and types for integration with MCP servers
 */

export * from './mcp/server.js';

export { registerTool, registerAllTools, toJsonSchema } from './exports/helpers.js';

export { ContainerAssistServer } from './exports/containerization-assist-server.js';

// Export tool names for type-safe registration
export { TOOL_NAMES } from './exports/tools.js';
export type { ToolName } from './exports/tools.js';

export type { MCPTool, MCPToolMetadata, MCPToolResult, MCPServer } from './exports/types';

export type { Tool, Result, Success, Failure } from './types';

// Export new utility functions for consolidation
export {
  // Tool helpers for logger and timer management
  getToolLogger,
  createToolTimer,
  initializeToolInstrumentation,
} from './lib/tool-helpers.js';

export {
  // Parameter defaulting utilities
  withDefaults,
  buildParams,
  K8S_DEFAULTS,
  CONTAINER_DEFAULTS,
  ACA_DEFAULTS,
  BUILD_DEFAULTS,
  getToolDefaults,
  type ParameterBuilder,
} from './lib/param-defaults.js';

export {
  // Result handling utilities
  propagateFailure,
  mapResult,
  chainResults,
  combineResults,
  tryExecute,
  tryExecuteAsync,
  unwrapOrThrow,
  unwrapOr,
  isSuccess,
  isFailure,
  mapError,
  withErrorContext,
} from './lib/result-utils.js';

export {
  // String validation utilities
  isEmptyString,
  isEmptyArray,
  isNullOrUndefined,
  requireNonEmptyString,
  requireNonEmptyArray,
  validateDockerTag,
  validateDockerImageName,
  validateRegistryUrl,
  validateK8sNamespace,
  normalizeRegistryUrl,
  sanitizeFilename,
  normalizeScore,
  weightedAverage,
} from './lib/string-validators.js';
