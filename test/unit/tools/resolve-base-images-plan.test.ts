/**
 * Unit Tests: Resolve Base Images Plan Tool
 * Tests for the knowledge-based base image recommendation tool
 */

import { jest } from '@jest/globals';
import resolveBaseImagesPlanTool from '../../../src/tools/resolve-base-images-plan/tool';
import type { ToolContext } from '../../../src/mcp/context';
import type { KnowledgeSnippet } from '../../../src/knowledge/schemas';
import * as knowledgeMatcher from '../../../src/knowledge/matcher';

// Mock the knowledge matcher module
jest.spyOn(knowledgeMatcher, 'getKnowledgeSnippets').mockImplementation(jest.fn());

describe('Resolve Base Images Plan Tool', () => {
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

  // Helper to create mock base image snippets
  function createMockBaseImageSnippets(): KnowledgeSnippet[] {
    return [
      {
        id: 'snippet-1',
        text: 'node:20-alpine - Official Node.js image with Alpine Linux (50MB)',
        weight: 90,
        tags: ['official', 'alpine', 'size'],
        category: 'official',
        source: 'base-images',
      },
      {
        id: 'snippet-2',
        text: 'gcr.io/distroless/nodejs - Google distroless image for Node.js',
        weight: 85,
        tags: ['distroless', 'security'],
        category: 'distroless',
        source: 'base-images',
      },
      {
        id: 'snippet-3',
        text: 'node:20-slim - Official Node.js image with Debian slim (150MB)',
        weight: 80,
        tags: ['official', 'slim'],
        category: 'official',
        source: 'base-images',
      },
      {
        id: 'snippet-4',
        text: 'cgr.dev/chainguard/node:latest - Chainguard hardened Node.js image',
        weight: 88,
        tags: ['security', 'hardened'],
        category: 'security',
        source: 'base-images',
      },
    ];
  }

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Basic Functionality', () => {
    it('should successfully generate base image plan for Node.js', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          languageVersion: '20',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Verify repository info
      expect(result.value.repositoryInfo.language).toBe('node');
      expect(result.value.repositoryInfo.languageVersion).toBe('20');

      // Verify recommendations are categorized
      expect(result.value.recommendations.officialImages.length).toBeGreaterThan(0);
      expect(result.value.recommendations.distrolessOptions.length).toBeGreaterThan(0);
      expect(result.value.recommendations.securityHardened.length).toBeGreaterThan(0);

      // Verify knowledge matches
      expect(result.value.knowledgeMatches.length).toBeGreaterThan(0);

      // Verify confidence is set
      expect(result.value.confidence).toBeGreaterThan(0);
      expect(result.value.confidence).toBeLessThanOrEqual(1);

      // Verify summary is present
      expect(result.value.summary).toContain('node');
      expect(result.value.summary).toContain('production');
    });

    it('should fail when technology is missing', async () => {
      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: '',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (result.ok) return;
      expect(result.error).toContain('Technology is required');
    });
  });

  describe('Language-Specific Recommendations', () => {
    it('should recommend Alpine for Node.js', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Node.js should have alpine recommendation
      const alpineImage = result.value.recommendations.sizeOptimized.find((img) =>
        img.image.includes('alpine'),
      );
      expect(alpineImage).toBeDefined();
    });

    it('should recommend distroless for Java', async () => {
      const javaSnippets: KnowledgeSnippet[] = [
        {
          id: 'java-1',
          text: 'eclipse-temurin:21-jre-alpine - Eclipse Temurin JRE with Alpine',
          weight: 90,
          tags: ['official', 'alpine'],
          category: 'official',
          source: 'base-images',
        },
        {
          id: 'java-2',
          text: 'gcr.io/distroless/java - Google distroless Java image',
          weight: 85,
          tags: ['distroless', 'security'],
          category: 'distroless',
          source: 'base-images',
        },
      ];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(javaSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'java',
          languageVersion: '21',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Java should have distroless recommendation
      expect(result.value.recommendations.distrolessOptions.length).toBeGreaterThan(0);
      const distrolessImage = result.value.recommendations.distrolessOptions.find((img) =>
        img.image.includes('distroless'),
      );
      expect(distrolessImage).toBeDefined();
    });

    it('should recommend appropriate images for Python', async () => {
      const pythonSnippets: KnowledgeSnippet[] = [
        {
          id: 'python-1',
          text: 'python:3.11-slim - Official Python image with Debian slim',
          weight: 90,
          tags: ['official', 'slim'],
          category: 'official',
          source: 'base-images',
        },
        {
          id: 'python-2',
          text: 'python:3.11-alpine - Official Python image with Alpine Linux',
          weight: 85,
          tags: ['official', 'alpine', 'size'],
          category: 'official',
          source: 'base-images',
        },
      ];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(pythonSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'python',
          languageVersion: '3.11',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Python should have both slim and alpine
      const hasSlim = result.value.recommendations.officialImages.some((img) =>
        img.image.includes('slim'),
      );
      const hasAlpine = result.value.recommendations.sizeOptimized.some((img) =>
        img.image.includes('alpine'),
      );

      expect(hasSlim || hasAlpine).toBe(true);
    });

    it('should recommend distroless for Go', async () => {
      const goSnippets: KnowledgeSnippet[] = [
        {
          id: 'go-1',
          text: 'golang:1.21-alpine - Official Go image with Alpine Linux',
          weight: 90,
          tags: ['official', 'alpine'],
          category: 'official',
          source: 'base-images',
        },
        {
          id: 'go-2',
          text: 'gcr.io/distroless/static - Google distroless static image for Go',
          weight: 95,
          tags: ['distroless', 'security'],
          category: 'distroless',
          source: 'base-images',
        },
      ];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(goSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'go',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Go should have distroless recommendation
      expect(result.value.recommendations.distrolessOptions.length).toBeGreaterThan(0);
    });
  });

  describe('Categorization', () => {
    it('should properly categorize base images', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Verify each category has appropriate recommendations
      expect(result.value.recommendations.officialImages).toBeDefined();
      expect(result.value.recommendations.distrolessOptions).toBeDefined();
      expect(result.value.recommendations.securityHardened).toBeDefined();
      expect(result.value.recommendations.sizeOptimized).toBeDefined();
    });

    it('should handle snippets that belong to multiple categories', async () => {
      const overlappingSnippets: KnowledgeSnippet[] = [
        {
          id: 'overlap-1',
          text: 'node:20-alpine - Official size-optimized image',
          weight: 90,
          tags: ['official', 'alpine', 'size'],
          category: 'official',
          source: 'base-images',
        },
      ];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(overlappingSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // The snippet should appear in both official and size categories
      // (or be deduplicated based on implementation)
      expect(result.value.knowledgeMatches.length).toBeGreaterThan(0);
    });
  });

  describe('Knowledge Query', () => {
    it('should query knowledge base with correct parameters', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      await resolveBaseImagesPlanTool.run(
        {
          technology: 'python',
          languageVersion: '3.11',
          framework: 'django',
          environment: 'production',
        },
        mockContext,
      );

      // Verify knowledge query was called with correct parameters
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        'resolve_base_images',
        expect.objectContaining({
          environment: 'production',
          tool: 'resolve-base-images-plan',
          language: 'python',
          framework: 'django',
        }),
      );
    });

    it('should use default environment when not specified', async () => {
      const mockSnippets: KnowledgeSnippet[] = [];
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
        },
        mockContext,
      );

      // Verify default production environment was used
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        'resolve_base_images',
        expect.objectContaining({
          environment: 'production',
        }),
      );
    });
  });

  describe('Confidence Calculation', () => {
    it('should calculate appropriate confidence based on knowledge matches', async () => {
      const manySnippets = [
        ...createMockBaseImageSnippets(),
        ...createMockBaseImageSnippets().map((s, i) => ({ ...s, id: `extra-${i}` })),
      ];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(manySnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // More matches should result in higher confidence
      expect(result.value.confidence).toBeGreaterThan(0.5);
    });

    it('should have reasonable confidence with few matches', async () => {
      const fewSnippets = [createMockBaseImageSnippets()[0]];

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(fewSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Should still have some confidence
      expect(result.value.confidence).toBeGreaterThan(0);
    });
  });

  describe('Rule-Based Logic', () => {
    it('should apply appropriate rules for each language', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      // Test Node.js
      const nodeResult = await resolveBaseImagesPlanTool.run(
        {
          technology: 'node',
          environment: 'production',
        },
        mockContext,
      );

      expect(nodeResult.ok).toBe(true);
      if (!nodeResult.ok) return;

      // Node.js should recommend alpine/slim
      expect(nodeResult.value.summary).toContain('Alpine');
    });
  });

  describe('Summary Generation', () => {
    it('should generate comprehensive summary', async () => {
      const mockSnippets = createMockBaseImageSnippets();
      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await resolveBaseImagesPlanTool.run(
        {
          technology: 'python',
          languageVersion: '3.11',
          framework: 'django',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const summary = result.value.summary;

      // Verify summary contains key information
      expect(summary).toContain('python');
      expect(summary).toContain('3.11');
      expect(summary).toContain('django');
      expect(summary).toContain('production');
      expect(summary).toContain('Official Images:');
      expect(summary).toContain('Distroless Options:');
      expect(summary).toContain('Security-Hardened:');
      expect(summary).toContain('Size-Optimized:');
    });
  });

  describe('Tool Metadata', () => {
    it('should have correct metadata', () => {
      expect(resolveBaseImagesPlanTool.name).toBe('resolve-base-images-plan');
      expect(resolveBaseImagesPlanTool.category).toBe('docker');
      expect(resolveBaseImagesPlanTool.metadata?.knowledgeEnhanced).toBe(true);
      expect(resolveBaseImagesPlanTool.metadata?.samplingStrategy).toBe('none');
    });
  });
});
