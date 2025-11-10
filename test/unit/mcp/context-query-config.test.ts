/**
 * Unit tests for ToolContext policy query interface
 */
import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { createToolContext } from '@/mcp/context';
import type { RegoEvaluator } from '@/config/policy-rego';
import type { Logger } from 'pino';
import type { DockerfileGenerationConfig, K8sGenerationConfig } from '@/config/policy-generation-config';

describe('ToolContext.queryConfig', () => {
  let mockLogger: Logger;
  let mockPolicyEvaluator: RegoEvaluator;

  beforeEach(() => {
    mockLogger = {
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
      child: jest.fn(() => mockLogger),
      fatal: jest.fn(),
      trace: jest.fn(),
      silent: jest.fn(),
      level: 'debug',
    } as unknown as Logger;

    mockPolicyEvaluator = {
      queryConfig: jest.fn(),
      evaluate: jest.fn(),
      close: jest.fn(),
      policyPaths: ['/test/policy.rego'],
    } as unknown as RegoEvaluator;
  });

  describe('when policy is configured', () => {
    it('should return typed config on successful evaluation', async () => {
      // Arrange
      const expectedConfig = {
        dockerfile: {
          buildStrategy: 'multi-stage',
          baseImageCategory: 'distroless',
        },
      };

      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue(expectedConfig);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      const result = await ctx.queryConfig('containerization.generation_config', {
        language: 'java',
        environment: 'production',
      });

      // Assert
      expect(result).toEqual(expectedConfig);
      expect(mockPolicyEvaluator.queryConfig).toHaveBeenCalledWith(
        'containerization.generation_config',
        { language: 'java', environment: 'production' }
      );
    });

    it('should return null when policy evaluation returns null', async () => {
      // Arrange
      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue(null);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      const result = await ctx.queryConfig('containerization.generation_config', {
        language: 'java',
      });

      // Assert
      expect(result).toBeNull();
    });

    it('should propagate errors from policy evaluation', async () => {
      // Arrange
      const error = new Error('OPA CLI not found');
      (mockPolicyEvaluator.queryConfig as jest.Mock).mockRejectedValue(error);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act & Assert
      await expect(
        ctx.queryConfig('containerization.generation_config', { language: 'java' })
      ).rejects.toThrow('OPA CLI not found');
    });

    it('should support querying dockerfile config', async () => {
      // Arrange
      interface GenerationConfig {
        dockerfile?: DockerfileGenerationConfig;
      }

      const expectedConfig: GenerationConfig = {
        dockerfile: {
          buildStrategy: 'multi-stage',
          baseImageCategory: 'distroless',
          optimizationPriority: 'security',
        },
      };

      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue(expectedConfig);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      const result = await ctx.queryConfig<GenerationConfig>(
        'containerization.generation_config',
        { language: 'java', environment: 'production' }
      );

      // Assert - TypeScript should recognize the type
      expect(result?.dockerfile?.buildStrategy).toBe('multi-stage');
      expect(result?.dockerfile?.baseImageCategory).toBe('distroless');
      expect(result?.dockerfile?.optimizationPriority).toBe('security');
    });

    it('should support querying kubernetes config', async () => {
      // Arrange
      interface GenerationConfig {
        kubernetes?: K8sGenerationConfig;
      }

      const expectedConfig: GenerationConfig = {
        kubernetes: {
          resourceDefaults: {
            cpuRequest: '500m',
            cpuLimit: '1',
            memoryRequest: '512Mi',
            memoryLimit: '1Gi',
          },
          replicas: 3,
        },
      };

      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue(expectedConfig);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      const result = await ctx.queryConfig<GenerationConfig>(
        'containerization.generation_config',
        { language: 'node', environment: 'production', appName: 'myapp' }
      );

      // Assert - TypeScript should recognize the type
      expect(result?.kubernetes?.resourceDefaults?.cpuRequest).toBe('500m');
      expect(result?.kubernetes?.replicas).toBe(3);
    });
  });

  describe('when policy is not configured', () => {
    it('should return null gracefully', async () => {
      // Arrange
      const ctx = createToolContext(mockLogger, { policy: undefined });

      // Act
      const result = await ctx.queryConfig('containerization.generation_config', {
        language: 'java',
      });

      // Assert
      expect(result).toBeNull();
      expect(mockLogger.debug).toHaveBeenCalledWith(
        { packageName: 'containerization.generation_config' },
        'No policy configured, returning null for config query'
      );
    });

    it('should not throw errors when policy is undefined', async () => {
      // Arrange
      const ctx = createToolContext(mockLogger);

      // Act & Assert - should not throw
      const result = await ctx.queryConfig('containerization.generation_config', {
        language: 'java',
        environment: 'production',
      });

      expect(result).toBeNull();
    });
  });

  describe('type safety', () => {
    it('should support typed generics for strong typing', async () => {
      // Arrange
      interface TestConfig {
        dockerfile?: { buildStrategy: string };
        kubernetes?: { replicas: number };
      }

      const expectedConfig: TestConfig = {
        dockerfile: { buildStrategy: 'multi-stage' },
        kubernetes: { replicas: 3 },
      };

      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue(expectedConfig);

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      const result = await ctx.queryConfig<TestConfig>(
        'containerization.generation_config',
        { language: 'java' }
      );

      // Assert - TypeScript should provide strong typing
      expect(result?.dockerfile?.buildStrategy).toBe('multi-stage');
      expect(result?.kubernetes?.replicas).toBe(3);
    });
  });

  describe('logging behavior', () => {
    it('should log debug message when policy is not configured', async () => {
      // Arrange
      const ctx = createToolContext(mockLogger, { policy: undefined });

      // Act
      await ctx.queryConfig('containerization.generation_config', {
        language: 'node',
        environment: 'dev',
      });

      // Assert
      expect(mockLogger.debug).toHaveBeenCalledWith(
        { packageName: 'containerization.generation_config' },
        'No policy configured, returning null for config query'
      );
    });

    it('should not log when policy is configured and query succeeds', async () => {
      // Arrange
      (mockPolicyEvaluator.queryConfig as jest.Mock).mockResolvedValue({ test: 'value' });

      const ctx = createToolContext(mockLogger, { policy: mockPolicyEvaluator });

      // Act
      await ctx.queryConfig('containerization.generation_config', { language: 'node' });

      // Assert - debug should not be called by queryConfig wrapper
      // (the policy evaluator itself may log, but that's internal)
      expect(mockLogger.debug).not.toHaveBeenCalled();
    });
  });
});
