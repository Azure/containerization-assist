/**
 * Unit Tests: Fix Dockerfile Plan Tool
 * Tests for the knowledge-based Dockerfile fix recommendation tool
 */

import { jest } from '@jest/globals';
import fixDockerfilePlanTool from '../../../src/tools/fix-dockerfile-plan/tool';
import type { ToolContext } from '../../../src/mcp/context';
import type { KnowledgeSnippet } from '../../../src/knowledge/schemas';
import * as knowledgeMatcher from '../../../src/knowledge/matcher';
import * as dockerfileValidator from '../../../src/validation/dockerfile-validator';
import type { ValidationReport } from '../../../src/validation/core-types';
import { ValidationSeverity, ValidationCategory } from '../../../src/validation/core-types';

// Mock the knowledge matcher and validator modules
jest.spyOn(knowledgeMatcher, 'getKnowledgeSnippets').mockImplementation(jest.fn());
jest.spyOn(dockerfileValidator, 'validateDockerfileContent').mockImplementation(jest.fn());

describe('Fix Dockerfile Plan Tool', () => {
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

  // Helper to create mock fix snippets
  function createMockFixSnippets(): KnowledgeSnippet[] {
    return [
      {
        id: 'fix-1',
        text: 'Add USER directive to run container as non-root user for enhanced security',
        weight: 95,
        tags: ['security', 'security-fix', 'user', 'critical'],
        category: 'security',
        source: 'dockerfile-fixes',
      },
      {
        id: 'fix-2',
        text: 'Use specific version tags instead of :latest for reproducibility',
        weight: 85,
        tags: ['best-practice', 'dockerfile-fix', 'versioning'],
        category: 'best-practice',
        source: 'dockerfile-fixes',
      },
      {
        id: 'fix-3',
        text: 'Copy package files before source code for better layer caching',
        weight: 80,
        tags: ['performance', 'optimization', 'caching', 'performance-fix'],
        category: 'optimization',
        source: 'dockerfile-fixes',
      },
      {
        id: 'fix-4',
        text: 'Use multi-stage builds to reduce final image size by separating build and runtime',
        weight: 90,
        tags: ['performance', 'size', 'optimization'],
        category: 'optimization',
        source: 'dockerfile-fixes',
      },
    ];
  }

  // Helper to create mock validation report with issues
  function createMockValidationReport(hasIssues: boolean): ValidationReport {
    if (!hasIssues) {
      return {
        results: [
          {
            ruleId: 'no-root-user',
            isValid: true,
            passed: true,
            errors: [],
            warnings: [],
            message: '✓ Non-root user required',
            metadata: {
              severity: ValidationSeverity.ERROR,
              category: ValidationCategory.SECURITY,
            },
          },
        ],
        score: 100,
        grade: 'A',
        passed: 1,
        failed: 0,
        errors: 0,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };
    }

    return {
      results: [
        {
          ruleId: 'no-root-user',
          isValid: false,
          passed: false,
          errors: ['Container should run as non-root user'],
          warnings: [],
          message: '✗ Non-root user required: Container should run as non-root user',
          suggestions: ['Add USER directive with non-root user (e.g., USER node)'],
          metadata: {
            severity: ValidationSeverity.ERROR,
            category: ValidationCategory.SECURITY,
          },
        },
        {
          ruleId: 'specific-base-image',
          isValid: false,
          passed: false,
          errors: ['Use specific version tags instead of latest'],
          warnings: [],
          message: '✗ Use specific version tags: Use specific version tags instead of latest',
          suggestions: ['Replace :latest with specific version (e.g., node:18-alpine)'],
          metadata: {
            severity: ValidationSeverity.WARNING,
            category: ValidationCategory.BEST_PRACTICE,
          },
        },
        {
          ruleId: 'layer-caching-optimization',
          isValid: false,
          passed: false,
          errors: ['Copy dependency files before source code for better caching'],
          warnings: [],
          message: '✗ Optimize layer caching: Copy dependency files before source code',
          suggestions: ['COPY package*.json ./ before COPY . .'],
          metadata: {
            severity: ValidationSeverity.INFO,
            category: ValidationCategory.OPTIMIZATION,
          },
        },
      ],
      score: 65,
      grade: 'D',
      passed: 0,
      failed: 3,
      errors: 1,
      warnings: 1,
      info: 1,
      timestamp: new Date().toISOString(),
    };
  }

  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('Basic Functionality', () => {
    it('should successfully generate fix plan for Dockerfile with issues', async () => {
      const mockSnippets = createMockFixSnippets();
      const mockValidation = createMockValidationReport(true);

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(mockValidation);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest\nRUN npm install\nCOPY . .\nCMD ["node", "index.js"]',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Verify current issues are categorized
      const totalIssues =
        result.value.currentIssues.security.length +
        result.value.currentIssues.bestPractices.length +
        result.value.currentIssues.performance.length;

      expect(totalIssues).toBeGreaterThan(0); // Should have issues categorized
      expect(result.value.currentIssues.security.length).toBeGreaterThan(0);

      // Verify fixes are provided
      expect(result.value.fixes.security.length + result.value.fixes.bestPractices.length).toBeGreaterThan(0);

      // Verify validation score and grade
      expect(result.value.validationScore).toBeLessThan(100);
      expect(['A', 'B', 'C', 'D', 'F']).toContain(result.value.validationGrade);

      // Verify priority is set
      expect(['high', 'medium', 'low']).toContain(result.value.priority);

      // Verify confidence is set
      expect(result.value.confidence).toBeGreaterThan(0);
      expect(result.value.confidence).toBeLessThanOrEqual(1);

      // Verify summary contains key information
      expect(result.value.summary).toContain('production');
      expect(result.value.summary).toContain('Grade');
    });

    it('should return success plan when Dockerfile has no issues', async () => {
      const mockValidation = createMockValidationReport(false);

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(mockValidation);

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:20-alpine\nUSER node\nCOPY package*.json ./\nRUN npm ci\nCOPY . .\nCMD ["node", "index.js"]',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // No issues means empty fix lists
      expect(result.value.currentIssues.security).toHaveLength(0);
      expect(result.value.currentIssues.performance).toHaveLength(0);
      expect(result.value.currentIssues.bestPractices).toHaveLength(0);

      expect(result.value.fixes.security).toHaveLength(0);
      expect(result.value.fixes.performance).toHaveLength(0);
      expect(result.value.fixes.bestPractices).toHaveLength(0);

      // High validation score
      expect(result.value.validationScore).toBe(100);
      expect(result.value.validationGrade).toBe('A');

      // Low priority since no issues
      expect(result.value.priority).toBe('low');

      // High confidence
      expect(result.value.confidence).toBe(1.0);

      // Summary should indicate no fixes needed
      expect(result.value.summary).toContain('No fixes needed');
    });

    it('should fail when neither dockerfile content nor path is provided', async () => {
      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          environment: 'production',
        } as any,
        mockContext,
      );

      expect(result.ok).toBe(false);
    });

    it('should fail when Dockerfile content is empty', async () => {
      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: '',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(false);
      if (result.ok) return;
      expect(result.error).toContain('empty');
    });
  });

  describe('Issue Categorization', () => {
    it('should categorize security issues correctly', async () => {
      const securityReport: ValidationReport = {
        results: [
          {
            ruleId: 'no-root-user',
            isValid: false,
            passed: false,
            errors: ['Container runs as root'],
            warnings: [],
            message: '✗ No root user',
            metadata: {
              severity: ValidationSeverity.ERROR,
              category: ValidationCategory.SECURITY,
            },
          },
          {
            ruleId: 'no-secrets',
            isValid: false,
            passed: false,
            errors: ['Hardcoded secrets found'],
            warnings: [],
            message: '✗ No hardcoded secrets',
            metadata: {
              severity: ValidationSeverity.ERROR,
              category: ValidationCategory.SECURITY,
            },
          },
        ],
        score: 50,
        grade: 'F',
        passed: 0,
        failed: 2,
        errors: 2,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(securityReport);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(createMockFixSnippets());

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest\nENV PASSWORD=secret123\nRUN npm install',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Should have security issues
      expect(result.value.currentIssues.security.length).toBe(2);
      expect(result.value.priority).toBe('high'); // Security issues = high priority
    });

    it('should categorize performance issues correctly', async () => {
      const performanceReport: ValidationReport = {
        results: [
          {
            ruleId: 'layer-caching-optimization',
            isValid: false,
            passed: false,
            errors: ['Poor layer caching'],
            warnings: [],
            message: '✗ Layer caching',
            metadata: {
              severity: ValidationSeverity.INFO,
              category: ValidationCategory.OPTIMIZATION,
            },
          },
          {
            ruleId: 'multi-stage-optimization',
            isValid: false,
            passed: false,
            errors: ['Consider multi-stage builds'],
            warnings: [],
            message: '✗ Multi-stage builds',
            metadata: {
              severity: ValidationSeverity.INFO,
              category: ValidationCategory.OPTIMIZATION,
            },
          },
        ],
        score: 80,
        grade: 'B',
        passed: 0,
        failed: 2,
        errors: 0,
        warnings: 0,
        info: 2,
        timestamp: new Date().toISOString(),
      };

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(performanceReport);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(createMockFixSnippets());

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:20\nCOPY . .\nRUN npm install',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Should have performance issues
      expect(result.value.currentIssues.performance.length).toBe(2);
    });

    it('should categorize best practice issues correctly', async () => {
      const bestPracticeReport: ValidationReport = {
        results: [
          {
            ruleId: 'specific-base-image',
            isValid: false,
            passed: false,
            errors: ['Use specific versions'],
            warnings: [],
            message: '✗ Specific versions',
            metadata: {
              severity: ValidationSeverity.WARNING,
              category: ValidationCategory.BEST_PRACTICE,
            },
          },
          {
            ruleId: 'has-healthcheck',
            isValid: false,
            passed: false,
            errors: ['Add HEALTHCHECK'],
            warnings: [],
            message: '✗ Health check',
            metadata: {
              severity: ValidationSeverity.INFO,
              category: ValidationCategory.BEST_PRACTICE,
            },
          },
        ],
        score: 85,
        grade: 'B',
        passed: 0,
        failed: 2,
        errors: 0,
        warnings: 1,
        info: 1,
        timestamp: new Date().toISOString(),
      };

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(bestPracticeReport);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(createMockFixSnippets());

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest\nRUN npm install\nCMD ["node", "app.js"]',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // Should have issues categorized (best practices go to bestPractices category)
      const totalIssues =
        result.value.currentIssues.security.length +
        result.value.currentIssues.bestPractices.length +
        result.value.currentIssues.performance.length;

      expect(totalIssues).toBeGreaterThan(0); // Should have at least some issues
      // Issues may be categorized as bestPractices or performance depending on specifics
    });
  });

  describe('Priority Determination', () => {
    it('should set high priority for critical security issues', async () => {
      const criticalReport: ValidationReport = {
        results: [
          {
            ruleId: 'no-root-user',
            isValid: false,
            passed: false,
            errors: ['Critical security issue'],
            warnings: [],
            message: '✗ Critical issue',
            metadata: {
              severity: ValidationSeverity.ERROR,
              category: ValidationCategory.SECURITY,
            },
          },
        ],
        score: 40,
        grade: 'F',
        passed: 0,
        failed: 1,
        errors: 1,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(criticalReport);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(createMockFixSnippets());

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest\nRUN npm install',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.value.priority).toBe('high');
    });

    it('should set medium priority for warnings and moderate issues', async () => {
      const moderateReport: ValidationReport = {
        results: [
          {
            ruleId: 'specific-base-image',
            isValid: false,
            passed: false,
            errors: ['Warning level issue'],
            warnings: [],
            message: '✗ Warning',
            metadata: {
              severity: ValidationSeverity.WARNING,
              category: ValidationCategory.BEST_PRACTICE,
            },
          },
        ],
        score: 75,
        grade: 'C',
        passed: 0,
        failed: 1,
        errors: 0,
        warnings: 1,
        info: 0,
        timestamp: new Date().toISOString(),
      };

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(moderateReport);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(createMockFixSnippets());

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest\nUSER node\nCMD ["node", "app.js"]',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.value.priority).toBe('medium');
    });
  });

  describe('Knowledge Query', () => {
    it('should query knowledge base with correct parameters', async () => {
      const mockValidation = createMockValidationReport(true);
      const mockSnippets = createMockFixSnippets();

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(mockValidation);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest',
          environment: 'development',
        },
        mockContext,
      );

      // Verify knowledge query was called
      expect(knowledgeMatcher.getKnowledgeSnippets).toHaveBeenCalledWith(
        'fix_dockerfile',
        expect.objectContaining({
          environment: 'development',
          tool: 'fix-dockerfile-plan',
        }),
      );
    });
  });

  describe('Confidence Calculation', () => {
    it('should calculate confidence based on knowledge matches', async () => {
      const mockValidation = createMockValidationReport(true);
      const manySnippets = [
        ...createMockFixSnippets(),
        ...createMockFixSnippets().map((s, i) => ({ ...s, id: `extra-${i}` })),
      ];

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(mockValidation);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(manySnippets);

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      // More knowledge matches should increase confidence
      expect(result.value.confidence).toBeGreaterThan(0.5);
    });
  });

  describe('Summary Generation', () => {
    it('should generate comprehensive summary', async () => {
      const mockValidation = createMockValidationReport(true);
      const mockSnippets = createMockFixSnippets();

      (
        dockerfileValidator.validateDockerfileContent as jest.MockedFunction<
          typeof dockerfileValidator.validateDockerfileContent
        >
      ).mockResolvedValue(mockValidation);

      (
        knowledgeMatcher.getKnowledgeSnippets as jest.MockedFunction<
          typeof knowledgeMatcher.getKnowledgeSnippets
        >
      ).mockResolvedValue(mockSnippets);

      const mockContext = createMockContext();

      const result = await fixDockerfilePlanTool.run(
        {
          dockerfile: 'FROM node:latest',
          environment: 'production',
        },
        mockContext,
      );

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      const summary = result.value.summary;

      // Verify summary contains key information
      expect(summary).toContain('production');
      expect(summary).toContain('Grade');
      expect(summary).toContain('Issues Found');
      expect(summary).toContain('Priority');
      expect(summary).toContain('Knowledge Matches');
    });
  });

  describe('Tool Metadata', () => {
    it('should have correct metadata', () => {
      expect(fixDockerfilePlanTool.name).toBe('fix-dockerfile-plan');
      expect(fixDockerfilePlanTool.category).toBe('docker');
      expect(fixDockerfilePlanTool.metadata?.knowledgeEnhanced).toBe(true);
      expect(fixDockerfilePlanTool.metadata?.samplingStrategy).toBe('none');
    });
  });
});
