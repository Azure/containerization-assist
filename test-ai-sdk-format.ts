// Test AI SDK correct format
import { generateText, tool } from 'ai';
import { createOpenAI } from '@ai-sdk/openai';
import { z } from 'zod';

const openai = createOpenAI({
  baseURL: 'http://localhost:4141/v1',
  apiKey: 'dummy',
});

// Test correct tool format
const testTool = tool({
  description: 'Test tool',
  parameters: z.object({
    message: z.string().describe('A test message'),
  }),
  execute: async ({ message }) => `Tool executed with: ${message}`,
});

console.log('Tool definition structure:', typeof testTool);

// Test message format
const messages = [
  { role: 'user' as const, content: 'Hello' },
  { role: 'assistant' as const, content: 'Hi there!' },
];

console.log('Message format:', messages);
console.log('Tool usage format should use z.object for parameters');