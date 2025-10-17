/**
 * Example: App Mod Telemetry Integration
 *
 * This example demonstrates how the App Mod team can wrap Container Assist tools
 * with their telemetry layer using the new MCPTool interface.
 *
 * The new interface exposes:
 * - `name`: Tool identifier
 * - `description`: Human-readable description
 * - `inputSchema`: ZodRawShape for MCP SDK registration
 * - `parse(args)`: Validates and converts to strongly-typed input
 * - `handler(typedInput, context)`: Executes with pre-validated input
 * - `metadata`: Tool capabilities and enhancement info
 *
 * TOOL CONTEXT REQUIREMENTS:
 * The ToolContext interface requires these properties:
 * - `logger`: Pino-compatible logger with debug/info/warn/error/fatal/trace/child methods
 * - `signal`: Optional AbortSignal for cancellation
 * - `progress`: Optional progress reporting function
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { buildImageTool, deployTool, ALL_TOOLS } from 'containerization-assist-mcp';
import type { MCPTool } from 'containerization-assist-mcp';
import type { ToolContext } from 'containerization-assist-mcp';

/**
 * Mock telemetry client interface
 * Replace with your actual telemetry implementation
 */
interface TelemetryClient {
  recordToolInvocation(data: {
    toolName: string;
    parameters: Record<string, unknown>;
    timestamp: number;
  }): void;
  recordToolResult(data: {
    toolName: string;
    success: boolean;
    duration: number;
    timestamp: number;
  }): void;
  recordError(error: unknown): void;
}

/**
 * Example telemetry client implementation
 */
const telemetryClient: TelemetryClient = {
  recordToolInvocation: (data) => {
    console.log('📊 Telemetry - Tool Invocation:', {
      tool: data.toolName,
      params: Object.keys(data.parameters),
      timestamp: new Date(data.timestamp).toISOString(),
    });
  },
  recordToolResult: (data) => {
    console.log('📊 Telemetry - Tool Result:', {
      tool: data.toolName,
      success: data.success ? '✅' : '❌',
      duration: `${data.duration}ms`,
      timestamp: new Date(data.timestamp).toISOString(),
    });
  },
  recordError: (error) => {
    console.error('📊 Telemetry - Error:', error);
  },
};

/**
 * Create a simple console-based logger that implements the Pino Logger interface
 * For production use, replace with actual Pino logger or your logging library
 */
function createSimpleLogger() {
  const logger: any = {
    debug: (msg: any, ...args: any[]) => console.debug('🔍', msg, ...args),
    info: (msg: any, ...args: any[]) => console.log('ℹ️', msg, ...args),
    warn: (msg: any, ...args: any[]) => console.warn('⚠️', msg, ...args),
    error: (msg: any, ...args: any[]) => console.error('❌', msg, ...args),
    fatal: (msg: any, ...args: any[]) => console.error('💀', msg, ...args),
    trace: (msg: any, ...args: any[]) => console.trace('🔎', msg, ...args),
    silent: () => { }, // No-op for silent logging
    level: 'info',
    child: (bindings?: any) => {
      console.log('📋 Logger child created with bindings:', bindings);
      return logger; // Return self for simplicity
    },
  };
  return logger;
}


/**
 * Create a complete example ToolContext for demonstration purposes
 * This shows all required interface properties with working implementations
 */
function createExampleToolContext(): ToolContext {
  const logger = createSimpleLogger();

  return {
    logger,
    signal: undefined,
    progress: async (message: string, current?: number, total?: number) => {
      const progressStr = current !== undefined && total !== undefined
        ? ` (${current}/${total})`
        : '';
      console.log(`⏳ Progress: ${message}${progressStr}`);
    },
  };
}

/**
 * Extract relevant properties from typed input for telemetry
 * Customize based on what properties you want to track
 */
function extractTelemetryProps(input: Record<string, unknown>): Record<string, unknown> {
  // Create a shallow copy and filter out sensitive data if needed
  const { ...props } = input;

  // You can customize this to:
  // 1. Remove sensitive fields (passwords, tokens, etc.)
  // 2. Extract only specific fields you want to track
  // 3. Transform or aggregate data

  return props;
}

/**
 * Format MCP response from Container Assist Result type
 */
function formatMCPResponse(result: { ok: boolean; value?: unknown; error?: string }) {
  if (!result.ok) {
    throw new Error(result.error || 'Tool execution failed');
  }

  return {
    content: [
      {
        type: 'text' as const,
        text: JSON.stringify(result.value, null, 2),
      },
    ],
  };
}

/**
 * Wrap a Container Assist tool with telemetry tracking
 *
 * This is the core pattern for integrating Container Assist tools
 * with external telemetry systems.
 */
function registerToolWithTelemetry(
  server: McpServer,
  tool: MCPTool,
  context: ToolContext,
) {
  // Register tool with MCP server using exposed properties
  server.tool(
    tool.name,                // String: Tool identifier
    tool.description,         // String: Human-readable description
    tool.inputSchema,         // ZodRawShape: For MCP SDK registration
    async (args) => {
      const startTime = Date.now();

      try {
        // Step 1: Parse to strongly-typed input
        // This uses Zod validation and throws ZodError if invalid
        const typedInput = tool.parse(args || {});

        // Step 2: Record telemetry with typed input properties
        telemetryClient.recordToolInvocation({
          toolName: tool.name,
          parameters: extractTelemetryProps(typedInput as Record<string, unknown>),
          timestamp: startTime,
        });

        // Step 3: Execute tool handler with typed input
        const result = await tool.handler(typedInput, context);

        // Step 4: Record result metrics
        telemetryClient.recordToolResult({
          toolName: tool.name,
          success: result.ok,
          duration: Date.now() - startTime,
          timestamp: Date.now(),
        });

        // Step 5: Format and return MCP response
        return formatMCPResponse(result);
      } catch (error) {
        // Record error telemetry
        telemetryClient.recordError(error);
        telemetryClient.recordToolResult({
          toolName: tool.name,
          success: false,
          duration: Date.now() - startTime,
          timestamp: Date.now(),
        });
        throw error;
      }
    },
  );
}

/**
 * Example: Register all Container Assist tools with telemetry
 */
async function main() {
  // Create MCP server
  const server = new McpServer({
    name: 'app-mod-containerization-server',
    version: '1.0.0',
  });

  // Create tool context with proper interface implementation
  const context: ToolContext = createExampleToolContext();

  // Register all Container Assist tools with telemetry wrapper
  console.log(`\n🚀 Registering ${ALL_TOOLS.length} Container Assist tools with telemetry...\n`);

  for (const tool of ALL_TOOLS) {
    registerToolWithTelemetry(server, tool, context);
    console.log(`✅ Registered: ${tool.name}`);
    console.log(`   Description: ${tool.description}`);
    console.log(`   Schema keys: ${Object.keys(tool.inputSchema).join(', ')}`);
    console.log(`   Knowledge-enhanced: ${tool.metadata.knowledgeEnhanced}`);
    console.log('');
  }

  console.log(`\n✨ All tools registered successfully!\n`);

  // Connect transport and start server
  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.log('🎉 MCP Server with telemetry is running!');
}

/**
 * Example: Demonstrate the telemetry pattern with a specific tool
 */
async function demonstratePattern() {
  console.log('\n📚 Telemetry Pattern Demonstration\n');
  console.log('='.repeat(60));

  // Example: Build Image Tool
  const buildTool = buildImageTool;

  console.log(`\n🔧 Tool: ${buildTool.name}`);
  console.log(`📝 Description: ${buildTool.description}`);
  console.log(`🔑 Input Schema Keys: ${Object.keys(buildTool.inputSchema).join(', ')}`);
  console.log(`🧠 Knowledge Enhanced: ${buildTool.metadata.knowledgeEnhanced}`);

  // Demonstrate parse and validation
  console.log('\n--- Step 1: Parse & Validate ---');
  const testArgs = {
    path: '/app',
    imageName: 'my-app:latest',
    buildArgs: { NODE_ENV: 'production' },
  };

  console.log('Input args:', testArgs);

  try {
    const typedInput = buildTool.parse(testArgs);
    console.log('✅ Parsed to typed input:', typedInput);
    console.log('   Type safety maintained through parse → handler flow');
  } catch (error) {
    console.error('❌ Validation error:', error);
  }

  // Demonstrate invalid input
  console.log('\n--- Step 2: Validation Error Handling ---');
  const invalidArgs = {
    path: 123, // Should be string
    imageName: 'test:latest',
  };

  console.log('Invalid args:', invalidArgs);

  try {
    buildTool.parse(invalidArgs as any);
    console.log('❌ Should have thrown validation error');
  } catch (error: any) {
    console.log('✅ Caught validation error (as expected)');
    console.log(`   Error type: ${error.constructor.name}`);
  }

  // Show metadata usage
  console.log('\n--- Step 3: Tool Metadata for Categorization ---');
  console.log('Tools can be categorized by metadata:');

  const knowledgeEnhancedTools = ALL_TOOLS.filter((t) => t.metadata.knowledgeEnhanced);
  const operationalTools = ALL_TOOLS.filter((t) => !t.metadata.knowledgeEnhanced);

  console.log(`\n📚 Knowledge-Enhanced Tools (${knowledgeEnhancedTools.length}):`);
  knowledgeEnhancedTools.forEach((t) => console.log(`   - ${t.name}`));

  console.log(`\n⚙️  Operational Tools (${operationalTools.length}):`);
  operationalTools.forEach((t) => console.log(`   - ${t.name}`));

  console.log('\n' + '='.repeat(60));
}

/**
 * Example: Advanced telemetry with custom metrics
 */
function createAdvancedTelemetryWrapper(
  server: McpServer,
  tool: MCPTool,
  context: ToolContext,
) {
  server.tool(tool.name, tool.description, tool.inputSchema, async (args) => {
    const startTime = Date.now();
    const metrics = {
      toolName: tool.name,
      knowledgeEnhanced: tool.metadata.knowledgeEnhanced,
      parseTime: 0,
      executionTime: 0,
      totalTime: 0,
    };

    try {
      // Measure parse time
      const parseStart = Date.now();
      const typedInput = tool.parse(args || {});
      metrics.parseTime = Date.now() - parseStart;

      // Record invocation with detailed parameter analysis
      telemetryClient.recordToolInvocation({
        toolName: tool.name,
        parameters: {
          ...extractTelemetryProps(typedInput as Record<string, unknown>),
          _meta: {
            paramCount: Object.keys(typedInput).length,
            knowledgeEnhanced: tool.metadata.knowledgeEnhanced,
          },
        },
        timestamp: startTime,
      });

      // Measure execution time
      const execStart = Date.now();
      const result = await tool.handler(typedInput, context);
      metrics.executionTime = Date.now() - execStart;
      metrics.totalTime = Date.now() - startTime;

      // Record detailed metrics
      telemetryClient.recordToolResult({
        toolName: tool.name,
        success: result.ok,
        duration: metrics.totalTime,
        timestamp: Date.now(),
      });

      console.log(`⏱️  Performance Metrics for ${tool.name}:`, metrics);

      return formatMCPResponse(result);
    } catch (error) {
      metrics.totalTime = Date.now() - startTime;
      telemetryClient.recordError(error);
      telemetryClient.recordToolResult({
        toolName: tool.name,
        success: false,
        duration: metrics.totalTime,
        timestamp: Date.now(),
      });
      throw error;
    }
  });
}

// Run the example if executed directly
if (import.meta.url === `file://${process.argv[1]}`) {
  demonstratePattern()
    .then(() => main())
    .catch(console.error);
}

// Export for use in other modules
export {
  registerToolWithTelemetry,
  createAdvancedTelemetryWrapper,
  extractTelemetryProps,
  formatMCPResponse,
  createExampleToolContext,
  createSimpleLogger,
};