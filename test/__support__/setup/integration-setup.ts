import { jest } from '@jest/globals';
import { createRealInfrastructure } from '../utilities/real-infrastructure';
import { setupTestEnvironment, cleanupTestEnvironment } from '../utilities/environment';

// Extended timeout for integration tests
jest.setTimeout(60000);

let testEnvironment: any;

beforeAll(async () => {
  // Set up real infrastructure for integration tests
  testEnvironment = await setupTestEnvironment();
  (global as any).testInfrastructure = createRealInfrastructure(testEnvironment);
});

afterAll(async () => {
  // Clean up test environment
  if (testEnvironment) {
    await cleanupTestEnvironment(testEnvironment);
  }
});

beforeEach(() => {
  (global as any).TEST_TIMEOUT = 60000;
});

afterEach(async () => {
  // Clean up test artifacts
  await (global as any).testInfrastructure?.cleanup?.();
});

export {};