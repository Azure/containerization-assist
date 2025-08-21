// Type definitions for @thgamble/container-assist-mcp

import { z } from 'zod';

/**
 * MCP tool metadata structure
 */
export interface MCPToolMetadata {
  title: string;
  description: string;
  inputSchema: Record<string, z.ZodType<any>>;
}

/**
 * MCP tool result structure
 */
export interface MCPToolResult {
  content: Array<{
    type: string;
    text?: string;
  }>;
}

/**
 * MCP tool definition
 */
export interface MCPTool {
  name: string;
  metadata: MCPToolMetadata;
  handler: (params: any) => Promise<MCPToolResult>;
}

/**
 * MCP server interface (partial - for typing the registration methods)
 * Supports both McpServer and low-level Server from @modelcontextprotocol/sdk
 */
export interface MCPServer {
  // McpServer style (high-level API)
  registerTool?(name: string, metadata: {
    title: string;
    description: string;
    inputSchema: Record<string, any>; // Zod schema
  }, handler: (params: any) => Promise<MCPToolResult>): void;
  
  // Low-level Server style
  addTool?(definition: {
    name: string;
    description: string;
    inputSchema: any; // JSON Schema
  }, handler: (params: any) => Promise<MCPToolResult>): void;
}

// Individual tool exports
export const analyzeRepository: MCPTool;
export const generateDockerfile: MCPTool;
export const buildImage: MCPTool;
export const scanImage: MCPTool;
export const tagImage: MCPTool;
export const pushImage: MCPTool;
export const generateK8sManifests: MCPTool;
export const prepareCluster: MCPTool;
export const deployApplication: MCPTool;
export const verifyDeployment: MCPTool;
export const listTools: MCPTool;
export const ping: MCPTool;
export const serverStatus: MCPTool;

// Tools collection
export const tools: {
  analyzeRepository: MCPTool;
  generateDockerfile: MCPTool;
  buildImage: MCPTool;
  scanImage: MCPTool;
  tagImage: MCPTool;
  pushImage: MCPTool;
  generateK8sManifests: MCPTool;
  prepareCluster: MCPTool;
  deployApplication: MCPTool;
  verifyDeployment: MCPTool;
  listTools: MCPTool;
  ping: MCPTool;
  serverStatus: MCPTool;
};

// Helper functions
export function registerTool(server: MCPServer | any, tool: MCPTool, customName?: string): void;
export function registerAllTools(server: MCPServer | any, nameMapping?: Record<string, string>): void;
export function createSession(): string;
export function convertZodToJsonSchema(zodSchema: any): any;