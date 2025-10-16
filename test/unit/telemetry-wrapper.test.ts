/**
 * Unit Tests: Telemetry Wrapper Pattern
 * Tests the new MCPTool interface properties that enable external telemetry wrapping
 * by the App Mod team and other consumers.
 */

import { describe, it, expect, jest } from '@jest/globals';
import { buildImageTool, deployTool, analyzeRepoTool } from '../../src/tools/index';
import type { ToolContext } from '../../src/mcp/context';

// Mock tool context for testing
function createMockToolContext(): ToolContext {
  return {
    logger: {
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
      debug: jest.fn(),
      trace: jest.fn(),
      fatal: jest.fn(),
      child: jest.fn().mockReturnThis(),
    },
    sessionManager: {
      create: jest.fn(),
      get: jest.fn(),
      update: jest.fn(),
    },
    session: {
      id: 'test-session-123',
      get: jest.fn(),
      set: jest.fn(),
      pushStep: jest.fn(),
    },
  } as any;
}

describe('Telemetry Wrapper Pattern', () => {
  describe('Tool Interface Properties', () => {
    it('should expose all required properties for telemetry wrapping', () => {
      // Verify build-image tool has all required properties
      expect(buildImageTool).toHaveProperty('name');
      expect(buildImageTool).toHaveProperty('description');
      expect(buildImageTool).toHaveProperty('inputSchema');
      expect(buildImageTool).toHaveProperty('parse');
      expect(buildImageTool).toHaveProperty('handler');
      expect(buildImageTool).toHaveProperty('schema');
      expect(buildImageTool).toHaveProperty('metadata');

      // Verify property types
      expect(typeof buildImageTool.name).toBe('string');
      expect(typeof buildImageTool.description).toBe('string');
      expect(typeof buildImageTool.inputSchema).toBe('object');
      expect(typeof buildImageTool.parse).toBe('function');
      expect(typeof buildImageTool.handler).toBe('function');
    });

    it('should expose properties for all tools', () => {
      const tools = [buildImageTool, deployTool, analyzeRepoTool];

      for (const tool of tools) {
        expect(tool).toHaveProperty('name');
        expect(tool).toHaveProperty('description');
        expect(tool).toHaveProperty('inputSchema');
        expect(tool).toHaveProperty('parse');
        expect(tool).toHaveProperty('handler');
        expect(typeof tool.name).toBe('string');
        expect(typeof tool.description).toBe('string');
        expect(typeof tool.inputSchema).toBe('object');
        expect(typeof tool.parse).toBe('function');
        expect(typeof tool.handler).toBe('function');
      }
    });
  });

  describe('inputSchema Property', () => {
    it('should expose ZodRawShape for MCP SDK registration', () => {
      expect(buildImageTool.inputSchema).toBeDefined();
      expect(typeof buildImageTool.inputSchema).toBe('object');

      // inputSchema should have the shape properties
      expect(buildImageTool.inputSchema).toHaveProperty('path');
    });

    it('should be directly usable with MCP server.tool()', () => {
      // This simulates how App Mod team will use it
      const { name, description, inputSchema } = buildImageTool;

      expect(name).toBe('build-image');
      expect(typeof description).toBe('string');
      expect(typeof inputSchema).toBe('object');
      expect(inputSchema).toHaveProperty('path');
    });
  });

  describe('parse Method', () => {
    it('should parse and validate valid parameters', () => {
      const validParams = {
        path: '/app',
        imageName: 'test:latest',
        buildArgs: { NODE_ENV: 'production' },
      };

      const typedInput = buildImageTool.parse(validParams);

      expect(typedInput).toMatchObject({
        path: '/app',
        imageName: 'test:latest',
        buildArgs: { NODE_ENV: 'production' },
      });
    });

    it('should throw ZodError on invalid parameters', () => {
      const invalidParams = {
        path: 123, // Should be string
        imageName: 'test:latest',
      };

      expect(() => {
        buildImageTool.parse(invalidParams);
      }).toThrow(); // Zod will throw ZodError
    });

    it('should handle missing optional parameters', () => {
      const minimalParams = {
        path: '/app',
      };

      // Should not throw - parse should handle optional params
      expect(() => {
        buildImageTool.parse(minimalParams);
      }).not.toThrow();
    });

    it('should throw on invalid input types', () => {
      expect(() => {
        buildImageTool.parse({ path: 123 }); // path should be string
      }).toThrow();

      expect(() => {
        buildImageTool.parse({ imageName: ['not', 'a', 'string'] }); // imageName should be string
      }).toThrow();
    });

    it('should validate complex nested structures', () => {
      const validDeployParams = {
        manifestsPath: '/manifests',
        namespace: 'default',
        replicas: 3,
        port: 8080,
      };

      const typedInput = deployTool.parse(validDeployParams);

      expect(typedInput).toMatchObject(validDeployParams);
    });
  });

  describe('handler Method', () => {
    it('should accept pre-validated typed input from parse', async () => {
      // Mock the filesystem and Docker client
      const mockContext = createMockToolContext();

      // For this test, we're just verifying the API signature works
      // Actual functionality is tested in tool-specific unit tests
      const validParams = {
        path: '/test/nonexistent/path',
        imageName: 'test:v1',
      };

      const typedInput = buildImageTool.parse(validParams);

      // Handler should accept the typed input
      // We expect it to fail due to missing Dockerfile, but that's ok
      const result = await buildImageTool.handler(typedInput, mockContext);

      expect(result).toBeDefined();
      expect(result).toHaveProperty('ok');
    });

    it('should work with the telemetry wrapper pattern', async () => {
      const mockContext = createMockToolContext();

      // Simulate App Mod's telemetry wrapper usage
      const telemetryData: any[] = [];

      // This is how App Mod team will use it
      const wrapWithTelemetry = async (args: any) => {
        const startTime = Date.now();

        try {
          // Step 1: Parse to strongly-typed input (uses Zod validation)
          const typedInput = buildImageTool.parse(args);

          // Step 2: Record telemetry with typed input properties
          telemetryData.push({
            toolName: buildImageTool.name,
            parameters: typedInput,
            timestamp: startTime,
          });

          // Step 3: Execute tool handler with typed input
          const result = await buildImageTool.handler(typedInput, mockContext);

          // Step 4: Record result metrics
          telemetryData.push({
            toolName: buildImageTool.name,
            success: result.ok,
            duration: Date.now() - startTime,
          });

          return result;
        } catch (error) {
          telemetryData.push({
            toolName: buildImageTool.name,
            error: true,
            duration: Date.now() - startTime,
          });
          throw error;
        }
      };

      // Test the wrapper pattern
      const args = {
        path: '/test/nonexistent',
        imageName: 'test:latest',
      };

      const result = await wrapWithTelemetry(args);

      // Verify telemetry was recorded
      expect(telemetryData.length).toBeGreaterThanOrEqual(2);
      expect(telemetryData[0]).toHaveProperty('toolName', 'build-image');
      expect(telemetryData[0]).toHaveProperty('parameters');
      expect(telemetryData[1]).toHaveProperty('success');
      expect(telemetryData[1]).toHaveProperty('duration');
    });
  });

  describe('Type Safety', () => {
    it('should maintain type safety through parse -> handler flow', () => {
      const params = {
        path: '/app',
        imageName: 'test:latest',
      };

      // Parse returns typed input
      const typedInput = buildImageTool.parse(params);

      // TypeScript should infer the correct type for typedInput
      expect(typedInput).toHaveProperty('path');
      expect(typedInput.path).toBe('/app');

      // Handler expects the same type
      // This is validated at compile time by TypeScript
    });
  });

  describe('Real-world Telemetry Integration', () => {
    it('should support extraction of telemetry-relevant properties', () => {
      const params = {
        path: '/my-app',
        imageName: 'my-app:v2.0.0',
        tags: ['my-app:latest', 'my-app:v2'],
        buildArgs: { NODE_ENV: 'production' },
        platform: 'linux/amd64',
      };

      const typedInput = buildImageTool.parse(params);

      // Extract telemetry properties (what App Mod team will do)
      const telemetryProps = {
        path: typedInput.path,
        imageName: typedInput.imageName,
        tagsCount: typedInput.tags?.length || 0,
        buildArgsCount: Object.keys(typedInput.buildArgs || {}).length,
        platform: typedInput.platform,
      };

      expect(telemetryProps).toMatchObject({
        path: '/my-app',
        imageName: 'my-app:v2.0.0',
        tagsCount: 2,
        buildArgsCount: 1,
        platform: 'linux/amd64',
      });
    });

    it('should support error tracking in telemetry', async () => {
      const mockContext = createMockToolContext();
      const errorLog: any[] = [];

      const trackErrors = async (args: any) => {
        try {
          const typedInput = buildImageTool.parse(args);
          return await buildImageTool.handler(typedInput, mockContext);
        } catch (error) {
          errorLog.push({
            tool: buildImageTool.name,
            error: error instanceof Error ? error.message : String(error),
            args,
          });
          throw error;
        }
      };

      // Invalid args should be caught and logged
      try {
        await trackErrors({ path: 123 }); // Invalid type
      } catch (error) {
        // Expected to throw
      }

      expect(errorLog.length).toBeGreaterThan(0);
      expect(errorLog[0]).toHaveProperty('tool', 'build-image');
      expect(errorLog[0]).toHaveProperty('error');
    });
  });

  describe('Metadata Property', () => {
    it('should expose metadata for all tools', () => {
      expect(buildImageTool.metadata).toBeDefined();
      expect(buildImageTool.metadata).toHaveProperty('knowledgeEnhanced');
      expect(typeof buildImageTool.metadata.knowledgeEnhanced).toBe('boolean');
    });

    it('should allow telemetry to track tool capabilities', () => {
      // Telemetry can use metadata to categorize tools
      const toolCapabilities = {
        name: buildImageTool.name,
        knowledgeEnhanced: buildImageTool.metadata.knowledgeEnhanced,
      };

      expect(toolCapabilities).toMatchObject({
        name: 'build-image',
        knowledgeEnhanced: true,
      });
    });
  });
});
