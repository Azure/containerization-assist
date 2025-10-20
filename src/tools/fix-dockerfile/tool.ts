/**
 * Fix Dockerfile Tool
 *
 * Analyzes existing Dockerfile for issues and queries knowledge base to gather
 * structured fix recommendations. This tool validates the Dockerfile and returns
 * actionable fixes categorized by security, performance, and best practices.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 */

import { Failure, type Result, Success, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { getToolLogger } from '@/lib/tool-helpers';
import { LIMITS } from '@/config/constants';
import {
  fixDockerfileSchema,
  type DockerfileFixPlan,
  type FixDockerfileParams,
  type FixRecommendation,
  type ValidationIssue,
  type PolicyViolation,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import { ValidationCategory, ValidationSeverity } from '@/validation/core-types';
import { validatePathOrFail } from '@/lib/validation-helpers';
import type { z } from 'zod';
import { promises as fs, existsSync, readdirSync } from 'node:fs';
import { loadPolicy } from '@/config/policy-io';
import { applyPolicy } from '@/config/policy-eval';
import type { Policy } from '@/config/policy-schemas';
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
 * Get default policy paths from policies/ directory
 */
function getDefaultPolicyPaths(logger: ReturnType<typeof getToolLogger>): string[] {
  try {
    const policiesDir = nodePath.join(process.cwd(), 'policies');

    if (!existsSync(policiesDir)) {
      logger.debug({ policiesDir }, 'Policies directory not found');
      return [];
    }

    const files = readdirSync(policiesDir);
    return files
      .filter((f: string) => f.endsWith('.yaml') || f.endsWith('.yml'))
      .sort((a: string, b: string) => a.localeCompare(b, undefined, { numeric: true }))
      .map((f: string) => nodePath.join(policiesDir, f));
  } catch (error) {
    logger.warn(
      { error, cwd: process.cwd() },
      'Failed to read policies directory - using no policies',
    );
    return [];
  }
}

/**
 * Merge multiple policies into a single unified policy
 */
function mergePolicies(policies: Policy[]): Policy {
  if (policies.length === 0) {
    throw new Error('Cannot merge empty policy list');
  }

  if (policies.length === 1) {
    const singlePolicy = policies[0];
    if (!singlePolicy) {
      throw new Error('Unexpected: policy array is empty');
    }
    return singlePolicy;
  }

  // Merge all policies - later policies override earlier ones for rules with same ID
  const ruleMap = new Map<string, Policy['rules'][0]>();
  let mergedDefaults = {};

  for (const policy of policies) {
    // Merge defaults (later overrides earlier)
    mergedDefaults = { ...mergedDefaults, ...policy.defaults };

    // Merge rules by ID (later overrides earlier)
    for (const rule of policy.rules) {
      ruleMap.set(rule.id, rule);
    }
  }

  const merged: Policy = {
    version: '2.0',
    metadata: {
      name: 'Merged Policies',
      description: `Merged from ${policies.length} policy files`,
    },
    defaults: mergedDefaults,
    rules: Array.from(ruleMap.values()).sort((a, b) => b.priority - a.priority),
  };

  return merged;
}

/**
 * Classify matched rules by severity based on actions
 */
function classifyViolation(
  ruleId: string,
  category: string | undefined,
  priority: number,
  actions: Record<string, unknown>,
  description?: string,
): PolicyViolation | null {
  // Check for blocking actions
  if (actions.block === true || actions.block_deployment === true || actions.block_build === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'block',
      message: (actions.message as string) || description || `Rule ${ruleId} violated`,
    };
  }

  // Check for warning actions
  if (actions.warn === true || actions.require_approval === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'warn',
      message: (actions.message as string) || description || `Rule ${ruleId} triggered warning`,
    };
  }

  // Check for suggestion actions
  if (actions.suggest === true || actions.suggest_pinned_version === true) {
    return {
      ruleId,
      category,
      priority,
      severity: 'suggest',
      message: (actions.message as string) || description || `Rule ${ruleId} suggests improvement`,
    };
  }

  // Rule matched but no actionable severity
  return null;
}

/**
 * Run policy validation on Dockerfile content
 */
function runPolicyValidation(
  content: string,
  policyPath: string | undefined,
  logger: ReturnType<typeof getToolLogger>,
): DockerfileFixPlan['policyValidation'] | undefined {
  // Load policies
  const policyPaths = policyPath ? [policyPath] : getDefaultPolicyPaths(logger);

  if (policyPaths.length === 0) {
    logger.debug('No policy files found - skipping policy validation');
    return undefined;
  }

  const policies: Policy[] = [];
  for (const policyFile of policyPaths) {
    const policyResult = loadPolicy(policyFile);
    if (policyResult.ok) {
      policies.push(policyResult.value);
      logger.debug({ policyFile, rulesCount: policyResult.value.rules.length }, 'Loaded policy');
    } else {
      logger.warn({ policyFile, error: policyResult.error }, 'Failed to load policy file');
    }
  }

  if (policies.length === 0) {
    logger.warn('No valid policies could be loaded - skipping policy validation');
    return undefined;
  }

  // Merge all policies
  const mergedPolicy = mergePolicies(policies);

  logger.info(
    {
      policiesLoaded: policies.length,
      totalRules: mergedPolicy.rules.length,
    },
    'Running policy validation',
  );

  // Apply policy to Dockerfile content
  const policyResults = applyPolicy(mergedPolicy, content);

  // Classify matched rules
  const violations: PolicyViolation[] = [];
  const warnings: PolicyViolation[] = [];
  const suggestions: PolicyViolation[] = [];

  let matchedRulesCount = 0;

  for (const result of policyResults) {
    if (!result.matched) continue;

    matchedRulesCount++;
    const violation = classifyViolation(
      result.rule.id,
      result.rule.category,
      result.rule.priority,
      result.rule.actions,
      result.rule.description,
    );

    if (!violation) continue;

    switch (violation.severity) {
      case 'block':
        violations.push(violation);
        break;
      case 'warn':
        warnings.push(violation);
        break;
      case 'suggest':
        suggestions.push(violation);
        break;
    }
  }

  const passed = violations.length === 0;

  logger.info(
    {
      totalRules: mergedPolicy.rules.length,
      matchedRules: matchedRulesCount,
      passed,
      violations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
    'Policy validation completed',
  );

  return {
    passed,
    violations,
    warnings,
    suggestions,
    summary: {
      totalRules: mergedPolicy.rules.length,
      matchedRules: matchedRulesCount,
      blockingViolations: violations.length,
      warnings: warnings.length,
      suggestions: suggestions.length,
    },
  };
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

interface ExtendedInput extends FixDockerfileParams {
  _validationResults?: ValidationIssue[];
  _dockerfileContent?: string;
}

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
    maxChars: LIMITS.MAX_PROMPT_CHARS,
    maxSnippets: LIMITS.MAX_PROMPT_SNIPPETS,
    extractFilters: (input) => {
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

      const securityIssues = issues.filter((i) => i.category === 'security');
      const performanceIssues = issues.filter((i) => i.category === 'performance');
      const bestPracticeIssues = issues.filter((i) => i.category === 'bestPractices');

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

      const estimatedImpact = [
        `Fixing ${rules.issueCount} issue(s) will improve:`,
        `- Security: ${securityIssues.length} fix(es) - ${rules.hasCriticalSecurity ? 'Critical' : 'Minor'} impact`,
        `- Performance: ${performanceIssues.length} fix(es) - ${rules.hasPerformanceIssues ? 'Moderate' : 'Minor'} impact`,
        `- Best Practices: ${bestPracticeIssues.length} fix(es) - Improved maintainability`,
      ].join('\n');

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
async function handleFixDockerfile(
  input: z.infer<typeof fixDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfileFixPlan>> {
  const logger = getToolLogger(ctx, 'fix-dockerfile');
  let content = input.dockerfile || '';

  if (input.path) {
    // Validate path upfront
    const pathValidation = await validatePathOrFail(input.path, {
      mustExist: true,
      mustBeFile: true,
      readable: true,
    });
    if (!pathValidation.ok) return pathValidation;

    const dockerfilePath = pathValidation.value;
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

  logger.info({ preview: content.substring(0, 100) }, 'Validating Dockerfile for issues');

  const validationReport = await validateDockerfileContent(content, {
    enableExternalLinter: true,
  });

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

  logger.info(
    {
      issueCount: validationIssues.length,
      score: validationReport.score,
      grade: validationReport.grade,
    },
    'Dockerfile validation completed',
  );

  // Run policy validation if policies exist
  const policyValidation = runPolicyValidation(content, input.policyPath, logger);

  if (validationIssues.length === 0 && (!policyValidation || policyValidation.passed)) {
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
      ...(policyValidation && { policyValidation }),
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

  const extendedInput: ExtendedInput = {
    ...input,
    _validationResults: validationIssues,
    _dockerfileContent: content,
  };

  const result = await runPattern(extendedInput, ctx);

  // Add policy validation to the result
  if (result.ok) {
    const plan: DockerfileFixPlan = {
      ...result.value,
      ...(policyValidation && { policyValidation }),
    };
    return Success(plan);
  }

  return result;
}

import { tool } from '@/types/tool';

export default tool({
  name,
  description,
  category: 'docker',
  version,
  schema: fixDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
  },
  chainHints: {
    success:
      'Dockerfile validation and analysis complete (includes built-in best practices + organizational policy validation if configured). Next: Apply recommended fixes, then call build-image to test the Dockerfile.',
    failure:
      'Dockerfile validation failed. Review validation errors, policy violations (if any), and apply recommended fixes.',
  },
  handler: handleFixDockerfile,
});
