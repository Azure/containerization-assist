/**
 * Test utilities for creating mock objects
 */

import { jest } from '@jest/globals';
import type { Result } from '@/types';

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

/**
 * Creates a mock implementation of validatePath for testing
 * This helper reduces duplication across test files
 */
export function createMockValidatePath() {
  return jest.fn().mockImplementation(async (pathStr: string, options?: any): Promise<Result<string>> => {
    // Check if mocks indicate file doesn't exist
    const fs = require('node:fs').promises;
    try {
      if (options?.mustExist) {
        await fs.access(pathStr);
      }
      if (options?.mustBeFile || options?.readable || options?.writable) {
        await fs.stat(pathStr);
      }
      return { ok: true, value: pathStr };
    } catch (error) {
      return {
        ok: false,
        error: `Path does not exist: ${pathStr}`,
        guidance: {
          hint: 'The specified path could not be found on the filesystem',
          resolution: 'Verify the path is correct and the file/directory exists',
        },
      };
    }
  });
}