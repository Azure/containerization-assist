/**
 * Unit Tests: Resolve Base Images Tool
 * Tests for knowledge-based base image resolution
 */

import { jest } from '@jest/globals';
import resolveBaseImagesTool from '../../../src/tools/resolve-base-images/tool';
import type { ResolveBaseImagesParams } from '../../../src/tools/resolve-base-images/schema';
import type { ToolContext } from '../../../src/mcp/context';
import * as knowledgeMatcher from '../../../src/knowledge/matcher';

// Mock the knowledge matcher
jest.mock('../../../src/knowledge/matcher', () => ({
  getKnowledgeSnippets: jest.fn().mockResolvedValue([
    {
      id: 'docker-base-1',
      text: 'Use node:lts-alpine for production to reduce image size by ~80%\nExample: FROM node:20-alpine AS build',
      category: 'size',
      tags: ['node', 'javascript', 'alpine', 'production'],
      weight: 58,
    },
    {
      id: 'docker-base-2',
      text: 'Recommended official image for javascript: ubuntu:22.04',
      category: 'official',
      tags: ['official', 'ubuntu'],
      weight: 1,
    },
  ]),
}));

describe('resolveBaseImagesTool', () => {
  // Helper function to create a mock context
  function createMockContext(): ToolContext {
    return {
      sampling: {
        createMessage: jest.fn().mockRejectedValue(
          new Error('Sampling should not be called for knowledge-based tools'),
        ),
      },
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
        trace: jest.fn(),
        fatal: jest.fn(),
        child: jest.fn().mockReturnThis(),
      },
    } as unknown as ToolContext;
  }

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('successful base image resolution', () => {
    it('should resolve base images for JavaScript application', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'javascript',
        environment: 'production',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImagesTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Check structure of knowledge-based response
        expect(result.value).toHaveProperty('recommendations');
        expect(result.value.recommendations).toHaveProperty('officialImages');
        expect(result.value.recommendations).toHaveProperty('sizeOptimized');
        expect(result.value.recommendations).toHaveProperty('securityHardened');
        expect(result.value.recommendations).toHaveProperty('distrolessOptions');
        expect(result.value).toHaveProperty('confidence');
        expect(result.value).toHaveProperty('summary');

        // Verify recommendations are arrays
        expect(Array.isArray(result.value.recommendations.officialImages)).toBe(true);
        expect(Array.isArray(result.value.recommendations.sizeOptimized)).toBe(true);
      }

      // Verify knowledge base was queried
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalled();
    });

    it('should handle Python applications', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'python',
        languageVersion: '3.11',
        environment: 'production',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImagesTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations');
        expect(result.value.repositoryInfo).toHaveProperty('language', 'python');
        expect(result.value.repositoryInfo).toHaveProperty('languageVersion', '3.11');
      }
    });

    it('should handle Java applications with framework', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'java',
        languageVersion: '21',
        framework: 'spring',
        buildSystem: 'maven',
        environment: 'production',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImagesTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.repositoryInfo).toHaveProperty('language', 'java');
        expect(result.value.repositoryInfo).toHaveProperty('framework', 'spring');
        expect(result.value.repositoryInfo).toHaveProperty('buildSystem', 'maven');
      }
    });
  });

  describe('error handling', () => {
    it('should require technology parameter', async () => {
      const config = {} as ResolveBaseImagesParams;

      const mockContext = createMockContext();
      const result = await resolveBaseImagesTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Technology is required');
      }
    });

    it('should handle empty knowledge base results', async () => {
      // Override mock to return empty results
      (knowledgeMatcher.getKnowledgeSnippets as jest.Mock).mockResolvedValueOnce([]);

      const config: ResolveBaseImagesParams = {
        technology: 'javascript',
        environment: 'production',
      };

      const mockContext = createMockContext();
      const result = await resolveBaseImagesTool.handler(config, mockContext);

      // Tool should still succeed with empty knowledge base
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('recommendations');
        // Should still have rule-based recommendations even if knowledge base is empty
        expect(
          result.value.recommendations.officialImages.length +
            result.value.recommendations.sizeOptimized.length,
        ).toBeGreaterThan(0);
      }
    });
  });

  describe('deterministic behavior', () => {
    it('should not call AI sampling', async () => {
      const config: ResolveBaseImagesParams = {
        technology: 'nodejs',
        environment: 'production',
      };

      const mockContext = createMockContext();
      await resolveBaseImagesTool.handler(config, mockContext);

      // Verify sampling was never called
      expect(mockContext.sampling.createMessage).not.toHaveBeenCalled();
    });

    it('should be knowledge-enhanced', () => {
      expect(resolveBaseImagesTool.metadata?.knowledgeEnhanced).toBe(true);
    });
  });
});
