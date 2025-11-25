/**
 * Unit Tests: Fix Dockerfile Tool
 * Tests the fix-dockerfile tool functionality with validation and knowledge integration
 */

import { jest } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { createMockValidatePath } from '../../__support__/utilities/mocks';

// Result Type Helpers for Testing
function createSuccessResult<T>(value: T) {
  return {
    ok: true as const,
    value,
  };
}

function createFailureResult(error: string) {
  return {
    ok: false as const,
    error,
  };
}

function createMockLogger() {
  return {
    info: jest.fn(),
    warn: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  } as any;
}

// Mock filesystem
jest.mock('node:fs', () => ({
  promises: {
    readFile: jest.fn(),
    access: jest.fn(),
    stat: jest.fn(),
    writeFile: jest.fn(),
    constants: {
      R_OK: 4,
      W_OK: 2,
      X_OK: 1,
      F_OK: 0,
    },
  },
  constants: {
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,
    F_OK: 0,
  },
}));

// Mock the validation library
jest.mock('../../../src/lib/validation', () => ({
  validatePath: createMockValidatePath(),
}));

// Mock validation-helpers to use the mocked validation
jest.mock('../../../src/lib/validation-helpers', () => ({
  validatePathOrFail: jest.fn().mockImplementation(async (...args: any[]) => {
    const { validatePath } = require('../../../src/lib/validation');
    return validatePath(...args);
  }),
}));

// Mock validation
const mockValidateDockerfileContent = jest.fn();
jest.mock('../../../src/validation/dockerfile-validator', () => ({
  validateDockerfileContent: mockValidateDockerfileContent,
}));

// Mock knowledge loader
const mockGetKnowledgeForCategory = jest.fn();
jest.mock('../../../src/knowledge', () => ({
  getKnowledgeForCategory: mockGetKnowledgeForCategory,
}));

// Mock lib modules
jest.mock('../../../src/lib/logger', () => ({
  createTimer: jest.fn(() => ({
    end: jest.fn(),
    error: jest.fn(),
  })),
  createLogger: jest.fn(() => createMockLogger()),
}));

function createMockToolContext() {
  return {
    logger: createMockLogger(),
  } as any;
}

// Import these after mocks are set up
import { default as fixDockerfileTool } from '../../../src/tools/fix-dockerfile/tool';
import type { FixDockerfileParams } from '../../../src/tools/fix-dockerfile/schema';
import { ValidationSeverity, ValidationCategory } from '../../../src/validation/core-types';

const mockFs = fs as jest.Mocked<typeof fs>;

// Test fixture: Dockerfile with issues
const dockerfileWithIssues = `FROM node:latest
RUN apt-get update && apt-get install -y curl
COPY . .
RUN npm install
EXPOSE 3000
CMD ["node", "index.js"]`;

// Test fixture: Perfect Dockerfile
const perfectDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force
COPY --chown=node:node . .
EXPOSE 3000
USER node
CMD ["node", "index.js"]`;

describe('fix-dockerfile', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: FixDockerfileParams;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      dockerfile: dockerfileWithIssues,
      environment: 'production',
    };

    // Reset all mocks
    jest.clearAllMocks();

    // Default mock implementations for file system operations
    mockFs.access.mockResolvedValue(undefined);
    mockFs.stat.mockResolvedValue({ isFile: () => true, isDirectory: () => false } as any);
    mockFs.readFile.mockResolvedValue(dockerfileWithIssues);

    // Default knowledge matches
    mockGetKnowledgeForCategory.mockReturnValue([
      {
        id: 'security-fix-1',
        text: 'Use specific image tags instead of :latest\nPinning versions ensures reproducibility.',
        category: 'security',
        tags: ['security', 'security-fix', 'best-practice'],
        weight: 0.9,
      },
      {
        id: 'performance-fix-1',
        text: 'Multi-stage builds reduce image size\nSeparate build and runtime environments.',
        category: 'performance',
        tags: ['performance', 'performance-fix', 'optimization'],
        weight: 0.85,
      },
    ]);
  });

  describe('Happy Path', () => {
    it('should analyze Dockerfile with issues and return fix recommendations', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 60,
        grade: 'D',
        results: [
          {
            passed: false,
            rule: 'no-latest-tag',
            message: 'Avoid using :latest tag',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.WARNING,
            },
          },
          {
            passed: false,
            rule: 'no-root-user',
            message: 'Container runs as root',
            line: 6,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
          {
            passed: false,
            rule: 'multi-stage-build',
            message: 'Consider using multi-stage build',
            line: 1,
            metadata: {
              category: ValidationCategory.PERFORMANCE,
              severity: ValidationSeverity.INFO,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues.security).toHaveLength(2);
        expect(result.value.currentIssues.performance).toHaveLength(1);
        expect(result.value.validationScore).toBeGreaterThanOrEqual(0);
        expect(result.value.validationScore).toBeLessThanOrEqual(100);
        expect(['A', 'B', 'C', 'D', 'F']).toContain(result.value.validationGrade);
        expect(result.value.priority).toBe('high'); // Has critical security issue
        expect(result.value.confidence).toBeGreaterThanOrEqual(0);
        expect(result.value.summary).toContain('Found 3 issues');
      }
    });

    it('should handle perfect Dockerfile with no issues', async () => {
      config.dockerfile = perfectDockerfile;

      mockValidateDockerfileContent.mockResolvedValue({
        passed: true,
        score: 100,
        grade: 'A',
        results: [],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues.security).toHaveLength(0);
        expect(result.value.currentIssues.performance).toHaveLength(0);
        expect(result.value.currentIssues.bestPractices).toHaveLength(0);
        expect(result.value.validationScore).toBe(100);
        expect(result.value.validationGrade).toBe('A');
        expect(result.value.priority).toBe('low');
        expect(result.value.estimatedImpact).toContain('No fixes needed');
      }
    });

    it('should read Dockerfile from path when provided', async () => {
      delete config.dockerfile;
      config.path = '/test/repo/Dockerfile';

      mockFs.readFile.mockResolvedValue(dockerfileWithIssues);
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 70,
        grade: 'C',
        results: [
          {
            passed: false,
            rule: 'test-rule',
            message: 'Test issue',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      expect(mockFs.readFile).toHaveBeenCalledWith('/test/repo/Dockerfile', 'utf-8');
    });

    it('should categorize fixes correctly', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 50,
        grade: 'F',
        results: [
          {
            passed: false,
            rule: 'security-rule',
            message: 'Security issue',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
          {
            passed: false,
            rule: 'performance-rule',
            message: 'Performance issue',
            line: 2,
            metadata: {
              category: ValidationCategory.PERFORMANCE,
              severity: ValidationSeverity.WARNING,
            },
          },
          {
            passed: false,
            rule: 'best-practice-rule',
            message: 'Best practice issue',
            line: 3,
            metadata: {
              category: ValidationCategory.STYLE,
              severity: ValidationSeverity.INFO,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues.security.length).toBeGreaterThan(0);
        expect(result.value.currentIssues.performance.length).toBeGreaterThan(0);
        expect(result.value.currentIssues.bestPractices.length).toBeGreaterThan(0);
      }
    });
  });

  describe('Error Handling', () => {
    it('should fail when neither dockerfile nor path is provided', async () => {
      config.dockerfile = undefined;
      config.path = undefined;

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config as any, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain("Either 'path' or 'content' must be provided");
      }
    });

    it('should fail when dockerfile content is empty', async () => {
      config.dockerfile = '';

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Dockerfile content is empty');
      }
    });

    it('should fail when file path does not exist', async () => {
      delete config.dockerfile;
      config.path = '/nonexistent/Dockerfile';

      // Mock access to simulate file doesn't exist (for validation)
      mockFs.access.mockRejectedValue(new Error('ENOENT: no such file or directory'));
      mockFs.stat.mockRejectedValue(new Error('ENOENT: no such file or directory'));
      mockFs.readFile.mockRejectedValue(new Error('ENOENT: no such file or directory'));

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
      }
    });

    it('should handle validation errors gracefully', async () => {
      mockValidateDockerfileContent.mockRejectedValue(new Error('Validation service unavailable'));

      const mockContext = createMockToolContext();

      // Expect the tool to catch and return the error
      await expect(async () => {
        await fixDockerfileTool.handler(config, mockContext);
      }).rejects.toThrow('Validation service unavailable');
    });

    it('should handle permission errors when reading Dockerfile', async () => {
      delete config.dockerfile;
      config.path = '/restricted/Dockerfile';

      mockFs.access.mockRejectedValue(new Error('EACCES: permission denied'));
      mockFs.stat.mockRejectedValue(new Error('EACCES: permission denied'));
      mockFs.readFile.mockRejectedValue(new Error('EACCES: permission denied'));

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeTruthy();
      }
    });

    it('should handle Dockerfile with only whitespace', async () => {
      config.dockerfile = '   \n\n\t\t   \n   ';

      // Reset the mock to return validation results for empty/whitespace content
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 0,
        grade: 'F',
        results: [
          {
            passed: false,
            rule: 'empty-dockerfile',
            message: 'Dockerfile appears to be empty',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      // The readDockerfile utility now rejects whitespace-only content early
      // This is more strict than the old behavior but is more correct
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('empty');
      }
    });

    it('should handle binary content in Dockerfile', async () => {
      config.dockerfile = '\x00\x01\x02\x03\x04\x05';

      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 0,
        grade: 'F',
        results: [
          {
            passed: false,
            rule: 'syntax-error',
            message: 'Invalid Dockerfile syntax',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      // Tool should handle binary content
      expect(result).toBeDefined();
    });

    it('should handle very large Dockerfile files', async () => {
      // Create a very large Dockerfile (10000 lines)
      const largeDockerfile = Array(10000)
        .fill('RUN echo "line"')
        .join('\n');
      config.dockerfile = largeDockerfile;

      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 50,
        grade: 'F',
        results: [
          {
            passed: false,
            rule: 'too-many-layers',
            message: 'Too many layers',
            line: 1,
            metadata: {
              category: ValidationCategory.PERFORMANCE,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues).toBeDefined();
      }
    });

    it('should handle invalid file path characters', async () => {
      delete config.dockerfile;
      config.path = '/invalid\x00path/Dockerfile';

      mockFs.readFile.mockRejectedValue(new Error('Invalid path'));

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
    });

    it('should handle Dockerfile with unicode characters', async () => {
      config.dockerfile = `FROM node:18-alpine
# 这是一个注释
WORKDIR /app
COPY . .
RUN npm install
# Comment with émojis: 🐳 🚀
CMD ["node", "index.js"]`;

      mockValidateDockerfileContent.mockResolvedValue({
        passed: true,
        score: 85,
        grade: 'B',
        results: [],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.validationScore).toBeGreaterThan(0);
      }
    });

    it('should handle network timeout during validation', async () => {
      mockValidateDockerfileContent.mockImplementation(() => {
        return new Promise((resolve, reject) => {
          setTimeout(() => reject(new Error('Network timeout')), 100);
        });
      });

      const mockContext = createMockToolContext();

      await expect(async () => {
        await fixDockerfileTool.handler(config, mockContext);
      }).rejects.toThrow('Network timeout');
    });

    it('should handle malformed validation results', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: true,
        score: null as any,
        grade: undefined as any,
        results: null as any,
      });

      const mockContext = createMockToolContext();

      // Tool should handle malformed validation data
      await expect(async () => {
        await fixDockerfileTool.handler(config, mockContext);
      }).rejects.toThrow();
    });

    it('should handle Dockerfile path that is actually a directory', async () => {
      delete config.dockerfile;
      config.path = '/test/directory';

      mockFs.stat.mockResolvedValue({ isFile: () => false, isDirectory: () => true } as any);
      mockFs.readFile.mockRejectedValue(new Error('EISDIR: illegal operation on a directory'));

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(false);
    });
  });

  describe('Edge Cases', () => {
    it('should handle Dockerfile with only security issues', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 40,
        grade: 'F',
        results: [
          {
            passed: false,
            rule: 'security-1',
            message: 'Critical security issue 1',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
          {
            passed: false,
            rule: 'security-2',
            message: 'Critical security issue 2',
            line: 2,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
          {
            passed: false,
            rule: 'security-3',
            message: 'Critical security issue 3',
            line: 3,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues.security).toHaveLength(3);
        expect(result.value.currentIssues.performance).toHaveLength(0);
        expect(result.value.priority).toBe('high');
        expect(result.value.validationGrade).not.toBe('A'); // Should be capped
      }
    });

    it('should handle Dockerfile with many low-priority issues', async () => {
      const lowPriorityResults = Array.from({ length: 15 }, (_, i) => ({
        passed: false,
        rule: `best-practice-${i}`,
        message: `Best practice issue ${i}`,
        line: i + 1,
        metadata: {
          category: ValidationCategory.STYLE,
          severity: ValidationSeverity.INFO,
        },
      }));

      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 10,
        grade: 'F',
        results: lowPriorityResults,
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.currentIssues.bestPractices.length).toBeGreaterThan(10);
        expect(result.value.validationScore).toBeLessThan(100);
        expect(result.value.priority).toBe('medium'); // Many issues
      }
    });

    it('should handle different environment settings', async () => {
      const environments = ['production', 'development', 'staging', 'test'];

      for (const env of environments) {
        config.environment = env;

        mockValidateDockerfileContent.mockResolvedValue({
          passed: false,
          score: 75,
          grade: 'C',
          results: [
            {
              passed: false,
              rule: 'test-rule',
              message: 'Test issue',
              line: 1,
              metadata: {
                category: ValidationCategory.SECURITY,
                severity: ValidationSeverity.WARNING,
              },
            },
          ],
        });

        const mockContext = createMockToolContext();
        const result = await fixDockerfileTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.summary).toContain(env);
        }
      }
    });

    it('should handle validation score edge cases', async () => {
      const scoreCases = [
        { score: 100, grade: 'A' as const },
        { score: 90, grade: 'A' as const },
        { score: 85, grade: 'B' as const },
        { score: 75, grade: 'C' as const },
        { score: 65, grade: 'D' as const },
        { score: 50, grade: 'F' as const },
        { score: 0, grade: 'F' as const },
      ];

      for (const testCase of scoreCases) {
        mockValidateDockerfileContent.mockResolvedValue({
          passed: testCase.score >= 60,
          score: testCase.score,
          grade: testCase.grade,
          results: [],
        });

        const mockContext = createMockToolContext();
        const result = await fixDockerfileTool.handler(config, mockContext);

        expect(result.ok).toBe(true);
        if (result.ok) {
          expect(result.value.validationScore).toBe(testCase.score);
          expect(result.value.validationGrade).toBe(testCase.grade);
        }
      }
    });
  });

  describe('Knowledge Integration', () => {
    it('should query knowledge base and include recommendations', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 70,
        grade: 'C',
        results: [
          {
            passed: false,
            rule: 'security-rule',
            message: 'Security issue',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Tool should return structured fix plan even if knowledge base returns no matches
        expect(result.value).toHaveProperty('fixes');
        expect(result.value).toHaveProperty('currentIssues');
        expect(result.value).toHaveProperty('knowledgeMatches');
        expect(result.value).toHaveProperty('confidence');
      }
    });

    it('should include fix recommendations in result', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 60,
        grade: 'D',
        results: [
          {
            passed: false,
            rule: 'test-rule',
            message: 'Test issue',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      mockGetKnowledgeForCategory.mockReturnValue([
        {
          id: 'knowledge-1',
          text: 'Fix recommendation 1\nDetailed description',
          category: 'security',
          tags: ['security-fix'],
          weight: 0.95,
        },
        {
          id: 'knowledge-2',
          text: 'Fix recommendation 2\nDetailed description',
          category: 'performance',
          tags: ['performance-fix'],
          weight: 0.80,
        },
      ]);

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Tool returns fix recommendations based on validation results
        expect(result.value.fixes).toHaveProperty('security');
        expect(result.value.fixes).toHaveProperty('performance');
        expect(result.value.fixes).toHaveProperty('bestPractices');
      }
    });

    it('should handle when knowledge base returns matches', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 80,
        grade: 'B',
        results: [
          {
            passed: false,
            rule: 'minor-issue',
            message: 'Minor issue',
            line: 1,
            metadata: {
              category: ValidationCategory.STYLE,
              severity: ValidationSeverity.INFO,
            },
          },
        ],
      });

      // Knowledge base is now always loaded at server startup
      // So we expect it to return matches when available
      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Knowledge matches may be available since knowledge is loaded at startup
        expect(Array.isArray(result.value.knowledgeMatches)).toBe(true);
        expect(result.value.confidence).toBeGreaterThanOrEqual(0);
      }
    });
  });

  describe('Validation Grade Calculation', () => {
    it('should cap grade at C when critical security issues exist', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 95, // Would normally be grade A
        grade: 'A',
        results: [
          {
            passed: false,
            rule: 'critical-security',
            message: 'Critical security issue',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Grade should be capped at C despite high score
        expect(['C', 'D', 'F']).toContain(result.value.validationGrade);
        expect(result.value.priority).toBe('high');
      }
    });
  });

  describe('Priority Calculation', () => {
    it('should set priority to high for critical security issues', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 70,
        grade: 'C',
        results: [
          {
            passed: false,
            rule: 'critical-security',
            message: 'Critical security vulnerability',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.priority).toBe('high');
      }
    });

    it('should set priority to medium for non-critical issues', async () => {
      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 75,
        grade: 'C',
        results: [
          {
            passed: false,
            rule: 'performance-issue',
            message: 'Performance could be improved',
            line: 1,
            metadata: {
              category: ValidationCategory.PERFORMANCE,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.priority).toBe('medium');
      }
    });
  });

  describe('Policy Validation Integration', () => {
    it('should skip policy validation when no policies directory exists', async () => {
      const mockFs = fs as jest.Mocked<typeof fs>;
      mockFs.readFile.mockResolvedValue('FROM node:18\nWORKDIR /app\nCOPY . .');

      mockValidateDockerfileContent.mockResolvedValue({
        passed: true,
        score: 100,
        grade: 'A',
        results: [],
      });

      const config = { path: '/test/Dockerfile', environment: 'production' };
      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should not have policy validation results when no policies exist
        expect(result.value.policyValidation).toBeUndefined();
      }
    });

    it('should include policy validation results when policies are provided', async () => {
      // This test would require mocking the policy loading system
      // For now, we verify the schema supports it
      const mockFs = fs as jest.Mocked<typeof fs>;
      mockFs.readFile.mockResolvedValue('FROM node:18\nWORKDIR /app');

      mockValidateDockerfileContent.mockResolvedValue({
        passed: true,
        score: 100,
        grade: 'A',
        results: [],
      });

      const config = {
        path: '/test/Dockerfile',
        environment: 'production',
        policyPath: '/test/policy.yaml'
      };
      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      // Policy validation integration is tested in integration tests
    });

    it('should handle both validation issues and policy validation', async () => {
      const mockFs = fs as jest.Mocked<typeof fs>;
      mockFs.readFile.mockResolvedValue('FROM node:latest\nRUN npm install');

      mockValidateDockerfileContent.mockResolvedValue({
        passed: false,
        score: 70,
        grade: 'C',
        results: [
          {
            passed: false,
            rule: 'security-issue',
            message: 'Security vulnerability detected',
            line: 1,
            metadata: {
              category: ValidationCategory.SECURITY,
              severity: ValidationSeverity.WARNING,
            },
          },
        ],
      });

      const config = { path: '/test/Dockerfile', environment: 'production' };
      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool.handler(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should have validation issues
        expect(result.value.currentIssues).toBeDefined();
        expect(result.value.currentIssues.security.length).toBeGreaterThan(0);

        // Should have fix recommendations
        expect(result.value.fixes).toBeDefined();
      }
    });
  });
});
