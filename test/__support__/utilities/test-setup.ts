/**
 * Simple Test Setup Helpers - De-Enterprise Refactoring
 *
 * Replaces complex integration test setup with simple, direct helpers.
 * No lifecycle management, registry patterns, or configuration methods.
 */

import {
  setupMockFactories,
  setupFailureMocks,
  setupNetworkErrorMocks,
  mockLogger,
} from '../mocks/mock-factories';

/**
 * Standard test setup - use this for most tests
 */
export const setupTest = () => {
  const mocks = setupMockFactories();

  return {
    mocks,
    // Simple cleanup - no complex registry management
    cleanup: () => {
      // Jest automatically resets mocks between tests
      // No manual cleanup needed with simple approach
    },
  };
};

/**
 * Failure scenario test setup
 */
export const setupFailureTest = () => {
  const mocks = setupFailureMocks();

  return { mocks };
};

/**
 * Network error test setup
 */
export const setupNetworkErrorTest = () => {
  const mocks = setupNetworkErrorMocks();

  return { mocks };
};

/**
 * Minimal test setup for unit tests
 */
export const setupUnitTest = () => ({
  logger: mockLogger(),
});