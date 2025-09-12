/**
 * MCP Module - Public API
 *
 * Exports the MCP server and related utilities using SDK-native patterns.
 * This module provides the clean public interface for MCP functionality.
 */

// Export the main server
export { MCPServer } from './server';
// TEMP: Commented out during Phase 1 error fixing
// export { DirectMCPServer } from './server-direct';

// Export tool registration utilities
// TEMP: Commented out during Phase 1 error fixing
// export { registerTools, getToolList, toolRegistry } from './tool-registrar';
// export type { RegistrationResult } from './tool-registrar';

// Export context creation utilities
export { createToolContext, createMCPToolContext, type ContextOptions } from './context';

// Export types
export type {
  ToolContext,
  SamplingRequest,
  SamplingResponse,
  TextMessage,
  PromptWithMessages,
  ProgressReporter,
} from './context';

// Export progress helpers
export {
  extractProgressToken,
  createProgressReporter,
  type EnhancedProgressReporter,
} from './context-helpers';

// Export error mapping utilities
export { toMcpError, getErrorDetails } from './error-mapper';

// Export progress utilities
export { createStandardProgress, STANDARD_STAGES } from './progress-helper';

// Re-export MCP SDK types for convenience
export { McpError, ErrorCode } from '@modelcontextprotocol/sdk/types.js';
