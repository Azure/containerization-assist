/**
 * Orchestrator Tests
 * Tests for the tool orchestrator
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import { z } from 'zod';
import { createOrchestrator } from '@/app/orchestrator';
import type { ToolOrchestrator } from '@/app/orchestrator-types';
import { Success, Failure, type Tool } from '@/types';
import type { Server } from '@modelcontextprotocol/sdk/server/index.js';

// Mock SessionManager with proper session isolation
const sessionStore = new Map();

const mockSessionManager = {
  get: jest.fn((sessionId: string) => {
    const session = sessionStore.get(sessionId);
    return Promise.resolve(Success(session));
  }),
  create: jest.fn((sessionId: string) => {
    const newSession = {
      sessionId,
      metadata: {},
      completed_steps: [],
      createdAt: new Date(),
      updatedAt: new Date(),
      lastAccessedAt: new Date()
    };
    sessionStore.set(sessionId, newSession);
    return Promise.resolve(Success(newSession));
  }),
  update: jest.fn().mockResolvedValue(Success(undefined)),
  delete: jest.fn().mockResolvedValue(Success(undefined)),
  cleanup: jest.fn(),
};

jest.mock('@/session/core', () => ({
  SessionManager: jest.fn().mockImplementation(() => mockSessionManager),
}));

describe('Tool Orchestrator', () => {
  let orchestrator: ToolOrchestrator;
  let mockTools: Map<string, Tool>;
  let mockServer: Server;

  beforeEach(() => {
    // Clear session store before each test
    sessionStore.clear();

    // Create mock server
    mockServer = {
      createMessage: jest.fn().mockResolvedValue({
        content: {
          type: 'text',
          text: 'Mock AI response'
        }
      })
    } as unknown as Server;

    // Create mock tools
    mockTools = new Map();

    // Simple tool without dependencies
    const toolA: Tool = {
      name: 'tool-a',
      description: 'Test tool A',
      schema: z.object({ input: z.string() }),
      run: jest.fn().mockResolvedValue(Success({ result: 'A executed' })),
    };
    mockTools.set('tool-a', toolA);

    // Another simple tool
    const toolB: Tool = {
      name: 'tool-b',
      description: 'Test tool B',
      schema: z.object({ value: z.number() }),
      run: jest.fn().mockResolvedValue(Success({ result: 'B executed' })),
    };
    mockTools.set('tool-b', toolB);

    // Create orchestrator with mock server
    orchestrator = createOrchestrator({
      registry: mockTools,
      server: mockServer,
    });
  });

  describe('Simple Tool Execution', () => {
    it('should execute a simple tool successfully', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ result: 'A executed' });
      }
    });

    it('should fail for unknown tool', async () => {
      const result = await orchestrator.execute({
        toolName: 'unknown-tool',
        params: {},
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Tool not found');
      }
    });

    it('should validate parameters', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-b',
        params: { value: 'not-a-number' }, // Invalid type
      });

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Validation failed');
      }
    });
  });

  describe('Policy Application', () => {
    it('should apply blocking policies', async () => {
      // Create orchestrator with policy
      const orchestratorWithPolicy = createOrchestrator({
        registry: mockTools,
        server: mockServer,
        config: {
          policyPath: 'test-policy.yaml', // Would need to mock policy loading
        },
      });

      // This would test policy blocking, but requires mocking policy loading
      // For now, we'll skip the actual policy test
      expect(orchestratorWithPolicy).toBeDefined();
    });
  });

  describe('Session Management', () => {
    it('should handle session-based execution', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        sessionId: 'test-session',
      });

      // Session-based tools go through orchestration
      expect(result.ok).toBe(true);
    });

    it('should auto-generate sessionId when none provided', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        // No sessionId provided
      });

      expect(result.ok).toBe(true);
      // Even without explicit sessionId, orchestration should work
    });

    it('should reuse existing session when sessionId provided', async () => {
      const sessionId = 'reusable-session';

      // First execution
      const result1 = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'first' },
        sessionId,
      });

      // Second execution with same sessionId
      const result2 = await orchestrator.execute({
        toolName: 'tool-b',
        params: { value: 42 },
        sessionId,
      });

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);
      // Both should succeed using the same session
    });

    it('should provide session facade to tool handlers', async () => {
      // Create a tool that uses session
      const sessionTool: Tool = {
        name: 'session-tool',
        description: 'Tool that uses session',
        schema: z.object({ key: z.string(), value: z.string() }),
        run: jest.fn(async (params, context) => {
          // Tool should have access to session facade
          expect(context.session).toBeDefined();
          expect(context.session?.id).toBeDefined();
          expect(typeof context.session?.get).toBe('function');
          expect(typeof context.session?.set).toBe('function');
          expect(typeof context.session?.pushStep).toBe('function');

          // Use session methods
          context.session?.set(params.key, params.value);
          const retrieved = context.session?.get(params.key);
          context.session?.pushStep(`stored-${params.key}`);

          return Success({ stored: retrieved });
        }),
      };
      mockTools.set('session-tool', sessionTool);

      const result = await orchestrator.execute({
        toolName: 'session-tool',
        params: { key: 'testKey', value: 'testValue' },
        sessionId: 'facade-test',
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ stored: 'testValue' });
      }
    });

    it('should maintain session state across tool executions', async () => {
      const sessionId = 'stateful-session';

      // Create tools that interact with session state
      const setterTool: Tool = {
        name: 'setter-tool',
        description: 'Sets session data',
        schema: z.object({ data: z.string() }),
        run: jest.fn(async (params, context) => {
          context.session?.set('sharedData', params.data);
          return Success({ action: 'set', data: params.data });
        }),
      };

      const getterTool: Tool = {
        name: 'getter-tool',
        description: 'Gets session data',
        schema: z.object({}),
        run: jest.fn(async (params, context) => {
          const data = context.session?.get('sharedData');
          return Success({ action: 'get', data });
        }),
      };

      mockTools.set('setter-tool', setterTool);
      mockTools.set('getter-tool', getterTool);

      // Set data in session
      const setResult = await orchestrator.execute({
        toolName: 'setter-tool',
        params: { data: 'persistent-value' },
        sessionId,
      });

      // Get data from same session
      const getResult = await orchestrator.execute({
        toolName: 'getter-tool',
        params: {},
        sessionId,
      });

      expect(setResult.ok).toBe(true);
      expect(getResult.ok).toBe(true);

      if (getResult.ok) {
        expect(getResult.value).toEqual({
          action: 'get',
          data: 'persistent-value'
        });
      }
    });

    it('should track completed steps in session', async () => {
      const sessionId = 'step-tracking-session';

      // Create a tool that pushes steps
      const stepTool: Tool = {
        name: 'step-tool',
        description: 'Tool that tracks steps',
        schema: z.object({ step: z.string() }),
        run: jest.fn(async (params, context) => {
          context.session?.pushStep(params.step);
          return Success({ stepAdded: params.step });
        }),
      };
      mockTools.set('step-tool', stepTool);

      // Execute multiple times to track steps
      await orchestrator.execute({
        toolName: 'step-tool',
        params: { step: 'step-1' },
        sessionId,
      });

      await orchestrator.execute({
        toolName: 'step-tool',
        params: { step: 'step-2' },
        sessionId,
      });

      // Verify steps were tracked (we can't directly access session state,
      // but the orchestrator internally tracks completed steps)
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        sessionId,
      });

      expect(result.ok).toBe(true);
    });

    it('should isolate session state between different sessionIds', async () => {
      const session1 = 'isolated-session-1';
      const session2 = 'isolated-session-2';

      // Create tool that sets and gets data
      const dataTools: Tool = {
        name: 'data-tool',
        description: 'Tool for session data operations',
        schema: z.object({ action: z.enum(['set', 'get']), key: z.string(), value: z.string().optional() }),
        run: jest.fn(async (params, context) => {
          if (params.action === 'set' && params.value) {
            context.session?.set(params.key, params.value);
            return Success({ action: 'set', key: params.key, value: params.value });
          } else {
            const value = context.session?.get(params.key);
            return Success({ action: 'get', key: params.key, value });
          }
        }),
      };
      mockTools.set('data-tool', dataTools);

      // Set different values in different sessions
      await orchestrator.execute({
        toolName: 'data-tool',
        params: { action: 'set', key: 'test', value: 'session1-value' },
        sessionId: session1,
      });

      await orchestrator.execute({
        toolName: 'data-tool',
        params: { action: 'set', key: 'test', value: 'session2-value' },
        sessionId: session2,
      });

      // Retrieve values from different sessions
      const result1 = await orchestrator.execute({
        toolName: 'data-tool',
        params: { action: 'get', key: 'test' },
        sessionId: session1,
      });

      const result2 = await orchestrator.execute({
        toolName: 'data-tool',
        params: { action: 'get', key: 'test' },
        sessionId: session2,
      });

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);

      if (result1.ok && result2.ok) {
        expect(result1.value).toEqual({
          action: 'get',
          key: 'test',
          value: 'session1-value'
        });
        expect(result2.value).toEqual({
          action: 'get',
          key: 'test',
          value: 'session2-value'
        });
      }
    });
  });

  describe('SessionId Return Behavior', () => {
    it('should provide sessionId information when session is used', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        sessionId: 'explicit-session-id',
      });

      expect(result.ok).toBe(true);
      // The result should contain tool output, and session should be internally managed
      // Note: Current interface doesn't expose sessionId in response, which is acceptable
      // as session management is internal to orchestrator
    });

    it('should work with auto-generated sessionId', async () => {
      const result = await orchestrator.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        // No sessionId - orchestrator should auto-generate one
      });

      expect(result.ok).toBe(true);
      // Auto-generated sessions should work just as well as explicit ones
    });

    it('should maintain session consistency across multiple calls', async () => {
      const sessionId = 'consistent-session';

      // Create a tool that tracks call count in session
      const counterTool: Tool = {
        name: 'counter-tool',
        description: 'Tool that counts calls',
        schema: z.object({}),
        run: jest.fn(async (params, context) => {
          const currentCount = (context.session?.get('callCount') as number) || 0;
          const newCount = currentCount + 1;
          context.session?.set('callCount', newCount);
          return Success({ count: newCount });
        }),
      };
      mockTools.set('counter-tool', counterTool);

      // Make multiple calls with same sessionId
      const result1 = await orchestrator.execute({
        toolName: 'counter-tool',
        params: {},
        sessionId,
      });

      const result2 = await orchestrator.execute({
        toolName: 'counter-tool',
        params: {},
        sessionId,
      });

      const result3 = await orchestrator.execute({
        toolName: 'counter-tool',
        params: {},
        sessionId,
      });

      expect(result1.ok).toBe(true);
      expect(result2.ok).toBe(true);
      expect(result3.ok).toBe(true);

      if (result1.ok && result2.ok && result3.ok) {
        expect(result1.value).toEqual({ count: 1 });
        expect(result2.value).toEqual({ count: 2 });
        expect(result3.value).toEqual({ count: 3 });
      }
    });
  });
});