#!/usr/bin/env node
/**
 * Test containerization with Azure GPT-5 using AI SDK
 */

import { ChatClient } from './test/llm-integration/infrastructure/chat-client.js';

async function testContainerizationWithAzure() {
  console.log('🧪 Testing containerization with Azure GPT-5 via AI SDK...\n');

  const client = new ChatClient();

  try {
    console.log('1️⃣ Testing AI SDK connection with Azure...');
    const isValid = await client.validateConnection();
    console.log(`   ✅ Connection: ${isValid ? 'SUCCESS' : 'FAILED'}\n`);

    if (isValid) {
      console.log('2️⃣ Testing containerization request...');
      const response = await client.sendMessage([
        {
          role: 'user',
          content: 'Create a Dockerfile for a Node.js Express application with the following requirements: production-ready, multi-stage build, and security best practices.'
        }
      ], { maxTokens: 1000 });

      console.log('📄 Azure GPT-5 Response:');
      console.log('─'.repeat(80));
      console.log(response.content);
      console.log('─'.repeat(80));
      console.log(`\n📊 Usage: ${response.usage?.totalTokens} tokens`);
      console.log(`⏱️  Latency: ${response.metadata?.latency}ms`);
      console.log(`🤖 Model: ${response.metadata?.model}`);
    }
  } catch (error) {
    console.error('❌ Test failed:', error);
  }
}

testContainerizationWithAzure().catch(console.error);