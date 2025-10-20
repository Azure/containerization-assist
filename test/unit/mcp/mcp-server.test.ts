import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { z } from 'zod';
import type { Tool } from '@/types/tool';
import { registerToolsWithServer, formatOutput, OUTPUTFORMAT } from '@/mcp/mcp-server';
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
      outputFormat: OUTPUTFORMAT.MARKDOWN,
    });

    expect(serverToolMock).toHaveBeenCalledTimes(1);
    const handler = serverToolMock.mock.calls[0][3] as any;

    const params = {
      foo: 'value',
      _meta: { progressToken: 'tok' },
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
      metadata: expect.objectContaining({
        progress: expect.objectContaining({ _meta: expect.objectContaining({ progressToken: 'tok' }) }),
        loggerContext: expect.objectContaining({ transport: 'stdio' }),
      }),
    });
  });

  it('wraps orchestrator failures in McpError', async () => {
    const tool = createTool('error-demo');
    (executeMock as any).mockResolvedValue(Failure('orchestrator boom'));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
      outputFormat: OUTPUTFORMAT.MARKDOWN,
    });

    const handler = serverToolMock.mock.calls[0][3] as any;

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '456',
    };

    await expect(handler({ foo: 'value' }, extra)).rejects.toBeInstanceOf(McpError);
    expect(executeMock).toHaveBeenCalled();
  });

  it('formats output according to specified outputFormat', async () => {
    const tool = createTool('format-demo');
    const mockResult = { name: 'test', version: '1.0' };
    (executeMock as any).mockResolvedValue(Success(mockResult));

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [tool],
      logger,
      transport: 'stdio',
      execute: executeMock,
      outputFormat: OUTPUTFORMAT.JSON,
    });

    const handler = serverToolMock.mock.calls[0][3] as any;

    const extra = {
      sendNotification: jest.fn(),
      signal: new AbortController().signal,
      requestId: '789',
    };

    const result = await handler({ foo: 'value' }, extra);

    expect(result.content[0].text).toBe('{\n  "name": "test",\n  "version": "1.0"\n}');
  });
});


describe('formatOutput', () => {
  it('formats as JSON when format is JSON', () => {
    const input = { name: 'test', version: 1 };

    const result = formatOutput(input, OUTPUTFORMAT.JSON);

    expect(result).toBe(JSON.stringify(input, null, 2));
  });

  it('formats objects as JSON code block when format is MARKDOWN', () => {
    const input = { name: 'test', enabled: true };

    const result = formatOutput(input, OUTPUTFORMAT.MARKDOWN);

    const expected = '```json\n' + JSON.stringify(input, null, 2) + '\n```';
    expect(result).toBe(expected);
  });

  it('formats complex nested objects as JSON code block when format is MARKDOWN', () => {
    const input = {
      metadata: {
        version: '2.0',
        tags: ['prod', 'api'],
        config: {
          timeout: 30,
          retries: null,
        },
      },
      enabled: false,
    };

    const result = formatOutput(input, OUTPUTFORMAT.MARKDOWN);

    const expected = '```json\n' + JSON.stringify(input, null, 2) + '\n```';
    expect(result).toBe(expected);
  });

  it('formats primitive values as string when format is MARKDOWN', () => {
    expect(formatOutput('hello', OUTPUTFORMAT.MARKDOWN)).toBe('hello');
    expect(formatOutput(42, OUTPUTFORMAT.MARKDOWN)).toBe('42');
    expect(formatOutput(true, OUTPUTFORMAT.MARKDOWN)).toBe('true');
    expect(formatOutput(null, OUTPUTFORMAT.MARKDOWN)).toBe('null');
  });

  it('formats objects as JSON when format is TEXT', () => {
    const input = { name: 'test', value: 123 };

    const result = formatOutput(input, OUTPUTFORMAT.TEXT);

    expect(result).toBe(JSON.stringify(input, null, 2));
  });

  it('formats primitives as string when format is TEXT', () => {
    expect(formatOutput('hello', OUTPUTFORMAT.TEXT)).toBe('hello');
    expect(formatOutput(42, OUTPUTFORMAT.TEXT)).toBe('42');
    expect(formatOutput(true, OUTPUTFORMAT.TEXT)).toBe('true');
  });

  it('handles invalid format by defaulting to TEXT behavior', () => {
    const input = { test: 'value' };

    const result = formatOutput(input, 'invalid' as any);

    expect(result).toBe(JSON.stringify(input, null, 2));
  });
});
