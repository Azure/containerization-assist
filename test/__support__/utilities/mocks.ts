/**
 * Test utilities for creating mock objects
 */

import { jest } from '@jest/globals';
import { nanoid } from 'nanoid';

export function createMockSession(overrides: any = {}) {
  return {
    id: nanoid(),
    project_name: 'test-project',
    status: 'active',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    workflow_state: {},
    metadata: {},
    ...overrides
  };
}

export function createMockContext(overrides: any = {}) {
  return {
    logger: {
      child: () => createMockContext().logger,
      info: jest.fn(),
      debug: jest.fn(),
      warn: jest.fn(),
      error: jest.fn()
    },
    sessionService: {
      get: jest.fn(),
      create: jest.fn(),
      updateAtomic: jest.fn(),
      delete: jest.fn()
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