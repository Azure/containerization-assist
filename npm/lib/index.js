/**
 * Containerization Assist MCP Tools
 * 
 * Individual tool exports for registration with any MCP server.
 * Each tool exports: name, metadata (with inputSchema), and handler.
 * 
 * @module containerization-assist-mcp
 */

// Import all tools
import analyzeRepository from './tools/analyze-repository.js';
import verifyDockerfile from './tools/verify-dockerfile.js';
import buildImage from './tools/build-image.js';
import scanImage from './tools/scan-image.js';
import tagImage from './tools/tag-image.js';
import pushImage from './tools/push-image.js';
import verifyK8sManifests from './tools/verify-k8s-manifests.js';
import prepareCluster from './tools/prepare-cluster.js';
import deployApplication from './tools/deploy-application.js';
import verifyDeployment from './tools/verify-deployment.js';
import listTools from './tools/list-tools.js';
import ping from './tools/ping.js';
import serverStatus from './tools/server-status.js';

// Import utilities
import { generateSessionId } from './executor.js';

// Collection of all tools for iteration
const tools = {
  analyzeRepository,
  verifyDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  verifyK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  listTools,
  ping,
  serverStatus
};

/**
 * Register a single tool with an MCP server
 * @param {Object} server - MCP server instance (SDK Server, McpServer, or custom implementation)
 * @param {Object} tool - Tool definition with name, metadata, and handler
 * @param {string} tool.name - Tool identifier
 * @param {Object} tool.metadata - Tool metadata containing title, description, and inputSchema
 * @param {string} tool.metadata.title - Tool display title
 * @param {string} tool.metadata.description - Tool description  
 * @param {Object} tool.metadata.inputSchema - Tool input schema (Zod schema object)
 * @param {Function} tool.handler - Async function that handles tool execution
 * @param {string} [customName] - Optional custom name to override tool.name
 * @throws {Error} If server doesn't have a supported registration method
 * @example
 * // With MCP SDK's McpServer
 * const { McpServer } = require('@modelcontextprotocol/sdk/server/mcp.js');
 * const { analyzeRepository, registerTool } = require('@thgamble/containerization-assist-mcp');
 * 
 * const server = new McpServer({ name: 'my-server', version: '1.0.0' });
 * registerTool(server, analyzeRepository);
 * 
 * @example  
 * // With MCP SDK's low-level Server
 * const { Server } = require('@modelcontextprotocol/sdk/server/index.js');
 * const { analyzeRepository, registerTool } = require('@thgamble/containerization-assist-mcp');
 * 
 * const server = new Server({ name: 'my-server', version: '1.0.0' });
 * registerTool(server, analyzeRepository);
 */
function registerTool(server, tool, customName = null) {
  const name = customName || tool.name;
  
  // Check if server has registerTool method
  if (typeof server.registerTool === 'function') {
    // Detect McpServer by constructor name or unique property
    // McpServer.registerTool expects (name, metadata, handler) where metadata includes inputSchema
    if (
      (server.constructor && server.constructor.name === 'McpServer') ||
      (server.isMcpServer === true) // Optionally, check for a unique property
    ) {
      // For McpServer, combine our metadata structure
      const metadata = {
        title: tool.metadata.title,
        description: tool.metadata.description,
        inputSchema: tool.metadata.inputSchema // Pass Zod schema directly
      };
      server.registerTool(name, metadata, tool.handler);
      return; // Success
    } else {
      // Assume mock/test server signature: (name, metadata, handler)
      server.registerTool(name, tool.metadata, tool.handler);
      return; // Success
    }
  }
  
  // Check if server has the MCP SDK's low-level Server addTool method
  if (typeof server.addTool === 'function') {
    // Low-level MCP SDK Server - needs JSON Schema
    server.addTool(
      {
        name: name,
        description: tool.metadata.description,
        inputSchema: convertZodToJsonSchema(tool.metadata.inputSchema)
      },
      tool.handler
    );
    return; // Success
  }
  
  // No compatible method found
  throw new Error(
    'Server must have either:\n' +
    '  - registerTool(name, metadata, handler) method (McpServer)\n' +
    '  - addTool({ name, description, inputSchema }, handler) method (Server)\n' +
    'Please ensure your server implements one of these signatures.'
  );
}

/**
 * Convert Zod schema to JSON Schema for MCP SDK compatibility
 * @param {Object} zodSchema - Zod schema object with validation rules
 * @returns {Object} JSON Schema compatible with MCP SDK
 * @private
 */
function convertZodToJsonSchema(zodSchema) {
  if (!zodSchema) {
    return { type: 'object', properties: {} };
  }
  
  const properties = {};
  const required = [];
  
  for (const [key, schema] of Object.entries(zodSchema)) {
    // Basic Zod to JSON Schema conversion
    const def = schema._def;
    let type = 'string'; // default
    
    if (def) {
      if (def.typeName === 'ZodString') type = 'string';
      else if (def.typeName === 'ZodNumber') type = 'number';
      else if (def.typeName === 'ZodBoolean') type = 'boolean';
      else if (def.typeName === 'ZodArray') type = 'array';
      else if (def.typeName === 'ZodObject') type = 'object';
      else if (def.typeName === 'ZodEnum') {
        type = 'string';
        properties[key] = {
          type,
          enum: def.values,
          description: def.description || ''
        };
        if (!def.isOptional) required.push(key);
        continue;
      }
    }
    
    properties[key] = {
      type,
      description: def?.description || ''
    };
    
    // Check if required (not optional)
    if (def && !def.isOptional) {
      required.push(key);
    }
  }
  
  return {
    type: 'object',
    properties,
    required: required.length > 0 ? required : undefined
  };
}

/**
 * Register all Container Kit tools with an MCP server
 * @param {Object} server - MCP server instance (SDK Server or custom implementation)
 * @param {Object} [nameMapping={}] - Optional mapping of original tool names to custom names
 * @example
 * const { Server } = require('@modelcontextprotocol/sdk');
 * const { registerAllTools } = require('@thgamble/containerization-assist-mcp');
 * 
 * const server = new Server();
 * 
 * // Register with default names
 * registerAllTools(server);
 * 
 * // Or with custom names
 * registerAllTools(server, {
 *   'analyze_repository': 'analyze',
 *   'build_image': 'docker-build'
 * });
 */
function registerAllTools(server, nameMapping = {}) {
  Object.values(tools).forEach(tool => {
    const customName = nameMapping[tool.name];
    registerTool(server, tool, customName);
  });
}

/**
 * Create a new session ID for workflow tracking
 * Session IDs are used to maintain state across multiple tool invocations
 * @returns {string} Generated session ID in format: session-TIMESTAMP-RANDOM
 * @example
 * const sessionId = createSession();
 * // Returns: "session-2024-01-15T10-30-45-abc123def"
 */
function createSession() {
  return generateSessionId();
}

// Named exports for individual tools
export {
  // Workflow tools
  analyzeRepository,
  verifyDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  verifyK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  
  // Utility tools
  listTools,
  ping,
  serverStatus,
  
  // Helper functions
  registerTool,
  registerAllTools,
  createSession,
  convertZodToJsonSchema,
  
  // Tools collection
  tools
};

// Function to get all tools (for backwards compatibility)
export function getAllTools() {
  return tools;
}

// Default export for CommonJS compatibility
export default {
  // Workflow tools
  analyzeRepository,
  verifyDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  verifyK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  
  // Utility tools
  listTools,
  ping,
  serverStatus,
  
  // Tools collection
  tools,
  getAllTools,
  
  // Helper functions
  registerTool,
  registerAllTools,
  createSession,
  convertZodToJsonSchema
};