/**
 * SDK Executor Tests
 *
 * Tests for the SDK executor function that provides direct tool execution
 * without MCP orchestration.
 */

import { describe, test, expect, jest, beforeEach } from '@jest/globals';
import { executeTool, _testing } from '../../../src/sdk/executor';
import { Success, Failure } from '../../../src/types/core';
import { z } from 'zod';
import { createMockLogger } from '../../__support__/utilities/test-helpers';
import {
  createMockTool,
  createMockToolWithSchema,
  type MockSchemaInput,
} from '../../__support__/utilities/mock-tools';
import type { ToolContext } from '../../../src/core/context';
import type { Result } from '../../../src/types/core';

describe('SDK Executor', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    // Reset knowledge state for test isolation
    _testing.resetKnowledgeState();
  });

  describe('executeTool', () => {
    test('should execute tool handler with valid input', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(Success({ result: 'success' }));
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { input: 'test' });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ result: 'success' });
      }
      expect(mockHandler).toHaveBeenCalledTimes(1);
    });

    test('should return Failure for invalid input', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(Success({ result: 'success' }));
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { wrong: 'field' });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Validation failed');
      }
      expect(mockHandler).not.toHaveBeenCalled();
    });

    test('should return Failure for missing required field', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(Success({ result: 'success' }));
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, {});

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Validation failed');
        expect(result.guidance).toBeDefined();
        // ZodError provides detailed field-level validation issues
        expect(result.guidance?.hint).toContain('Validation issues');
        expect(result.guidance?.hint).toContain('input');
      }
    });

    test('should pass abort signal to context', async () => {
      let capturedCtx: ToolContext | undefined;
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          capturedCtx = ctx;
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);
      const controller = new AbortController();

      await executeTool(mockTool, { input: 'test' }, { signal: controller.signal });

      expect(mockHandler).toHaveBeenCalled();
      expect(capturedCtx?.signal).toBe(controller.signal);
    });

    test('should call onProgress callback', async () => {
      const onProgress = jest.fn();
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          if (ctx.progress) {
            await ctx.progress('Step 1', 1, 3);
            await ctx.progress('Step 2', 2, 3);
          }
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await executeTool(mockTool, { input: 'test' }, { onProgress });

      expect(onProgress).toHaveBeenCalledTimes(2);
      expect(onProgress).toHaveBeenCalledWith('Step 1', 1, 3);
      expect(onProgress).toHaveBeenCalledWith('Step 2', 2, 3);
    });

    test('should handle tool handler returning Failure', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(
          Failure('Something went wrong', {
            message: 'Error occurred',
            hint: 'Try again',
            resolution: 'Check input',
          }),
        );
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { input: 'test' });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBe('Something went wrong');
        expect(result.guidance?.hint).toBe('Try again');
      }
    });

    test('should use custom logger when provided', async () => {
      const customLogger = createMockLogger();
      let capturedCtx: ToolContext | undefined;

      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          capturedCtx = ctx;
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await executeTool(mockTool, { input: 'test' }, { logger: customLogger });

      expect(mockHandler).toHaveBeenCalled();
      expect(capturedCtx?.logger).toBe(customLogger);
    });

    test('should handle optional parameters in input', async () => {
      const mockHandler = jest
        .fn<
          (
            input: MockSchemaInput,
            ctx: ToolContext,
          ) => Promise<Result<{ input: string; optional: number | undefined }>>
        >()
        .mockImplementation(async (input) => {
          return Success({ input: input.input, optional: input.optional });
        });
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { input: 'test', optional: 42 });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ input: 'test', optional: 42 });
      }
    });

    test('should work without any options', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(Success({ result: 'success' }));
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { input: 'test' });

      expect(result.ok).toBe(true);
      expect(mockHandler).toHaveBeenCalledTimes(1);
    });
  });

  describe('Validation Error Messages', () => {
    test('should include field path in validation error for nested fields', async () => {
      const nestedSchema = z.object({
        config: z.object({
          name: z.string(),
          port: z.number(),
        }),
      });

      const mockHandler = jest
        .fn<
          (
            input: z.infer<typeof nestedSchema>,
            ctx: ToolContext,
          ) => Promise<Result<Record<string, unknown>>>
        >()
        .mockResolvedValue(Success({}));
      const mockTool = createMockToolWithSchema(nestedSchema, mockHandler);

      const result = await executeTool(mockTool, {
        config: { name: 'test', port: 'not-a-number' },
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        // The error should contain info about the invalid field
        expect(result.error).toContain('Validation failed');
      }
    });

    test('should provide actionable guidance for type mismatch', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockResolvedValue(Success({ result: 'success' }));
      const mockTool = createMockTool(mockHandler);

      const result = await executeTool(mockTool, { input: 123 }); // number instead of string

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.guidance).toBeDefined();
        // ZodError provides resolution pointing to tool schema
        expect(result.guidance?.resolution).toContain('tool schema');
      }
    });
  });

  describe('Error Handling', () => {
    test('should propagate unhandled exceptions from tool handler', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockRejectedValue(new Error('Unexpected error'));
      const mockTool = createMockTool(mockHandler);

      await expect(executeTool(mockTool, { input: 'test' })).rejects.toThrow('Unexpected error');
    });

    test('should handle non-Error thrown values', async () => {
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockRejectedValue('string error');
      const mockTool = createMockTool(mockHandler);

      await expect(executeTool(mockTool, { input: 'test' })).rejects.toBe('string error');
    });
  });

  describe('Cancellation', () => {
    test('should pass aborted signal to tool', async () => {
      const controller = new AbortController();
      controller.abort();

      let receivedSignal: AbortSignal | undefined;
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          receivedSignal = ctx.signal;
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await executeTool(mockTool, { input: 'test' }, { signal: controller.signal });

      expect(receivedSignal?.aborted).toBe(true);
    });

    test('should pass abort reason to context', async () => {
      const controller = new AbortController();
      const abortReason = new Error('User cancelled');
      controller.abort(abortReason);

      let receivedSignal: AbortSignal | undefined;
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          receivedSignal = ctx.signal;
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await executeTool(mockTool, { input: 'test' }, { signal: controller.signal });

      expect(receivedSignal?.aborted).toBe(true);
      expect(receivedSignal?.reason).toBe(abortReason);
    });
  });

  describe('Progress Callback', () => {
    test('should handle progress callback that throws', async () => {
      const onProgress = jest.fn().mockImplementation(() => {
        throw new Error('Progress callback error');
      });

      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          // This should throw when progress callback fails
          await ctx.progress?.('Step 1', 1, 3);
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await expect(executeTool(mockTool, { input: 'test' }, { onProgress })).rejects.toThrow(
        'Progress callback error',
      );
    });

    test('should not provide progress if onProgress not specified', async () => {
      let progressWasUndefined = false;
      const mockHandler = jest
        .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
        .mockImplementation(async (_input, ctx) => {
          progressWasUndefined = ctx.progress === undefined;
          return Success({ result: 'success' });
        });
      const mockTool = createMockTool(mockHandler);

      await executeTool(mockTool, { input: 'test' });

      expect(progressWasUndefined).toBe(true);
    });
  });

  describe('Knowledge State', () => {
    test('should reset knowledge state between tests', () => {
      // Verify the reset function works
      _testing.resetKnowledgeState();
      expect(_testing.isKnowledgeLoaded()).toBe(false);
    });
  });
});

describe('SDK Executor Context', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    _testing.resetKnowledgeState();
  });

  test('should provide logger in context', async () => {
    let hasLogger = false;
    const mockHandler = jest
      .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
      .mockImplementation(async (_input, ctx) => {
        hasLogger = ctx.logger !== undefined;
        return Success({ result: 'success' });
      });
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' });

    expect(hasLogger).toBe(true);
  });

  test('should provide queryConfig method in context', async () => {
    let hasQueryConfig = false;
    const mockHandler = jest
      .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
      .mockImplementation(async (_input, ctx) => {
        hasQueryConfig = typeof ctx.queryConfig === 'function';
        return Success({ result: 'success' });
      });
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' });

    expect(hasQueryConfig).toBe(true);
  });

  test('should return null from queryConfig when no policy is configured', async () => {
    let queryResult: unknown;
    const mockHandler = jest
      .fn<(input: MockSchemaInput, ctx: ToolContext) => Promise<Result<{ result: string }>>>()
      .mockImplementation(async (_input, ctx) => {
        queryResult = await ctx.queryConfig('test.package', {});
        return Success({ result: 'success' });
      });
    const mockTool = createMockTool(mockHandler);

    await executeTool(mockTool, { input: 'test' });

    expect(queryResult).toBeNull();
  });
});
