#!/usr/bin/env node
import { ChatClient } from './test/llm-integration/infrastructure/chat-client.js';

async function testAzureAISDK() {
  console.log('Testing AI SDK with Azure GPT-5...');

  const client = new ChatClient();

  try {
    console.log('1. Testing connection validation...');
    const isValid = await client.validateConnection();
    console.log(`Connection validation: ${isValid ? 'SUCCESS' : 'FAILED'}`);

    if (isValid) {
      console.log('\n2. Testing simple message...');
      const response = await client.sendMessage([
        { role: 'user', content: 'Say "Hello from Azure GPT-5 via AI SDK!"' }
      ]);

      console.log('Response:', response.content);
      console.log('Usage:', response.usage);
      console.log('Metadata:', response.metadata);
    }
  } catch (error) {
    console.error('Test failed:', error);
  }
}

testAzureAISDK().catch(console.error);