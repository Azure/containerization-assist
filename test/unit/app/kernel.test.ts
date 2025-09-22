/**
 * Kernel Tests
 * Tests for the unified application kernel
 */

import { describe, it, expect, beforeEach, jest } from '@jest/globals';
import { z } from 'zod';
import {
  createKernel,
  InMemorySessionManager,
  ExecutionPlanner,
  type Kernel,
  type KernelConfig,
  type RegisteredTool,
  type ToolContext,
  type ExecuteRequest,
} from '@/app/kernel';
import { Success, Failure } from '@/types/index';

describe('Application Kernel', () => {
  let kernel: Kernel;
  let tools: Map<string, RegisteredTool>;

  beforeEach(async () => {
    // Create test tools
    tools = new Map();

    // Tool A: No dependencies
    tools.set('tool-a', {
      name: 'tool-a',
      description: 'Test tool A',
      schema: z.object({
        input: z.string(),
      }),
      handler: async (params: any, context: ToolContext) => {
        return Success({ output: `A: ${params.input}` });
      },
      provides: ['step-a'],
    });

    // Tool B: Depends on A
    tools.set('tool-b', {
      name: 'tool-b',
      description: 'Test tool B',
      schema: z.object({
        input: z.string(),
      }),
      handler: async (params: any, context: ToolContext) => {
        return Success({ output: `B: ${params.input}` });
      },
      requires: ['tool-a'],
      provides: ['step-b'],
    });

    // Tool C: Depends on B
    tools.set('tool-c', {
      name: 'tool-c',
      description: 'Test tool C',
      schema: z.object({
        input: z.string(),
        optional: z.number().optional(),
      }),
      handler: async (params: any, context: ToolContext) => {
        return Success({ output: `C: ${params.input}` });
      },
      requires: ['tool-b'],
      provides: ['step-c'],
    });

    // Tool that fails
    tools.set('tool-fail', {
      name: 'tool-fail',
      description: 'Test tool that fails',
      schema: z.object({
        shouldFail: z.boolean(),
      }),
      handler: async (params: any, context: ToolContext) => {
        if (params.shouldFail) {
          return Failure('Intentional failure');
        }
        return Success({ output: 'Success' });
      },
    });

    // Create kernel
    const config: KernelConfig = {
      maxRetries: 2,
      retryDelay: 100,
    };

    kernel = await createKernel(config, tools);
  });

  describe('Tool Execution', () => {
    it('should execute a simple tool', async () => {
      const request: ExecuteRequest = {
        toolName: 'tool-a',
        params: { input: 'test' },
      };

      const result = await kernel.execute(request);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toEqual({ output: 'A: test' });
      }
    });

    it('should validate parameters', async () => {
      const request: ExecuteRequest = {
        toolName: 'tool-a',
        params: { wrong: 'field' }, // Invalid params
      };

      const result = await kernel.execute(request);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Validation failed');
      }
    });

    it('should handle missing tools', async () => {
      const request: ExecuteRequest = {
        toolName: 'non-existent',
        params: {},
      };

      const result = await kernel.execute(request);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Tool not found');
      }
    });

    it('should handle tool failures', async () => {
      const request: ExecuteRequest = {
        toolName: 'tool-fail',
        params: { shouldFail: true },
      };

      const result = await kernel.execute(request);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Intentional failure');
      }
    });
  });

  describe('Dependency Resolution', () => {
    it('should execute dependencies in order', async () => {
      const executionOrder: string[] = [];

      // Override tool handlers to track execution order
      const originalHandlers = new Map<string, any>();
      ['tool-a', 'tool-b', 'tool-c'].forEach(name => {
        const tool = tools.get(name)!;
        originalHandlers.set(name, tool.handler);
        tool.handler = async (params: any, context: ToolContext) => {
          executionOrder.push(name);
          return originalHandlers.get(name)(params, context);
        };
      });

      const request: ExecuteRequest = {
        toolName: 'tool-c',
        params: { input: 'test' },
      };

      const result = await kernel.execute(request);

      expect(result.ok).toBe(true);
      expect(executionOrder).toEqual(['tool-a', 'tool-b', 'tool-c']);
    });

    it('should skip already completed steps', async () => {
      // Create session
      const sessionResult = await kernel.createSession();
      expect(sessionResult.ok).toBe(true);

      const sessionId = sessionResult.ok ? sessionResult.value.sessionId : '';

      // Execute tool-a
      await kernel.execute({
        toolName: 'tool-a',
        params: { input: 'first' },
        sessionId,
      });

      // Track execution for tool-b
      let toolBExecuted = false;
      const originalHandler = tools.get('tool-b')!.handler;
      tools.get('tool-b')!.handler = async (params: any, context: ToolContext) => {
        toolBExecuted = true;
        return originalHandler(params, context);
      };

      // Execute tool-b (should not re-execute tool-a)
      const executionOrder: string[] = [];
      ['tool-a', 'tool-b'].forEach(name => {
        const tool = tools.get(name)!;
        const original = tool.handler;
        tool.handler = async (params: any, context: ToolContext) => {
          executionOrder.push(name);
          return original(params, context);
        };
      });

      await kernel.execute({
        toolName: 'tool-b',
        params: { input: 'second' },
        sessionId,
      });

      expect(toolBExecuted).toBe(true);
      expect(executionOrder).toEqual(['tool-b']); // tool-a should not be re-executed
    });
  });

  describe('Planning', () => {
    it('should generate execution plan', async () => {
      const plan = await kernel.getPlan('tool-c');

      expect(plan).toEqual(['tool-a', 'tool-b', 'tool-c']);
    });

    it('should generate partial plan with session', async () => {
      // Create session and execute tool-a
      const sessionResult = await kernel.createSession();
      const sessionId = sessionResult.ok ? sessionResult.value.sessionId : '';

      await kernel.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        sessionId,
      });

      // Get plan for tool-c
      const plan = await kernel.getPlan('tool-c', sessionId);

      expect(plan).toEqual(['tool-b', 'tool-c']); // tool-a already completed
    });

    it('should check execution prerequisites', async () => {
      const result = await kernel.canExecute('tool-c');

      expect(result.canExecute).toBe(false);
      expect(result.missing).toContain('tool-b');
    });

    it('should check execution with completed steps', async () => {
      // Create session and execute dependencies
      const sessionResult = await kernel.createSession();
      const sessionId = sessionResult.ok ? sessionResult.value.sessionId : '';

      await kernel.execute({
        toolName: 'tool-b',
        params: { input: 'test' },
        sessionId,
      });

      const result = await kernel.canExecute('tool-c', sessionId);

      expect(result.canExecute).toBe(true);
      expect(result.missing).toEqual([]);
      expect(result.completed).toContain('tool-a');
      expect(result.completed).toContain('tool-b');
    });
  });

  describe('Session Management', () => {
    it('should create new sessions', async () => {
      const result = await kernel.createSession();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.sessionId).toBeDefined();
        expect(result.value.completed_steps).toEqual([]);
        expect(result.value.data).toEqual({});
      }
    });

    it('should get existing sessions', async () => {
      const createResult = await kernel.createSession();
      expect(createResult.ok).toBe(true);

      if (createResult.ok) {
        const getResult = await kernel.getSession(createResult.value.sessionId);
        expect(getResult.ok).toBe(true);
        if (getResult.ok) {
          expect(getResult.value.sessionId).toBe(createResult.value.sessionId);
        }
      }
    });

    it('should update session with tool results', async () => {
      const sessionResult = await kernel.createSession();
      const sessionId = sessionResult.ok ? sessionResult.value.sessionId : '';

      await kernel.execute({
        toolName: 'tool-a',
        params: { input: 'test' },
        sessionId,
      });

      const updatedSession = await kernel.getSession(sessionId);
      expect(updatedSession.ok).toBe(true);

      if (updatedSession.ok) {
        expect(updatedSession.value.completed_steps).toContain('tool-a');
        expect(updatedSession.value.data['tool-a']).toEqual({ output: 'A: test' });
      }
    });
  });

  describe('Registry Management', () => {
    it('should return all registered tools', () => {
      const registeredTools = kernel.tools();

      expect(registeredTools.size).toBe(4);
      expect(registeredTools.has('tool-a')).toBe(true);
      expect(registeredTools.has('tool-b')).toBe(true);
      expect(registeredTools.has('tool-c')).toBe(true);
      expect(registeredTools.has('tool-fail')).toBe(true);
    });

    it('should get specific tool', () => {
      const tool = kernel.getTool('tool-a');

      expect(tool).toBeDefined();
      expect(tool?.name).toBe('tool-a');
      expect(tool?.description).toBe('Test tool A');
    });

    it('should return undefined for non-existent tool', () => {
      const tool = kernel.getTool('non-existent');

      expect(tool).toBeUndefined();
    });
  });

  describe('Health and Metrics', () => {
    it('should report healthy status', () => {
      const health = kernel.getHealth();

      expect(health.status).toBe('healthy');
      expect(health.metrics).toBeDefined();
    });

    it('should track execution metrics', async () => {
      // Execute some tools
      await kernel.execute({
        toolName: 'tool-a',
        params: { input: 'test1' },
      });

      await kernel.execute({
        toolName: 'tool-a',
        params: { input: 'test2' },
      });

      const metrics = kernel.getMetrics();

      // Should have duration metrics for tool-a
      const durationMetric = Array.from(metrics.values()).find(
        m => m.name === 'tool-a.duration'
      );

      expect(durationMetric).toBeDefined();
      if (durationMetric) {
        expect(durationMetric.count).toBe(2);
        expect(durationMetric.min).toBeGreaterThanOrEqual(0);
        expect(durationMetric.max).toBeGreaterThanOrEqual(durationMetric.min);
      }
    });

    it('should track error metrics', async () => {
      // Execute failing tool
      await kernel.execute({
        toolName: 'tool-fail',
        params: { shouldFail: true },
      });

      const metrics = kernel.getMetrics();

      // Should have error metrics
      const errorMetric = Array.from(metrics.values()).find(
        m => m.name === 'tool-fail.errors'
      );

      expect(errorMetric).toBeDefined();
      if (errorMetric) {
        expect(errorMetric.value).toBeGreaterThan(0);
      }
    });
  });

  describe('Execution Planner', () => {
    it('should build correct execution plan', () => {
      const planner = new ExecutionPlanner(tools);
      const plan = planner.buildPlan('tool-c', new Set());

      expect(plan.steps).toEqual(['tool-a', 'tool-b', 'tool-c']);
      expect(plan.remaining).toEqual(['tool-a', 'tool-b', 'tool-c']);
      expect(plan.completed.size).toBe(0);
    });

    it('should handle completed steps', () => {
      const planner = new ExecutionPlanner(tools);
      const completed = new Set(['tool-a']);
      const plan = planner.buildPlan('tool-c', completed);

      expect(plan.steps).toEqual(['tool-b', 'tool-c']);
      expect(plan.remaining).toEqual(['tool-b', 'tool-c']);
      expect(plan.completed.has('tool-a')).toBe(true);
    });

    it('should handle tools without dependencies', () => {
      const planner = new ExecutionPlanner(tools);
      const plan = planner.buildPlan('tool-a', new Set());

      expect(plan.steps).toEqual(['tool-a']);
      expect(plan.remaining).toEqual(['tool-a']);
    });

    it('should check execution prerequisites', () => {
      const planner = new ExecutionPlanner(tools);

      // tool-c requires tool-b
      const result1 = planner.canExecute('tool-c', new Set());
      expect(result1.canExecute).toBe(false);
      expect(result1.missing).toContain('tool-b');

      // With tool-b completed
      const result2 = planner.canExecute('tool-c', new Set(['tool-b']));
      expect(result2.canExecute).toBe(true);
      expect(result2.missing).toEqual([]);
    });
  });

  describe('In-Memory Session Manager', () => {
    let sessionManager: InMemorySessionManager;

    beforeEach(() => {
      sessionManager = new InMemorySessionManager();
    });

    it('should create sessions', async () => {
      const result = await sessionManager.create();

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.sessionId).toMatch(/^session-\d+$/);
        expect(result.value.completed_steps).toEqual([]);
      }
    });

    it('should get sessions', async () => {
      const createResult = await sessionManager.create();
      expect(createResult.ok).toBe(true);

      if (createResult.ok) {
        const getResult = await sessionManager.get(createResult.value.sessionId);
        expect(getResult.ok).toBe(true);
        if (getResult.ok) {
          expect(getResult.value.sessionId).toBe(createResult.value.sessionId);
        }
      }
    });

    it('should update sessions', async () => {
      const createResult = await sessionManager.create();
      expect(createResult.ok).toBe(true);

      if (createResult.ok) {
        const updateResult = await sessionManager.update(
          createResult.value.sessionId,
          {
            completed_steps: ['step1'],
            data: { key: 'value' },
          }
        );

        expect(updateResult.ok).toBe(true);

        const getResult = await sessionManager.get(createResult.value.sessionId);
        expect(getResult.ok).toBe(true);
        if (getResult.ok) {
          expect(getResult.value.completed_steps).toEqual(['step1']);
          expect(getResult.value.data).toEqual({ key: 'value' });
        }
      }
    });

    it('should delete sessions', async () => {
      const createResult = await sessionManager.create();
      expect(createResult.ok).toBe(true);

      if (createResult.ok) {
        const deleteResult = await sessionManager.delete(createResult.value.sessionId);
        expect(deleteResult.ok).toBe(true);

        const getResult = await sessionManager.get(createResult.value.sessionId);
        expect(getResult.ok).toBe(false);
      }
    });

    it('should list sessions', async () => {
      await sessionManager.create();
      await sessionManager.create();

      const listResult = await sessionManager.list();
      expect(listResult.ok).toBe(true);
      if (listResult.ok) {
        expect(listResult.value).toHaveLength(2);
        expect(listResult.value[0]).toMatch(/^session-\d+$/);
        expect(listResult.value[1]).toMatch(/^session-\d+$/);
      }
    });
  });
});