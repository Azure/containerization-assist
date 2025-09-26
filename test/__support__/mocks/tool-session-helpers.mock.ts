/**
 * Creates a mock implementation for tool-session-helpers module
 */
export function createToolSessionHelpersMock() {
  return {
    ensureSession: jest.fn(),
    getSession: jest.fn(),
    updateSession: jest.fn(),
    createSession: jest.fn(),
    completeStep: jest.fn(),
  };
}

/**
 * Apply the mock to jest.mock() for tool-session-helpers
 * Usage: jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());
 */
export function mockToolSessionHelpers() {
  jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());
}
