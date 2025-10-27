/**
 * MCP Server Integration with Telemetry and Type Safety
 *
 * This example shows how to integrate Container Assist tools with an MCP server
 * while maintaining full control over telemetry, error handling, and lifecycle hooks.
 *
 * ✨ NEW: Full TypeScript type safety for params and results!
 *
 * When you use literal tool names (e.g., 'build-image') with createToolHandler,
 * TypeScript automatically infers the specific types for:
 * - params: Strongly typed input parameters (e.g., BuildImageInput)
 * - result: Strongly typed result object (e.g., BuildImageResult)
 * - toolName: Literal type (e.g., 'build-image' instead of string)
 *
 * Use this pattern when you need:
 * - Custom telemetry tracking for tool executions with type-safe data access
 * - Error reporting and monitoring with typed parameters
 * - Custom logging and observability with full IntelliSense support
 * - Fine-grained control over tool registration
 *
 * Type Safety Summary:
 * ✅ createToolHandler(app, 'build-image', {...}) - Fully typed callbacks
 * ⚠️ registerTools(server, app, ALL_TOOLS, {...}) - Union types (broader, less specific)
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  createApp,
  ALL_TOOLS,
  createToolHandler,
  registerTools,
  type ToolHandlerOptions,
} from 'containerization-assist';

/**
 * Example telemetry service
 * Replace this with your actual telemetry provider
 */
class TelemetryService {
  trackToolExecution(toolName: string, success: boolean, durationMs?: number) {
    console.log(`[TELEMETRY] Tool: ${toolName}, Success: ${success}, Duration: ${durationMs}ms`);
    // Send to your telemetry backend (e.g., Application Insights, DataDog, etc.)
  }

  trackError(toolName: string, error: unknown) {
    console.error(`[TELEMETRY] Tool ${toolName} failed:`, error);
    // Send to your error tracking service
  }
}

const telemetry = new TelemetryService();

/**
 * Option 1: Use createToolHandler for maximum control and type safety
 *
 * This gives you full control over each tool registration with strongly-typed
 * callbacks when you use literal tool names.
 */
function registerWithMaximumControl(server: McpServer) {
  const app = createApp({
    outputFormat: 'natural-language',
    chainHintsMode: 'enabled',
  });

  // Example 1: Type-safe registration with literal tool name
  // TypeScript automatically infers the types for 'build-image'
  server.tool(
    'build-image',
    ALL_TOOLS.find((t) => t.name === 'build-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'build-image')!.inputSchema,
    createToolHandler(app, 'build-image', {
      transport: 'my-integration',

      // ✅ result is typed as BuildImageResult
      // ✅ params is typed as BuildImageInput
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);

        // Fully typed access to result properties!
        console.log(`[SUCCESS] Built image: ${result.imageId}`);
        console.log(`[SUCCESS] Image size: ${result.size} bytes`);
        console.log(`[SUCCESS] Tags: ${result.tags.join(', ')}`);

        // Fully typed access to params!
        console.log(`[SUCCESS] Built from: ${params.path || 'current directory'}`);
      },

      onError: (error, toolName, params) => {
        telemetry.trackError(toolName, error);

        // Fully typed params in error handler too!
        console.error(`[ERROR] Failed to build ${params.imageName}`);
      },
    }),
  );

  // Example 2: Type-safe registration for deploy tool
  server.tool(
    'deploy',
    ALL_TOOLS.find((t) => t.name === 'deploy')!.description,
    ALL_TOOLS.find((t) => t.name === 'deploy')!.inputSchema,
    createToolHandler(app, 'deploy', {
      transport: 'my-integration',

      // ✅ result is typed as DeployResult
      // ✅ params is typed as DeployInput
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);

        // Access typed deployment details
        console.log(`[SUCCESS] Deployed to namespace: ${result.namespace}`);
        console.log(`[SUCCESS] Status: ${result.status}`);
        console.log(`[SUCCESS] Replicas ready: ${result.readyReplicas}/${result.replicas}`);

        // Access typed input parameters
        console.log(`[SUCCESS] Deployment name: ${params.deploymentName}`);
      },

      onError: (error, toolName, params) => {
        telemetry.trackError(toolName, error);
        console.error(`[ERROR] Deploy failed for ${params.deploymentName}`);
      },
    }),
  );

  // Example 3: Scan image with typed security results
  server.tool(
    'scan-image',
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.inputSchema,
    createToolHandler(app, 'scan-image', {
      transport: 'my-integration',

      // ✅ result is typed as ScanImageResult with vulnerability details
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);

        // Access typed vulnerability data
        console.log(`[SECURITY] Scanned ${params.imageName}`);
        console.log(`[SECURITY] Critical: ${result.summary?.critical || 0}`);
        console.log(`[SECURITY] High: ${result.summary?.high || 0}`);
        console.log(`[SECURITY] Medium: ${result.summary?.medium || 0}`);
        console.log(`[SECURITY] Low: ${result.summary?.low || 0}`);

        // Send detailed metrics to telemetry
        telemetry.trackToolExecution(toolName, true, undefined);
      },

      onError: (error, toolName, params) => {
        telemetry.trackError(toolName, error);
        console.error(`[ERROR] Security scan failed for ${params.imageName}`);
      },
    }),
  );

  // For other tools, you can still use a loop with broader types
  const remainingTools = ALL_TOOLS.filter(
    (t) => !['build-image', 'deploy', 'scan-image'].includes(t.name),
  );

  for (const tool of remainingTools) {
    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      createToolHandler(app, tool.name as never, {
        transport: 'my-integration',
        onSuccess: (result, toolName) => {
          telemetry.trackToolExecution(toolName, true);
          console.log(`[SUCCESS] ${toolName} completed`);
        },
        onError: (error, toolName) => {
          telemetry.trackError(toolName, error);
          console.error(`[ERROR] ${toolName} failed`);
        },
      }),
    );
  }

  console.log(`✅ Registered ${ALL_TOOLS.length} tools with type-safe telemetry`);
}

/**
 * Option 2: Use registerTools for convenience
 *
 * This is simpler when you want the same telemetry configuration for all tools.
 * Note: params and result are union types (all possible tool inputs/outputs),
 * so you won't get specific type safety. Best for simple logging/metrics.
 */
function registerWithConvenience(server: McpServer) {
  const app = createApp({
    outputFormat: 'natural-language',
  });

  // Register all tools at once with shared telemetry configuration
  // ⚠️ result and params are union types here - not fully type-safe
  registerTools(server, app, ALL_TOOLS, {
    transport: 'my-integration',

    onSuccess: (result, toolName, params) => {
      // toolName is correctly typed as the specific tool name
      telemetry.trackToolExecution(toolName, true);

      // For type-safe access, use type guards based on toolName
      if (toolName === 'build-image') {
        // Now TypeScript knows this is BuildImageResult
        const buildResult = result as { imageId: string; size: number; tags: string[] };
        console.log(`[TELEMETRY] Built image: ${buildResult.imageId}`);
      } else if (toolName === 'scan-image') {
        // TypeScript knows this is ScanImageResult
        const scanResult = result as { summary?: { critical?: number; high?: number } };
        console.log(`[TELEMETRY] Vulnerabilities: ${scanResult.summary?.critical || 0} critical`);
      }
    },

    onError: (error, toolName, params) => {
      telemetry.trackToolExecution(toolName, false);
      telemetry.trackError(toolName, error);

      // Log error message with proper handling
      const errorMsg = error instanceof Error ? error.message : String(error);
      console.error(`[TELEMETRY] ${toolName} failed: ${errorMsg}`);
    },
  });

  console.log(`✅ Registered ${ALL_TOOLS.length} tools with shared telemetry`);
}

/**
 * Option 3: Per-tool telemetry with different configurations and full type safety
 *
 * Use different telemetry settings for different categories of tools
 * while maintaining type safety for critical tools.
 */
function registerWithPerToolConfig(server: McpServer) {
  const app = createApp();

  // Critical tools with detailed, type-safe telemetry
  // Register each with literal tool names for full type inference

  // Build image - with typed telemetry
  server.tool(
    'build-image',
    ALL_TOOLS.find((t) => t.name === 'build-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'build-image')!.inputSchema,
    createToolHandler(app, 'build-image', {
      transport: 'critical-path',
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);
        // ✅ Fully typed - result is BuildImageResult
        console.log(`[CRITICAL] Built ${result.tags[0]}, size: ${result.size} bytes`);
        // Send detailed metrics to monitoring system
        // monitoringSystem.recordMetric('build.size', result.size);
        // monitoringSystem.recordMetric('build.duration', result.buildTime);
      },
      onError: (error, toolName, params) => {
        telemetry.trackToolExecution(toolName, false);
        telemetry.trackError(toolName, error);
        // ✅ Fully typed params
        console.error(`[ALERT] Failed to build ${params.imageName}!`);
        // Send alert to PagerDuty or similar
      },
    }),
  );

  // Deploy - with typed telemetry
  server.tool(
    'deploy',
    ALL_TOOLS.find((t) => t.name === 'deploy')!.description,
    ALL_TOOLS.find((t) => t.name === 'deploy')!.inputSchema,
    createToolHandler(app, 'deploy', {
      transport: 'critical-path',
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);
        // ✅ Fully typed - result is DeployResult
        console.log(
          `[CRITICAL] Deployed ${params.deploymentName} - ${result.readyReplicas}/${result.replicas} ready`,
        );
        if (result.readyReplicas !== result.replicas) {
          console.warn(`[CRITICAL] Not all replicas ready for ${params.deploymentName}`);
        }
      },
      onError: (error, toolName, params) => {
        telemetry.trackToolExecution(toolName, false);
        telemetry.trackError(toolName, error);
        console.error(`[ALERT] Deploy failed for ${params.deploymentName}!`);
      },
    }),
  );

  // Security scan - with typed vulnerability tracking
  server.tool(
    'scan-image',
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.inputSchema,
    createToolHandler(app, 'scan-image', {
      transport: 'critical-path',
      onSuccess: (result, toolName, params) => {
        telemetry.trackToolExecution(toolName, true);
        // ✅ Fully typed - result is ScanImageResult
        const criticalCount = result.summary?.critical || 0;
        const highCount = result.summary?.high || 0;

        console.log(
          `[CRITICAL] Scanned ${params.imageName}: ${criticalCount} critical, ${highCount} high vulnerabilities`,
        );

        // Alert if critical vulnerabilities found
        if (criticalCount > 0) {
          console.error(`[ALERT] ${criticalCount} CRITICAL vulnerabilities in ${params.imageName}!`);
          // Send security alert
        }
      },
      onError: (error, toolName, params) => {
        telemetry.trackToolExecution(toolName, false);
        telemetry.trackError(toolName, error);
        console.error(`[ALERT] Security scan failed for ${params.imageName}!`);
      },
    }),
  );

  // Other tools with basic telemetry (no type safety needed)
  const otherTools = ALL_TOOLS.filter(
    (t) => !['build-image', 'deploy', 'scan-image'].includes(t.name),
  );

  registerTools(server, app, otherTools, {
    transport: 'standard',
    onSuccess: (result, toolName) => {
      telemetry.trackToolExecution(toolName, true);
      console.log(`[INFO] ${toolName} completed successfully`);
    },
    onError: (error, toolName) => {
      telemetry.trackToolExecution(toolName, false);
      console.error(`[INFO] ${toolName} failed`);
    },
  });

  console.log(`✅ Registered 3 critical tools (type-safe) and ${otherTools.length} standard tools`);
}

/**
 * Main server setup
 */
async function main() {
  const server = new McpServer({
    name: 'containerization-assist-with-telemetry',
    version: '1.0.0',
  });

  // Choose your registration strategy:

  // Option 1: Maximum control (recommended for complex telemetry)
  registerWithMaximumControl(server);

  // Option 2: Convenience (recommended for simple telemetry)
  // registerWithConvenience(server);

  // Option 3: Per-tool configuration (recommended for mixed requirements)
  // registerWithPerToolConfig(server);

  // Start the server
  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.log('🚀 Container Assist MCP server started with telemetry integration');
}

// Handle graceful shutdown
process.on('SIGINT', () => {
  console.log('\n👋 Shutting down server...');
  process.exit(0);
});

// Start the server
main().catch((error) => {
  console.error('Failed to start server:', error);
  process.exit(1);
});
