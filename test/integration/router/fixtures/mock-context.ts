/**
 * Mock MCP context for router integration testing
 */

import type { ToolContext } from '@mcp/context';

export const createMockContext = (): ToolContext => {
  return {
    meta: {
      request_id: 'test-request-123',
      timestamp: new Date().toISOString(),
    },
    session: {
      id: 'test-session',
      type: 'test',
    },
    client: {
      name: 'test-client',
      version: '1.0.0',
    },
    // Add a mock sampling method to satisfy the interface
    _internal: {
      sampling: {
        requestSample: () => Promise.resolve({
          prompt: 'test prompt',
          content: [],
        }),
      },
    },
  } as unknown as ToolContext;
};