/**
 * Base scorer interfaces and abstract class for unified scoring across the codebase
 */

/**
 * Scoring result interface that provides both total and breakdown scores
 */
export interface ScoringResult {
  /** Total score (0-100) */
  total: number;
  /** Breakdown scores by category */
  breakdown: Record<string, number>;
  /** Optional feedback messages */
  feedback?: string[];
}

/**
 * Common scoring criteria used across different content types
 */
export interface ScoringCriteria {
  /** How well the content can be parsed/validated (0-100) */
  parseability: number;
  /** Security-related score (0-100) */
  security: number;
  /** Best practices adherence score (0-100) */
  bestPractices: number;
  /** Performance/optimization score (0-100) */
  performance: number;
}

/**
 * Context information passed to scorers
 */
export interface ScoringContext {
  /** Type of content being scored */
  contentType?: string;
  /** Additional options for scoring */
  options?: Record<string, unknown>;
  /** Focus area for scoring (security, performance, etc.) */
  focus?: string;
}

/**
 * Abstract base class for content scorers
 * Provides a unified interface while allowing specific implementations
 */
export abstract class BaseScorer<T = string> {
  /**
   * Score the given content and return detailed results
   * @param content Content to score
   * @param context Optional context for scoring
   * @returns Scoring result with total and breakdown
   */
  abstract score(content: T, context?: ScoringContext): ScoringResult;

  /**
   * Validate content parseability - implemented by subclasses
   * @param content Content to validate
   * @returns Whether content is parseable
   */
  protected abstract validateParseable(content: T): boolean;

  /**
   * Normalize a score to 0-100 range
   * @param score Raw score
   * @param maxScore Maximum possible score for this category
   * @returns Normalized score (0-100)
   */
  protected normalizeScore(score: number, maxScore: number): number {
    return Math.min(100, Math.max(0, Math.round((score / maxScore) * 100)));
  }

  /**
   * Calculate total score from breakdown scores
   * @param breakdown Score breakdown by category
   * @param weights Optional weights for each category (defaults to equal weight)
   * @returns Weighted total score
   */
  protected calculateTotal(
    breakdown: Record<string, number>,
    weights?: Record<string, number>,
  ): number {
    const categories = Object.keys(breakdown);
    const defaultWeight = 1 / categories.length;

    let totalScore = 0;
    let totalWeight = 0;

    for (const category of categories) {
      const weight = weights?.[category] ?? defaultWeight;
      const score = breakdown[category];
      if (score !== undefined) {
        totalScore += score * weight;
        totalWeight += weight;
      }
    }

    return totalWeight > 0 ? Math.round(totalScore / totalWeight) : 0;
  }

  /**
   * Create a scoring result from breakdown scores
   * @param breakdown Score breakdown by category
   * @param weights Optional weights for calculating total
   * @param feedback Optional feedback messages
   * @returns Complete scoring result
   */
  protected createResult(
    breakdown: Record<string, number>,
    weights?: Record<string, number>,
    feedback?: string[],
  ): ScoringResult {
    return {
      total: this.calculateTotal(breakdown, weights),
      breakdown,
      feedback: feedback || [],
    };
  }
}

/**
 * Specialized base class for content that has text parsing requirements
 */
export abstract class TextContentScorer extends BaseScorer<string> {
  /**
   * Check if content matches expected patterns
   * @param content Text content
   * @param patterns Array of regex patterns to match
   * @returns Number of patterns matched
   */
  protected countPatternMatches(content: string, patterns: RegExp[]): number {
    return patterns.filter((pattern) => pattern.test(content)).length;
  }

  /**
   * Check if content contains security anti-patterns
   * @param content Text content
   * @returns Array of security issues found
   */
  protected findSecurityIssues(content: string): string[] {
    const issues: string[] = [];

    // Common security patterns to avoid
    if (/(password|secret|api_key|token)\s*[=:]\s*[^\s]+/i.test(content)) {
      issues.push('Hardcoded credentials detected');
    }

    if (/USER\s+root/i.test(content)) {
      issues.push('Running as root user');
    }

    return issues;
  }

  /**
   * Extract version tags from content (for base image analysis)
   * @param content Text content
   * @returns Array of version tags found
   */
  protected extractVersions(content: string): string[] {
    const versionPattern = /(?:FROM\s+[^\s:]+:|version:\s*['"]?)([^'"\s\n]+)['"]?/gi;
    const matches: string[] = [];
    let match;

    while ((match = versionPattern.exec(content)) !== null) {
      if (match[1]) {
        matches.push(match[1]);
      }
    }

    return matches;
  }
}
