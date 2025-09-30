/**
 * Unit tests for Knowledge Enhancement Service
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import {
  enhanceWithKnowledge,
  createEnhancementFromValidation,
  type KnowledgeEnhancementRequest,
} from '@/mcp/ai/knowledge-enhancement';
import type { ToolContext } from '@/mcp/context';
import { Success, Failure } from '@/types';

// Mock dependencies
jest.mock('@/mcp/ai/sampling-runner');
jest.mock('@/ai/prompt-engine');
jest.mock('@/mcp/ai/message-converter');
jest.mock('@/mcp/ai/response-parser');
jest.mock('@/mcp/ai/quality');

const mockSampleWithRerank = jest.mocked(require('@/mcp/ai/sampling-runner').sampleWithRerank);
const mockBuildMessages = jest.mocked(require('@/ai/prompt-engine').buildMessages);
const mockToMCPMessages = jest.mocked(require('@/mcp/ai/message-converter').toMCPMessages);
const mockParseAIResponse = jest.mocked(require('@/mcp/ai/response-parser').parseAIResponse);
const mockScoreResponse = jest.mocked(require('@/mcp/ai/quality').scoreResponse);

describe('Knowledge Enhancement Service', () => {
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

    // Setup default mocks
    mockBuildMessages.mockResolvedValue([]);
    mockToMCPMessages.mockReturnValue({ messages: [] });
    mockScoreResponse.mockReturnValue({ total: 100, breakdown: { quality: 100 } });

    // Mock the parseAIResponse to return parsed JSON
    mockParseAIResponse.mockResolvedValue(
      Success({
        enhancedContent: `FROM node:18-alpine
USER node
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY --chown=node:node . .
EXPOSE 3000
CMD ["npm", "start"]`,
        knowledgeApplied: [
          'Multi-stage build optimization: Reduced image size',
          'Security hardening: Non-root user execution',
        ],
        confidence: 0.92,
        suggestions: [
          'Consider using specific version tags instead of latest',
          'Add health check for better container monitoring',
        ],
        analysis: {
          improvementsSummary: 'Applied Docker security best practices and optimized image layers.',
          enhancementAreas: [
            {
              area: 'Security',
              description: 'Added non-root user',
              impact: 'high',
            },
            {
              area: 'Performance',
              description: 'Optimized package installation',
              impact: 'medium',
            },
          ],
          knowledgeSources: [
            'Docker Security Best Practices',
            'Container Image Optimization Guide',
          ],
          bestPracticesApplied: [
            'Non-root user execution',
            'Layer optimization',
            'Package version pinning',
          ],
        },
      }),
    );
    mockSampleWithRerank.mockResolvedValue(
      Success({
        text: JSON.stringify({
          enhancedContent: `FROM node:18-alpine
USER node
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY --chown=node:node . .
EXPOSE 3000
CMD ["npm", "start"]`,
          knowledgeApplied: [
            'Multi-stage build optimization: Reduced image size',
            'Security hardening: Non-root user execution',
          ],
          confidence: 0.92,
          suggestions: [
            'Consider using specific version tags instead of latest',
            'Add health check for better container monitoring',
          ],
          analysis: {
            improvementsSummary: 'Applied Docker security best practices and optimized image layers.',
            enhancementAreas: [
              {
                area: 'Security',
                description: 'Added non-root user',
                impact: 'high',
              },
              {
                area: 'Performance',
                description: 'Optimized package installation',
                impact: 'medium',
              },
            ],
            knowledgeSources: [
              'Docker Security Best Practices',
              'Container Image Optimization Guide',
            ],
            bestPracticesApplied: [
              'Non-root user execution',
              'Layer optimization',
              'Package version pinning',
            ],
          },
        }),
        winner: { score: 92 },
        model: 'claude-3-5-sonnet-20241022',
        usage: {
          inputTokens: 1000,
          outputTokens: 500,
          totalTokens: 1500,
        },
      }),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('enhanceWithKnowledge', () => {
    it('should enhance Dockerfile content with knowledge successfully', async () => {
      const request: KnowledgeEnhancementRequest = {
        content: `FROM node:latest
WORKDIR /app
COPY . .
RUN npm install
EXPOSE 3000
CMD ["npm", "start"]`,
        context: 'dockerfile',
        targetImprovement: 'security',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.enhancedContent).toContain('FROM node:18-alpine');
        expect(result.value.enhancedContent).toContain('USER node');
        expect(result.value.knowledgeApplied).toEqual([
          'Multi-stage build optimization: Reduced image size',
          'Security hardening: Non-root user execution',
        ]);
        expect(result.value.confidence).toBe(0.92);
        expect(result.value.suggestions).toEqual([
          'Consider using specific version tags instead of latest',
          'Add health check for better container monitoring',
        ]);
        expect(result.value.analysis.improvementsSummary).toContain('Docker security best practices');
        expect(result.value.analysis.enhancementAreas).toHaveLength(2);
        expect(result.value.metadata.qualityScore).toBeGreaterThanOrEqual(0);
      }

      expect(mockSampleWithRerank).toHaveBeenCalledTimes(1);
      expect(mockBuildMessages).toHaveBeenCalledTimes(1);
    });

    it('should enhance Kubernetes manifests with knowledge successfully', async () => {
      const request: KnowledgeEnhancementRequest = {
        content: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: app
        image: myapp:latest
        ports:
        - containerPort: 8080`,
        context: 'kubernetes',
        targetImprovement: 'all',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.confidence).toBeGreaterThan(0.8);
        expect(result.value.knowledgeApplied).toHaveLength(2);
        expect(result.value.metadata.sources).toContain('Docker Security Best Practices');
        expect(result.value.metadata.model).toBe('claude-3-5-sonnet-20241022');
      }
    });

    it('should handle validation context in enhancement request', async () => {
      const request: KnowledgeEnhancementRequest = {
        content: 'FROM ubuntu:latest\nRUN apt-get update',
        context: 'dockerfile',
        validationContext: [
          {
            message: 'Using latest tag is not recommended',
            severity: 'warning',
            category: 'best-practices',
          },
          {
            message: 'Missing USER directive',
            severity: 'error',
            category: 'security',
          },
        ],
        userQuery: 'Make this Dockerfile more secure',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.enhancedContent).toBeDefined();
        expect(result.value.analysis.enhancementAreas.length).toBeGreaterThan(0);
      }

      // Verify that validation context was included in prompt
      expect(mockBuildMessages).toHaveBeenCalledWith(
        expect.objectContaining({
          basePrompt: expect.stringContaining('Validation Issues Found'),
        }),
      );
    });

    it('should provide confidence scoring', async () => {
      const request: KnowledgeEnhancementRequest = {
        content: 'FROM node:18\nWORKDIR /app',
        context: 'dockerfile',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.confidence).toBeGreaterThanOrEqual(0);
        expect(result.value.confidence).toBeLessThanOrEqual(1);
        expect(typeof result.value.confidence).toBe('number');
      }
    });

    it('should handle different improvement targets', async () => {
      const testCases: KnowledgeEnhancementRequest['targetImprovement'][] = [
        'security',
        'performance',
        'best-practices',
        'optimization',
        'all',
      ];

      for (const target of testCases) {
        const request: KnowledgeEnhancementRequest = {
          content: 'FROM node:18',
          context: 'dockerfile',
          targetImprovement: target,
        };

        const result = await enhanceWithKnowledge(request, mockContext);
        expect(result.ok).toBe(true);
      }

      expect(mockSampleWithRerank).toHaveBeenCalledTimes(testCases.length);
    });

    it('should handle sampling failure gracefully', async () => {
      mockSampleWithRerank.mockResolvedValue(Failure('Sampling failed'));

      const request: KnowledgeEnhancementRequest = {
        content: 'FROM node:18',
        context: 'dockerfile',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Knowledge enhancement sampling failed');
      }
    });

    it('should handle malformed AI response gracefully', async () => {
      // Mock parseAIResponse to simulate parsing failure
      mockParseAIResponse.mockResolvedValue(
        Failure('Failed to parse AI response: Invalid JSON structure'),
      );

      const request: KnowledgeEnhancementRequest = {
        content: 'FROM node:18',
        context: 'dockerfile',
      };

      const result = await enhanceWithKnowledge(request, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse enhancement response');
      }
    });

    it('should handle different contexts correctly', async () => {
      const contexts: KnowledgeEnhancementRequest['context'][] = [
        'dockerfile',
        'kubernetes',
        'security',
        'optimization',
      ];

      for (const context of contexts) {
        const request: KnowledgeEnhancementRequest = {
          content: 'test content',
          context,
        };

        const result = await enhanceWithKnowledge(request, mockContext);
        expect(result.ok).toBe(true);
      }

      expect(mockBuildMessages).toHaveBeenCalledTimes(contexts.length);
    });
  });

  describe('createEnhancementFromValidation', () => {
    it('should create enhancement request from validation results', () => {
      const content = 'FROM ubuntu:latest';
      const context = 'dockerfile';
      const validationResults = [
        {
          message: 'Using latest tag is not recommended',
          severity: 'warning',
          category: 'best-practices',
        },
        {
          message: 'Missing security hardening',
          severity: 'error',
          category: 'security',
        },
      ];

      const request = createEnhancementFromValidation(
        content,
        context,
        validationResults,
        'security',
      );

      expect(request.content).toBe(content);
      expect(request.context).toBe(context);
      expect(request.targetImprovement).toBe('security');
      expect(request.validationContext).toHaveLength(2);
      expect(request.validationContext![0].message).toBe('Using latest tag is not recommended');
      expect(request.validationContext![0].severity).toBe('warning');
      expect(request.validationContext![1].severity).toBe('error');
    });

    it('should default to "all" improvement when not specified', () => {
      const request = createEnhancementFromValidation(
        'FROM node:18',
        'dockerfile',
        [{ message: 'test', severity: 'info' }],
      );

      expect(request.targetImprovement).toBe('all');
    });

    it('should handle empty validation results', () => {
      const request = createEnhancementFromValidation(
        'FROM node:18',
        'dockerfile',
        [],
      );

      expect(request.validationContext).toHaveLength(0);
    });

    it('should normalize severity values', () => {
      const request = createEnhancementFromValidation(
        'FROM node:18',
        'dockerfile',
        [{ message: 'test', severity: 'unknown' }],
      );

      expect(request.validationContext![0].severity).toBe('warning');
    });
  });
});