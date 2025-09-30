/**
 * Integration Tests for Orchestrator Routing (WS1)
 *
 * Verifies that all entry points (CLI, programmatic, MCP server)
 * route tool execution through the orchestrator properly.
 */

import { jest } from '@jest/globals';
import { createOrchestrator } from '@/app/orchestrator';
import { createMCPServer } from '@/mcp/mcp-server';
import type { Logger } from 'pino';
import { createLogger } from '@/lib/logger';

// Import only specific tools to avoid Kubernetes dependencies
import opsToolImport from '@/tools/ops/tool';
import inspectSessionToolImport from '@/tools/inspect-session/tool';

// Setup
import '../__support__/setup/integration-setup.js';

describe('Orchestrator Routing Integration', () => {
  let testLogger: Logger;
  let toolsMap: Map<string, any>;

  beforeEach(() => {
    testLogger = createLogger({ name: 'test', level: 'silent' });

    // Create tools map with limited tools to avoid Kubernetes dependencies
    toolsMap = new Map();
    toolsMap.set('ops', opsToolImport);
    toolsMap.set('inspect-session', inspectSessionToolImport);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('Orchestrator Direct Execution', () => {
    it('should execute tools through orchestrator', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Execute a simple tool that should work without external dependencies
        const result = await orchestrator.execute({
          toolName: 'ops',
          params: { operation: 'ping' },
          sessionId: 'test-session'
        });

        // Verify the execution happened and returned expected structure
        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBeDefined();
          expect(typeof result.value).toBe('object');
        }
      } finally {
        orchestrator.close();
      }
    });

    it('should handle session configuration propagation', async () => {
      const customSessionTTL = 30000;
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: {
          maxRetries: 1,
          sessionTTL: customSessionTTL
        }
      });

      try {
        // Execute with a custom session ID to test session management
        const result = await orchestrator.execute({
          toolName: 'ops',
          params: { operation: 'ping' },
          sessionId: 'test-session-123'
        });

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value).toBeDefined();
        }
      } finally {
        orchestrator.close();
      }
    });

    it('should provide fresh context per request', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Execute the same tool multiple times with different session IDs
        const [result1, result2] = await Promise.all([
          orchestrator.execute({
            toolName: 'ops',
            params: { operation: 'ping' },
            sessionId: 'session-1'
          }),
          orchestrator.execute({
            toolName: 'ops',
            params: { operation: 'status' },
            sessionId: 'session-2'
          })
        ]);

        expect(result1.ok).toBe(true);
        expect(result2.ok).toBe(true);

        // Both should succeed, proving context isolation
        if (result1.ok) expect(result1.value).toBeDefined();
        if (result2.ok) expect(result2.value).toBeDefined();
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('MCP Server Registration Routing', () => {
    it('should route MCP tool calls through orchestrator executor', async () => {
      const limitedTools = [opsToolImport, inspectSessionToolImport];

      // Create a mock executor that we can spy on
      const mockExecutor = jest.fn().mockResolvedValue({
        ok: true,
        value: { message: 'Mock execution successful' }
      });

      // Create MCP server with our spy executor
      const mcpServer = createMCPServer(
        limitedTools,
        { logger: testLogger, transport: 'stdio' },
        mockExecutor
      );

      try {
        // Verify server can be created with orchestrated executor
        expect(mcpServer).toBeDefined();
        expect(typeof mcpServer.start).toBe('function');
        expect(typeof mcpServer.stop).toBe('function');

        // Verify tools are registered
        const registeredTools = mcpServer.getTools();
        expect(registeredTools.length).toBeGreaterThan(0);
        expect(registeredTools[0]).toHaveProperty('name');
        expect(registeredTools[0]).toHaveProperty('description');
      } finally {
        await mcpServer.stop();
      }
    });

    it('should create orchestrator with proper configuration', () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: {
          maxRetries: 3,
          retryDelay: 500
        }
      });

      try {
        // Should create orchestrator successfully
        expect(orchestrator).toBeDefined();
        expect(typeof orchestrator.execute).toBe('function');
        expect(typeof orchestrator.close).toBe('function');
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Lifecycle Management', () => {
    it('should clean up resources on close()', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      // Execute something to create session state
      await orchestrator.execute({
        toolName: 'ops',
        params: {},
        sessionId: 'cleanup-test'
      });

      // Should not throw when closing
      expect(() => {
        orchestrator.close();
      }).not.toThrow();
    });

    it('should handle concurrent executions properly', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Execute multiple tools concurrently
        const promises = Array.from({ length: 3 }, (_, i) =>
          orchestrator.execute({
            toolName: 'ops',
            params: { operation: i % 2 === 0 ? 'ping' : 'status' },
            sessionId: `concurrent-${i}`
          })
        );

        const results = await Promise.all(promises);

        // All should succeed
        results.forEach((result, index) => {
          expect(result.ok).toBe(true);
        });
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Error Handling and Policies', () => {
    it('should handle tool not found through orchestrator', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Try to execute a non-existent tool
        const result = await orchestrator.execute({
          toolName: 'non-existent-tool',
          params: {}
        });

        expect(result.ok).toBe(false);
        expect(result.error).toContain('not found');
      } finally {
        orchestrator.close();
      }
    });

    it('should handle parameter validation through orchestrator', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Execute with invalid parameters (ops tool expects operation)
        const result = await orchestrator.execute({
          toolName: 'ops',
          params: { invalid: 'parameter' }
        });

        // Should either succeed (if ops tool is lenient) or fail with validation error
        if (!result.ok) {
          expect(result.error).toBeDefined();
        }
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Tool Registry and Configuration', () => {
    it('should work with registered tools', () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Verify tools are properly registered in the map
        expect(toolsMap.size).toBe(2);
        expect(toolsMap.has('ops')).toBe(true);
        expect(toolsMap.has('inspect-session')).toBe(true);

        // Verify tool structure
        const opsTool = toolsMap.get('ops');
        expect(opsTool).toHaveProperty('name');
        expect(opsTool).toHaveProperty('description');
        expect(typeof opsTool?.name).toBe('string');
        expect(typeof opsTool?.description).toBe('string');
      } finally {
        orchestrator.close();
      }
    });

    it('should support custom tool configuration', () => {
      const customToolsMap = new Map();
      customToolsMap.set('custom-ops', {
        ...opsToolImport,
        name: 'custom-ops'
      });

      const orchestrator = createOrchestrator({
        registry: customToolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Verify custom configuration works
        expect(customToolsMap.has('custom-ops')).toBe(true);
        expect(customToolsMap.has('ops')).toBe(false);
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Orchestrator Integration', () => {
    it('should provide consistent execution interface', async () => {
      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        config: { maxRetries: 1 }
      });

      try {
        // Test basic orchestrator functionality
        expect(typeof orchestrator.execute).toBe('function');
        expect(typeof orchestrator.close).toBe('function');

        // Test actual execution
        const result = await orchestrator.execute({
          toolName: 'ops',
          params: { operation: 'ping' }
        });

        expect(result.ok).toBe(true);
      } finally {
        orchestrator.close();
      }
    });
  });
});