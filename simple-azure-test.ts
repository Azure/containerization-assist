#!/usr/bin/env node
import { ChatClient } from './test/llm-integration/infrastructure/chat-client.js';

async function simpleTest() {
  const client = new ChatClient();

  console.log('Testing simple message...');
  const response = await client.sendMessage([
    { role: 'user', content: 'What is Docker?' }
  ], { maxTokens: 100 });

  console.log('Response:', response.content);
  console.log('Length:', response.content?.length);
  console.log('Usage:', response.usage);
}

simpleTest().catch(console.error);