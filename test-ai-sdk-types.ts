// Test AI SDK type requirements
import { generateText } from 'ai';
import { createOpenAI } from '@ai-sdk/openai';

const openai = createOpenAI({
  baseURL: 'http://localhost:4141/v1',
  apiKey: 'dummy',
});

// Test correct message format
async function testMessages() {
  // Simple messages
  const simpleMessages = [
    { role: 'user' as const, content: 'Hello' },
  ];

  // Tool result message
  const toolMessages = [
    { role: 'user' as const, content: 'Hello' },
    {
      role: 'assistant' as const,
      content: 'I need to use a tool',
      toolInvocations: [
        {
          state: 'call' as const,
          toolCallId: 'call_123',
          toolName: 'test-tool',
          args: { message: 'test' },
        }
      ]
    },
    {
      role: 'tool' as const,
      content: [
        {
          type: 'tool-result' as const,
          toolCallId: 'call_123',
          toolName: 'test-tool',
          result: 'Tool executed successfully', // Try different formats
        }
      ],
    }
  ];

  console.log('Message formats appear valid');
  return { simpleMessages, toolMessages };
}

testMessages().catch(console.error);