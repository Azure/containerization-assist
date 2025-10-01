/**
 * Integration Tests for Transport Metadata Propagation (WS1 - T1.4)
 *
 * Verifies that metadata fields (signal, maxTokens, stopSequences, sessionId)
 * are properly extracted from RequestHandlerExtra and _meta, then propagated
 * through the orchestrator to tool contexts.
 */

import { jest } from '@jest/globals';
import { createOrchestrator } from '@/app/orchestrator';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';
import { z } from 'zod';
import type { Tool } from '@/types/tool';
import type { ToolContext } from '@/mcp/context';
import { Success, Failure } from '@/types';
import type { ExecuteRequest } from '@/app/orchestrator-types';

// Setup
import '../__support__/setup/integration-setup.js';

describe('Metadata Propagation Integration', () => {
  let testLogger: Logger;

  beforeEach(() => {
    testLogger = createLogger({ name: 'test', level: 'silent' });
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('Cancellation via AbortSignal', () => {
    it('should abort tool execution when signal is triggered', async () => {
      const abortController = new AbortController();
      let capturedSignal: AbortSignal | undefined;

      // Create a tool that captures the signal and respects cancellation
      const cancellableTool: Tool<z.ZodObject<any>, any> = {
        name: 'cancellable-tool',
        description: 'A tool that respects cancellation',
        version: '1.0.0',
        schema: z.object({ input: z.string() }),
        run: async (params: any, ctx: ToolContext) => {
          capturedSignal = ctx.signal;

          // Simulate some work that checks for cancellation
          return new Promise((resolve) => {
            const timer = setTimeout(() => {
              resolve(Success({ completed: true }));
            }, 1000);

            // Listen for abort signal
            if (ctx.signal) {
              ctx.signal.addEventListener('abort', () => {
                clearTimeout(timer);
                resolve(Failure('Operation cancelled'));
              });
            }
          });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('cancellable-tool', cancellableTool);

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
      });

      try {
        // Start execution
        const executionPromise = orchestrator.execute({
          toolName: 'cancellable-tool',
          params: { input: 'test' },
          metadata: {
            signal: abortController.signal,
          },
        } as ExecuteRequest);

        // Cancel after a short delay
        setTimeout(() => abortController.abort(), 100);

        const result = await executionPromise;

        // Verify signal was propagated
        expect(capturedSignal).toBeDefined();
        expect(capturedSignal).toBe(abortController.signal);

        // Verify cancellation worked
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('cancelled');
        }
      } finally {
        orchestrator.close();
      }
    });

    it('should propagate pre-aborted signal correctly', async () => {
      const abortController = new AbortController();
      abortController.abort(); // Abort before execution

      let capturedSignal: AbortSignal | undefined;

      const checkSignalTool: Tool<z.ZodObject<any>, any> = {
        name: 'check-signal-tool',
        description: 'Checks if signal is aborted',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, ctx: ToolContext) => {
          capturedSignal = ctx.signal;
          if (ctx.signal?.aborted) {
            return Failure('Signal already aborted');
          }
          return Success({ status: 'ok' });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('check-signal-tool', checkSignalTool);

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
      });

      try {
        const result = await orchestrator.execute({
          toolName: 'check-signal-tool',
          params: {},
          metadata: {
            signal: abortController.signal,
          },
        } as ExecuteRequest);

        expect(capturedSignal).toBeDefined();
        expect(capturedSignal?.aborted).toBe(true);
        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toContain('already aborted');
        }
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Token Ceiling Propagation', () => {
    it('should propagate maxTokens to tool context', async () => {
      let capturedMaxTokens: number | undefined;

      const tokenAwareTool: Tool<z.ZodObject<any>, any> = {
        name: 'token-aware-tool',
        description: 'A tool that checks maxTokens',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, ctx: ToolContext) => {
          // Note: maxTokens would be available in sampling context options
          // For this test, we verify it's in the metadata that gets passed to createToolContext
          return Success({ received: 'ok' });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('token-aware-tool', tokenAwareTool);

      // Create a custom context factory to capture maxTokens
      const contextFactory = jest.fn(async (input: any) => {
        capturedMaxTokens = input.request.metadata?.maxTokens;
        return {
          sampling: {
            createMessage: jest.fn(),
          },
          getPrompt: jest.fn(),
          signal: undefined,
          progress: undefined,
          sessionManager: input.sessionManager,
          session: input.sessionFacade,
          logger: input.logger,
        } as any;
      });

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        contextFactory,
      });

      try {
        const result = await orchestrator.execute({
          toolName: 'token-aware-tool',
          params: {},
          metadata: {
            maxTokens: 4096,
          },
        } as ExecuteRequest);

        expect(result.ok).toBe(true);
        expect(capturedMaxTokens).toBe(4096);
      } finally {
        orchestrator.close();
      }
    });

    it('should propagate stopSequences to tool context', async () => {
      let capturedStopSequences: string[] | undefined;

      const stopSequenceTool: Tool<z.ZodObject<any>, any> = {
        name: 'stop-sequence-tool',
        description: 'A tool that checks stopSequences',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, ctx: ToolContext) => {
          return Success({ received: 'ok' });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('stop-sequence-tool', stopSequenceTool);

      // Create a custom context factory to capture stopSequences
      const contextFactory = jest.fn(async (input: any) => {
        capturedStopSequences = input.request.metadata?.stopSequences;
        return {
          sampling: {
            createMessage: jest.fn(),
          },
          getPrompt: jest.fn(),
          signal: undefined,
          progress: undefined,
          sessionManager: input.sessionManager,
          session: input.sessionFacade,
          logger: input.logger,
        } as any;
      });

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        contextFactory,
      });

      try {
        const result = await orchestrator.execute({
          toolName: 'stop-sequence-tool',
          params: {},
          metadata: {
            stopSequences: ['STOP', 'END', '###'],
          },
        } as ExecuteRequest);

        expect(result.ok).toBe(true);
        expect(capturedStopSequences).toEqual(['STOP', 'END', '###']);
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('SessionId Propagation', () => {
    it('should use transport-level sessionId when provided', async () => {
      let capturedSessionId: string | undefined;

      const sessionTool: Tool<z.ZodObject<any>, any> = {
        name: 'session-tool',
        description: 'A tool that checks session',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, ctx: ToolContext) => {
          capturedSessionId = ctx.session?.id;
          return Success({ sessionId: ctx.session?.id });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('session-tool', sessionTool);

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
      });

      try {
        const result = await orchestrator.execute({
          toolName: 'session-tool',
          params: {},
          sessionId: 'transport-session-123',
        });

        expect(result.ok).toBe(true);
        expect(capturedSessionId).toBe('transport-session-123');
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Combined Metadata Propagation', () => {
    it('should propagate all metadata fields together', async () => {
      const abortController = new AbortController();
      let capturedContext: Partial<ToolContext> = {};

      const combinedTool: Tool<z.ZodObject<any>, any> = {
        name: 'combined-tool',
        description: 'A tool that captures all context',
        version: '1.0.0',
        schema: z.object({ input: z.string() }),
        run: async (_params: any, ctx: ToolContext) => {
          capturedContext = {
            signal: ctx.signal,
            session: ctx.session,
            progress: ctx.progress,
          };
          return Success({ captured: true });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('combined-tool', combinedTool);

      // Custom context factory to verify all metadata
      let capturedMetadata: any;
      const contextFactory = jest.fn(async (input: any) => {
        capturedMetadata = input.request.metadata;
        return {
          sampling: {
            createMessage: jest.fn(),
          },
          getPrompt: jest.fn(),
          signal: input.request.metadata?.signal,
          progress: input.request.metadata?.progress,
          sessionManager: input.sessionManager,
          session: input.sessionFacade,
          logger: input.logger,
        } as any;
      });

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
        contextFactory,
      });

      try {
        const result = await orchestrator.execute({
          toolName: 'combined-tool',
          params: { input: 'test' },
          sessionId: 'combined-session',
          metadata: {
            signal: abortController.signal,
            maxTokens: 2048,
            stopSequences: ['###'],
            progress: {},
          },
        } as ExecuteRequest);

        expect(result.ok).toBe(true);

        // Verify all metadata was captured
        expect(capturedMetadata).toBeDefined();
        expect(capturedMetadata.signal).toBe(abortController.signal);
        expect(capturedMetadata.maxTokens).toBe(2048);
        expect(capturedMetadata.stopSequences).toEqual(['###']);

        // Verify context received the metadata
        expect(capturedContext.signal).toBe(abortController.signal);
        expect(capturedContext.session?.id).toBe('combined-session');
      } finally {
        orchestrator.close();
      }
    });
  });

  describe('Edge Cases', () => {
    it('should handle missing metadata gracefully', async () => {
      const simpleTool: Tool<z.ZodObject<any>, any> = {
        name: 'simple-tool',
        description: 'A simple tool',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, ctx: ToolContext) => {
          return Success({
            hasSignal: ctx.signal !== undefined,
            hasProgress: ctx.progress !== undefined,
          });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('simple-tool', simpleTool);

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
      });

      try {
        // Execute without any metadata
        const result = await orchestrator.execute({
          toolName: 'simple-tool',
          params: {},
        });

        expect(result.ok).toBe(true);
        if (result.ok) {
          // Tool should run successfully even without metadata
          expect(result.value).toBeDefined();
        }
      } finally {
        orchestrator.close();
      }
    });

    it('should handle invalid metadata values gracefully', async () => {
      const tolerantTool: Tool<z.ZodObject<any>, any> = {
        name: 'tolerant-tool',
        description: 'A tolerant tool',
        version: '1.0.0',
        schema: z.object({}),
        run: async (_params: any, _ctx: ToolContext) => {
          return Success({ ok: true });
        },
      };

      const toolsMap = new Map();
      toolsMap.set('tolerant-tool', tolerantTool);

      const orchestrator = createOrchestrator({
        registry: toolsMap,
        logger: testLogger,
      });

      try {
        // Try with invalid maxTokens (should be filtered out)
        const result = await orchestrator.execute({
          toolName: 'tolerant-tool',
          params: {},
          metadata: {
            maxTokens: 'invalid' as any, // Wrong type
          },
        } as ExecuteRequest);

        // Should still execute successfully
        expect(result.ok).toBe(true);
      } finally {
        orchestrator.close();
      }
    });
  });
});
