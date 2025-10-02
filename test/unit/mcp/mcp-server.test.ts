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

  describe('structured response formatting', () => {
    it('returns single text block for primitive string result', async () => {
      const tool = createTool('primitive-demo');
      executeMock.mockResolvedValue(Success('Simple string result'));

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
        requestId: '200',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toEqual([{ type: 'text', text: 'Simple string result' }]);
    });

    it('returns single text block for number result', async () => {
      const tool = createTool('number-demo');
      executeMock.mockResolvedValue(Success(42));

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
        requestId: '201',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toEqual([{ type: 'text', text: '42' }]);
    });

    it('returns single JSON text block for object without summary', async () => {
      const tool = createTool('object-demo');
      const resultValue = { items: ['a', 'b', 'c'], count: 3 };
      executeMock.mockResolvedValue(Success(resultValue));

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
        requestId: '202',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toHaveLength(1);
      expect(result.content[0].type).toBe('text');
      expect(JSON.parse(result.content[0].text)).toEqual(resultValue);
    });

    it('returns two text blocks for object with summary and data', async () => {
      const tool = createTool('summary-data-demo');
      const resultValue = {
        summary: 'Processed 3 items successfully',
        items: ['item1', 'item2', 'item3'],
        metrics: { processed: 3, failed: 0 },
      };
      executeMock.mockResolvedValue(Success(resultValue));

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
        requestId: '203',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toHaveLength(2);
      expect(result.content[0]).toEqual({
        type: 'text',
        text: 'Processed 3 items successfully',
      });
      expect(result.content[1].type).toBe('text');
      expect(result.content[1].text).toContain('📊 Data:');
      const parsedData = JSON.parse(result.content[1].text.split('📊 Data:\n')[1]);
      expect(parsedData).toEqual({
        items: ['item1', 'item2', 'item3'],
        metrics: { processed: 3, failed: 0 },
      });
    });

    it('returns single text block for object with only summary field', async () => {
      const tool = createTool('summary-only-demo');
      executeMock.mockResolvedValue(Success({ summary: 'Operation completed' }));

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
        requestId: '204',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toEqual([{ type: 'text', text: 'Operation completed' }]);
    });

    it('handles null result', async () => {
      const tool = createTool('null-demo');
      executeMock.mockResolvedValue(Success(null));

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
        requestId: '205',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toEqual([{ type: 'text', text: 'null' }]);
    });

    it('handles array results with JSON formatting', async () => {
      const tool = createTool('array-demo');
      const resultValue = [{ id: 1 }, { id: 2 }, { id: 3 }];
      executeMock.mockResolvedValue(Success(resultValue));

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
        requestId: '206',
      };

      const result = await handler({ foo: 'value' }, extra);

      expect(result.content).toHaveLength(1);
      expect(result.content[0].type).toBe('text');
      expect(JSON.parse(result.content[0].text)).toEqual(resultValue);
    });
  });
});
