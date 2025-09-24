/**
 * MCP Message Converter - Utilities for converting between AI message formats and MCP protocol formats
 */

import type { AIMessages, AIMessage } from '@/types';

/**
 * MCP-compatible message format
 */
export interface MCPMessage {
  role: 'user' | 'assistant';
  content: Array<{ type: 'text'; text: string }>;
  /** Allow additional properties for MCP compatibility */
  [key: string]: unknown;
}

/**
 * MCP messages container
 */
export interface MCPMessages {
  messages: MCPMessage[];
}

/**
 * Converts AIMessages to MCP-compatible TextMessage format.
 *
 * MCP protocol only supports 'user' and 'assistant' roles, so we need to:
 * - Combine system and developer messages into the first user message
 * - Ensure all messages have the proper content array format
 *
 * @param aiMessages - Messages from the prompt engine
 * @returns MCP-compatible TextMessage array
 */
export function toMCPMessages(aiMessages: AIMessages): MCPMessages {
  const messages = aiMessages.messages;
  const mcpMessages: MCPMessage[] = [];

  // Collect system and developer messages
  const systemMessages: string[] = [];

  for (const message of messages) {
    if (message.role === 'system' || message.role === 'developer') {
      const text =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';
      if (text) {
        systemMessages.push(text);
      }
    } else if (message.role === 'user') {
      // Combine system messages with the first user message
      let userText =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';

      if (systemMessages.length > 0) {
        userText = `${systemMessages.join('\n\n')}\n\n${userText}`;
        systemMessages.length = 0; // Clear after using
      }

      mcpMessages.push({
        role: 'user',
        content: [{ type: 'text', text: userText }],
      });
    } else if (message.role === 'assistant') {
      const text =
        typeof message.content === 'string' ? message.content : message.content[0]?.text || '';

      mcpMessages.push({
        role: 'assistant',
        content: [{ type: 'text', text }],
      });
    }
  }

  return { messages: mcpMessages };
}

/**
 * Converts MCP messages back to AIMessages format
 *
 * @param mcpMessages - MCP-formatted messages
 * @returns AI messages in internal format
 */
export function fromMCPMessages(mcpMessages: MCPMessages): AIMessages {
  const messages: AIMessage[] = [];

  for (const mcpMessage of mcpMessages.messages) {
    messages.push({
      role: mcpMessage.role,
      content: mcpMessage.content,
    });
  }

  return { messages };
}

/**
 * Validates that MCP messages meet protocol requirements
 *
 * @param messages - Messages to validate
 * @returns True if messages are valid for MCP protocol
 */
export function validateMCPMessages(messages: MCPMessages): boolean {
  if (!messages.messages || messages.messages.length === 0) {
    return false;
  }

  // Must have at least one user message
  const hasUserMessage = messages.messages.some((m) => m.role === 'user');
  if (!hasUserMessage) {
    return false;
  }

  // Validate each message structure
  for (const message of messages.messages) {
    // Check role is valid
    if (message.role !== 'user' && message.role !== 'assistant') {
      return false;
    }

    // Check content structure
    if (!Array.isArray(message.content) || message.content.length === 0) {
      return false;
    }

    // Check each content block
    for (const block of message.content) {
      if (block.type !== 'text' || typeof block.text !== 'string' || block.text.trim() === '') {
        return false;
      }
    }
  }

  return true;
}
