/**
 * Test dependencies factory
 */

import { MockMCPSampler } from './mock-mcp-sampler';
import { createMockLogger } from './test-helpers';
import type { Logger } from 'pino';

export interface TestDependencies {
  logger: Logger;
  mcpSampler: MockMCPSampler;
  structuredSampler: {
    sampleJSON: jest.Mock;
  };
  contentValidator: {
    validateContent: jest.Mock;
  };
  progressEmitter: {
    emit: jest.Mock;
    subscribe: jest.Mock;
  };
  dockerClient: {
    build: jest.Mock;
    tag: jest.Mock;
    push: jest.Mock;
    scan: jest.Mock;
  };
  kubernetesClient: {
    apply: jest.Mock;
    get: jest.Mock;
    delete: jest.Mock;
  };
}

export async function createTestDependencies(): Promise<TestDependencies> {
  const logger = createMockLogger();
  const mockSampler = new MockMCPSampler();
  
  return {
    logger,
    mcpSampler: mockSampler,
    structuredSampler: {
      sampleJSON: jest.fn()
    },
    contentValidator: {
      validateContent: jest.fn().mockReturnValue({ isValid: true, issues: [] })
    },
    progressEmitter: {
      emit: jest.fn(),
      subscribe: jest.fn()
    },
    dockerClient: {
      build: jest.fn(),
      tag: jest.fn(),
      push: jest.fn(),
      scan: jest.fn()
    },
    kubernetesClient: {
      apply: jest.fn(),
      get: jest.fn(),
      delete: jest.fn()
    }
  };
}