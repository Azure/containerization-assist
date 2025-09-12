/**
 * Main MCP Server Entry Point
 *
 * Exports the SDK-native server as the primary server implementation.
 */

export { DirectMCPServer as MCPServer } from './server-direct';

// Export types for external use
export type { Tool, Result, Success, Failure } from '../types';
