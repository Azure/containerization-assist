// Test correct AI SDK imports
import { generateText, streamText, tool } from 'ai';
import { createAzure } from '@ai-sdk/azure';
import { createOpenAI } from '@ai-sdk/openai';

// Check available types
console.log('Imports successful');
console.log('generateText:', typeof generateText);
console.log('createAzure:', typeof createAzure);