/**
 * Unit tests for the AI Prompt Engine
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { buildMessages, buildPromptEnvelope, estimateMessageSize, validateMessages } from '@/ai/prompt-engine';
import type { BuildPromptParams, AIMessages, OutputContract } from '@/types';
import * as knowledgeMatcher from '@/knowledge/matcher';
import * as policyPrompt from '@/config/policy-prompt';

// Mock dependencies
jest.mock('@/knowledge/matcher');
jest.mock('@/config/policy-prompt');

const mockedKnowledgeMatcher = jest.mocked(knowledgeMatcher);
const mockedPolicyPrompt = jest.mocked(policyPrompt);

describe('Prompt Engine', () => {
  beforeEach(() => {
    jest.clearAllMocks();

    // Default mock implementations
    mockedKnowledgeMatcher.getKnowledgeSnippets.mockResolvedValue([]);
    mockedPolicyPrompt.buildPolicyConstraints.mockReturnValue([]);
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('buildMessages', () => {
    const baseParams: BuildPromptParams = {
      basePrompt: 'Generate a Dockerfile for a Node.js application',
      topic: 'generate_dockerfile',
      tool: 'generate-dockerfile',
      environment: 'production',
    };

    it('should build messages with all three roles when policies and knowledge exist', async () => {
      // Mock policy constraints
      mockedPolicyPrompt.buildPolicyConstraints.mockReturnValue([
        'Use only approved base images',
        'Include security scanning',
      ]);

      // Mock knowledge snippets
      mockedKnowledgeMatcher.getKnowledgeSnippets.mockResolvedValue([
        {
          id: 'snippet-1',
          text: 'Use multi-stage builds for smaller images',
          weight: 10,
          source: 'best-practices',
        },
        {
          id: 'snippet-2',
          text: 'Pin package versions for reproducibility',
          weight: 8,
          source: 'security',
        },
      ]);

      const params: BuildPromptParams = {
        ...baseParams,
        contract: {
          name: 'dockerfile_v1',
          description: 'Generate optimized Dockerfile',
        },
        knowledgeBudget: 5000,
      };

      const result = await buildMessages(params);

      expect(result.messages).toHaveLength(3);

      // Check system message
      const systemMessage = result.messages[0];
      expect(systemMessage.role).toBe('system');
      expect(systemMessage.content[0].text).toContain('You must follow these organizational policies');
      expect(systemMessage.content[0].text).toContain('Use only approved base images');
      expect(systemMessage.content[0].text).toContain('Include security scanning');

      // Check developer message
      const developerMessage = result.messages[1];
      expect(developerMessage.role).toBe('developer');
      expect(developerMessage.content[0].text).toContain('Output strictly as JSON matching schema "dockerfile_v1"');
      expect(developerMessage.content[0].text).toContain('Generate optimized Dockerfile');

      // Check user message
      const userMessage = result.messages[2];
      expect(userMessage.role).toBe('user');
      expect(userMessage.content[0].text).toContain('Generate a Dockerfile for a Node.js application');
      expect(userMessage.content[0].text).toContain('Relevant knowledge:');
      expect(userMessage.content[0].text).toContain('[best-practices] Use multi-stage builds');
      expect(userMessage.content[0].text).toContain('[security] Pin package versions');
    });

    it('should omit system message when no policies exist', async () => {
      mockedPolicyPrompt.buildPolicyConstraints.mockReturnValue([]);

      const result = await buildMessages(baseParams);

      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].role).toBe('user');
    });

    it('should omit developer message when no contract exists', async () => {
      const result = await buildMessages(baseParams);

      const hasDeveloperMessage = result.messages.some(m => m.role === 'developer');
      expect(hasDeveloperMessage).toBe(false);
    });

    it('should handle knowledge budget correctly', async () => {
      const params = { ...baseParams, knowledgeBudget: 1500 };

      await buildMessages(params);

      expect(mockedKnowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        'generate_dockerfile',
        expect.objectContaining({
          environment: 'production',
          tool: 'generate-dockerfile',
          maxChars: 1500,
        })
      );
    });

    it('should handle knowledge retrieval failure gracefully', async () => {
      mockedKnowledgeMatcher.getKnowledgeSnippets.mockRejectedValue(new Error('Knowledge load failed'));

      const result = await buildMessages(baseParams);

      // Should still build messages without knowledge
      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].role).toBe('user');
      expect(result.messages[0].content[0].text).not.toContain('Relevant knowledge:');
    });

    it('should handle policy loading failure gracefully', async () => {
      mockedPolicyPrompt.buildPolicyConstraints.mockImplementation(() => {
        throw new Error('Policy load failed');
      });

      const result = await buildMessages(baseParams);

      // Should still build messages without policies
      expect(result.messages).toHaveLength(1);
      expect(result.messages[0].role).toBe('user');
    });

    it('should force roles when options specify', async () => {
      const result = await buildMessages(baseParams, {
        forceSystemRole: true,
        forceDeveloperRole: true,
      });

      const systemMessage = result.messages.find(m => m.role === 'system');
      const developerMessage = result.messages.find(m => m.role === 'developer');

      expect(systemMessage).toBeDefined();
      expect(developerMessage).toBeDefined();
    });

    it('should truncate messages that exceed maxLength', async () => {
      const longPrompt = 'A'.repeat(150); // 150 chars
      const params: BuildPromptParams = {
        ...baseParams,
        basePrompt: longPrompt,
      };

      const result = await buildMessages(params, { maxLength: 100 });

      const userMessage = result.messages[0];
      expect(userMessage.content[0].text).toContain('[truncated]');
      expect(userMessage.content[0].text.length).toBeLessThanOrEqual(112); // 100 + '\n[truncated]'
    });

    it('should not truncate messages within maxLength', async () => {
      const shortPrompt = 'Short prompt';
      const params: BuildPromptParams = {
        ...baseParams,
        basePrompt: shortPrompt,
      };

      const result = await buildMessages(params, { maxLength: 1000 });

      const userMessage = result.messages[0];
      expect(userMessage.content[0].text).not.toContain('[truncated]');
      expect(userMessage.content[0].text).toContain('Short prompt');
    });

    it('should use default maxLength of 10000 when not specified', async () => {
      const longPrompt = 'B'.repeat(10005); // Slightly over default
      const params: BuildPromptParams = {
        ...baseParams,
        basePrompt: longPrompt,
      };

      const result = await buildMessages(params);

      const userMessage = result.messages[0];
      expect(userMessage.content[0].text).toContain('[truncated]');
    });

    it('should handle truncation with knowledge text', async () => {
      // Mock a long knowledge snippet
      mockedKnowledgeMatcher.getKnowledgeSnippets.mockResolvedValue([
        {
          id: 'long-snippet',
          text: 'K'.repeat(80), // 80 chars of knowledge
          weight: 10,
          source: 'test',
        },
      ]);

      const params: BuildPromptParams = {
        ...baseParams,
        basePrompt: 'P'.repeat(50), // 50 chars prompt
      };

      // Combined should be > 100 chars
      const result = await buildMessages(params, { maxLength: 100 });

      const userMessage = result.messages[0];
      expect(userMessage.content[0].text).toContain('[truncated]');
      expect(userMessage.content[0].text.length).toBeLessThanOrEqual(112); // 100 + '\n[truncated]'
    });
  });

  describe('buildPromptEnvelope', () => {
    const baseParams: BuildPromptParams = {
      basePrompt: 'Test prompt',
      topic: 'test_topic',
      tool: 'test-tool',
      environment: 'development',
    };

    it('should build envelope with metadata', async () => {
      mockedPolicyPrompt.buildPolicyConstraints.mockReturnValue([
        'Policy 1',
        'Policy 2',
      ]);

      mockedKnowledgeMatcher.getKnowledgeSnippets.mockResolvedValue([
        { id: '1', text: 'Knowledge 1', weight: 10 },
        { id: '2', text: 'Knowledge 2', weight: 5 },
      ]);

      const result = await buildPromptEnvelope(baseParams);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const envelope = result.value;

        expect(envelope.system).toContain('You must follow these organizational policies');
        expect(envelope.user).toContain('Test prompt');
        expect(envelope.metadata).toEqual({
          tool: 'test-tool',
          environment: 'development',
          topic: 'test_topic',
          knowledgeCount: 2,
          policyCount: 2,
        });
      }
    });

    it('should handle knowledge retrieval errors gracefully', async () => {
      // Knowledge retrieval failures are handled gracefully
      mockedKnowledgeMatcher.getKnowledgeSnippets.mockRejectedValue(new Error('Boom!'));

      const result = await buildPromptEnvelope(baseParams);

      // Should still succeed with empty knowledge
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.user).toContain('Test prompt');
        expect(result.value.metadata?.knowledgeCount).toBe(0);
      }
    });
  });


  describe('estimateMessageSize', () => {
    it('should calculate total character count for string content', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'user', content: 'Hello world' },
          { role: 'assistant', content: 'Hi there' },
        ],
      };

      const size = estimateMessageSize(messages);
      expect(size).toBe(19); // "Hello world" (11) + "Hi there" (8)
    });

    it('should calculate total character count for array content', () => {
      const messages: AIMessages = {
        messages: [
          {
            role: 'user',
            content: [
              { type: 'text', text: 'First part' },
              { type: 'text', text: 'Second part' },
            ],
          },
        ],
      };

      const size = estimateMessageSize(messages);
      expect(size).toBe(21); // "First part" (10) + "Second part" (11)
    });

    it('should handle mixed content types', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'user', content: 'String content' },
          {
            role: 'assistant',
            content: [{ type: 'text', text: 'Array content' }],
          },
        ],
      };

      const size = estimateMessageSize(messages);
      expect(size).toBe(27); // "String content" (14) + "Array content" (13)
    });
  });

  describe('validateMessages', () => {
    it('should validate valid messages', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'user', content: [{ type: 'text', text: 'Valid content' }] },
        ],
      };

      const result = validateMessages(messages);
      expect(result.ok).toBe(true);
    });

    it('should reject empty messages array', () => {
      const messages: AIMessages = { messages: [] };

      const result = validateMessages(messages);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No messages provided');
      }
    });

    it('should reject messages without user role', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'system', content: 'System only' },
        ],
      };

      const result = validateMessages(messages);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('at least one user message');
      }
    });

    it('should reject messages with empty content', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'user', content: '' },
        ],
      };

      const result = validateMessages(messages);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Empty content');
      }
    });

    it('should reject messages with empty content arrays', () => {
      const messages: AIMessages = {
        messages: [
          { role: 'user', content: [] },
        ],
      };

      const result = validateMessages(messages);
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Empty content');
      }
    });
  });
});