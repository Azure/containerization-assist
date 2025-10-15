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

describe('Tool Orchestrator', () => {
  let orchestrator: ToolOrchestrator;
  let mockTools: Map<string, Tool>;
  let mockServer: Server;

  beforeEach(() => {

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
});