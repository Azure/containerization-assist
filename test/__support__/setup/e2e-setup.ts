import { jest } from '@jest/globals';
import { setupMCPTestEnvironment, cleanupMCPTestEnvironment } from '../utilities/mcp-environment';

// Extended timeout for e2e tests
jest.setTimeout(120000);

let mcpEnvironment: any;

beforeAll(async () => {
  // Set up MCP server and test environment
  mcpEnvironment = await setupMCPTestEnvironment();
  (global as any).mcpClient = mcpEnvironment.client;
  (global as any).testRepositories = mcpEnvironment.repositories;
});

afterAll(async () => {
  // Clean up MCP environment
  if (mcpEnvironment) {
    await cleanupMCPTestEnvironment(mcpEnvironment);
  }
});

beforeEach(() => {
  (global as any).TEST_TIMEOUT = 120000;
});

export {};