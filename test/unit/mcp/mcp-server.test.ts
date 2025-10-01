import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { z } from 'zod';
import type { Tool } from '@/types/tool';
import { registerToolsWithServer } from '@/mcp/mcp-server';
import { Success, Failure } from '@/types';
import type { Logger } from 'pino';
import { McpError } from '@modelcontextprotocol/sdk/types.js';

function createTool(name: string): Tool<ReturnType<typeof z.object>, unknown> {
  return {
    name,
    description: `${name} tool`,
    schema: z.object({ foo: z.string() }),
    run: jest.fn(),
  };
}

function createLoggerStub(): Logger {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as unknown as Logger;
}

let executeMock: jest.Mock;
let serverToolMock: jest.Mock;
let logger: Logger;

beforeEach(() => {
  executeMock = jest.fn();
  serverToolMock = jest.fn();
  logger = createLoggerStub();
});

describe('registerToolsWithServer', () => {
  it('sanitizes params and forwards execution to orchestrator', async () => {
    const tool = createTool('exec-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    expect(serverToolMock).toHaveBeenCalledTimes(1);
    const handler = serverToolMock.mock.calls[0][3];

    const params = {
      foo: 'value',
      _meta: { sessionId: 'abc', progressToken: 'tok' },
    } as Record<string, unknown>;

    const extra = {
      sendNotification: jest.fn(),
      _meta: { progressToken: 'tok' },
      signal: new AbortController().signal,
      requestId: '123',
    };

    await handler(params, extra);

    expect(executeMock).toHaveBeenCalledWith({
      toolName: tool.name,
      params: { foo: 'value' },
      sessionId: 'abc',
      metadata: expect.objectContaining({
        progress: expect.objectContaining({ _meta: expect.objectContaining({ progressToken: 'tok' }) }),
        loggerContext: expect.objectContaining({ transport: 'stdio' }),
      }),
    });
  });

  it('wraps orchestrator failures in McpError', async () => {
    const tool = createTool('error-demo');
    executeMock.mockResolvedValue(Failure('orchestrator boom'));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '456',
    };

    await expect(handler({ foo: 'value' }, extra)).rejects.toBeInstanceOf(McpError);
    expect(executeMock).toHaveBeenCalled();
  });

  it('extracts signal from RequestHandlerExtra', async () => {
    const tool = createTool('signal-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const abortController = new AbortController();

    const extra = {
      sendNotification: jest.fn(),
      signal: abortController.signal,
      requestId: '789',
    };

    await handler({ foo: 'value' }, extra);

    expect(executeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          signal: abortController.signal,
        }),
      }),
    );
  });

  it('extracts sessionId from extra.sessionId (transport-level)', async () => {
    const tool = createTool('session-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '101',
      sessionId: 'transport-session-123',
    };

    await handler({ foo: 'value' }, extra);

    expect(executeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        sessionId: 'transport-session-123',
      }),
    );
  });

  it('prefers extra.sessionId over _meta.sessionId', async () => {
    const tool = createTool('session-priority-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];

    const params = {
      foo: 'value',
      _meta: { sessionId: 'params-session' },
    };

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '102',
      sessionId: 'transport-session',
    };

    await handler(params, extra);

    expect(executeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        sessionId: 'transport-session', // Transport wins
      }),
    );
  });

  it('extracts maxTokens from _meta', async () => {
    const tool = createTool('maxTokens-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];

    const params = {
      foo: 'value',
      _meta: { maxTokens: 4096 },
    };

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '103',
    };

    await handler(params, extra);

    expect(executeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          maxTokens: 4096,
        }),
      }),
    );
  });

  it('extracts stopSequences from _meta', async () => {
    const tool = createTool('stopSequences-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];

    const params = {
      foo: 'value',
      _meta: { stopSequences: ['STOP', 'END'] },
    };

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '104',
    };

    await handler(params, extra);

    expect(executeMock).toHaveBeenCalledWith(
      expect.objectContaining({
        metadata: expect.objectContaining({
          stopSequences: ['STOP', 'END'],
        }),
      }),
    );
  });

  it('extracts all metadata fields together', async () => {
    const tool = createTool('all-metadata-demo');
    executeMock.mockResolvedValue(Success({ ok: true }));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const abortController = new AbortController();

    const params = {
      foo: 'value',
      _meta: {
        sessionId: 'params-session',
        maxTokens: 2048,
        stopSequences: ['###'],
        progressToken: 'progress-123',
      },
    };

    const extra = {
      sendNotification: jest.fn(),
      signal: abortController.signal,
      requestId: '105',
      sessionId: 'transport-session',
    };

    await handler(params, extra);

    expect(executeMock).toHaveBeenCalledWith({
      toolName: 'all-metadata-demo',
      params: { foo: 'value' }, // _meta stripped
      sessionId: 'transport-session', // From extra
      metadata: expect.objectContaining({
        signal: abortController.signal,
        maxTokens: 2048,
        stopSequences: ['###'],
        loggerContext: expect.objectContaining({
          transport: 'stdio',
          requestId: '105',
        }),
      }),
    });
  });
});
