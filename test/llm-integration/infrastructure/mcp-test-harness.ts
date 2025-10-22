/**
 * MCP Integration Test Harness
 * Sets up real MCP server instances for testing LLM-tool interactions
 */

import { promises as fs } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createApp } from '../../../src/app/index.js';
import type { AppRuntime } from '../../../src/types/runtime.js';
import type { ToolDefinition, LLMTestContext, ToolCall, ToolResponse } from './llm-client-types.js';
import { ChatClient } from './chat-client.js';
import type { Logger } from 'pino';
import { createLogger } from '../../../src/lib/logger.js';
import { ALL_TOOLS } from '../../../src/tools/index.js';

export interface MCPTestServerConfig {
  port?: number;
  timeout?: number;
  workingDirectory?: string;
  enableTools?: string[]; // If provided, only these tools will be enabled
  disableTools?: string[]; // Tools to explicitly disable
}

export interface MCPTestServer {
  app: AppRuntime;
  tools: ToolDefinition[];
  workingDirectory: string;
  isRunning: boolean;
}

export class MCPTestHarness {
  private servers: Map<string, MCPTestServer> = new Map();
  private logger: Logger;

  constructor() {
    this.logger = createLogger({ name: 'mcp-test-harness' });
  }

  /**
   * Create and start an MCP server for testing
   */
  async createTestServer(
    serverName: string = 'default',
    config: MCPTestServerConfig = {}
  ): Promise<MCPTestServer> {
    if (this.servers.has(serverName)) {
      throw new Error(`MCP test server '${serverName}' already exists`);
    }

    // Create working directory
    const workingDirectory = config.workingDirectory ||
      await fs.mkdtemp(join(tmpdir(), `mcp-test-${serverName}-`));

    // Create app instance
    const app = createApp({
      logger: this.logger,
    });

    // Get available tools from the app
    const availableTools = app.listTools().map(tool => tool.name);

    // Filter tools based on config
    let enabledToolNames = availableTools;
    if (config.enableTools) {
      enabledToolNames = availableTools.filter(tool => config.enableTools!.includes(tool));
    }
    if (config.disableTools) {
      enabledToolNames = enabledToolNames.filter(tool => !config.disableTools!.includes(tool));
    }

    this.logger.info(
      {
        serverName,
        toolCount: enabledToolNames.length,
        workingDirectory,
        enabledTools: enabledToolNames
      },
      'Creating MCP test server'
    );

    const toolDefinitions: ToolDefinition[] = enabledToolNames.map(toolName => {
      const actualTool = ALL_TOOLS.find(tool => tool.name === toolName);

      if (!actualTool) {
        this.logger.error({ toolName, availableTools: ALL_TOOLS.map(t => t.name) }, 'Tool not found in ALL_TOOLS');
        throw new Error(`Tool '${toolName}' not found in ALL_TOOLS. Available tools: ${ALL_TOOLS.map(t => t.name).join(', ')}`);
      }

      console.log(`📋 Schema for ${actualTool.name}: Using original Zod schema directly`);

      return {
        name: actualTool.name,
        description: actualTool.description,
        inputSchema: actualTool.schema, // Use original Zod schema directly
        zodSchema: actualTool.schema, // Also provide direct access to Zod schema
      };
    });

    const testServer: MCPTestServer = {
      app,
      tools: toolDefinitions,
      workingDirectory,
      isRunning: false,
    };

    this.servers.set(serverName, testServer);

    // Start the server
    await this.startServer(serverName);

    return testServer;
  }

  /**
   * Start an MCP server
   */
  private async startServer(serverName: string): Promise<void> {
    const testServer = this.servers.get(serverName);
    if (!testServer) {
      throw new Error(`MCP test server '${serverName}' not found`);
    }

    try {
      // App is ready to use immediately (no separate startup needed)
      testServer.isRunning = true;
      this.logger.info({ serverName }, 'MCP test server started');
    } catch (error) {
      this.logger.error({ serverName, error }, 'Failed to start MCP test server');
      throw error;
    }
  }

  /**
   * Execute a tool call through the MCP server
   */
  async executeToolCall(
    serverName: string,
    toolCall: ToolCall
  ): Promise<ToolResponse> {
    const testServer = this.servers.get(serverName);
    if (!testServer || !testServer.isRunning) {
      throw new Error(`MCP test server '${serverName}' is not running`);
    }

    const startTime = Date.now();

    try {
      // Find the tool
      const tool = testServer.tools.find(t => t.name === toolCall.name);
      if (!tool) {
        throw new Error(`Tool '${toolCall.name}' not found on server '${serverName}'`);
      }

      // Log tool input clearly
      this.logger.info(
        {
          serverName,
          toolName: toolCall.name,
          toolId: toolCall.id,
        },
        `🔧 TOOL CALL INPUT: ${toolCall.name}`
      );
      console.log(`📥 Tool: ${toolCall.name}`);
      console.log(`📋 Arguments:`, JSON.stringify(toolCall.arguments, null, 2));

      // Debug: Log what the tool expects vs what we're sending
      console.log(`🔍 DEBUG: Validating arguments for ${toolCall.name}`);
      try {
        const parsed = tool.zodSchema.parse(toolCall.arguments);
        console.log(`✅ Arguments are valid:`, JSON.stringify(parsed, null, 2));
      } catch (error) {
        console.log(`❌ Argument validation failed:`, error instanceof Error ? error.message : String(error));
      }

      // Transform arguments if LLM uses common wrong parameter names
      const transformedArgs = { ...toolCall.arguments };

      // Execute the actual tool through the app
      const result = await testServer.app.execute(toolCall.name, transformedArgs);

      const endTime = Date.now();

      // Log tool output clearly
      this.logger.info(
        {
          serverName,
          toolName: toolCall.name,
          toolId: toolCall.id,
          latency: endTime - startTime,
          success: result.ok,
        },
        `✅ TOOL CALL OUTPUT: ${toolCall.name} (${endTime - startTime}ms)`
      );

      if (result.ok) {
        console.log(`📤 Result: ${toolCall.name} SUCCESS`);
        console.log(`📄 Content:`, typeof result.value === 'string' ? result.value : JSON.stringify(result.value, null, 2));
      } else {
        console.log(`❌ Result: ${toolCall.name} FAILED`);
        console.log(`🚫 Error:`, result.error?.message || 'Unknown error');
      }
      console.log('─'.repeat(80));

      return {
        toolCallId: toolCall.id,
        toolName: toolCall.name,
        content: result.ok ? result.value : null,
        error: result.ok ? undefined : result.error?.message,
      };
    } catch (error) {
      const endTime = Date.now();
      const errorMessage = error instanceof Error ? error.message : String(error);

      this.logger.error(
        {
          serverName,
          toolName: toolCall.name,
          toolId: toolCall.id,
          latency: endTime - startTime,
          error: errorMessage,
        },
        'Tool call failed'
      );

      return {
        toolCallId: toolCall.id,
        toolName: toolCall.name,
        content: null,
        error: errorMessage,
      };
    }
  }

  /**
   * Create a complete test context with LLM client and MCP server
   */
  async createTestContext(config: {
    serverName?: string;
    mcpConfig?: MCPTestServerConfig;
    llmProvider?: 'chat';
  } = {}): Promise<LLMTestContext> {
    const serverName = config.serverName || 'default';

    // Create MCP server if it doesn't exist
    let testServer = this.servers.get(serverName);
    if (!testServer) {
      testServer = await this.createTestServer(serverName, config.mcpConfig);
    }

    // Create LLM client - fail fast if no connection
    const client = new ChatClient(
    );

    // Validate connection - tests should fail if no LLM available
    const isConnected = await client.validateConnection();
    if (!isConnected) {
      throw new Error('LLM client connection failed - tests require a working LLM endpoint');
    }

    const session = client.createSession();

    return {
      client,
      session,
      mcpServer: {
        url: `stdio://${serverName}`, // Simulated URL for stdio transport
        tools: testServer.tools,
        executeToolCall: (toolCall: ToolCall) => this.executeToolCall(serverName, toolCall),
      },
    };
  }

  /**
   * Stop and cleanup a test server
   */
  async stopServer(serverName: string): Promise<void> {
    const testServer = this.servers.get(serverName);
    if (!testServer) {
      return;
    }

    try {
      if (testServer.isRunning) {
        testServer.isRunning = false;
      }

      // Cleanup working directory
      await fs.rm(testServer.workingDirectory, { recursive: true, force: true });

      this.servers.delete(serverName);

      this.logger.info({ serverName }, 'MCP test server stopped and cleaned up');
    } catch (error) {
      this.logger.error({ serverName, error }, 'Error stopping MCP test server');
    }
  }

  /**
   * Stop and cleanup all test servers
   */
  async cleanup(): Promise<void> {
    const serverNames = Array.from(this.servers.keys());
    await Promise.all(serverNames.map(name => this.stopServer(name)));
  }

  /**
   * Get available tools for a server
   */
  getAvailableTools(serverName: string): ToolDefinition[] {
    const testServer = this.servers.get(serverName);
    if (!testServer) {
      throw new Error(`MCP test server '${serverName}' not found`);
    }
    return testServer.tools;
  }

  /**
   * Get information about running servers
   */
  getServerInfo(): Array<{ name: string; isRunning: boolean; toolCount: number; workingDir: string }> {
    return Array.from(this.servers.entries()).map(([name, server]) => ({
      name,
      isRunning: server.isRunning,
      toolCount: server.tools.length,
      workingDir: server.workingDirectory,
    }));
  }

}

// Global test harness instance
let globalHarness: MCPTestHarness | null = null;

/**
 * Get or create the global test harness
 */
export function getMCPTestHarness(): MCPTestHarness {
  if (!globalHarness) {
    globalHarness = new MCPTestHarness();
  }
  return globalHarness;
}

/**
 * Cleanup global test harness (call in test teardown)
 */
export async function cleanupMCPTestHarness(): Promise<void> {
  if (globalHarness) {
    await globalHarness.cleanup();
    globalHarness = null;
  }
}