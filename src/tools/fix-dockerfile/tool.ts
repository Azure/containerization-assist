/**
 * Fix Dockerfile Tool
 *
 * Analyzes existing Dockerfile for issues and queries knowledge base to gather
 * structured fix recommendations. This tool validates the Dockerfile and returns
 * actionable fixes categorized by security, performance, and best practices.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 *
 * @category docker
 * @version 2.0.0
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Failure, type Result, Success, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import {
  fixDockerfileSchema,
  type DockerfileFixPlan,
  type FixDockerfileParams,
  type FixRecommendation,
  type ValidationIssue,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import { ValidationCategory, ValidationSeverity } from '@/validation/core-types';
import type { z } from 'zod';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';

const name = 'fix-dockerfile';
const description = 'Analyze Dockerfile for issues and return knowledge-based fix recommendations';
const version = '2.0.0';

// Score calculation constant
const SCORE_PENALTY_PER_ISSUE = 10;

// Define category types for better type safety
type FixCategory = 'security' | 'performance' | 'bestPractices';

// Define rule results interface
interface DockerfileFixRules {
  hasCriticalSecurity: boolean;
  hasPerformanceIssues: boolean;
  hasBestPracticeIssues: boolean;
  overallPriority: 'high' | 'medium' | 'low';
  issueCount: number;
}

/**
 * Map validation issue to fix recommendation category
 */
function mapValidationCategory(category?: string): FixCategory {
  if (!category) {
    return 'bestPractices';
  }

  // Normalize to handle both enum values and string values
  const normalized = category.toLowerCase();

  if (normalized === 'security' || normalized === ValidationCategory.SECURITY.toLowerCase()) {
    return 'security';
  }

  if (
    normalized === 'performance' ||
    normalized === 'optimization' ||
    normalized === ValidationCategory.PERFORMANCE.toLowerCase() ||
    normalized === ValidationCategory.OPTIMIZATION.toLowerCase()
  ) {
    return 'performance';
  }

  // Best practices, compliance, or anything else
  return 'bestPractices';
}

/**
 * Determine priority from validation severity
 */
function getPriority(severity?: ValidationSeverity): 'high' | 'medium' | 'low' {
  switch (severity) {
    case ValidationSeverity.ERROR:
      return 'high';
    case ValidationSeverity.WARNING:
      return 'medium';
    case ValidationSeverity.INFO:
    default:
      return 'low';
  }
}

// Extended input type that includes validation results
interface ExtendedInput extends FixDockerfileParams {
  _validationResults?: ValidationIssue[];
  _dockerfileContent?: string;
}

// Create the tool runner using the shared pattern
const runPattern = createKnowledgeTool<
  ExtendedInput,
  DockerfileFixPlan,
  FixCategory,
  DockerfileFixRules
>({
  name,
  query: {
    topic: TOPICS.FIX_DOCKERFILE,
    category: CATEGORY.DOCKERFILE,
    maxChars: 5000,
    maxSnippets: 25,
    extractFilters: (input) => {
      // Extract issue categories to filter knowledge
      const issues = input._validationResults || [];
      const hasSecurityIssues = issues.some((i) => i.category === 'security');
      const hasPerformanceIssues = issues.some((i) => i.category === 'performance');

      return {
        environment: input.environment || 'production',
        // Use tags to filter for relevant fix knowledge
        tags: [
          hasSecurityIssues ? 'security-fix' : '',
          hasPerformanceIssues ? 'performance-fix' : '',
          'dockerfile-fix',
        ].filter(Boolean),
      };
    },
  },
  categorization: {
    categoryNames: ['security', 'performance', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<FixCategory>({
      security: (s) =>
        Boolean(
          s.category === 'security' ||
            s.tags?.includes('security') ||
            s.tags?.includes('security-fix'),
        ),
      performance: (s) =>
        Boolean(
          s.tags?.includes('performance') ||
            s.tags?.includes('optimization') ||
            s.tags?.includes('caching') ||
            s.tags?.includes('size') ||
            s.tags?.includes('performance-fix'),
        ),
      bestPractices: () => true, // Catch remaining snippets as best practices
    }),
  },
  rules: {
    applyRules: (input) => {
      const issues = input._validationResults || [];

      const securityIssues = issues.filter((i) => i.category === 'security');
      const performanceIssues = issues.filter((i) => i.category === 'performance');
      const bestPracticeIssues = issues.filter((i) => i.category === 'bestPractices');

      const hasCriticalSecurity = securityIssues.some((i) => i.priority === 'high');
      const hasPerformanceIssues = performanceIssues.length > 0;
      const hasBestPracticeIssues = bestPracticeIssues.length > 0;

      // Determine overall priority
      let overallPriority: 'high' | 'medium' | 'low' = 'low';
      if (hasCriticalSecurity || securityIssues.length > 2) {
        overallPriority = 'high';
      } else if (
        hasPerformanceIssues ||
        securityIssues.length > 0 ||
        bestPracticeIssues.length > 0
      ) {
        overallPriority = 'medium';
      }

      return {
        hasCriticalSecurity,
        hasPerformanceIssues,
        hasBestPracticeIssues,
        overallPriority,
        issueCount: issues.length,
      };
    },
  },
  plan: {
    buildPlan: (input, knowledge, rules, confidence) => {
      const issues = input._validationResults || [];

      // Categorize issues
      const securityIssues = issues.filter((i) => i.category === 'security');
      const performanceIssues = issues.filter((i) => i.category === 'performance');
      const bestPracticeIssues = issues.filter((i) => i.category === 'bestPractices');

      // Map knowledge snippets to fix recommendations
      const knowledgeMatches: FixRecommendation[] = knowledge.all.map((snippet) => {
        const category = mapValidationCategory(snippet.category);
        return {
          id: snippet.id,
          category,
          title: snippet.text.split('\n')[0] || snippet.text.substring(0, 80),
          description: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          priority: (snippet.tags?.includes('critical') ? 'high' : 'medium') as
            | 'high'
            | 'medium'
            | 'low',
          matchScore: snippet.weight,
        };
      });

      // Create categorized fix recommendations
      const securityFixes: FixRecommendation[] = (knowledge.categories.security || []).map(
        (snippet) => ({
          id: snippet.id,
          category: 'security' as const,
          title: snippet.text.split('\n')[0] || snippet.text.substring(0, 80),
          description: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          priority: snippet.tags?.includes('critical') ? ('high' as const) : ('medium' as const),
          matchScore: snippet.weight,
        }),
      );

      const performanceFixes: FixRecommendation[] = (knowledge.categories.performance || []).map(
        (snippet) => ({
          id: snippet.id,
          category: 'performance' as const,
          title: snippet.text.split('\n')[0] || snippet.text.substring(0, 80),
          description: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          priority: 'medium' as const,
          matchScore: snippet.weight,
        }),
      );

      const bestPracticeFixes: FixRecommendation[] = (knowledge.categories.bestPractices || [])
        .filter((snippet) => {
          // Exclude snippets already in security or performance
          const isInSecurity = (knowledge.categories.security || []).some(
            (s) => s.id === snippet.id,
          );
          const isInPerformance = (knowledge.categories.performance || []).some(
            (s) => s.id === snippet.id,
          );
          return !isInSecurity && !isInPerformance;
        })
        .map((snippet) => ({
          id: snippet.id,
          category: 'bestPractices' as const,
          title: snippet.text.split('\n')[0] || snippet.text.substring(0, 80),
          description: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          priority: 'low' as const,
          matchScore: snippet.weight,
        }));

      // Calculate validation score and grade (from validation report if available)
      const validationScore =
        issues.length === 0 ? 100 : Math.max(0, 100 - issues.length * SCORE_PENALTY_PER_ISSUE);
      let validationGrade: 'A' | 'B' | 'C' | 'D' | 'F' = 'F';
      if (validationScore >= 90) validationGrade = 'A';
      else if (validationScore >= 80) validationGrade = 'B';
      else if (validationScore >= 70) validationGrade = 'C';
      else if (validationScore >= 60) validationGrade = 'D';

      // Cap grade at C if critical security issues
      if (rules.hasCriticalSecurity && (validationGrade === 'A' || validationGrade === 'B')) {
        validationGrade = 'C';
      }

      // Estimated impact
      const estimatedImpact = `Fixing ${rules.issueCount} issue(s) will improve:
- Security: ${securityIssues.length} fix(es) - ${rules.hasCriticalSecurity ? 'Critical' : 'Minor'} impact
- Performance: ${performanceIssues.length} fix(es) - ${rules.hasPerformanceIssues ? 'Moderate' : 'Minor'} impact
- Best Practices: ${bestPracticeIssues.length} fix(es) - Improved maintainability`;

      const summary = `
Dockerfile Fix Planning Summary:
- Environment: ${input.environment || 'production'}
- Validation Score: ${validationScore}/100 (Grade: ${validationGrade})
- Issues Found: ${rules.issueCount} (${securityIssues.length} security, ${performanceIssues.length} performance, ${bestPracticeIssues.length} best practices)
- Overall Priority: ${rules.overallPriority.toUpperCase()}
- Knowledge Matches: ${knowledgeMatches.length} fix recommendations found
  - Security Fixes: ${securityFixes.length}
  - Performance Fixes: ${performanceFixes.length}
  - Best Practice Fixes: ${bestPracticeFixes.length}
      `.trim();

      return {
        currentIssues: {
          security: securityIssues,
          performance: performanceIssues,
          bestPractices: bestPracticeIssues,
        },
        fixes: {
          security: securityFixes,
          performance: performanceFixes,
          bestPractices: bestPracticeFixes,
        },
        validationScore,
        validationGrade,
        priority: rules.overallPriority,
        estimatedImpact,
        knowledgeMatches,
        confidence,
        summary,
      };
    },
  },
});

/**
 * Main run function with validation preprocessing
 */
async function run(
  input: z.infer<typeof fixDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfileFixPlan>> {
  // Get Dockerfile content from either path or direct content
  let content = input.dockerfile || '';

  if (input.path) {
    const dockerfilePath = nodePath.isAbsolute(input.path)
      ? input.path
      : nodePath.resolve(process.cwd(), input.path);
    try {
      content = await fs.readFile(dockerfilePath, 'utf-8');
    } catch (error) {
      return Failure(
        `Failed to read Dockerfile at ${dockerfilePath}: ${error instanceof Error ? error.message : String(error)}`,
      );
    }
  }

  if (!content) {
    return Failure('Dockerfile content is empty. Provide valid Dockerfile content or path.');
  }

  ctx.logger.info({ preview: content.substring(0, 100) }, 'Validating Dockerfile for issues');

  // Step 1: Validate Dockerfile to identify issues
  const validationReport = await validateDockerfileContent(content, {
    enableExternalLinter: true,
  });

  // Step 2: Categorize issues
  const validationIssues: ValidationIssue[] = validationReport.results
    .filter((r) => !r.passed)
    .map((result) => {
      const category = mapValidationCategory(result.metadata?.category);
      const priority = getPriority(result.metadata?.severity);

      return {
        ...result,
        category,
        priority,
      };
    });

  ctx.logger.info(
    {
      issueCount: validationIssues.length,
      score: validationReport.score,
      grade: validationReport.grade,
    },
    'Dockerfile validation completed',
  );

  // If no issues, return a success plan with no fixes needed
  if (validationIssues.length === 0) {
    return Success({
      currentIssues: {
        security: [],
        performance: [],
        bestPractices: [],
      },
      fixes: {
        security: [],
        performance: [],
        bestPractices: [],
      },
      validationScore: validationReport.score,
      validationGrade: validationReport.grade,
      priority: 'low',
      estimatedImpact: 'Dockerfile is already well-optimized. No fixes needed.',
      knowledgeMatches: [],
      confidence: 1.0,
      summary: `
Dockerfile Fix Planning Summary:
- Environment: ${input.environment || 'production'}
- Validation Score: ${validationReport.score}/100 (Grade: ${validationReport.grade})
- Issues Found: 0
- Overall Priority: LOW
- Status: Dockerfile passes all validation checks! No fixes needed.
      `.trim(),
    });
  }

  // Step 3: Query knowledge base for fixes using the pattern
  const extendedInput: ExtendedInput = {
    ...input,
    _validationResults: validationIssues,
    _dockerfileContent: content,
  };

  return runPattern(extendedInput, ctx);
}

const tool: MCPTool<typeof fixDockerfileSchema, DockerfileFixPlan> = {
  name,
  description,
  category: 'docker',
  version,
  schema: fixDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['validation', 'recommendations'],
  },
  run,
};

export default tool;
