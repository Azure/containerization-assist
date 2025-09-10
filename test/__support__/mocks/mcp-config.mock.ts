import type { MCPConfig } from '../../src/config/mcp-config.js';

/**
 * Mock configuration presets for different testing scenarios
 */
export const MOCK_CONFIG_PRESETS = {
  /**
   * Fast configuration for unit tests
   */
  fast: {
    resources: {
      maxSize: 1024 * 1024, // 1MB
      defaultTtl: 60000, // 1 minute
      cacheDir: './test-cache',
      enableCompression: false,
    },
    sampling: {
      maxCandidates: 3,
      defaultCandidates: 2,
      scoringWeights: {
        buildSpeed: 0.3,
        imageSize: 0.2,
        security: 0.3,
        maintainability: 0.1,
        performance: 0.1,
      },
      cacheTTL: 30000, // 30 seconds
      enableDeterministicScoring: true,
    },
    progress: {
      enableNotifications: true,
      batchSize: 5,
      flushInterval: 100,
      retainHistory: false,
      historyTTL: 60000,
    },
    tools: {
      enableResourceLinks: true,
      maxToolResponse: 512 * 1024, // 512KB
      timeoutMs: 30000, // 30 seconds
      retryAttempts: 1,
      enableDynamicEnablement: true,
    },
    testing: {
      enableInspector: true,
      benchmarkSamples: 3,
      performanceThresholds: {
        toolResponse: 200, // ms
        candidateGeneration: 10000, // 10s
        endToEndWorkflow: 60000, // 1 minute
      },
      enableRegressionDetection: true,
    },
    integration: {
      enableOrchestration: true,
      maxConcurrentOperations: 2,
      workflowTimeout: 120000, // 2 minutes
      enableDeploymentVerification: false, // Disabled for faster tests
      verificationTimeout: 30000,
    },
  } as MCPConfig,

  /**
   * Development configuration with debugging enabled
   */
  development: {
    resources: {
      maxSize: 10 * 1024 * 1024, // 10MB
      defaultTtl: 1800000, // 30 minutes
      cacheDir: './dev-cache',
      enableCompression: true,
    },
    sampling: {
      maxCandidates: 5,
      defaultCandidates: 3,
      scoringWeights: {
        buildSpeed: 0.25,
        imageSize: 0.20,
        security: 0.30,
        maintainability: 0.15,
        performance: 0.10,
      },
      cacheTTL: 600000, // 10 minutes
      enableDeterministicScoring: true,
    },
    progress: {
      enableNotifications: true,
      batchSize: 10,
      flushInterval: 500,
      retainHistory: true,
      historyTTL: 3600000, // 1 hour
    },
    tools: {
      enableResourceLinks: true,
      maxToolResponse: 2 * 1024 * 1024, // 2MB
      timeoutMs: 120000, // 2 minutes
      retryAttempts: 2,
      enableDynamicEnablement: true,
    },
    testing: {
      enableInspector: true,
      benchmarkSamples: 5,
      performanceThresholds: {
        toolResponse: 150,
        candidateGeneration: 20000, // 20s
        endToEndWorkflow: 180000, // 3 minutes
      },
      enableRegressionDetection: true,
    },
    integration: {
      enableOrchestration: true,
      maxConcurrentOperations: 3,
      workflowTimeout: 600000, // 10 minutes
      enableDeploymentVerification: true,
      verificationTimeout: 180000, // 3 minutes
    },
  } as MCPConfig,

  /**
   * Minimal configuration for basic functionality testing
   */
  minimal: {
    resources: {
      maxSize: 512 * 1024, // 512KB
      defaultTtl: 300000, // 5 minutes
      cacheDir: './minimal-cache',
      enableCompression: false,
    },
    sampling: {
      maxCandidates: 2,
      defaultCandidates: 1,
      scoringWeights: {
        buildSpeed: 0.5,
        imageSize: 0.3,
        security: 0.2,
        maintainability: 0.0,
        performance: 0.0,
      },
      cacheTTL: 60000,
      enableDeterministicScoring: false,
    },
    progress: {
      enableNotifications: false,
      batchSize: 1,
      flushInterval: 1000,
      retainHistory: false,
      historyTTL: 0,
    },
    tools: {
      enableResourceLinks: false,
      maxToolResponse: 256 * 1024, // 256KB
      timeoutMs: 15000,
      retryAttempts: 0,
      enableDynamicEnablement: false,
    },
    testing: {
      enableInspector: false,
      benchmarkSamples: 1,
      performanceThresholds: {
        toolResponse: 1000,
        candidateGeneration: 60000,
        endToEndWorkflow: 300000,
      },
      enableRegressionDetection: false,
    },
    integration: {
      enableOrchestration: false,
      maxConcurrentOperations: 1,
      workflowTimeout: 300000,
      enableDeploymentVerification: false,
      verificationTimeout: 60000,
    },
  } as MCPConfig,

  /**
   * Stress test configuration for performance testing
   */
  stress: {
    resources: {
      maxSize: 50 * 1024 * 1024, // 50MB
      defaultTtl: 3600000,
      cacheDir: './stress-cache',
      enableCompression: true,
    },
    sampling: {
      maxCandidates: 10,
      defaultCandidates: 5,
      scoringWeights: {
        buildSpeed: 0.2,
        imageSize: 0.2,
        security: 0.2,
        maintainability: 0.2,
        performance: 0.2,
      },
      cacheTTL: 1800000,
      enableDeterministicScoring: true,
    },
    progress: {
      enableNotifications: true,
      batchSize: 50,
      flushInterval: 2000,
      retainHistory: true,
      historyTTL: 7200000, // 2 hours
    },
    tools: {
      enableResourceLinks: true,
      maxToolResponse: 10 * 1024 * 1024, // 10MB
      timeoutMs: 600000, // 10 minutes
      retryAttempts: 3,
      enableDynamicEnablement: true,
    },
    testing: {
      enableInspector: true,
      benchmarkSamples: 20,
      performanceThresholds: {
        toolResponse: 50,
        candidateGeneration: 15000,
        endToEndWorkflow: 120000,
      },
      enableRegressionDetection: true,
    },
    integration: {
      enableOrchestration: true,
      maxConcurrentOperations: 10,
      workflowTimeout: 1800000, // 30 minutes
      enableDeploymentVerification: true,
      verificationTimeout: 600000, // 10 minutes
    },
  } as MCPConfig,
} as const;

export type MockConfigPreset = keyof typeof MOCK_CONFIG_PRESETS;

/**
 * Get a mock configuration by preset name
 */
export const getMockConfig = (preset: MockConfigPreset): MCPConfig => {
  return JSON.parse(JSON.stringify(MOCK_CONFIG_PRESETS[preset])) as MCPConfig;
};

/**
 * Create a custom mock configuration with overrides
 */
export const createMockConfig = (
  basePreset: MockConfigPreset = 'development',
  overrides: Partial<MCPConfig> = {},
): MCPConfig => {
  const baseConfig = getMockConfig(basePreset);
  return {
    ...baseConfig,
    ...overrides,
    resources: { ...baseConfig.resources, ...overrides.resources },
    sampling: {
      ...baseConfig.sampling,
      ...overrides.sampling,
      scoringWeights: {
        ...baseConfig.sampling.scoringWeights,
        ...overrides.sampling?.scoringWeights,
      },
    },
    progress: { ...baseConfig.progress, ...overrides.progress },
    tools: { ...baseConfig.tools, ...overrides.tools },
    testing: {
      ...baseConfig.testing,
      ...overrides.testing,
      performanceThresholds: {
        ...baseConfig.testing.performanceThresholds,
        ...overrides.testing?.performanceThresholds,
      },
    },
    integration: { ...baseConfig.integration, ...overrides.integration },
  };
};

/**
 * Specialized configuration helpers for different testing scenarios
 */
export const getTestConfigForResources = (): MCPConfig => createMockConfig('development', {
  resources: { maxSize: 20 * 1024 * 1024 }, // Larger for testing
  progress: { enableNotifications: true, retainHistory: true },
});

export const getTestConfigForSampling = (): MCPConfig => createMockConfig('development', {
  sampling: { maxCandidates: 7, defaultCandidates: 4 },
  resources: { cacheTTL: 300000 },
});

export const getTestConfigForInspection = (): MCPConfig => createMockConfig('development', {
  testing: {
    enableInspector: true,
    benchmarkSamples: 10,
    enableRegressionDetection: true,
  },
});

export const getTestConfigForTools = (): MCPConfig => createMockConfig('development', {
  tools: {
    enableResourceLinks: true,
    enableDynamicEnablement: true,
    maxToolResponse: 5 * 1024 * 1024,
  },
});

export const getTestConfigForIntegration = (): MCPConfig => createMockConfig('development', {
  integration: {
    enableOrchestration: true,
    maxConcurrentOperations: 5,
    enableDeploymentVerification: true,
  },
});

/**
 * Configuration validator for testing
 */
export const validateMockConfig = (config: MCPConfig): { valid: boolean; errors: string[] } => {
  const errors: string[] = [];

  // Validate scoring weights
  const totalWeight = Object.values(config.sampling.scoringWeights).reduce((sum, w) => sum + w, 0);
  if (Math.abs(totalWeight - 1.0) > 0.01) {
    errors.push(`Scoring weights must sum to 1.0, got ${totalWeight}`);
  }

  // Validate positive values
  if (config.resources.maxSize <= 0) errors.push('Resource maxSize must be positive');
  if (config.sampling.maxCandidates <= 0) errors.push('Max candidates must be positive');
  if (config.tools.timeoutMs <= 0) errors.push('Tool timeout must be positive');

  // Validate relationships
  if (config.sampling.defaultCandidates > config.sampling.maxCandidates) {
    errors.push('Default candidates cannot exceed max candidates');
  }

  return {
    valid: errors.length === 0,
    errors,
  };
};
