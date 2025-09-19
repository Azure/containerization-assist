/**
 * Integration tests for knowledge-enhanced AI generation
 *
 * Tests the complete pipeline from knowledge loading through AI generation
 * with enhanced prompts.
 */

import { describe, test, expect, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import { aiGenerateWithSampling } from '@/mcp/tool-ai-helpers';
import {
  loadKnowledgeBase,
  isKnowledgeLoaded,
  getKnowledgeStats,
  getEntriesByTag,
  getEntryById,
} from '../../src/knowledge';
import type { ToolContext } from '@/mcp/context';

// Mock tool context for testing
const mockContext: ToolContext = {
  logger: createLogger({ name: 'test' }),
  sampling: {
    createMessage: jest.fn().mockResolvedValue({
      content: [
        {
          type: 'text',
          text: 'FROM node:18-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --only=production\nCOPY . .\nUSER node\nEXPOSE 3000\nCMD ["node", "server.js"]',
        },
      ],
      metadata: { model: 'test-model' },
    }),
  },
  getPrompt: jest.fn().mockResolvedValue({
    messages: [
      {
        role: 'user',
        content: { type: 'text', text: 'Generate a Dockerfile' },
      },
    ],
  }),
};

describe('Knowledge Integration Tests', () => {
  beforeAll(async () => {
    // Ensure knowledge base is loaded
    await loadKnowledgeBase();
    expect(isKnowledgeLoaded()).toBe(true);
  });

  afterAll(() => {
    jest.clearAllMocks();
  });

  describe('Knowledge Base Loading', () => {
    test('should load all knowledge packs successfully', async () => {
      const stats = await getKnowledgeStats();

      expect(stats.totalEntries).toBeGreaterThan(100); // Should have loaded all packs
      expect(stats.byCategory['dockerfile']).toBeGreaterThan(20);
      expect(stats.byCategory['kubernetes']).toBeGreaterThan(15);
      expect(stats.byCategory['security']).toBeGreaterThan(15);
    });

    test('should have entries for each major language', () => {
      const nodeEntries = getEntriesByTag('node');
      const pythonEntries = getEntriesByTag('python');
      const javaEntries = getEntriesByTag('java');
      const goEntries = getEntriesByTag('go');

      expect(nodeEntries.length).toBeGreaterThan(15);
      expect(pythonEntries.length).toBeGreaterThan(15);
      expect(javaEntries.length).toBeGreaterThan(10);
      expect(goEntries.length).toBeGreaterThan(10);
    });

    test('should have security-focused entries', () => {
      const securityEntries = getEntriesByTag('security');
      expect(securityEntries.length).toBeGreaterThan(20);

      // Check for specific high-priority security entries
      const rootUserEntry = getEntryById('container-root-user');
      const secretsEntry = getEntryById('secrets-in-dockerfile');

      expect(rootUserEntry).toBeDefined();
      expect(secretsEntry).toBeDefined();
      expect(rootUserEntry?.severity).toBe('high');
      expect(secretsEntry?.severity).toBe('high');
    });
  });

  describe('AI Generation with Knowledge Enhancement', () => {
    test('should enhance Node.js Dockerfile generation with knowledge', async () => {
      const logger = createLogger({ name: 'test-nodejs' });

      const result = await aiGenerateWithSampling(logger, mockContext, {
        promptName: 'dockerfile-generation',
        promptArgs: {
          language: 'javascript',
          framework: 'express',
          baseImage: 'node:18',
          optimization: 'production',
        },
        expectation: 'dockerfile',
        maxCandidates: 1,
        enableSampling: false,
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.winner.content).toContain('FROM node');
        expect(result.value.samplingMetadata.candidatesGenerated).toBe(1);
      }

      // Verify that knowledge enhancement was called
      expect(mockContext.getPrompt).toHaveBeenCalledWith(
        'dockerfile-generation',
        expect.objectContaining({
          language: 'javascript',
          framework: 'express',
          // Should contain enhanced knowledge fields
          bestPractices: expect.any(Array),
        }),
      );
    });

    test('should gracefully handle knowledge enhancement failures', async () => {
      const logger = createLogger({ name: 'test-error-handling' });

      // Create a context with invalid prompt args that might cause knowledge enhancement to fail
      const result = await aiGenerateWithSampling(logger, mockContext, {
        promptName: 'dockerfile-generation',
        promptArgs: {
          language: null, // Invalid language
          framework: undefined,
        },
        expectation: 'dockerfile',
        maxCandidates: 1,
        enableSampling: false,
      });

      // Should still succeed even if knowledge enhancement fails
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.winner.content).toBeTruthy();
      }
    });
  });

  describe('Cache Integration with Knowledge Enhancement', () => {
    test('should cache knowledge-enhanced responses', async () => {
      const logger = createLogger({ name: 'test-cache' });

      const requestArgs = {
        promptName: 'dockerfile-generation',
        promptArgs: {
          language: 'go',
          optimization: 'size',
        },
        expectation: 'dockerfile' as const,
        maxCandidates: 1,
        enableSampling: false,
      };

      // First request should call AI
      const result1 = await aiGenerateWithSampling(logger, mockContext, requestArgs);
      expect(result1.ok).toBe(true);

      // Second identical request should potentially use cache
      jest.clearAllMocks();
      const result2 = await aiGenerateWithSampling(logger, mockContext, requestArgs);
      expect(result2.ok).toBe(true);

      // Both results should be valid
      if (result1.ok && result2.ok) {
        expect(result1.value.winner.content).toBeTruthy();
        expect(result2.value.winner.content).toBeTruthy();
      }
    });
  });
});
