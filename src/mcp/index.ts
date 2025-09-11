/**
 * MCP Module - Public API
 *
 * Exports the MCP server and related utilities using SDK-native patterns.
 * This module provides the clean public interface for MCP functionality.
 */

// Export the main server
export { MCPServer } from './server/index';
export { DirectMCPServer } from './server/direct';

// Export context creation utilities
export {
  createToolContext,
  createMCPToolContext,
  createTestContext,
  type ToolContextDeps,
  type CreateContextOptions,
} from './context/tool-context-builder';

// Export types
export type {
  ToolContext,
  SamplingRequest,
  SamplingResponse,
  TextMessage,
  PromptWithMessages,
  ProgressReporter,
  ToolContextConfig,
} from './context/types';

// Export error mapping utilities
export { toMcpError, getErrorDetails } from './utils/error-mapper';

// Export progress utilities
export { createStandardProgress, STANDARD_STAGES } from './utils/progress-helper';

// Re-export MCP SDK types for convenience
export { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
