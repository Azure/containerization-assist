/**
 * Unit tests for AI Validator Service
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import {
  AIValidator,
  createAIValidator,
  type AIValidationOptions,
} from '@/validation/ai-validator';
import type { ToolContext } from '@/mcp/context';
import { ValidationSeverity, ValidationCategory } from '@/validation/core-types';
import { Success, Failure } from '@/types';

// Mock dependencies
jest.mock('@/mcp/ai/sampling-runner');
jest.mock('@/mcp/ai/response-parser');

const mockSampleWithRerank = jest.mocked(require('@/mcp/ai/sampling-runner').sampleWithRerank);
const mockParseAIResponse = jest.mocked(require('@/mcp/ai/response-parser').parseAIResponse);

describe('AI Validator Service', () => {
  let validator: AIValidator;
  let mockContext: ToolContext;

  beforeEach(() => {
    jest.clearAllMocks();
    validator = createAIValidator();

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

    // Setup default successful mock response
    mockSampleWithRerank.mockResolvedValue(
      Success({
        text: JSON.stringify({
          passed: false,
          results: [
            {
              isValid: false,
              ruleId: 'docker-security-001',
              message: 'Running as root user is not recommended',
              errors: ['Container runs as root'],
              warnings: [],
              confidence: 0.9,
              metadata: {
                severity: 'error',
                category: 'security',
                location: 'line:5',
                aiEnhanced: true,
                fixSuggestion: 'Add USER directive to run as non-root user',
              },
            },
            {
              isValid: false,
              ruleId: 'docker-performance-001',
              message: 'Using latest tag is not recommended',
              errors: [],
              warnings: ['Image tag "latest" should be avoided'],
              confidence: 0.8,
              metadata: {
                severity: 'warning',
                category: 'best_practice',
                location: 'line:1',
                aiEnhanced: true,
                fixSuggestion: 'Specify a specific version tag',
              },
            },
          ],
          summary: {
            totalIssues: 2,
            errorCount: 1,
            warningCount: 1,
            categories: {
              security: 1,
              performance: 0,
              best_practice: 1,
              compliance: 0,
              optimization: 0,
            },
          },
        }),
        score: 88,
        model: 'claude-3-5-sonnet-20241022',
        usage: {
          inputTokens: 800,
          outputTokens: 400,
          totalTokens: 1200,
        },
      }),
    );

    // Mock parseAIResponse to return the parsed data
    mockParseAIResponse.mockResolvedValue(
      Success({
        passed: false,
        results: [
          {
            isValid: false,
            ruleId: 'docker-security-001',
            message: 'Running as root user is not recommended',
            errors: ['Container runs as root'],
            warnings: [],
            confidence: 0.9,
            metadata: {
              severity: ValidationSeverity.ERROR,
              category: ValidationCategory.SECURITY,
              location: 'line:5',
              aiEnhanced: true,
              fixSuggestion: 'Add USER directive to run as non-root user',
            },
          },
          {
            isValid: false,
            ruleId: 'docker-performance-001',
            message: 'Using latest tag is not recommended',
            errors: [],
            warnings: ['Image tag "latest" should be avoided'],
            confidence: 0.8,
            metadata: {
              severity: ValidationSeverity.WARNING,
              category: ValidationCategory.BEST_PRACTICE,
              location: 'line:1',
              aiEnhanced: true,
              fixSuggestion: 'Specify a specific version tag',
            },
          },
        ],
        summary: {
          totalIssues: 2,
          errorCount: 1,
          warningCount: 1,
          categories: {
            security: 1,
            performance: 0,
            best_practice: 1,
            compliance: 0,
            optimization: 0,
          },
        },
      }),
    );
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('validateWithAI', () => {
    it('should validate Dockerfile content with AI successfully', async () => {
      const content = `FROM ubuntu:latest
RUN apt-get update && apt-get install -y nodejs
COPY . /app
WORKDIR /app
CMD ["node", "server.js"]`;

      const options: AIValidationOptions = {
        contentType: 'dockerfile',
        focus: 'security',
        confidence: 0.8,
      };

      const result = await validator.validateWithAI(content, options, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value;
        expect(report.results).toHaveLength(2);
        expect(report.passed).toBe(0); // Based on mock data
        expect(report.failed).toBe(2);
        expect(report.errors).toBe(1);
        expect(report.warnings).toBe(1);
        expect(report.aiMetadata.confidence).toBe(0.88);
        expect(report.aiMetadata.model).toBe('claude-3-5-sonnet-20241022');
        expect(typeof report.aiMetadata.processingTime).toBe('number');

        // Check individual results
        const securityResult = report.results.find(r => r.ruleId === 'docker-security-001');
        expect(securityResult).toBeDefined();
        expect(securityResult!.isValid).toBe(false);
        expect(securityResult!.message).toBe('Running as root user is not recommended');
        expect(securityResult!.metadata?.severity).toBe(ValidationSeverity.ERROR);
        expect(securityResult!.metadata?.category).toBe(ValidationCategory.SECURITY);
        expect(securityResult!.metadata?.fixSuggestion).toBe('Add USER directive to run as non-root user');
      }

      expect(mockSampleWithRerank).toHaveBeenCalledTimes(1);
      expect(mockSampleWithRerank).toHaveBeenCalledWith(
        mockContext,
        expect.any(Function),
        expect.any(Function),
        {},
      );
    });

    it('should validate Kubernetes manifests with AI successfully', async () => {
      const content = `apiVersion: apps/v1
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
        - containerPort: 8080`;

      const options: AIValidationOptions = {
        contentType: 'kubernetes',
        focus: 'all',
        confidence: 0.7,
      };

      const result = await validator.validateWithAI(content, options, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.aiMetadata.confidence).toBeGreaterThanOrEqual(0.7);
        expect(result.value.results.length).toBeGreaterThan(0);
      }
    });

    it('should handle different validation focuses correctly', async () => {
      const focuses: AIValidationOptions['focus'][] = [
        'security',
        'performance',
        'best-practices',
        'all',
      ];

      for (const focus of focuses) {
        const options: AIValidationOptions = {
          contentType: 'dockerfile',
          focus,
        };

        const result = await validator.validateWithAI('FROM node:18', options, mockContext);
        expect(result.ok).toBe(true);
      }

      expect(mockSampleWithRerank).toHaveBeenCalledTimes(focuses.length);
    });

    it('should handle different content types correctly', async () => {
      const contentTypes: AIValidationOptions['contentType'][] = [
        'dockerfile',
        'kubernetes',
        'security',
        'general',
      ];

      for (const contentType of contentTypes) {
        const options: AIValidationOptions = {
          contentType,
          focus: 'all',
        };

        const result = await validator.validateWithAI('test content', options, mockContext);
        expect(result.ok).toBe(true);
      }

      expect(mockSampleWithRerank).toHaveBeenCalledTimes(contentTypes.length);
    });

    it('should handle sampling failure gracefully', async () => {
      mockSampleWithRerank.mockResolvedValue(Failure('AI model unavailable'));

      const options: AIValidationOptions = {
        contentType: 'dockerfile',
        focus: 'security',
      };

      const result = await validator.validateWithAI('FROM node:18', options, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('AI validation failed');
        expect(result.error).toContain('AI model unavailable');
      }
    });

    it('should handle invalid JSON response gracefully', async () => {
      mockSampleWithRerank.mockResolvedValue(
        Success({
          text: 'This is not valid JSON',
          score: 50,
        }),
      );

      // Mock parseAIResponse to simulate parsing failure
      mockParseAIResponse.mockResolvedValueOnce(
        Failure('Failed to parse AI response: Invalid JSON structure')
      );

      const options: AIValidationOptions = {
        contentType: 'dockerfile',
        focus: 'security',
      };

      const result = await validator.validateWithAI('FROM node:18', options, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse AI validation response');
      }
    });

    it('should handle malformed response structure gracefully', async () => {
      mockSampleWithRerank.mockResolvedValue(
        Success({
          text: JSON.stringify({
            // Missing required fields
            somefield: 'value',
          }),
          score: 70,
        }),
      );

      // Mock parseAIResponse to simulate schema validation failure
      mockParseAIResponse.mockResolvedValueOnce(
        Failure('Schema validation failed: Required fields missing')
      );

      const options: AIValidationOptions = {
        contentType: 'dockerfile',
        focus: 'security',
      };

      const result = await validator.validateWithAI('FROM node:18', options, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to parse AI validation response');
      }
    });

    it('should handle partial response data gracefully', async () => {
      mockSampleWithRerank.mockResolvedValue(
        Success({
          text: JSON.stringify({
            passed: true,
            results: [
              {
                // Missing some optional fields
                isValid: true,
                message: 'All good',
              },
            ],
          }),
          score: 85,
        }),
      );

      // Mock parseAIResponse to return partial but valid data
      mockParseAIResponse.mockResolvedValueOnce(
        Success({
          passed: true,
          results: [
            {
              isValid: true,
              ruleId: 'ai-rule-001',
              message: 'All good',
              errors: [],
              warnings: [],
              confidence: 0.7,
              metadata: {
                severity: ValidationSeverity.INFO,
                category: ValidationCategory.BEST_PRACTICE,
                aiEnhanced: true,
              },
            },
          ],
          summary: {
            totalIssues: 0,
            errorCount: 0,
            warningCount: 0,
            categories: {
              security: 0,
              performance: 0,
              best_practice: 1,
              compliance: 0,
              optimization: 0,
            },
          },
        })
      );

      const options: AIValidationOptions = {
        contentType: 'dockerfile',
        focus: 'security',
      };

      const result = await validator.validateWithAI('FROM node:18', options, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value;
        expect(report.results).toHaveLength(1);

        const validationResult = report.results[0];
        expect(validationResult.isValid).toBe(true);
        expect(validationResult.ruleId).toBe('ai-rule-001');
        expect(validationResult.confidence).toBe(0.7);
        expect(validationResult.metadata?.aiEnhanced).toBe(true);
      }
    });

    it('should include model preferences and hints in sampling call', async () => {
      const options: AIValidationOptions = {
        contentType: 'kubernetes',
        focus: 'performance',
        confidence: 0.9,
      };

      await validator.validateWithAI('test content', options, mockContext);

      expect(mockSampleWithRerank).toHaveBeenCalledWith(
        mockContext,
        expect.any(Function),
        expect.any(Function),
        {},
      );

      // Verify the messages function was called with model preferences
      const messagesFunction = mockSampleWithRerank.mock.calls[0][1];
      const messagesResult = await messagesFunction();

      expect(messagesResult.modelPreferences).toEqual({
        hints: [
          { name: 'validation-kubernetes' },
          { name: 'focus-performance' },
        ],
        intelligencePriority: 0.9,
        costPriority: 0.2,
      });
    });

    it('should calculate score and grade correctly', async () => {
      // Mock response with mixed validation results
      mockSampleWithRerank.mockResolvedValue(
        Success({
          text: JSON.stringify({
            passed: false,
            results: [
              { isValid: true, message: 'Good practice' },
              { isValid: false, message: 'Security issue' },
              { isValid: true, message: 'Another good practice' },
              { isValid: false, message: 'Performance issue' },
            ],
          }),
          score: 75,
        }),
      );

      // Mock parseAIResponse with mixed results
      mockParseAIResponse.mockResolvedValueOnce(
        Success({
          passed: false,
          results: [
            {
              isValid: true,
              ruleId: 'good-practice-001',
              message: 'Good practice',
              errors: [],
              warnings: [],
              confidence: 0.9,
              metadata: {
                severity: ValidationSeverity.INFO,
                category: ValidationCategory.BEST_PRACTICE,
                aiEnhanced: true,
              },
            },
            {
              isValid: false,
              ruleId: 'security-001',
              message: 'Security issue',
              errors: ['Security vulnerability detected'],
              warnings: [],
              confidence: 0.8,
              metadata: {
                severity: ValidationSeverity.ERROR,
                category: ValidationCategory.SECURITY,
                aiEnhanced: true,
              },
            },
            {
              isValid: true,
              ruleId: 'good-practice-002',
              message: 'Another good practice',
              errors: [],
              warnings: [],
              confidence: 0.9,
              metadata: {
                severity: ValidationSeverity.INFO,
                category: ValidationCategory.BEST_PRACTICE,
                aiEnhanced: true,
              },
            },
            {
              isValid: false,
              ruleId: 'performance-001',
              message: 'Performance issue',
              errors: ['Performance bottleneck detected'],
              warnings: [],
              confidence: 0.7,
              metadata: {
                severity: ValidationSeverity.WARNING,
                category: ValidationCategory.PERFORMANCE,
                aiEnhanced: true,
              },
            },
          ],
          summary: {
            totalIssues: 4,
            errorCount: 2,
            warningCount: 0,
            categories: {
              security: 1,
              performance: 1,
              best_practice: 2,
              compliance: 0,
              optimization: 0,
            },
          },
        })
      );

      const result = await validator.validateWithAI('test', {
        contentType: 'dockerfile',
        focus: 'all',
      }, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        const report = result.value;
        expect(report.score).toBe(50); // 2 valid out of 4 = 50%
        expect(report.grade).toBe('F'); // 50% is F grade
        expect(report.passed).toBe(2);
        expect(report.failed).toBe(2);
      }
    });

    it('should handle network errors and exceptions gracefully', async () => {
      mockSampleWithRerank.mockRejectedValue(new Error('Network timeout'));

      const result = await validator.validateWithAI('test content', {
        contentType: 'dockerfile',
        focus: 'security',
      }, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('AI validation error');
        expect(result.error).toContain('Network timeout');
      }
    });
  });

  describe('createAIValidator', () => {
    it('should create a new AIValidator instance', () => {
      const validator = createAIValidator();
      expect(validator).toBeInstanceOf(AIValidator);
    });

    it('should create independent instances', () => {
      const validator1 = createAIValidator();
      const validator2 = createAIValidator();
      expect(validator1).not.toBe(validator2);
    });
  });
});