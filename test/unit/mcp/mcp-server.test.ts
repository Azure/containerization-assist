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

  it('formats primitive values as JSON code block when format is MARKDOWN', () => {
    expect(formatOutput('hello', OUTPUTFORMAT.MARKDOWN)).toBe('```json\n"hello"\n```');
    expect(formatOutput(42, OUTPUTFORMAT.MARKDOWN)).toBe('```json\n42\n```');
    expect(formatOutput(true, OUTPUTFORMAT.MARKDOWN)).toBe('```json\ntrue\n```');
    expect(formatOutput(null, OUTPUTFORMAT.MARKDOWN)).toBe('```json\nnull\n```');
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

  describe('with summary field', () => {
    it('shows only summary when format is TEXT', () => {
      const input = {
        summary: '✅ Operation completed successfully',
        details: { foo: 'bar', count: 42 },
      };

      const result = formatOutput(input, OUTPUTFORMAT.TEXT);

      expect(result).toBe('✅ Operation completed successfully');
      expect(result).not.toContain('details');
      expect(result).not.toContain('foo');
    });

    it('shows summary with collapsible details when format is MARKDOWN', () => {
      const input = {
        summary: '✅ Build completed in 45s',
        imageId: 'sha256:abc123',
        size: 245000000,
      };

      const result = formatOutput(input, OUTPUTFORMAT.MARKDOWN);

      expect(result).toContain('✅ Build completed in 45s');
      expect(result).toContain('<details>');
      expect(result).toContain('<summary>View detailed output</summary>');
      expect(result).toContain('```json');
      expect(result).toContain('imageId');
      expect(result).toContain('size');
      expect(result).not.toContain('"summary"');
    });

    it('falls back to full JSON in MARKDOWN if no summary', () => {
      const input = { foo: 'bar', baz: 123 };

      const result = formatOutput(input, OUTPUTFORMAT.MARKDOWN);

      expect(result).toBe('```json\n' + JSON.stringify(input, null, 2) + '\n```');
      expect(result).not.toContain('<details>');
    });

    it('uses summary for NATURAL_LANGUAGE with fallback', () => {
      const input = {
        summary: '✅ Deployment successful',
        namespace: 'production',
        replicas: 3,
      };

      const result = formatOutput(input, OUTPUTFORMAT.NATURAL_LANGUAGE);

      // Since we don't have a type guard match, it falls back to summary
      expect(result).toBe('✅ Deployment successful');
    });
  });

  describe('NATURAL_LANGUAGE format with type detection', () => {
    it('detects and formats scan-image results', () => {
      const scanResult = {
        summary: '✅ Scan passed',
        vulnerabilities: {
          critical: 0,
          high: 0,
          medium: 2,
          low: 5,
          negligible: 10,
          unknown: 0,
          total: 17,
        },
        scanTime: '2025-01-22T10:00:00Z',
        passed: true,
        success: true,
        remediationGuidance: [],
      };

      const result = formatOutput(scanResult, OUTPUTFORMAT.NATURAL_LANGUAGE);

      expect(result).toContain('Security Scan');
      expect(result).toContain('PASSED');
      expect(result).toContain('Vulnerabilities:');
      expect(result).toContain('Next Steps:');
    });

    it('detects and formats build-image results', () => {
      const buildResult = {
        summary: '✅ Built image',
        success: true,
        imageId: 'sha256:abc123',
        requestedTags: ['myapp:latest', 'myapp:1.0.0'],
        createdTags: ['myapp:latest', 'myapp:1.0.0'],
        size: 245000000,
        buildTime: 45000,
        logs: [],
      };

      const result = formatOutput(buildResult, OUTPUTFORMAT.NATURAL_LANGUAGE);

      expect(result).toContain('Image Built Successfully');
      expect(result).toContain('**Image:**');
      expect(result).toContain('**Tags Created:**');
      expect(result).toContain('**Size:**');
      expect(result).toContain('**Build Time:**');
      expect(result).toContain('Next Steps:');
    });

    it('detects and formats analyze-repo results', () => {
      const analyzeResult = {
        summary: '✅ Analyzed repository',
        modules: [
          {
            name: 'api-service',
            modulePath: '/app/api',
            language: 'javascript' as const,
            languageVersion: '18.0.0',
            frameworks: [{ name: 'Express', version: '4.18.0' }],
            ports: [3000],
          },
        ],
        isMonorepo: false,
        analyzedPath: '/app',
      };

      const result = formatOutput(analyzeResult, OUTPUTFORMAT.NATURAL_LANGUAGE);

      expect(result).toContain('Repository Analysis Complete');
      expect(result).toContain('**Path:**');
      expect(result).toContain('**Type:**');
      expect(result).toContain('**Modules Found:**');
      expect(result).toContain('Next Steps:');
    });

    it('falls back to summary when type is not recognized', () => {
      const unknownResult = {
        summary: '✅ Custom operation completed',
        customField: 'value',
        someData: 123,
      };

      const result = formatOutput(unknownResult, OUTPUTFORMAT.NATURAL_LANGUAGE);

      expect(result).toBe('✅ Custom operation completed');
    });

    it('falls back to JSON when no summary and type not recognized', () => {
      const unknownResult = {
        customField: 'value',
        someData: 123,
      };

      const result = formatOutput(unknownResult, OUTPUTFORMAT.NATURAL_LANGUAGE);

      expect(result).toBe(JSON.stringify(unknownResult, null, 2));
    });
  });
});
