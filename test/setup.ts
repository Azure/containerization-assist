/**
 * Test Setup - Containerization Assist MCP Server
 * ESM-compatible test configuration and mocks
 */

import { jest } from '@jest/globals';

console.log('Setting up tests for Containerization Assist MCP TypeScript implementation');

// Set test environment
process.env.NODE_ENV = 'test';
process.env.LOG_LEVEL = process.env.LOG_LEVEL || 'error';
process.env.SILENT_TESTS = 'true';

// Jest configuration for ESM
jest.setTimeout(30000);

// Global test utilities
(global as any).jest = jest;
(global as any).testTimeout = 30000;
(global as any).testConfig = {
  timeout: 30000,
  retries: 2
};

// Mock console methods to reduce noise in tests
const originalConsole = console;
(global as any).console = {
  ...originalConsole,
  log: jest.fn(),
  warn: jest.fn(), 
  error: originalConsole.error, // Keep errors visible
};


export { };