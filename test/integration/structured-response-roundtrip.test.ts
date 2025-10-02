/**
 * Integration test for structured response round-trip through MCP server
 */

import { describe, it, expect, jest } from '@jest/globals';
import { registerToolsWithServer } from '@/mcp/mcp-server';
import structuredResponseTool from '../fixtures/structured-response-tool';
import type { Logger } from 'pino';

function createLoggerStub(): Logger {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as unknown as Logger;
}

describe('Structured Response Round-Trip', () => {
  it('preserves primitive string result through MCP formatting', async () => {
    const executeMock = jest.fn();
    const serverToolMock = jest.fn();
    const logger = createLoggerStub();

    // Execute the tool directly
    const toolResult = await structuredResponseTool.run({ responseType: 'primitive' });
    expect(toolResult.ok).toBe(true);
    if (!toolResult.ok) return;

    // Simulate MCP server execution
    executeMock.mockResolvedValue(toolResult);

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [structuredResponseTool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const result = await handler(
      { responseType: 'primitive' },
      { sendNotification: jest.fn(), signal: new AbortController().signal, requestId: '1' },
    );

    // Verify single text block
    expect(result.content).toHaveLength(1);
    expect(result.content[0].type).toBe('text');
    expect(result.content[0].text).toBe('Simple string result');
  });

  it('preserves object data without summary through MCP formatting', async () => {
    const executeMock = jest.fn();
    const serverToolMock = jest.fn();
    const logger = createLoggerStub();

    const toolResult = await structuredResponseTool.run({ responseType: 'data-only' });
    expect(toolResult.ok).toBe(true);
    if (!toolResult.ok) return;

    executeMock.mockResolvedValue(toolResult);

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [structuredResponseTool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const result = await handler(
      { responseType: 'data-only' },
      { sendNotification: jest.fn(), signal: new AbortController().signal, requestId: '2' },
    );

    // Verify single JSON text block
    expect(result.content).toHaveLength(1);
    expect(result.content[0].type).toBe('text');
    const parsed = JSON.parse(result.content[0].text);
    expect(parsed).toEqual({
      items: ['item1', 'item2', 'item3'],
      count: 3,
    });
  });

  it('formats summary + data as two text blocks', async () => {
    const executeMock = jest.fn();
    const serverToolMock = jest.fn();
    const logger = createLoggerStub();

    const toolResult = await structuredResponseTool.run({ responseType: 'summary-with-data' });
    expect(toolResult.ok).toBe(true);
    if (!toolResult.ok) return;

    executeMock.mockResolvedValue(toolResult);

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [structuredResponseTool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const result = await handler(
      { responseType: 'summary-with-data' },
      { sendNotification: jest.fn(), signal: new AbortController().signal, requestId: '3' },
    );

    // Verify two text blocks: summary + data
    expect(result.content).toHaveLength(2);
    expect(result.content[0].type).toBe('text');
    expect(result.content[0].text).toBe('Processed 3 items successfully');

    expect(result.content[1].type).toBe('text');
    expect(result.content[1].text).toContain('📊 Data:');

    const dataSection = result.content[1].text.split('📊 Data:\n')[1];
    const parsedData = JSON.parse(dataSection);
    expect(parsedData).toHaveProperty('data');
    const data = parsedData.data as Record<string, unknown>;
    expect(data).toHaveProperty('items');
    expect(data).toHaveProperty('metrics');
    expect(data.items).toEqual(['item1', 'item2', 'item3']);
  });

  it('formats summary-only result as single text block', async () => {
    const executeMock = jest.fn();
    const serverToolMock = jest.fn();
    const logger = createLoggerStub();

    const toolResult = await structuredResponseTool.run({ responseType: 'summary-only' });
    expect(toolResult.ok).toBe(true);
    if (!toolResult.ok) return;

    executeMock.mockResolvedValue(toolResult);

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [structuredResponseTool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const result = await handler(
      { responseType: 'summary-only' },
      { sendNotification: jest.fn(), signal: new AbortController().signal, requestId: '4' },
    );

    // Verify single text block with just summary
    expect(result.content).toHaveLength(1);
    expect(result.content[0].type).toBe('text');
    expect(result.content[0].text).toBe('Operation completed successfully');
  });

  it('ensures JSON data survives serialization round-trip', async () => {
    const executeMock = jest.fn();
    const serverToolMock = jest.fn();
    const logger = createLoggerStub();

    const toolResult = await structuredResponseTool.run({ responseType: 'summary-with-data' });
    expect(toolResult.ok).toBe(true);
    if (!toolResult.ok) return;

    executeMock.mockResolvedValue(toolResult);

    const fakeServer = {
      tool: serverToolMock,
    } as unknown as Parameters<typeof registerToolsWithServer>[0]['server'];

    registerToolsWithServer({
      server: fakeServer,
      tools: [structuredResponseTool],
      logger,
      transport: 'stdio',
      execute: executeMock,
    });

    const handler = serverToolMock.mock.calls[0][3];
    const result = await handler(
      { responseType: 'summary-with-data' },
      { sendNotification: jest.fn(), signal: new AbortController().signal, requestId: '5' },
    );

    // Extract and verify data from second block
    const dataSection = result.content[1].text.split('📊 Data:\n')[1];
    const parsedData = JSON.parse(dataSection);

    // Re-serialize and parse to ensure no corruption
    const reserialized = JSON.stringify(parsedData);
    const reparsed = JSON.parse(reserialized);

    expect(reparsed).toEqual(parsedData);
    const data = reparsed.data as Record<string, unknown>;
    const metrics = data.metrics as Record<string, unknown>;
    expect(metrics).toEqual({ processed: 3, failed: 0, duration: 125 });
  });
});
