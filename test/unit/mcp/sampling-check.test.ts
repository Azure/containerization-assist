/**
 * Sampling Check Tests
 * Tests for the centralized sampling availability checker
 */

import { describe, it, expect, jest } from '@jest/globals';
import { checkSamplingAvailability, VERBOSE_MODE_MESSAGE } from '@/mcp/sampling-check';
import type { ToolContext } from '@/mcp/context';
import { createLogger } from '@/lib/logger';

describe('Sampling Check', () => {
  describe('checkSamplingAvailability', () => {
    it('should return available=true when sampling works', async () => {
      const mockContext = {
        sampling: {
          createMessage: jest.fn().mockResolvedValue({
            role: 'assistant',
            content: [{ type: 'text', text: 'test' }],
          }),
        },
        getPrompt: jest.fn(),
        signal: undefined,
        progress: undefined,
        logger: createLogger({ name: 'test', level: 'silent' }),
      } as unknown as ToolContext;

      const result = await checkSamplingAvailability(mockContext);

      expect(result.available).toBe(true);
      expect(result.message).toBe('');
      expect(mockContext.sampling.createMessage).toHaveBeenCalledWith({
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'test' }],
          },
        ],
        maxTokens: 1,
      });
    });

    it('should return available=false when sampling fails', async () => {
      const mockContext = {
        sampling: {
          createMessage: jest.fn().mockRejectedValue(new Error('Not available')),
        },
        getPrompt: jest.fn(),
        signal: undefined,
        progress: undefined,
        logger: createLogger({ name: 'test', level: 'silent' }),
      } as unknown as ToolContext;

      const result = await checkSamplingAvailability(mockContext);

      expect(result.available).toBe(false);
      expect(result.message).toBe(VERBOSE_MODE_MESSAGE);
    });
  });
});
