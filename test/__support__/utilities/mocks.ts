/**
 * Test utilities for creating mock objects
 */

import { jest } from '@jest/globals';

export function createMockContext(overrides: any = {}) {
  return {
    logger: {
      child: () => createMockContext().logger,
      info: jest.fn(),
      debug: jest.fn(),
      warn: jest.fn(),
      error: jest.fn()
    },
    progressEmitter: {
      emit: jest.fn()
    },
    mcpSampler: {
      sample: jest.fn()
    },
    dockerService: {
      buildImage: jest.fn(),
      tagImage: jest.fn(),
      pushImage: jest.fn(),
      scanImage: jest.fn()
    },
    kubernetesService: {
      deployApplication: jest.fn(),
      getClusterInfo: jest.fn()
    },
    ...overrides
  };
}

export function createMockProgressEmitter() {
  return {
    emit: jest.fn().mockResolvedValue(undefined)
  };
}

export function createMockMCPSampler() {
  return {
    sample: jest.fn().mockResolvedValue({
      success: true,
      content: 'mocked response'
    })
  };
}