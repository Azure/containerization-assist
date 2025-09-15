/**
 * Main MCP Server Entry Point
 *
 * Exports the SDK-native server as the primary server implementation.
 */

export {
  createDirectMCPServer as createMCPServer,
  type IDirectMCPServer as IMCPServer,
} from './server-direct';

// Export types for external use
export type { Tool, Result, Success, Failure } from '@types';
