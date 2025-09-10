/**
 * Main MCP Server Entry Point
 *
 * Exports the SDK-native server as the primary server implementation.
 * This replaces the old ContainerizationMCPServer with the improved MCPServer.
 */

export { MCPServer as ContainerizationMCPServer } from './server/index';
export { MCPServer } from './server/index';

// Export types for external use
export type { MCPServerOptions } from './server/types';
export type { Tool, Result, Success, Failure } from '@types';
