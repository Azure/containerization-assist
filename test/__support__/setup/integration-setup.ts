import { jest } from '@jest/globals';

// Extended timeout for integration tests
jest.setTimeout(60000);

beforeEach(() => {
  (global as any).TEST_TIMEOUT = 60000;
});

export {};