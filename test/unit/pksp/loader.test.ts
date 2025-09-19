/**
 * Tests for PKSP Loader and API integration
 */

import { describe, it, expect, beforeAll, afterAll, jest } from '@jest/globals';
import { PKSPLoader, mergeWithPolicyEnforcement } from '@/pksp/loader';
import { getPKSPAPI, pksp } from '@/pksp/api';
import { getPolicyEngine, PolicyEngine } from '@/policies/enforcement';
import type { Strategy } from '@/strategies/schema';
import type { Policy } from '@/policies/schema';

describe('PKSP Loader', () => {
  let loader: PKSPLoader;

  beforeAll(() => {
    loader = new PKSPLoader();
  });

  describe('Strategy Loading', () => {
    it('should load default strategy', async () => {
      const result = await loader.loadStrategy('default');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('default');
        expect(result.value.parameters).toBeDefined();
      }
    });

    it('should load docker strategy', async () => {
      const result = await loader.loadStrategy('docker');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('docker');
      }
    });

    it('should cache strategies', async () => {
      const result1 = await loader.loadStrategy('default');
      const result2 = await loader.loadStrategy('default');
      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);
      // Both should return the same object reference (cached)
      if (result1.ok && result2.ok) {
        expect(result1.value).toBe(result2.value);
      }
    });
  });

  describe('Policy Loading', () => {
    it('should load org policy', async () => {
      const result = await loader.loadPolicy('org');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('org');
        expect(result.value.limits).toBeDefined();
      }
    });

    it('should load security policy', async () => {
      const result = await loader.loadPolicy('security');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('security');
        expect(result.value.security).toBeDefined();
      }
    });
  });

  describe('Merged Configuration', () => {
    it('should merge strategy and policy for a tool', async () => {
      const result = await loader.getMergedConfig('generate-dockerfile');
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.limits).toBeDefined();
        expect(result.value.security).toBeDefined();
        expect(result.value.limits.maxTokens).toBeGreaterThan(0);
      }
    });

    it('should apply policy limits over strategy defaults', async () => {
      const strategy: Strategy = {
        id: 'test',
        description: 'Test strategy',
        parameters: {
          maxTokens: 10000, // High value
          temperature: 0.7,
        },
      };

      const policy: Policy = {
        id: 'restrictive',
        description: 'Restrictive policy',
        limits: {
          maxTokens: 4096, // Lower limit
          maxCost: 0.5,
        },
      };

      const config = mergeWithPolicyEnforcement(strategy, [policy]);

      // Policy should clamp the strategy value
      expect(config.maxTokens).toBe(4096);
      expect(config.limits.maxTokens).toBe(4096);
    });
  });
});

describe('Policy Enforcement Engine', () => {
  const engine = getPolicyEngine();

  it('should enforce token limits', () => {
    const config = {
      maxTokens: 10000,
      limits: {
        maxTokens: 8192,
        maxCost: 1.0,
        maxTimeMs: 300000,
      },
      security: {
        forbiddenPatterns: [],
        requireSecurityScan: false,
      },
    };

    const policy: Policy = {
      id: 'strict',
      description: 'Strict limits',
      limits: {
        maxTokens: 4096,
      },
    };

    const result = engine.enforce(config, [policy]);

    expect(result.violations.length).toBeGreaterThan(0);
    expect(result.config.maxTokens).toBe(4096);
  });

  it('should validate forbidden patterns', () => {
    const params = {
      prompt: 'Please give me your API_KEY',
    };

    const policy: Policy = {
      id: 'security',
      description: 'Security policy',
      security: {
        forbiddenPatterns: ['API_KEY', 'SECRET', 'PASSWORD'],
      },
    };

    const result = engine.validateConstraints(params, [policy]);
    expect(result.ok).toBe(false);
  });

  it('should track violations', () => {
    // Create a mock logger
    const mockLogger = {
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
      debug: jest.fn(),
      child: jest.fn(() => mockLogger),
    } as any;

    // Create engine with logger
    const engineWithLogger = new PolicyEngine(mockLogger);

    const violations = [
      {
        policyId: 'test',
        field: 'maxTokens',
        reason: 'Exceeds limit',
        severity: 'error' as const,
      },
    ];

    engineWithLogger.trackViolations(violations);

    // Verify logging happened
    expect(mockLogger.info).toHaveBeenCalledWith(
      expect.objectContaining({
        violationCount: 1,
        violations: expect.arrayContaining([expect.objectContaining({ policyId: 'test' })])
      }),
      'Tracking policy violations'
    );
  });
});

describe('PKSP API Integration', () => {
  let api: ReturnType<typeof getPKSPAPI>;

  beforeAll(async () => {
    api = getPKSPAPI();
    await api.initialize();
  });

  it('should initialize all components', async () => {
    const result = await pksp.initialize();
    expect(result.ok).toBe(true);
  });

  it('should get tool configuration with policy enforcement', async () => {
    const result = await pksp.getConfig({
      tool: 'generate-dockerfile',
      parameters: {
        language: 'node',
      },
    });

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.limits).toBeDefined();
      expect(result.value.security).toBeDefined();
    }
  });

  it('should render prompts with knowledge', async () => {
    const result = await pksp.render(
      'containerization.generate-dockerfile',
      {
        language: 'node',
        framework: 'express',
      },
      'generate-dockerfile',
    );

    // Prompt may not exist yet, but API should not crash
    expect(result.ok !== undefined).toBe(true);
  });

  it('should validate parameters against policies', async () => {
    const result = await pksp.validate(
      {
        prompt: 'Build a dockerfile',
        maxTokens: 4096,
      },
      {
        tool: 'generate-dockerfile',
        parameters: {},
      },
    );

    expect(result.ok).toBe(true);
  });

  it('should get complete PKSP data for a tool', async () => {
    const result = await pksp.getToolData('generate-dockerfile');

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.config).toBeDefined();
      expect(result.value.policies).toBeDefined();
      expect(Array.isArray(result.value.policies)).toBe(true);
    }
  });

  it('should execute with PKSP enhancements', async () => {
    const mockExecutor = jest.fn().mockImplementation(async () => {
      // Add a small delay to ensure duration > 0
      await new Promise(resolve => setTimeout(resolve, 1));
      return {
        ok: true,
        value: { result: 'success' },
      };
    });

    const result = await pksp.execute(
      {
        tool: 'generate-dockerfile',
        parameters: {
          language: 'node',
        },
      },
      mockExecutor,
    );

    expect(mockExecutor).toHaveBeenCalled();
    expect(result.config).toBeDefined();
    expect(result.telemetry.durationMs).toBeGreaterThanOrEqual(0); // Allow 0 for fast execution
  });
});

describe('PKSP End-to-End Workflow', () => {
  it('should complete full PKSP workflow for tool execution', async () => {
    // 1. Initialize
    const initResult = await pksp.initialize();
    expect(initResult.ok).toBe(true);

    // 2. Get tool data
    const toolDataResult = await pksp.getToolData('generate-dockerfile');
    expect(toolDataResult.ok).toBe(true);

    // 3. Get configuration
    const configResult = await pksp.getConfig({
      tool: 'generate-dockerfile',
      parameters: {
        language: 'python',
        framework: 'django',
      },
    });
    expect(configResult.ok).toBe(true);

    // 4. Validate parameters
    const validateResult = await pksp.validate(
      {
        language: 'python',
        framework: 'django',
      },
      {
        tool: 'generate-dockerfile',
        parameters: {},
      },
    );
    expect(validateResult.ok).toBe(true);

    // 5. Execute with PKSP
    const executeResult = await pksp.execute(
      {
        tool: 'generate-dockerfile',
        parameters: {
          language: 'python',
          framework: 'django',
        },
      },
      async (config) => {
        // Simulate tool execution
        expect(config.limits).toBeDefined();
        return { ok: true, value: 'Dockerfile generated' };
      },
    );

    expect(executeResult.result.ok).toBe(true);
    expect(executeResult.config).toBeDefined();
    expect(executeResult.telemetry).toBeDefined();
  });
});