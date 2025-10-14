/**
 * Unit Tests: Knowledge Tool Pattern
 * Tests for the reusable knowledge-based tool pattern
 */

import { jest } from '@jest/globals';
import {
  createKnowledgeTool,
  createSimpleCategorizer,
  defaultConfidenceCalculation,
  type KnowledgeToolConfig,
} from '../../../src/tools/shared/knowledge-tool-pattern';
import type { ToolContext } from '../../../src/mcp/context';
import type { KnowledgeSnippet } from '../../../src/knowledge/schemas';
import { TOPICS } from '../../../src/types';
import { CATEGORY } from '../../../src/knowledge/types';
import * as knowledgeMatcher from '../../../src/knowledge/matcher';

// Mock the knowledge matcher module
jest.spyOn(knowledgeMatcher, 'getKnowledgeSnippets').mockImplementation(jest.fn());

describe('Knowledge Tool Pattern', () => {
  // Helper to create mock context
  function createMockContext(): ToolContext {
    return {
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
        fatal: jest.fn(),
        child: jest.fn().mockReturnThis(),
      },
      sampling: {} as any,
    } as unknown as ToolContext;
  }

  // Helper to create mock snippets
  function createMockSnippets(): KnowledgeSnippet[] {
    return [
      {
        id: 'snippet-1',
        text: 'Use non-root user for security',
        weight: 85,
        tags: ['security', 'user'],
        category: 'security',
        source: 'dockerfile-security',
      },
      {
        id: 'snippet-2',
        text: 'Use layer caching for faster builds',
        weight: 70,
        tags: ['optimization', 'caching'],
        category: 'optimization',
        source: 'dockerfile-optimization',
      },
      {
        id: 'snippet-3',
        text: 'Always use COPY instead of ADD',
        weight: 60,
        tags: ['best-practice'],
        category: 'generic',
        source: 'dockerfile-best-practices',
      },
      {
        id: 'snippet-4',
        text: 'Set appropriate resource limits',
        weight: 75,
        tags: ['security', 'resources'],
        category: 'security',
        source: 'dockerfile-security',
      },
    ];
  }

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('defaultConfidenceCalculation', () => {
    it('should return 0.5 for zero matches', () => {
      expect(defaultConfidenceCalculation(0)).toBe(0.5);
    });

    it('should increase confidence with match count', () => {
      expect(defaultConfidenceCalculation(1)).toBe(0.55);
      expect(defaultConfidenceCalculation(5)).toBe(0.75);
      expect(defaultConfidenceCalculation(9)).toBe(0.95);
    });

    it('should cap confidence at 0.95', () => {
      expect(defaultConfidenceCalculation(10)).toBe(0.95);
      expect(defaultConfidenceCalculation(20)).toBe(0.95);
      expect(defaultConfidenceCalculation(100)).toBe(0.95);
    });
  });

  describe('createSimpleCategorizer', () => {
    it('should categorize snippets based on predicates', () => {
      const categorize = createSimpleCategorizer({
        security: (s) => s.category === 'security' || s.tags?.includes('security'),
        optimization: (s) => s.tags?.includes('optimization') || s.tags?.includes('caching'),
        bestPractices: (s) => s.tags?.includes('best-practice'),
      });

      const snippet: KnowledgeSnippet = {
        id: 'test-1',
        text: 'Test snippet',
        weight: 80,
        tags: ['security', 'optimization'],
        category: 'security',
      };

      const categories = categorize(snippet);

      expect(categories).toContain('security');
      expect(categories).toContain('optimization');
      expect(categories).not.toContain('bestPractices');
    });

    it('should return empty array when no predicates match', () => {
      const categorize = createSimpleCategorizer({
        security: (s) => s.category === 'security',
      });

      const snippet: KnowledgeSnippet = {
        id: 'test-1',
        text: 'Test snippet',
        weight: 80,
        category: 'generic',
      };

      const categories = categorize(snippet);

      expect(categories).toEqual([]);
    });

    it('should handle snippets with no tags or category', () => {
      const categorize = createSimpleCategorizer({
        generic: (s) => !s.tags || s.tags.length === 0,
      });

      const snippet: KnowledgeSnippet = {
        id: 'test-1',
        text: 'Test snippet',
        weight: 80,
      };

      const categories = categorize(snippet);

      expect(categories).toContain('generic');
    });
  });

  describe('createKnowledgeTool', () => {
    interface TestInput {
      language: string;
      framework?: string;
      environment?: string;
    }

    interface TestRules {
      multistage: boolean;
      useAlpine: boolean;
    }

    interface TestPlan {
      language: string;
      recommendations: {
        security: KnowledgeSnippet[];
        optimization: KnowledgeSnippet[];
        bestPractices: KnowledgeSnippet[];
      };
      rules: TestRules;
      confidence: number;
      matchCount: number;
    }

    it('should create a working tool runner', async () => {
      const mockSnippets = createMockSnippets();
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      const config: KnowledgeToolConfig<TestInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: TOPICS.DOCKERFILE,
          category: CATEGORY.DOCKERFILE,
          maxChars: 8000,
          maxSnippets: 20,
          extractFilters: (input) => ({
            environment: input.environment || 'production',
            language: input.language,
            framework: input.framework,
          }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: createSimpleCategorizer({
            security: (s) => s.category === 'security' || s.tags?.includes('security'),
            optimization: (s) => s.tags?.includes('optimization') || s.tags?.includes('caching'),
            bestPractices: (s) => s.tags?.includes('best-practice'),
          }),
        },
        rules: {
          applyRules: (input) => ({
            multistage: ['java', 'go', 'rust'].includes(input.language),
            useAlpine: input.language === 'node',
          }),
        },
        plan: {
          buildPlan: (input, knowledge, rules, confidence) => ({
            language: input.language,
            recommendations: {
              security: knowledge.categories.security,
              optimization: knowledge.categories.optimization,
              bestPractices: knowledge.categories.bestPractices,
            },
            rules,
            confidence,
            matchCount: knowledge.all.length,
          }),
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      const result = await run(
        { language: 'node', environment: 'production' },
        mockContext,
      );

      // Verify result is successful
      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Verify plan structure
      expect(result.value.language).toBe('node');
      expect(result.value.matchCount).toBe(4);
      expect(result.value.rules.multistage).toBe(false);
      expect(result.value.rules.useAlpine).toBe(true);

      // Verify categorization
      expect(result.value.recommendations.security).toHaveLength(2); // snippets 1 and 4
      expect(result.value.recommendations.optimization).toHaveLength(1); // snippet 2
      expect(result.value.recommendations.bestPractices).toHaveLength(1); // snippet 3

      // Verify knowledge query was called correctly
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        TOPICS.DOCKERFILE,
        expect.objectContaining({
          environment: 'production',
          tool: 'test-tool',
          language: 'node',
          maxChars: 8000,
          maxSnippets: 20,
          category: CATEGORY.DOCKERFILE,
        }),
      );

      // Verify logging
      expect(mockContext.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          topic: TOPICS.DOCKERFILE,
        }),
        expect.stringContaining('Querying knowledge base'),
      );

      expect(mockContext.logger.info).toHaveBeenCalledWith(
        expect.objectContaining({
          knowledgeMatchCount: 4,
          confidence: expect.any(Number),
        }),
        expect.stringContaining('Planning completed'),
      );
    });

    it('should use default filters when not provided in input', async () => {
      const mockSnippets: KnowledgeSnippet[] = [];
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      const config: KnowledgeToolConfig<TestInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: TOPICS.DOCKERFILE,
          category: CATEGORY.DOCKERFILE,
          extractFilters: (input) => ({
            environment: input.environment || 'production',
            language: input.language,
          }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: () => [],
        },
        rules: {
          applyRules: () => ({ multistage: false, useAlpine: false }),
        },
        plan: {
          buildPlan: (input, knowledge, rules, confidence) => ({
            language: input.language,
            recommendations: {
              security: [],
              optimization: [],
              bestPractices: [],
            },
            rules,
            confidence,
            matchCount: 0,
          }),
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      await run({ language: 'python' }, mockContext);

      // Verify default environment was used
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        TOPICS.DOCKERFILE,
        expect.objectContaining({
          environment: 'production',
          language: 'python',
        }),
      );
    });

    it('should support dynamic topic selection', async () => {
      const mockSnippets: KnowledgeSnippet[] = [];
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      interface TopicInput {
        type: 'dockerfile' | 'kubernetes';
        language: string;
      }

      const config: KnowledgeToolConfig<TopicInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: (input) => input.type === 'dockerfile' ? TOPICS.DOCKERFILE : TOPICS.KUBERNETES,
          category: CATEGORY.DOCKERFILE,
          extractFilters: (input) => ({ language: input.language }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: () => [],
        },
        rules: {
          applyRules: () => ({ multistage: false, useAlpine: false }),
        },
        plan: {
          buildPlan: (input, knowledge, rules, confidence) => ({
            language: input.language,
            recommendations: {
              security: [],
              optimization: [],
              bestPractices: [],
            },
            rules,
            confidence,
            matchCount: 0,
          }),
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      await run({ type: 'kubernetes', language: 'node' }, mockContext);

      // Verify correct topic was used
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        TOPICS.KUBERNETES,
        expect.any(Object),
      );
    });

    it('should support async rules', async () => {
      const mockSnippets: KnowledgeSnippet[] = [];
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      const config: KnowledgeToolConfig<TestInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: TOPICS.DOCKERFILE,
          category: CATEGORY.DOCKERFILE,
          extractFilters: (input) => ({ language: input.language }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: () => [],
        },
        rules: {
          applyRules: async (input) => {
            // Simulate async operation
            await new Promise((resolve) => setTimeout(resolve, 10));
            return {
              multistage: input.language === 'java',
              useAlpine: input.language === 'node',
            };
          },
        },
        plan: {
          buildPlan: (input, knowledge, rules, confidence) => ({
            language: input.language,
            recommendations: {
              security: [],
              optimization: [],
              bestPractices: [],
            },
            rules,
            confidence,
            matchCount: 0,
          }),
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      const result = await run({ language: 'java' }, mockContext);

      expect(result.ok).toBe(true);
      if (!result.ok) return;
      expect(result.value.rules.multistage).toBe(true);
    });

    it('should support async plan building', async () => {
      const mockSnippets: KnowledgeSnippet[] = [];
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      const config: KnowledgeToolConfig<TestInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: TOPICS.DOCKERFILE,
          category: CATEGORY.DOCKERFILE,
          extractFilters: (input) => ({ language: input.language }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: () => [],
        },
        rules: {
          applyRules: () => ({ multistage: false, useAlpine: false }),
        },
        plan: {
          buildPlan: async (input, knowledge, rules, confidence) => {
            // Simulate async operation (e.g., reading files)
            await new Promise((resolve) => setTimeout(resolve, 10));
            return {
              language: input.language,
              recommendations: {
                security: [],
                optimization: [],
                bestPractices: [],
              },
              rules,
              confidence,
              matchCount: 0,
            };
          },
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      const result = await run({ language: 'python' }, mockContext);

      expect(result.ok).toBe(true);
      if (!result.ok) return;
      expect(result.value.language).toBe('python');
    });

    it('should use custom confidence calculation when provided', async () => {
      const mockSnippets = createMockSnippets();
      (knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<typeof knowledgeMatcher.getKnowledgeSnippets>).mockResolvedValue(
        mockSnippets,
      );

      const customConfidenceCalc = jest.fn().mockReturnValue(0.88);

      const config: KnowledgeToolConfig<TestInput, TestPlan, 'security' | 'optimization' | 'bestPractices', TestRules> = {
        name: 'test-tool',
        query: {
          topic: TOPICS.DOCKERFILE,
          category: CATEGORY.DOCKERFILE,
          extractFilters: (input) => ({ language: input.language }),
        },
        categorization: {
          categoryNames: ['security', 'optimization', 'bestPractices'] as const,
          categorize: () => [],
        },
        rules: {
          applyRules: () => ({ multistage: false, useAlpine: false }),
        },
        confidence: {
          calculateConfidence: customConfidenceCalc,
        },
        plan: {
          buildPlan: (input, knowledge, rules, confidence) => ({
            language: input.language,
            recommendations: {
              security: [],
              optimization: [],
              bestPractices: [],
            },
            rules,
            confidence,
            matchCount: 0,
          }),
        },
      };

      const run = createKnowledgeTool(config);
      const mockContext = createMockContext();

      const result = await run({ language: 'node' }, mockContext);

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Verify custom confidence was called and used
      expect(customConfidenceCalc).toHaveBeenCalledWith(4);
      expect(result.value.confidence).toBe(0.88);
    });
  });
});
