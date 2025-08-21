/**
 * Container Kit MCP Tools
 * 
 * Individual tool exports for registration with any MCP server.
 * Each tool exports: name, metadata (with inputSchema), and handler.
 * 
 * @module containerization-assist-mcp
 */

// Import all tools
import analyzeRepository from './tools/analyze-repository.js';
import generateDockerfile from './tools/generate-dockerfile.js';
import buildImage from './tools/build-image.js';
import scanImage from './tools/scan-image.js';
import tagImage from './tools/tag-image.js';
import pushImage from './tools/push-image.js';
import generateK8sManifests from './tools/generate-k8s-manifests.js';
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
  generateDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  generateK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  listTools,
  ping,
  serverStatus
};

/**
 * Register a single tool with an MCP server
 * @param {Object} server - MCP server instance (SDK Server or custom implementation)
 * @param {Object} tool - Tool definition with name, description, inputSchema, and handler
 * @param {string} tool.name - Tool identifier
 * @param {string} tool.description - Tool description
 * @param {Object} tool.inputSchema - Tool input schema (Zod schema object)
 * @param {Function} tool.handler - Async function that handles tool execution
 * @param {string} [customName] - Optional custom name to override tool.name
 * @throws {Error} If server doesn't have addTool or registerTool method
 * @example
 * const { Server } = require('@modelcontextprotocol/sdk');
 * const { analyzeRepository, registerTool } = require('@thgamble/containerization-assist-mcp');
 * 
 * const server = new Server();
 * registerTool(server, analyzeRepository);
 */
function registerTool(server, tool, customName = null) {
  const name = customName || tool.name;
  
  // Check if server has the MCP SDK's addTool method
  if (typeof server.addTool === 'function') {
    // Real MCP SDK Server
    server.addTool(
      {
        name: name,
        description: tool.description,
        inputSchema: convertZodToJsonSchema(tool.inputSchema)
      },
      tool.handler
    );
  } else if (typeof server.registerTool === 'function') {
    // Mock or custom server with registerTool method
    server.registerTool(name, { description: tool.description, inputSchema: tool.inputSchema }, tool.handler);
  } else {
    throw new Error('Server must have either addTool() or registerTool() method');
  }
}

/**
 * Convert Zod schema to JSON Schema for MCP SDK compatibility
 * @param {Object} zodSchema - Zod schema object with validation rules
 * @returns {Object} JSON Schema compatible with MCP SDK
 * @private
 */
function convertZodToJsonSchema(zodSchema) {
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

// Export individual tools
export default {
  // Workflow tools
  analyzeRepository,
  generateDockerfile,
  buildImage,
  scanImage,
  tagImage,
  pushImage,
  generateK8sManifests,
  prepareCluster,
  deployApplication,
  verifyDeployment,
  
  // Utility tools
  listTools,
  ping,
  serverStatus,
  
  // Tools collection
  tools,
  
  // Helper functions
  registerTool,
  registerAllTools,
  createSession
};