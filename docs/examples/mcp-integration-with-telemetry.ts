/**
 * MCP Server Integration with Telemetry and Type Safety
 *
 * This example shows how to integrate Container Assist tools with an MCP server
 * while maintaining full control over telemetry, error handling, and lifecycle hooks.
 *
 * ✨ Full TypeScript type safety for params and results
 * 🔒 Safe telemetry practices to protect customer data
 *
 * When you use literal tool names (e.g., 'build-image') with createToolHandler,
 * TypeScript automatically infers the specific types:
 * - params: Strongly typed input parameters (e.g., BuildImageInput)
 * - result: Strongly typed result object (e.g., BuildImageResult)
 * - toolName: Literal type (e.g., 'build-image' instead of string)
 *
 * This gives you:
 * - Custom telemetry tracking with type-safe data access
 * - Error reporting with typed parameters
 * - Full IntelliSense support
 * - Compile-time safety
 *
 * Security Principle: "When in doubt, hash it out"
 * - Never log customer file paths, code content, or sensitive identifiers
 * - Use telemetry sanitization utilities to hash/obfuscate sensitive data
 * - Only log aggregate metrics and enum values
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  createApp,
  ALL_TOOLS,
  createToolHandler,
  type ToolInputMap,
  type ToolResultMap,
} from 'containerization-assist';
import { CHAINHINTSMODE } from 'containerization-assist-mcp/app/orchestrator-types';
import {
  createSafeTelemetryEvent,
  type SafeTelemetryEvent,
} from 'containerization-assist/lib/telemetry-utils';

/**
 * Example telemetry service with safe logging
 * Replace this with your actual telemetry provider
 */
class SafeTelemetryService {
  /**
   * Track tool execution with sanitized data
   * All customer-specific information is hashed or omitted
   */
  trackToolExecution(event: SafeTelemetryEvent) {
    // Log safe metrics only
    console.log('[TELEMETRY]', JSON.stringify(event, null, 2));

    // Send to your telemetry backend
    // Example integrations:
    // - Application Insights: this.appInsights.trackEvent('tool-execution', event);
    // - DataDog: this.datadogClient.increment('tool.execution', event);
    // - Custom backend: await fetch('/api/telemetry', { body: JSON.stringify(event) });
  }

  /**
   * Track errors without exposing customer data
   */
  trackError(toolName: string, errorType: string) {
    console.error(`[TELEMETRY] Tool ${toolName} failed: ${errorType}`);
    // Send error type only (not full error message which may contain paths)
  }
}

const telemetry = new SafeTelemetryService();

/**
 * Register tools with type-safe telemetry
 *
 * Uses createToolHandler for per-tool control with strongly-typed callbacks
 * and automatic customer data sanitization.
 */
function registerWithSafeTelemetry(server: McpServer) {
  const app = createApp({
    outputFormat: 'natural-language',
    chainHintsMode: CHAINHINTSMODE.ENABLED,
  });

  // Example 1: Safe telemetry with build-image
  // Customer paths and image names are hashed, only metrics are logged
  server.tool(
    'build-image',
    ALL_TOOLS.find((t) => t.name === 'build-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'build-image')!.inputSchema,
    createToolHandler(app, 'build-image', {
      transport: 'my-integration',

      // ✅ result is typed as BuildImageResult
      // ✅ params is typed as BuildImageInput
      // 🔒 Customer data is sanitized before logging
      onSuccess: (result, toolName, params) => {
        // Create safe telemetry event - paths/names are hashed
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: true, value: result as Record<string, unknown> },
        );

        telemetry.trackToolExecution(event);

        // ✅ Safe: Log aggregate metrics only
        console.log(`[SUCCESS] Image built - Size: ${result.size} bytes, Build time: ${result.buildTime}ms`);

        // ❌ UNSAFE: Don't log customer paths or image names directly
        // console.log(`[UNSAFE] Built image: ${result.imageId}`); // Contains customer data!
        // console.log(`[UNSAFE] Tags: ${result.tags.join(', ')}`); // Contains customer data!
      },

      onError: (error, toolName, params) => {
        // Create safe telemetry event for errors
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: false, error: String(error) },
        );

        telemetry.trackToolExecution(event);

        // ✅ Safe: Log error type only
        console.error(`[ERROR] Build failed: ${error instanceof Error ? error.constructor.name : 'Error'}`);

        // ❌ UNSAFE: Don't log customer image names
        // console.error(`[UNSAFE] Failed to build ${params.imageName}`); // Exposes customer data!
      },
    }),
  );

  // Example 2: Safe telemetry with deploy tool
  server.tool(
    'deploy',
    ALL_TOOLS.find((t) => t.name === 'deploy')!.description,
    ALL_TOOLS.find((t) => t.name === 'deploy')!.inputSchema,
    createToolHandler(app, 'deploy', {
      transport: 'my-integration',

      // ✅ result is typed as DeployResult
      // ✅ params is typed as DeployInput
      // 🔒 Deployment names and namespaces are hashed
      onSuccess: (result, toolName, params) => {
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: true, value: result as Record<string, unknown> },
        );

        telemetry.trackToolExecution(event);

        // ✅ Safe: Log aggregate metrics only
        console.log(`[SUCCESS] Deployment complete - Replicas: ${result.readyReplicas}/${result.replicas}, Status: ${result.status}`);

        // ❌ UNSAFE: Don't log customer namespace/deployment names
        // console.log(`[UNSAFE] Deployed to namespace: ${result.namespace}`); // Customer data!
        // console.log(`[UNSAFE] Deployment name: ${params.deploymentName}`); // Customer data!
      },

      onError: (error, toolName, params) => {
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: false, error: String(error) },
        );

        telemetry.trackToolExecution(event);
        console.error(`[ERROR] Deployment failed: ${error instanceof Error ? error.constructor.name : 'Error'}`);
      },
    }),
  );

  // Example 3: Safe telemetry with security scanning
  server.tool(
    'scan-image',
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.description,
    ALL_TOOLS.find((t) => t.name === 'scan-image')!.inputSchema,
    createToolHandler(app, 'scan-image', {
      transport: 'my-integration',

      // ✅ result is typed as ScanImageResult with vulnerability details
      // 🔒 Image names are hashed, only vulnerability counts are logged
      onSuccess: (result, toolName, params) => {
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: true, value: result as Record<string, unknown> },
        );

        telemetry.trackToolExecution(event);

        // ✅ Safe: Log aggregate vulnerability counts
        const critical = result.summary?.critical || 0;
        const high = result.summary?.high || 0;
        const medium = result.summary?.medium || 0;
        const low = result.summary?.low || 0;

        console.log(`[SECURITY] Scan complete - Critical: ${critical}, High: ${high}, Medium: ${medium}, Low: ${low}`);

        // Alert if critical vulnerabilities found (safe - just counts)
        if (critical > 0) {
          console.error(`[ALERT] ${critical} CRITICAL vulnerabilities detected!`);
        }

        // ❌ UNSAFE: Don't log customer image names
        // console.log(`[UNSAFE] Scanned ${params.imageName}`); // Customer data!
      },

      onError: (error, toolName, params) => {
        const event = createSafeTelemetryEvent(
          toolName,
          params as Record<string, unknown>,
          { ok: false, error: String(error) },
        );

        telemetry.trackToolExecution(event);
        console.error(`[ERROR] Security scan failed: ${error instanceof Error ? error.constructor.name : 'Error'}`);
      },
    }),
  );

  // For other tools, use safe telemetry with broader types
  const remainingTools = ALL_TOOLS.filter(
    (t) => !['build-image', 'deploy', 'scan-image'].includes(t.name),
  );

  for (const tool of remainingTools) {
    server.tool(
      tool.name,
      tool.description,
      tool.inputSchema,
      createToolHandler(app, tool.name, {
        transport: 'my-integration',
        onSuccess: (result, toolName, params) => {
          // Create safe telemetry event
          const event = createSafeTelemetryEvent(
            toolName,
            params as Record<string, unknown>,
            { ok: true, value: result as Record<string, unknown> },
          );

          telemetry.trackToolExecution(event);
          console.log(`[SUCCESS] ${toolName} completed`);
        },
        onError: (error, toolName, params) => {
          // Create safe telemetry event for errors
          const event = createSafeTelemetryEvent(
            toolName,
            params as Record<string, unknown>,
            { ok: false, error: String(error) },
          );

          telemetry.trackToolExecution(event);
          console.error(`[ERROR] ${toolName} failed`);
        },
      }),
    );
  }

  console.log(`✅ Registered ${ALL_TOOLS.length} tools with safe telemetry`);
}

/**
 * Main server setup
 */
async function main() {
  const server = new McpServer({
    name: 'containerization-assist-with-telemetry',
    version: '1.0.0',
  });

  // Register tools with type-safe telemetry
  registerWithSafeTelemetry(server);

  // Start the server
  const transport = new StdioServerTransport();
  await server.connect(transport);

  console.log('🚀 Container Assist MCP server started with safe telemetry integration');
  console.log('📊 All customer data is sanitized - only aggregate metrics are logged');
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
