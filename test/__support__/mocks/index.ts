/**
 * MCP infrastructure implementations and mocks
 *
 * This module provides both real implementations and mocks:
 * - Real implementations for production use
 * - Mock implementations for testing and development
 * - Factory functions for easy switching between real and mock
 *
 * Usage:
 * - Use createMCPInfrastructure() for real implementations
 * - Use createMockMCPInfrastructure() for mocks/testing
 * - Automatically uses mocks in test environment (NODE_ENV=test)
 */

import pino from 'pino';
import { loadMCPConfig } from '../../src/config/mcp-config.js';
import { McpResourceManager } from '../../src/mcp/resources/manager.js';
import { McpProgressNotifier } from '../../src/mcp/events/emitter.js';
import type { ResourceManager } from '../../src/mcp/resources/types.js';
import type { ProgressNotifier } from '../../src/mcp/events/types.js';
import type { MCPConfig } from '../../src/config/mcp-config.js';
import { createMockProgressNotifier } from './orchestration-mocks.js';

// Export mock implementations for testing
export {
  MockResourceManager,
  createMockResourceManager,
} from './resource-manager.mock.js';

export {
  MOCK_CONFIG_PRESETS,
  getMockConfig,
  createMockConfig,
  getTestConfigForResources,
  getTestConfigForInspection,
  getTestConfigForTools,
  getTestConfigForIntegration,
  validateMockConfig,
} from './mcp-config.mock.js';

export type { MockConfigPreset } from './mcp-config.mock.js';

// VS Code mock factories
export {
  createMockCancellationToken,
  createMockInvocationOptions,
  type MockCancellationToken,
} from './mock-factories.js';

/**
 * Create real MCP infrastructure with production implementations
 */
export function createMCPInfrastructure(configOverrides?: Partial<MCPConfig>): {
  config: MCPConfig;
  resourceManager: ResourceManager;
  progressNotifier: ProgressNotifier;
} {
  // Load real configuration
  const configResult = loadMCPConfig();
  if (!configResult.success) {
    throw new Error(`Failed to load MCP config: ${configResult.error}`);
  }

  const config = configOverrides 
    ? { ...configResult.data, ...configOverrides }
    : configResult.data;

  // Create logger
  const logger = pino({ 
    level: process.env.LOG_LEVEL || 'info',
    name: 'mcp-infrastructure'
  });

  // Create real implementations
  const resourceManager = new McpResourceManager(
    {
      defaultTtl: config.resources.defaultTtl,
      maxResourceSize: config.resources.maxSize,
      cacheConfig: {
        defaultTtl: config.resources.defaultTtl
      }
    },
    logger
  );

  const progressNotifier = new McpProgressNotifier(logger);

  return {
    config,
    resourceManager,
    progressNotifier,
  };
}

/**
 * Complete mock setup for all core MCP infrastructure  
 * Useful for testing and development
 */
export function createMockMCPInfrastructure(preset: 'fast' | 'development' | 'minimal' | 'stress' = 'development'): {
  config: any;
  resourceManager: any;
  progressNotifier: any;
} {
  const config = getMockConfig(preset);
  const resourceManager = createMockResourceManager({
    maxSize: config.resources.maxSize,
    defaultTtl: config.resources.defaultTtl,
    simulateLatency: preset === 'stress',
    failureRate: preset === 'stress' ? 0.02 : 0, // 2% failure rate for stress testing
  });

  const logger = pino({ level: preset === 'minimal' ? 'warn' : 'info' });
  const progressNotifier = createMockProgressNotifier(logger);

  return {
    config,
    resourceManager,
    progressNotifier,
  };
}

/**
 * Smart factory that uses real or mock implementations based on explicit mode or environment
 * 
 * @param configOverrides - Optional configuration overrides for real implementations
 * @param forceMode - Explicit mode selection: 'real' or 'mock'
 *                   - 'real': Always use real implementations
 *                   - 'mock': Always use mock implementations  
 *                   - undefined: Auto-detect based on NODE_ENV (test = mocks, others = real)
 */
export function createInfrastructure(
  configOverrides?: Partial<MCPConfig>,
  forceMode?: 'real' | 'mock'
): {
  config: MCPConfig | any;
  resourceManager: ResourceManager | any;
  progressNotifier: ProgressNotifier | any;
} {
  // Determine whether to use mocks based on explicit mode or test environment
  const useMocks = forceMode === 'mock' || 
                  (forceMode !== 'real' && process.env.NODE_ENV === 'test');

  if (useMocks) {
    // Use mocks for testing/development
    const preset = process.env.NODE_ENV === 'test' ? 'fast' : 'development';
    return createMockMCPInfrastructure(preset);
  } else {
    // Use real implementations
    return createMCPInfrastructure(configOverrides);
  }
}

/**
 * MCP infrastructure factory for different use cases
 */
export const MCPInfrastructure = {
  /** Standard configuration for general use */
  standard: () => createInfrastructure(),
  /** Configuration optimized for testing workflows */
  testing: () => createInfrastructure({
    testing: { enableInspector: true, benchmarkSamples: 10 }
  }),
  /** Configuration optimized for enhanced tooling */
  tooling: () => createInfrastructure({
    tools: { enableResourceLinks: true, enableDynamicEnablement: true }
  }),
  /** Configuration optimized for integration workflows */
  integration: () => createInfrastructure({
    integration: { enableOrchestration: true, maxConcurrentOperations: 5 }
  }),
} as const;

/**
 * Mock MCP infrastructure factory for testing and development
 */
export const MockMCPInfrastructure = {
  /** Standard mock setup for development */
  standard: () => createMockMCPInfrastructure('development'),
  /** Fast mocks for unit testing */
  fast: () => createMockMCPInfrastructure('fast'),
  /** Minimal mocks for basic testing */
  minimal: () => createMockMCPInfrastructure('minimal'),
  /** Stress testing mocks with failures */
  stress: () => createMockMCPInfrastructure('stress'),
} as const;


/**
 * Environment-based mock selection
 * Automatically chooses appropriate mocks based on NODE_ENV
 */
export function createEnvironmentMocks(): {
  config: any;
  resourceManager: any;
  progressNotifier: any;
} {
  const env = process.env.NODE_ENV || 'development';

  switch (env) {
    case 'test':
      return createMockMCPInfrastructure('fast');
    case 'development':
      return createMockMCPInfrastructure('development');
    case 'production':
      // In production, return minimal mocks for fallback scenarios
      return createMockMCPInfrastructure('minimal');
    default:
      return createMockMCPInfrastructure('development');
  }
}

/**
 * Mock utilities for testing
 */
export const MockUtils = {
  /**
   * Wait for mock operations to complete (useful for async testing)
   */
  async waitForOperations(ms: number = 100): Promise<void> {
    await new Promise(resolve => setTimeout(resolve, ms));
  },

  /**
   * Create a test scenario with predictable mock behavior
   */
  createTestScenario(name: string, config: {
    resourceFailureRate?: number;
    progressLatency?: boolean;
    maxResources?: number;
  } = {}) {
    return {
      name,
      resourceManager: createMockResourceManager({
        failureRate: config.resourceFailureRate || 0,
        maxSize: config.maxResources || 1024 * 1024,
      }),
      progressNotifier: createMockProgressNotifier(pino({ level: 'warn' })),
      config: getMockConfig('fast'),
    };
  },
};

/**
 * Type guards for mock identification
 */
export function isMockResourceManager(manager: any): manager is MockResourceManager {
  return manager && typeof manager.getResourceCount === 'function';
}
