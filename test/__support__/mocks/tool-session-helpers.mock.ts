import type { SessionManager } from '@lib/session/session-manager';

/**
 * Creates a mock implementation for tool-session-helpers module
 */
export function createToolSessionHelpersMock() {
  return {
    ensureSession: jest.fn(),
    useSessionSlice: jest.fn((toolName: string, io: any, context: any) => {
      // Simulate the actual behavior where patch calls sessionManager.update
      return {
        get: jest.fn(),
        set: jest.fn(),
        patch: jest.fn(async (sessionId: string, data: any) => {
          if (context?.sessionManager?.update) {
            await context.sessionManager.update(sessionId, {
              metadata: {
                toolSlices: {
                  [toolName]: data,
                },
              },
            });
          }
        }),
        clear: jest.fn(),
      };
    }),
    defineToolIO: jest.fn((input: any, output: any) => ({ input, output })),
    getSessionSlice: jest.fn(),
    updateSessionSlice: jest.fn(),
  };
}

/**
 * Apply the mock to jest.mock() for tool-session-helpers
 * Usage: jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());
 */
export function mockToolSessionHelpers() {
  jest.mock('../../../src/mcp/tool-session-helpers', () => createToolSessionHelpersMock());
}