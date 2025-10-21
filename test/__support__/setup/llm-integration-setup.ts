import { beforeAll, afterAll } from '@jest/globals';

// Extend Jest timeout for LLM interactions
jest.setTimeout(180000); // 3 minutes default timeout

// Global test environment setup
beforeAll(() => {
  // Set test environment variables
  process.env.NODE_ENV = 'test';

  console.log('LLM integration tests initialized');

  // Reminder: Use 'source ../azure_keys.sh' to load Azure credentials
  // NEVER use mock endpoints like localhost:4141 for real tests
  if (process.env.OPENAI_BASE_URL?.includes('localhost:4141')) {
    console.warn('⚠️  WARNING: Using mock endpoint! Run "source ../azure_keys.sh" for real Azure credentials');
  }
});

afterAll(() => {
  // Global cleanup
});