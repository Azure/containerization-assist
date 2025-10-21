// Test AI SDK usage object structure
import { generateText } from 'ai';
import { createOpenAI } from '@ai-sdk/openai';

const openai = createOpenAI({
  baseURL: 'http://localhost:4141/v1',
  apiKey: 'dummy',
});

async function testUsage() {
  try {
    const result = await generateText({
      model: openai('gpt-4o'),
      prompt: 'Say hi',
      maxOutputTokens: 50,
    });

    console.log('Usage object structure:');
    console.log(JSON.stringify(result.usage, null, 2));
    console.log('Usage properties:');
    console.log(Object.keys(result.usage || {}));
  } catch (error) {
    console.error('Error:', error.message);
  }
}

testUsage();