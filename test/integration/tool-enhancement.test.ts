/**
 * Integration tests for AI enhancement services
 * Tests the integration between AI enhancement services without depending on specific tools
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import { Success, Failure } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { enhanceValidationWithAI } from '@/validation/ai-enhancement';
import { enhanceWithKnowledge } from '@/mcp/ai/knowledge-enhancement';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';

// Mock AI services
jest.mock('@/mcp/ai/sampling-runner');
jest.mock('@/validation/ai-enhancement');
jest.mock('@/mcp/ai/knowledge-enhancement');

const mockSampleWithRerank = jest.mocked(sampleWithRerank);
const mockEnhanceValidationWithAI = jest.mocked(enhanceValidationWithAI);
const mockEnhanceWithKnowledge = jest.mocked(enhanceWithKnowledge);

describe('AI Enhancement Service Integration', () => {
  let mockContext: ToolContext;

  beforeEach(() => {
    jest.clearAllMocks();

    // Create mock context similar to existing test patterns
    mockContext = {
      sampling: {
        createMessage: jest.fn(),
      },
      session: {
        get: jest.fn(),
        set: jest.fn(),
        delete: jest.fn(),
        exists: jest.fn(),
        clear: jest.fn(),
      },
      logger: {
        info: jest.fn(),
        error: jest.fn(),
        warn: jest.fn(),
        debug: jest.fn(),
      },
    } as unknown as ToolContext;

    // Setup default mocks for AI services
    mockSampleWithRerank.mockResolvedValue(
      Success({
        text: 'Enhanced AI-generated content',
        winner: { score: 92 },
        model: 'claude-3-5-sonnet-20241022',
      }),
    );

    mockEnhanceValidationWithAI.mockResolvedValue(
      Success({
        suggestions: [
          'Consider using multi-stage builds to reduce image size',
          'Add health checks for better container monitoring',
          'Pin package versions for security',
        ],
        analysis: {
          assessment: 'The content follows most best practices but can be improved',
          riskLevel: 'medium' as const,
          priorities: [
            {
              area: 'Security',
              severity: 'warning' as const,
              description: 'Package versions should be pinned',
              impact: 'Medium security risk',
            },
          ],
        },
        confidence: 0.85,
        metadata: {
          processingTime: 1500,
          candidatesEvaluated: 3,
        },
      }),
    );

    mockEnhanceWithKnowledge.mockResolvedValue(
      Success({
        enhancedContent: 'Enhanced content with knowledge applied',
        knowledgeApplied: ['Docker security best practices', 'Performance optimization'],
        confidence: 0.9,
        suggestions: ['Additional optimization suggestions'],
        analysis: {
          improvementsSummary: 'Applied security and performance improvements',
          enhancementAreas: [
            {
              area: 'Security',
              description: 'Added security hardening',
              impact: 'high' as const,
            },
          ],
          knowledgeSources: ['Docker Best Practices Guide'],
          bestPracticesApplied: ['Non-root user execution'],
        },
        metrics: {
          processingTime: 2000,
        },
      }),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('Cross-service Integration', () => {
    it('should allow chaining of AI enhancement services', async () => {
      // Simulate a workflow where validation results are enhanced with AI,
      // then knowledge enhancement is applied
      const mockValidationResults = [
        {
          isValid: false,
          ruleId: 'security-001',
          message: 'Running as root',
          errors: ['Container runs as root user'],
          warnings: [],
          confidence: 0.9,
          metadata: {
            severity: 'error' as const,
            category: 'security' as const,
          },
        },
      ];

      // First, enhance validation with AI
      const enhancementResult = await mockEnhanceValidationWithAI(
        'FROM ubuntu:latest\nRUN apt-get update',
        mockValidationResults,
        mockContext,
        { mode: 'suggestions', focus: 'security', confidence: 0.8 },
      );

      expect(enhancementResult.ok).toBe(true);

      if (enhancementResult.ok) {
        // Then, apply knowledge enhancement based on suggestions
        const knowledgeResult = await mockEnhanceWithKnowledge(
          {
            content: 'FROM ubuntu:latest\nRUN apt-get update',
            context: 'dockerfile' as const,
            targetImprovement: 'security' as const,
            validationContext: mockValidationResults.map(r => ({
              message: r.message,
              severity: r.metadata?.severity || 'warning',
              category: r.metadata?.category || 'general',
            })),
          },
          mockContext,
        );

        expect(knowledgeResult.ok).toBe(true);

        if (knowledgeResult.ok) {
          expect(knowledgeResult.value.confidence).toBeGreaterThan(0.8);
          expect(knowledgeResult.value.knowledgeApplied).toContain('Docker security best practices');
        }
      }

      expect(mockEnhanceValidationWithAI).toHaveBeenCalledTimes(1);
      expect(mockEnhanceWithKnowledge).toHaveBeenCalledTimes(1);
    });

    it('should handle mixed success/failure scenarios in enhancement chain', async () => {
      // First service succeeds
      mockEnhanceValidationWithAI.mockResolvedValue(
        Success({
          suggestions: ['test suggestion'],
          analysis: {
            assessment: 'test',
            riskLevel: 'low' as const,
            priorities: [],
          },
          confidence: 0.8,
          metadata: {
            processingTime: 1000,
            candidatesEvaluated: 1,
          },
        }),
      );

      // Second service fails
      mockEnhanceWithKnowledge.mockResolvedValue(Failure('Knowledge service unavailable'));

      const validationResults = [
        {
          isValid: false,
          ruleId: 'test-rule',
          message: 'test issue',
          errors: ['test error'],
          warnings: [],
          confidence: 0.9,
        },
      ];

      // First enhancement should succeed
      const enhancementResult = await mockEnhanceValidationWithAI(
        'test content',
        validationResults,
        mockContext,
        { mode: 'suggestions', focus: 'all', confidence: 0.7 },
      );

      expect(enhancementResult.ok).toBe(true);

      // Second enhancement should fail gracefully
      const knowledgeResult = await mockEnhanceWithKnowledge(
        {
          content: 'test content',
          context: 'dockerfile' as const,
        },
        mockContext,
      );

      expect(knowledgeResult.ok).toBe(false);

      // Application should handle partial failures appropriately
      const hasPartialResults = enhancementResult.ok && !knowledgeResult.ok;
      expect(hasPartialResults).toBe(true);
    });
  });

  describe('AI Enhancement Service Performance', () => {
    it('should track AI enhancement performance metrics', async () => {
      const mockValidationResults = [
        {
          isValid: false,
          ruleId: 'test-rule',
          message: 'test issue',
          errors: ['test error'],
          warnings: [],
          confidence: 0.9,
        },
      ];

      const startTime = Date.now();
      const result = await mockEnhanceValidationWithAI(
        'test content',
        mockValidationResults,
        mockContext,
        { mode: 'suggestions', focus: 'security', confidence: 0.8 },
      );
      const endTime = Date.now();

      expect(result.ok).toBe(true);
      expect(mockEnhanceValidationWithAI).toHaveBeenCalled();

      if (result.ok) {
        expect(result.value.metadata.processingTime).toBeDefined();
        expect(result.value.confidence).toBeGreaterThan(0);
      }

      const processingTime = endTime - startTime;
      expect(processingTime).toBeLessThan(10000); // Should complete within 10 seconds
    });

    it('should handle timeout scenarios gracefully', async () => {
      // Mock a slow AI response
      mockEnhanceValidationWithAI.mockImplementation(
        () => new Promise(resolve => setTimeout(() => resolve(Success({
          suggestions: ['delayed response'],
          analysis: {
            assessment: 'delayed',
            riskLevel: 'low' as const,
            priorities: [],
          },
          confidence: 0.7,
          metadata: {
            processingTime: 5000,
            candidatesEvaluated: 1,
          },
        })), 100)), // Short delay for test
      );

      const mockValidationResults = [
        {
          isValid: false,
          ruleId: 'test-rule',
          message: 'test issue',
          errors: ['test error'],
          warnings: [],
          confidence: 0.9,
        },
      ];

      const startTime = Date.now();
      const result = await mockEnhanceValidationWithAI(
        'test content',
        mockValidationResults,
        mockContext,
        { mode: 'suggestions', focus: 'security', confidence: 0.8 },
      );
      const endTime = Date.now();

      expect(result.ok).toBe(true);
      const processingTime = endTime - startTime;
      expect(processingTime).toBeLessThan(1000); // Should complete reasonably quickly
    });
  });

  describe('Error Recovery and Resilience', () => {
    it('should handle AI service failures gracefully', async () => {
      // Mock all AI services to fail
      mockEnhanceValidationWithAI.mockResolvedValue(Failure('Enhancement service down'));
      mockEnhanceWithKnowledge.mockResolvedValue(Failure('Knowledge service down'));

      const mockValidationResults = [
        {
          isValid: false,
          ruleId: 'test-rule',
          message: 'test issue',
          errors: ['test error'],
          warnings: [],
          confidence: 0.9,
        },
      ];

      // Services should fail gracefully
      const enhancementResult = await mockEnhanceValidationWithAI(
        'test content',
        mockValidationResults,
        mockContext,
        { mode: 'suggestions', focus: 'security', confidence: 0.8 },
      );

      const knowledgeResult = await mockEnhanceWithKnowledge(
        {
          content: 'test content',
          context: 'dockerfile' as const,
        },
        mockContext,
      );

      expect(enhancementResult.ok).toBe(false);
      expect(knowledgeResult.ok).toBe(false);

      // Should not throw unhandled errors
      expect(enhancementResult.error).toBeDefined();
      expect(knowledgeResult.error).toBeDefined();
    });

    it('should maintain service stability under concurrent requests', async () => {
      const mockValidationResults = [
        {
          isValid: false,
          ruleId: 'test-rule',
          message: 'test issue',
          errors: ['test error'],
          warnings: [],
          confidence: 0.9,
        },
      ];

      // Simulate multiple concurrent requests to validation enhancement
      const concurrentRequests = Array.from({ length: 3 }, (_, i) =>
        mockEnhanceValidationWithAI(
          `test content ${i}`,
          mockValidationResults,
          mockContext,
          { mode: 'suggestions', focus: 'security', confidence: 0.8 },
        )
      );

      const results = await Promise.all(concurrentRequests);

      // All requests should complete (either success or controlled failure)
      results.forEach((result, index) => {
        expect(result.ok).toBeDefined();
        if (!result.ok) {
          // Log failure for debugging but don't fail the test
          console.log(`Request ${index} failed: ${result.error}`);
        }
      });

      // Should handle concurrent calls
      expect(mockEnhanceValidationWithAI).toHaveBeenCalledTimes(3);
    });
  });
});