/**
 * Main export file for external tool consumption
 * Provides tools, helpers, and types for integration with MCP servers
 */

// Re-export server for backwards compatibility
export * from './mcp/server.js';
export { MCPServer as default } from './mcp/server.js';

// Export individual MCPTools from the tools collection
import { tools as _tools } from './exports/tools.js';

// Export with consistent naming (both original and aliased names)
export const analyzeRepo = _tools.analyzeRepo;
export const analyzeRepository = _tools.analyzeRepo; // alias for consistency with Go version
export const generateDockerfile = _tools.generateDockerfile;
export const buildImage = _tools.buildImage;
export const scanImage = _tools.scanImage;
export const tagImage = _tools.tagImage;
export const pushImage = _tools.pushImage;
export const generateK8sManifests = _tools.generateK8sManifests;
export const prepareCluster = _tools.prepareCluster;
export const deployApplication = _tools.deployApplication;
export const verifyDeployment = _tools.verifyDeployment;
export const fixDockerfile = _tools.fixDockerfile;
export const resolveBaseImages = _tools.resolveBaseImages;
export const ping = _tools.ops; // ops tool contains ping functionality
export const serverStatus = _tools.ops; // ops tool contains serverStatus functionality
export const workflow = _tools.workflow;
export const executeStep = _tools.workflow; // workflow tool contains executeStep functionality

// Export tool collection object
export { tools, getAllTools } from './exports/tools.js';

// Export helper functions
export {
  registerTool,
  registerAllTools,
  convertZodToJsonSchema,
  createSession,
} from './exports/helpers.js';

// Export the new clean API
export { ContainerAssistServer } from './exports/container-assist-server.js';

// Export types for external use
export type { MCPTool, MCPToolMetadata, MCPToolResult, MCPServer } from './exports/types.js';

// Re-export core types
export type { Tool, Result, Success, Failure } from './domain/types.js';
