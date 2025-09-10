/**
 * Tests for AI Helpers Module
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import type { Logger } from 'pino';
import type { ToolContext, SamplingResponse, PromptWithMessages } from '../../../../src/mcp/context/types';
import { aiGenerate, withAIFallback, structureError, aiError } from '../../../../src/mcp/tools/ai-helpers';
import { Success, Failure } from '../../../../src/types';

describe('AI Helpers', () => {
  let mockLogger: jest.Mocked<Logger>;
  let mockContext: jest.Mocked<ToolContext>;
  
  beforeEach(() => {
    // Create mock logger
    mockLogger = {
      debug: jest.fn(),
      info: jest.fn(),
      warn: jest.fn(),
      error: jest.fn(),
    } as any;
    
    // Create mock context
    mockContext = {
      sampling: {
        createMessage: jest.fn(),
      },
      getPrompt: jest.fn(),
    } as any;
  });

  describe('aiGenerate', () => {
    it('should successfully generate AI response with valid content', async () => {
      // Setup mock prompt response
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [
          { 
            role: 'user', 
            content: [{ type: 'text', text: 'Generate a Dockerfile' }] 
          }
        ],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      // Setup mock AI response
      const mockResponse: SamplingResponse = {
        role: 'assistant',
        content: [{ type: 'text', text: 'FROM node:18\nWORKDIR /app\nCOPY . .\nCMD ["node", "app.js"]' }],
        metadata: {
          model: 'claude-3',
          usage: { inputTokens: 10, outputTokens: 20, totalTokens: 30 },
        },
      };
      mockContext.sampling.createMessage.mockResolvedValue(mockResponse);
      
      // Test the function
      const result = await aiGenerate(mockLogger, mockContext, {
        promptName: 'dockerfile-generation',
        promptArgs: { framework: 'node' },
        expectation: 'dockerfile',
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.content).toContain('FROM node:18');
        expect(result.value.model).toBe('claude-3');
        expect(result.value.usage).toEqual({ inputTokens: 10, outputTokens: 20, totalTokens: 30 });
      }
      
      expect(mockContext.getPrompt).toHaveBeenCalledWith('dockerfile-generation', { framework: 'node' });
      expect(mockContext.sampling.createMessage).toHaveBeenCalled();
    });

    it('should validate dockerfile format', async () => {
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [{ role: 'user', content: [{ type: 'text', text: 'Test' }] }],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      // Invalid Dockerfile without FROM
      const mockResponse: SamplingResponse = {
        role: 'assistant',
        content: [{ type: 'text', text: 'WORKDIR /app\nCOPY . .' }],
      };
      mockContext.sampling.createMessage.mockResolvedValue(mockResponse);
      
      const result = await aiGenerate(mockLogger, mockContext, {
        promptName: 'test',
        promptArgs: {},
        expectation: 'dockerfile',
        fallbackBehavior: 'error',
      });
      
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Invalid Dockerfile');
      }
    });

    it('should validate JSON format', async () => {
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [{ role: 'user', content: [{ type: 'text', text: 'Test' }] }],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      // Valid JSON
      const mockResponse: SamplingResponse = {
        role: 'assistant',
        content: [{ type: 'text', text: '{"key": "value", "number": 42}' }],
      };
      mockContext.sampling.createMessage.mockResolvedValue(mockResponse);
      
      const result = await aiGenerate(mockLogger, mockContext, {
        promptName: 'test',
        promptArgs: {},
        expectation: 'json',
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        const parsed = JSON.parse(result.value.content);
        expect(parsed.key).toBe('value');
        expect(parsed.number).toBe(42);
      }
    });

    it('should validate YAML format', async () => {
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [{ role: 'user', content: [{ type: 'text', text: 'Test' }] }],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      // Valid YAML
      const mockResponse: SamplingResponse = {
        role: 'assistant',
        content: [{ type: 'text', text: 'apiVersion: v1\nkind: Service\nmetadata:\n  name: test' }],
      };
      mockContext.sampling.createMessage.mockResolvedValue(mockResponse);
      
      const result = await aiGenerate(mockLogger, mockContext, {
        promptName: 'test',
        promptArgs: {},
        expectation: 'yaml',
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.content).toContain('apiVersion');
        expect(result.value.content).toContain('kind: Service');
      }
    });

    it('should retry on failure with exponential backoff', async () => {
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [{ role: 'user', content: [{ type: 'text', text: 'Test' }] }],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      // First two calls fail, third succeeds
      mockContext.sampling.createMessage
        .mockRejectedValueOnce(new Error('Network error'))
        .mockRejectedValueOnce(new Error('Timeout'))
        .mockResolvedValueOnce({
          role: 'assistant',
          content: [{ type: 'text', text: 'Success content' }],
        });
      
      const result = await aiGenerate(mockLogger, mockContext, {
        promptName: 'test',
        promptArgs: {},
        fallbackBehavior: 'retry',
        maxRetries: 3,
        retryDelay: 10, // Small delay for testing
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.content).toBe('Success content');
      }
      
      expect(mockContext.sampling.createMessage).toHaveBeenCalledTimes(3);
      expect(mockLogger.error).toHaveBeenCalledTimes(2);
    });

    it('should use model hints when provided', async () => {
      const mockPrompt: PromptWithMessages = {
        description: 'Test prompt',
        messages: [{ role: 'user', content: [{ type: 'text', text: 'Test' }] }],
      };
      mockContext.getPrompt.mockResolvedValue(mockPrompt);
      
      mockContext.sampling.createMessage.mockResolvedValue({
        role: 'assistant',
        content: [{ type: 'text', text: 'Response' }],
      });
      
      await aiGenerate(mockLogger, mockContext, {
        promptName: 'test',
        promptArgs: {},
        modelHints: ['claude-3-opus', 'claude-3-sonnet'],
      });
      
      const call = mockContext.sampling.createMessage.mock.calls[0][0];
      expect(call.modelPreferences).toEqual({
        hints: [
          { name: 'claude-3-opus' },
          { name: 'claude-3-sonnet' },
        ],
      });
    });
  });

  describe('withAIFallback', () => {
    it('should return operation result when successful', async () => {
      const operation = jest.fn().mockResolvedValue(Success('operation result'));
      const fallback = jest.fn().mockReturnValue('fallback result');
      
      const result = await withAIFallback(operation, fallback, { logger: mockLogger });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('operation result');
      }
      expect(operation).toHaveBeenCalledTimes(1);
      expect(fallback).not.toHaveBeenCalled();
    });

    it('should use fallback when operation fails', async () => {
      const operation = jest.fn().mockResolvedValue(Failure('operation failed'));
      const fallback = jest.fn().mockReturnValue('fallback result');
      
      const result = await withAIFallback(operation, fallback, { 
        logger: mockLogger,
        maxRetries: 1,
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('fallback result');
      }
      expect(operation).toHaveBeenCalledTimes(1);
      expect(fallback).toHaveBeenCalledTimes(1);
      expect(mockLogger.info).toHaveBeenCalledWith(
        expect.objectContaining({ lastError: 'operation failed' }),
        'Using fallback after operation failure'
      );
    });

    it('should retry operation before using fallback', async () => {
      const operation = jest.fn()
        .mockResolvedValueOnce(Failure('first failure'))
        .mockResolvedValueOnce(Success('retry success'));
      const fallback = jest.fn().mockReturnValue('fallback result');
      
      const result = await withAIFallback(operation, fallback, { 
        logger: mockLogger,
        maxRetries: 2,
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('retry success');
      }
      expect(operation).toHaveBeenCalledTimes(2);
      expect(fallback).not.toHaveBeenCalled();
    });

    it('should handle fallback failure', async () => {
      const operation = jest.fn().mockResolvedValue(Failure('operation failed'));
      const fallback = jest.fn().mockRejectedValue(new Error('fallback failed'));
      
      const result = await withAIFallback(operation, fallback, { 
        logger: mockLogger,
        maxRetries: 1,
      });
      
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Both operation and fallback failed');
        expect(result.error).toContain('operation failed');
        expect(result.error).toContain('fallback failed');
      }
    });

    it('should support async fallback functions', async () => {
      const operation = jest.fn().mockResolvedValue(Failure('operation failed'));
      const fallback = jest.fn().mockResolvedValue('async fallback result');
      
      const result = await withAIFallback(operation, fallback, { 
        logger: mockLogger,
        maxRetries: 1,
      });
      
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toBe('async fallback result');
      }
    });
  });

  describe('structureError', () => {
    it('should format error with context', () => {
      const error = new Error('Test error');
      const context = { phase: 'validation', attempt: 2 };
      
      const message = structureError(error, context);
      
      expect(message).toContain('Error: Test error');
      expect(message).toContain('phase="validation"');
      expect(message).toContain('attempt=2');
    });

    it('should handle string errors', () => {
      const message = structureError('Simple error', { code: 'E001' });
      
      expect(message).toBe('Simple error [code="E001"]');
    });

    it('should handle no context', () => {
      const error = new Error('Test error');
      const message = structureError(error);
      
      expect(message).toBe('Error: Test error');
    });
  });

  describe('aiError', () => {
    it('should create structured failure for prompt phase', () => {
      const result = aiError('prompt', new Error('Prompt not found'), { 
        promptName: 'test-prompt' 
      });
      
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('AI prompt error');
        expect(result.error).toContain('Prompt not found');
        expect(result.error).toContain('promptName="test-prompt"');
        expect(result.error).toContain('phase="prompt"');
      }
    });

    it('should create structured failure for validation phase', () => {
      const result = aiError('validation', 'Invalid JSON format', { 
        expectation: 'json',
        contentLength: 100 
      });
      
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('AI validation error');
        expect(result.error).toContain('Invalid JSON format');
        expect(result.error).toContain('expectation="json"');
        expect(result.error).toContain('contentLength=100');
      }
    });
  });
});