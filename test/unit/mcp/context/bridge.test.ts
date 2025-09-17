/**
 * Tests for ToolContext bridge implementation
 */

import { describe, test, expect, jest, beforeEach } from '@jest/globals';
import type { Logger } from 'pino';
import { createToolContext, createMCPToolContext } from '@/mcp/context';
import { extractProgressToken, createProgressReporter } from '@/mcp/context';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';

// Mock server and logger
const createMockServer = (): Server =>
  ({
    createMessage: jest.fn(),
  }) as any;

const createMockLogger = (): Logger =>
  ({
    debug: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    child: jest.fn(() => createMockLogger()),
  }) as any;

describe('ToolContext Bridge', () => {
  let mockServer: Server;
  let mockLogger: Logger;

  beforeEach(() => {
    mockServer = createMockServer();
    mockLogger = createMockLogger();
    jest.clearAllMocks();
  });

  describe('createToolContext', () => {
    test('creates valid ToolContext with sampling capability', async () => {
      const mockResponse = {
        content: { type: 'text', text: 'AI generated response' },
      };
      (mockServer.createMessage as jest.Mock).mockResolvedValue(mockResponse);

      const context = createToolContext(mockServer, mockLogger);

      expect(context).toHaveProperty('sampling');
      expect(context).toHaveProperty('getPrompt');
      expect(context).toHaveProperty('signal');
      expect(context).toHaveProperty('progress');
    });

    test('sampling.createMessage works correctly', async () => {
      const mockResponse = {
        content: { type: 'text', text: 'AI generated response' },
      };
      (mockServer.createMessage as jest.Mock).mockResolvedValue(mockResponse);

      const context = createToolContext(mockServer, mockLogger);
      const result = await context.sampling.createMessage({
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'Test prompt' }],
          },
        ],
      });

      expect(result).toEqual({
        role: 'assistant',
        content: [{ type: 'text', text: 'AI generated response' }],
        metadata: {
          finishReason: 'stop',
        },
      });

      // Verify the server was called with correct format
      expect(mockServer.createMessage).toHaveBeenCalledWith(
        expect.objectContaining({
          messages: [
            {
              role: 'user',
              content: {
                type: 'text',
                text: 'Test prompt',
              },
            },
          ],
          maxTokens: 2048,
          stopSequences: ['```', '\n\n```', '\n\n# ', '\n\n---'],
          includeContext: 'thisServer',
        }),
      );
    });

    test('handles empty AI response', async () => {
      const mockResponse = {
        content: { type: 'text', text: '' },
      };
      (mockServer.createMessage as jest.Mock).mockResolvedValue(mockResponse);

      const context = createToolContext(mockServer, mockLogger);

      await expect(
        context.sampling.createMessage({
          messages: [
            {
              role: 'user',
              content: [{ type: 'text', text: 'Test prompt' }],
            },
          ],
        }),
      ).rejects.toThrow('Empty response from sampling after processing');
    });

    test('handles invalid response format', async () => {
      const mockResponse = {
        content: { type: 'image', data: 'base64...' },
      };
      (mockServer.createMessage as jest.Mock).mockResolvedValue(mockResponse as any);

      const context = createToolContext(mockServer, mockLogger);

      await expect(
        context.sampling.createMessage({
          messages: [
            {
              role: 'user',
              content: [{ type: 'text', text: 'Test prompt' }],
            },
          ],
        }),
      ).rejects.toThrow('Empty or invalid response from sampling - no text content found');
    });

    test('getPrompt returns error response when no prompt registry available', async () => {
      const context = createToolContext(mockServer, mockLogger);

      const result = await context.getPrompt('test-prompt');

      expect(result).toEqual({
        description: 'Prompt not available - no registry',
        messages: [
          {
            role: 'user',
            content: [
              {
                type: 'text',
                text: "Error: No prompt registry available for prompt 'test-prompt'",
              },
            ],
          },
        ],
      });
    });

    test('getPrompt works with prompt registry provided', async () => {
      const mockPromptRegistry = {
        getPromptWithMessages: jest.fn().mockResolvedValue({
          description: 'Test prompt',
          messages: [
            {
              role: 'user' as const,
              content: [{ type: 'text' as const, text: 'Test prompt content' }],
            },
          ],
        }),
      };

      const context = createToolContext(mockServer, mockLogger, {
        promptRegistry: mockPromptRegistry as any,
      });

      const result = await context.getPrompt('test-prompt', { arg1: 'value1' });

      expect(mockPromptRegistry.getPromptWithMessages).toHaveBeenCalledWith('test-prompt', {
        arg1: 'value1',
      });
      expect(result).toEqual({
        description: 'Test prompt',
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'Test prompt content' }],
          },
        ],
      });
    });

    test('forwards abort signal', () => {
      const abortController = new AbortController();
      const context = createToolContext(mockServer, mockLogger, { signal: abortController.signal });

      expect(context.signal).toBe(abortController.signal);
    });

    test('includes progress reporter if provided', () => {
      const mockProgressReporter = jest.fn();
      const context = createToolContext(mockServer, mockLogger, { progress: mockProgressReporter });

      expect(context.progress).toBe(mockProgressReporter);
    });
  });

  describe('extractProgressToken', () => {
    test('extracts progress token from request metadata', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 'test-token-123',
          },
        },
      };

      const token = extractProgressToken(request);
      expect(token).toBe('test-token-123');
    });

    test('returns undefined for missing metadata', () => {
      expect(extractProgressToken({})).toBeUndefined();
      expect(extractProgressToken({ params: {} })).toBeUndefined();
      expect(extractProgressToken({ params: { _meta: {} } })).toBeUndefined();
    });

    test('handles non-string progress tokens', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 12345, // Not a string
          },
        },
      };

      const token = extractProgressToken(request);
      expect(token).toBeUndefined();
    });

    test('handles null/undefined request safely', () => {
      expect(extractProgressToken(null)).toBeUndefined();
      expect(extractProgressToken(undefined)).toBeUndefined();
    });
  });

  describe('createProgressReporter', () => {
    test('returns undefined when no progress token provided', () => {
      const reporter = createProgressReporter(mockServer, undefined, mockLogger);
      expect(reporter).toBeUndefined();
    });

    test('creates progress reporter when token provided', () => {
      const reporter = createProgressReporter(mockServer, 'test-token', mockLogger);
      expect(reporter).toBeInstanceOf(Function);
    });

    test('progress reporter logs progress (placeholder implementation)', () => {
      const reporter = createProgressReporter(mockServer, 'test-token', mockLogger);

      if (reporter) {
        reporter('Processing...', 50, 100);
        expect(mockLogger.debug).toHaveBeenCalledWith(
          expect.objectContaining({
            progressToken: 'test-token',
            message: 'Processing...',
            progress: 50,
            total: 100,
            type: 'progress_notification',
          }),
          'Progress notification logged - MCP transport implementation pending',
        );
      }
    });
  });

  describe('createToolContextWithProgress', () => {
    test('creates context with progress token extraction', () => {
      const request = {
        params: {
          _meta: {
            progressToken: 'test-token-123',
          },
        },
      };

      const context = createMCPToolContext(mockServer, request, mockLogger, {});

      expect(context).toHaveProperty('progress');
      expect(context.progress).toBeInstanceOf(Function);
    });

    test('creates context without progress when no token', () => {
      const request = { params: {} };

      const context = createMCPToolContext(mockServer, request, mockLogger, {});

      expect(context).toHaveProperty('progress');
      expect(context.progress).toBeUndefined();
    });
  });

  describe('error handling and logging', () => {
    test('logs sampling request start and completion', async () => {
      const mockResponse = {
        content: { type: 'text', text: 'Response' },
      };
      (mockServer.createMessage as jest.Mock).mockResolvedValue(mockResponse);

      const context = createToolContext(mockServer, mockLogger);
      await context.sampling.createMessage({
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'Test' }],
          },
        ],
      });

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({
          messageCount: 1,
          maxTokens: 2048,
          includeContext: 'thisServer',
        }),
        'Making sampling request',
      );

      expect(mockLogger.debug).toHaveBeenCalledWith(
        expect.objectContaining({
          duration: expect.any(Number),
          responseLength: 8, // 'Response' length
        }),
        'Sampling request completed',
      );
    });

    test('logs sampling errors', async () => {
      const mockError = new Error('Sampling failed');
      (mockServer.createMessage as jest.Mock).mockRejectedValue(mockError);

      const context = createToolContext(mockServer, mockLogger);

      await expect(
        context.sampling.createMessage({
          messages: [
            {
              role: 'user',
              content: [{ type: 'text', text: 'Test' }],
            },
          ],
        }),
      ).rejects.toThrow('Sampling failed: Sampling failed');

      expect(mockLogger.error).toHaveBeenCalledWith(
        expect.objectContaining({
          duration: expect.any(Number),
          error: 'Sampling failed',
          messageCount: 1,
        }),
        'Sampling request failed',
      );
    });

    test('handles progress reporting errors gracefully', () => {
      const reporter = createProgressReporter(mockServer, 'test-token', mockLogger);

      // Mock logger methods to throw
      (mockLogger.debug as jest.Mock).mockImplementation(() => {
        throw new Error('Logger error');
      });

      if (reporter) {
        // Should not throw despite logger error
        expect(() => reporter('test', 50, 100)).not.toThrow();
      }
    });
  });
});
