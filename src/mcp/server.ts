/**
 * Main MCP Server Entry Point
 *
 * Exports the SDK-native server as the primary server implementation.
 */

export { MCPServer } from './server/index';

// Export types for external use
export type { MCPServerOptions } from './server/types';
export type { Tool, Result, Success, Failure } from '@types';
