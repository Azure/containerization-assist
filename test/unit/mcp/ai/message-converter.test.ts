/**
 * Unit tests for the MCP Message Converter
 */

import { describe, it, expect } from '@jest/globals';
import {
  toMCPMessages,
  fromMCPMessages,
  validateMCPMessages,
  type MCPMessages,
} from '@/mcp/ai/message-converter';
import type { AIMessages } from '@/types';

describe('MCP Message Converter', () => {
  describe('toMCPMessages', () => {
    it('should combine system and developer messages into first user message', () => {
      const aiMessages: AIMessages = {
        messages: [
          {
            role: 'system',
            content: [{ type: 'text', text: 'System policy constraints' }],
          },
          {
            role: 'developer',
            content: [{ type: 'text', text: 'Output as JSON' }],
          },
          {
            role: 'user',
            content: [{ type: 'text', text: 'User prompt' }],
          },
        ],
      };

      const result = toMCPMessages(aiMessages);

      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].role).toBe('user');
      expect(result.messages[0].content[0].text).toBe(
        'System policy constraints\n\nOutput as JSON\n\nUser prompt',
      );
    });

    it('should handle string content', () => {
      const aiMessages: AIMessages = {
        messages: [{ role: 'user', content: 'String content' }],
      };

      const result = toMCPMessages(aiMessages);

      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].content[0].text).toBe('String content');
    });

    it('should preserve assistant messages', () => {
      const aiMessages: AIMessages = {
        messages: [
          { role: 'user', content: [{ type: 'text', text: 'Question' }] },
          { role: 'assistant', content: [{ type: 'text', text: 'Answer' }] },
          { role: 'user', content: [{ type: 'text', text: 'Follow-up' }] },
        ],
      };

      const result = toMCPMessages(aiMessages);

      expect(result.messages).toHaveLength(3);
      expect(result.messages[0].role).toBe('user');
      expect(result.messages[1].role).toBe('assistant');
      expect(result.messages[2].role).toBe('user');
    });

    it('should handle empty system messages gracefully', () => {
      const aiMessages: AIMessages = {
        messages: [
          { role: 'system', content: '' },
          { role: 'user', content: [{ type: 'text', text: 'User prompt' }] },
        ],
      };

      const result = toMCPMessages(aiMessages);

      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].content[0].text).toBe('User prompt');
    });
  });

  describe('fromMCPMessages', () => {
    it('should convert MCP messages back to AIMessages format', () => {
      const mcpMessages: MCPMessages = {
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'User question' }],
          },
          {
            role: 'assistant',
            content: [{ type: 'text', text: 'Assistant response' }],
          },
        ],
      };

      const result = fromMCPMessages(mcpMessages);

      expect(result.messages).toHaveLength(2);
      expect(result.messages[0].role).toBe('user');
      expect(result.messages[0].content).toEqual([{ type: 'text', text: 'User question' }]);
      expect(result.messages[1].role).toBe('assistant');
      expect(result.messages[1].content).toEqual([{ type: 'text', text: 'Assistant response' }]);
    });
  });

  describe('validateMCPMessages', () => {
    it('should validate correct MCP messages', () => {
      const validMessages: MCPMessages = {
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'Valid message' }],
          },
        ],
      };

      expect(validateMCPMessages(validMessages)).toBe(true);
    });

    it('should reject messages without user role', () => {
      const invalidMessages: MCPMessages = {
        messages: [
          {
            role: 'assistant',
            content: [{ type: 'text', text: 'Only assistant' }],
          },
        ],
      };

      expect(validateMCPMessages(invalidMessages)).toBe(false);
    });

    it('should reject messages with invalid roles', () => {
      const invalidMessages = {
        messages: [
          {
            role: 'system' as any,
            content: [{ type: 'text', text: 'Invalid role' }],
          },
        ],
      } as MCPMessages;

      expect(validateMCPMessages(invalidMessages)).toBe(false);
    });

    it('should reject messages with empty content', () => {
      const invalidMessages: MCPMessages = {
        messages: [
          {
            role: 'user',
            content: [],
          },
        ],
      };

      expect(validateMCPMessages(invalidMessages)).toBe(false);
    });

    it('should reject messages with invalid content type', () => {
      const invalidMessages = {
        messages: [
          {
            role: 'user',
            content: [{ type: 'image' as any, text: 'Wrong type' }],
          },
        ],
      } as MCPMessages;

      expect(validateMCPMessages(invalidMessages)).toBe(false);
    });

    it('should reject messages with empty text', () => {
      const invalidMessages: MCPMessages = {
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: '   ' }],
          },
        ],
      };

      expect(validateMCPMessages(invalidMessages)).toBe(false);
    });

    it('should accept valid multi-message conversations', () => {
      const validMessages: MCPMessages = {
        messages: [
          {
            role: 'user',
            content: [{ type: 'text', text: 'First question' }],
          },
          {
            role: 'assistant',
            content: [{ type: 'text', text: 'First answer' }],
          },
          {
            role: 'user',
            content: [{ type: 'text', text: 'Follow-up' }],
          },
        ],
      };

      expect(validateMCPMessages(validMessages)).toBe(true);
    });
  });
});