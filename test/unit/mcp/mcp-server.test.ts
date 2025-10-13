import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { z } from 'zod';
import type { Tool } from '@/types/tool';
import { registerToolsWithServer, objectToMarkdownRecursive, formatOutput, OUTPUTFORMAT } from '@/mcp/mcp-server';
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
});

describe('objectToMarkdownRecursive', () => {
  it('converts simple key-value pairs to markdown headings', () => {
    const input = {
      name: 'test',
      version: '1.0.0',
      enabled: true,
    };

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## Name

test

## Version

1.0.0

## Enabled

true

`);
  });

  it('handles null and undefined values', () => {
    const input = {
      nullValue: null,
      undefinedValue: undefined,
    };

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## NullValue

null

## UndefinedValue

undefined

`);
  });

  it('converts arrays to markdown headings with numbered lists', () => {
    const input = {
      items: ['apple', 'banana', 'cherry'],
      numbers: [1, 2, 3],
    };

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## Items

1. apple

2. banana

3. cherry

## Numbers

1. 1

2. 2

3. 3

`);
  });

  it('handles nested objects with increasing heading levels', () => {
    const input = {
      config: {
        database: {
          host: 'localhost',
          port: 5432,
        },
        cache: {
          enabled: true,
        },
      },
    };

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## Config

### Database

#### Host

localhost

#### Port

5432

### Cache

#### Enabled

true

`);
  });

  it('handles arrays with object elements', () => {
    const input = {
      users: [
        { name: 'Alice', age: 30 },
        { name: 'Bob', age: 25 },
      ],
    };

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## Users

### 1

#### Name

Alice

#### Age

30

### 2

#### Name

Bob

#### Age

25

`);
  });

  it('uses custom heading level', () => {
    const input = {
      title: 'Custom Level',
    };

    const result = objectToMarkdownRecursive(input, 4);

    expect(result).toBe(`#### Title

Custom Level

`);
  });

  it('handles empty objects', () => {
    const input = {};

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe('');
  });

  it('handles complex mixed data structures', () => {
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

    const result = objectToMarkdownRecursive(input);

    expect(result).toBe(`## Metadata

### Version

2.0

### Tags

1. prod

2. api

### Config

#### Timeout

30

#### Retries

null

## Enabled

false

`);
  });
});

describe('formatOutput', () => {
  it('formats as JSON when format is JSON', () => {
    const input = { name: 'test', version: 1 };

    const result = formatOutput(input, OUTPUTFORMAT.JSON);

    expect(result).toBe(JSON.stringify(input, null, 2));
  });

  it('formats objects as markdown when format is TEXT', () => {
    const input = { name: 'test', enabled: true };

    const result = formatOutput(input, OUTPUTFORMAT.TEXT);

    expect(result).toBe(`## Name

test

## Enabled

true

`);
  });

  it('formats primitive values as string when format is TEXT', () => {
    expect(formatOutput('hello', OUTPUTFORMAT.TEXT)).toBe('hello');
    expect(formatOutput(42, OUTPUTFORMAT.TEXT)).toBe('42');
    expect(formatOutput(true, OUTPUTFORMAT.TEXT)).toBe('true');
    expect(formatOutput(null, OUTPUTFORMAT.TEXT)).toBe('null');
  });

  it('handles invalid format by defaulting to string', () => {
    const input = { test: 'value' };

    const result = formatOutput(input, 'invalid' as any);

    expect(result).toBe('[object Object]');
  });
});
