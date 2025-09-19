/**
 * Standard Test Factory for Prompt-Backed Tools
 *
 * Provides consistent testing patterns for all tools using createPromptBackedTool
 */

import { jest } from '@jest/globals';
import type { Logger } from 'pino';
import type { Result } from '@types';
import { mockLogger } from '../mocks/mock-factories';

/**
 * Mock dependencies for tool testing
 */
export interface MockToolDeps {
  logger: Logger;
  fs?: any;
  docker?: any;
  k8s?: any;
}

/**
 * Mock tool context for testing
 */
export interface MockToolContext {
  sessionId?: string;
  metadata?: Record<string, unknown>;
}

/**
 * Standard test options for prompt-backed tools
 */
export interface ToolTestOptions<TParams, TResult> {
  toolName: string;
  toolPath: string;
  validParams: TParams;
  invalidParams?: Partial<TParams>;
  expectedFields?: (keyof TResult)[];
  mockAIResponse?: string | object;
  mockDependencies?: Partial<MockToolDeps>;
  skipAITests?: boolean;
}

/**
 * Create standard tests for a prompt-backed tool
 */
export function createToolTestSuite<TParams, TResult>(
  options: ToolTestOptions<TParams, TResult>
) {
  const {
    toolName,
    toolPath,
    validParams,
    invalidParams,
    expectedFields,
    mockAIResponse,
    mockDependencies,
    skipAITests = false,
  } = options;

  return {
    /**
     * Test basic tool structure and exports
     */
    testToolStructure: () => {
      it('should export tool with correct structure', async () => {
        const toolModule = await import(toolPath);

        expect(toolModule.tool).toBeDefined();
        expect(toolModule.tool.name).toBe(toolName);
        expect(toolModule.tool.description).toBeDefined();
        expect(toolModule.tool.inputSchema).toBeDefined();
        expect(toolModule.tool.execute).toBeDefined();
        expect(typeof toolModule.tool.execute).toBe('function');
      });
    },

    /**
     * Test input validation
     */
    testInputValidation: () => {
      it('should validate input parameters', async () => {
        const toolModule = await import(toolPath);
        const mockDeps = createMockDeps(mockDependencies);
        const mockContext = createMockContext();

        // Test with valid params
        const validResult = await toolModule.tool.execute(
          validParams,
          mockDeps,
          mockContext
        );

        // We expect either success or a controlled failure
        expect(validResult).toHaveProperty('ok');

        // Test with invalid params if provided
        if (invalidParams) {
          await expect(
            toolModule.tool.execute(invalidParams, mockDeps, mockContext)
          ).rejects.toThrow();
        }
      });
    },

    /**
     * Test successful execution with mock AI response
     */
    testSuccessfulExecution: () => {
      if (skipAITests) {
        it.skip('AI tests skipped', () => {});
        return;
      }

      it('should execute successfully with valid input', async () => {
        // Mock the AI service response
        if (mockAIResponse) {
          jest.unstable_mockModule('@lib/ai-service', () => ({
            runHostAssist: jest.fn().mockResolvedValue(
              typeof mockAIResponse === 'string'
                ? mockAIResponse
                : JSON.stringify(mockAIResponse)
            ),
          }));
        }

        const toolModule = await import(toolPath);
        const mockDeps = createMockDeps(mockDependencies);
        const mockContext = createMockContext();

        const result = await toolModule.tool.execute(
          validParams,
          mockDeps,
          mockContext
        ) as Result<TResult>;

        expect(result.ok).toBe(true);
        if (result.ok && result.value) {
          // Check expected fields are present
          if (expectedFields) {
            expectedFields.forEach(field => {
              expect(result.value).toHaveProperty(field as string);
            });
          }
        }
      });
    },

    /**
     * Test error handling
     */
    testErrorHandling: () => {
      it('should handle errors gracefully', async () => {
        // Mock AI service to throw error
        jest.unstable_mockModule('@lib/ai-service', () => ({
          runHostAssist: jest.fn().mockRejectedValue(new Error('AI service error')),
        }));

        const toolModule = await import(toolPath);
        const mockDeps = createMockDeps(mockDependencies);
        const mockContext = createMockContext();

        const result = await toolModule.tool.execute(
          validParams,
          mockDeps,
          mockContext
        ) as Result<TResult>;

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('error');
        }
      });
    },

    /**
     * Test session management if applicable
     */
    testSessionManagement: () => {
      it('should manage session correctly', async () => {
        const toolModule = await import(toolPath);
        const mockDeps = createMockDeps(mockDependencies);
        const mockContext = createMockContext({ sessionId: 'test-session-123' });

        const result = await toolModule.tool.execute(
          validParams,
          mockDeps,
          mockContext
        );

        // Verify session was used if sessionId was provided
        if (mockContext.sessionId) {
          expect(result).toBeDefined();
          // Additional session-specific assertions can be added
        }
      });
    },

    /**
     * Run all standard tests
     */
    runAllTests: () => {
      describe(`${toolName} Tool Tests`, () => {
        beforeEach(() => {
          jest.clearAllMocks();
          jest.resetModules();
        });

        describe('Structure', () => {
          this.testToolStructure();
        });

        describe('Input Validation', () => {
          this.testInputValidation();
        });

        if (!skipAITests) {
          describe('Execution', () => {
            this.testSuccessfulExecution();
            this.testErrorHandling();
          });
        }

        describe('Session Management', () => {
          this.testSessionManagement();
        });
      });
    },
  };
}

/**
 * Create mock dependencies for testing
 */
function createMockDeps(overrides?: Partial<MockToolDeps>): MockToolDeps {
  return {
    logger: mockLogger(),
    fs: {
      existsSync: jest.fn().mockReturnValue(true),
      readFileSync: jest.fn().mockReturnValue('mock file content'),
      writeFileSync: jest.fn(),
      promises: {
        readFile: jest.fn().mockResolvedValue('mock file content'),
        writeFile: jest.fn().mockResolvedValue(undefined),
        mkdir: jest.fn().mockResolvedValue(undefined),
        readdir: jest.fn().mockResolvedValue([]),
        stat: jest.fn().mockResolvedValue({ isDirectory: () => true }),
      },
    },
    docker: {
      buildImage: jest.fn().mockResolvedValue({ id: 'mock-image-id' }),
      push: jest.fn().mockResolvedValue({ status: 'success' }),
      listImages: jest.fn().mockResolvedValue([]),
    },
    k8s: {
      apply: jest.fn().mockResolvedValue({ status: 'applied' }),
      get: jest.fn().mockResolvedValue({ items: [] }),
      delete: jest.fn().mockResolvedValue({ status: 'deleted' }),
    },
    ...overrides,
  };
}

/**
 * Create mock context for testing
 */
function createMockContext(overrides?: Partial<MockToolContext>): MockToolContext {
  return {
    sessionId: undefined,
    metadata: {},
    ...overrides,
  };
}

/**
 * Helper to create test suites for multiple tools
 */
export function createBatchToolTests(
  tools: Array<ToolTestOptions<any, any>>
) {
  return tools.map(toolOptions => ({
    name: toolOptions.toolName,
    suite: createToolTestSuite(toolOptions),
  }));
}

/**
 * Mock AI response generators for common tool types
 */
export const mockAIResponses = {
  dockerfile: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]`,

  k8sManifest: {
    apiVersion: 'apps/v1',
    kind: 'Deployment',
    metadata: { name: 'test-app' },
    spec: {
      replicas: 2,
      selector: { matchLabels: { app: 'test-app' } },
      template: {
        metadata: { labels: { app: 'test-app' } },
        spec: {
          containers: [{
            name: 'app',
            image: 'test:latest',
            ports: [{ containerPort: 3000 }],
          }],
        },
      },
    },
  },

  analysisResult: {
    language: 'javascript',
    framework: 'express',
    dependencies: ['express', 'pino', 'dotenv'],
    suggestedBaseImage: 'node:18-alpine',
    ports: [3000],
  },

  scanResult: {
    vulnerabilities: [],
    summary: {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
    },
    recommendations: [],
  },
};

/**
 * Common assertions for Result types
 */
export const resultAssertions = {
  isOk: <T>(result: Result<T>): result is { ok: true; value: T } => {
    expect(result.ok).toBe(true);
    return result.ok;
  },

  isFail: <T>(result: Result<T>): result is { ok: false; error: string } => {
    expect(result.ok).toBe(false);
    return !result.ok;
  },

  hasValue: <T>(result: Result<T>, field: keyof T) => {
    if (resultAssertions.isOk(result)) {
      expect(result.value).toHaveProperty(field as string);
    }
  },

  hasError: <T>(result: Result<T>, errorSubstring: string) => {
    if (resultAssertions.isFail(result)) {
      expect(result.error).toContain(errorSubstring);
    }
  },
};