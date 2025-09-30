import { TextContentScorer, type ScoringResult, type ScoringContext } from './base-scorer';

/**
 * Helm chart-specific scoring criteria
 */
interface HelmScoringBreakdown extends Record<string, number> {
  parseability: number;
  chartStructure: number;
  templating: number;
  values: number;
  bestPractices: number;
}

/**
 * Scorer for Helm chart content
 *
 * Evaluates Helm charts for:
 * - YAML parsing and Chart.yaml structure
 * - Template quality and Helm templating best practices
 * - Values.yaml organization and documentation
 * - Security and deployment best practices
 */
export class HelmScorer extends TextContentScorer {
  /**
   * Score Helm chart content with comprehensive analysis
   */
  score(content: string, _context?: ScoringContext): ScoringResult {
    const breakdown: HelmScoringBreakdown = {
      parseability: this.scoreParseability(content),
      chartStructure: this.scoreChartStructure(content),
      templating: this.scoreTemplating(content),
      values: this.scoreValues(content),
      bestPractices: this.scoreBestPractices(content),
    };

    const feedback = this.generateFeedback(content, breakdown);

    // Weight different aspects of Helm charts
    const weights = {
      parseability: 0.2,
      chartStructure: 0.2,
      templating: 0.25,
      values: 0.2,
      bestPractices: 0.15,
    };

    return this.createResult(breakdown, weights, feedback);
  }

  /**
   * Validate that the content is parseable YAML with Helm structure
   */
  protected validateParseable(content: string): boolean {
    try {
      // Check for basic Helm chart indicators
      return (
        content.includes('apiVersion:') ||
        content.includes('{{') ||
        content.includes('Chart.yaml') ||
        content.includes('values.yaml')
      );
    } catch {
      return false;
    }
  }

  /**
   * Score YAML parseability and basic Helm structure
   */
  private scoreParseability(content: string): number {
    let score = 0;

    // Basic parseability (50 points)
    if (!this.validateParseable(content)) {
      return 0;
    }
    score += 50;

    // YAML structure validity (30 points)
    if (!/\t/g.test(content)) score += 10; // No tabs, consistent spacing
    if (!/^\s*$/.test(content)) score += 5; // Not empty
    if (content.split('\n').length > 5) score += 5; // Non-trivial content
    if (!/^-{3}/.test(content) || content.includes('---')) score += 10; // YAML document separators

    // Helm-specific structure (20 points)
    if (content.includes('{{') && content.includes('}}')) score += 10; // Has templating
    if (/\{\{.*\.Values\./.test(content)) score += 10; // References values

    return Math.min(score, 100);
  }

  /**
   * Score Chart.yaml structure and metadata
   */
  private scoreChartStructure(content: string): number {
    let score = 0;

    // Required Chart.yaml fields (60 points)
    if (content.includes('apiVersion: v2')) score += 15;
    if (content.includes('name:') && /name:\s*\S+/.test(content)) score += 15;
    if (content.includes('version:') && /version:\s*\d+\.\d+\.\d+/.test(content)) score += 15;
    if (content.includes('description:')) score += 15;

    // Optional but recommended fields (25 points)
    if (content.includes('type: application')) score += 10;
    if (content.includes('appVersion:')) score += 10;
    if (content.includes('home:')) score += 5;

    // Advanced Chart.yaml features (15 points)
    if (content.includes('maintainers:')) score += 5;
    if (content.includes('dependencies:')) score += 5;
    if (content.includes('keywords:')) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score Helm templating quality and best practices
   */
  private scoreTemplating(content: string): number {
    let score = 0;

    // Template control structures (40 points)
    if (content.includes('{{- if')) score += 10; // Conditional logic
    if (content.includes('{{- range')) score += 10; // Loops
    if (content.includes('{{- with')) score += 10; // Context scoping
    if (content.includes('{{- end }}')) score += 10; // Proper closing

    // Template functions and helpers (35 points)
    if (content.includes('{{- include')) score += 15; // Template includes
    if (content.includes('{{- toYaml')) score += 10; // YAML helpers
    if (/\{\{.*\|\s*quote/.test(content)) score += 5; // Quoting functions
    if (/\{\{.*\|\s*indent/.test(content)) score += 5; // Indentation helpers

    // Values references (25 points)
    if (/\{\{.*\.Values\./.test(content)) score += 15; // Uses values
    if (/\{\{.*\.Release\./.test(content)) score += 5; // Release info
    if (/\{\{.*\.Chart\./.test(content)) score += 5; // Chart metadata

    return Math.min(score, 100);
  }

  /**
   * Score Values.yaml organization and documentation
   */
  private scoreValues(content: string): number {
    let score = 0;

    // Documentation (25 points)
    if (content.includes('# Default values') || content.includes('# -- ')) score += 15;
    if (/^#.*\n\w+:/gm.test(content)) score += 10; // Commented values

    // Common configuration patterns (50 points)
    if (content.includes('replicaCount:')) score += 10;
    if (content.includes('image:') && content.includes('tag:')) score += 10;
    if (content.includes('service:') && content.includes('port:')) score += 10;
    if (content.includes('ingress:')) score += 5;
    if (content.includes('resources:')) score += 10;
    if (content.includes('autoscaling:')) score += 5;

    // Advanced configuration (25 points)
    if (content.includes('serviceAccount:')) score += 5;
    if (content.includes('podAnnotations:')) score += 5;
    if (content.includes('nodeSelector:')) score += 5;
    if (content.includes('tolerations:')) score += 5;
    if (content.includes('affinity:')) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score deployment and security best practices
   */
  private scoreBestPractices(content: string): number {
    let score = 0;

    // Security configurations (40 points)
    if (content.includes('securityContext:')) score += 15;
    if (content.includes('podSecurityContext:')) score += 10;
    if (content.includes('runAsNonRoot: true')) score += 10;
    if (content.includes('readOnlyRootFilesystem: true')) score += 5;

    // Health and reliability (35 points)
    if (content.includes('livenessProbe:') || content.includes('readinessProbe:')) score += 15;
    if (/replicas:\s*\{\{.*\.Values\.replicaCount/.test(content)) score += 10;
    if (content.includes('strategy:')) score += 10;

    // Resource management (25 points)
    if (content.includes('resources:') && /\{\{.*\.Values\.resources/.test(content)) score += 15;
    if (content.includes('limits:') && content.includes('requests:')) score += 10;

    return Math.min(score, 100);
  }

  /**
   * Generate helpful feedback messages based on scoring
   */
  private generateFeedback(content: string, scores: HelmScoringBreakdown): string[] {
    const feedback: string[] = [];

    if (scores.parseability < 50) {
      feedback.push('Chart has YAML parsing or structure issues');
    }

    if (scores.chartStructure < 60) {
      if (!content.includes('apiVersion: v2')) {
        feedback.push('Use Helm Chart API version v2 for better features');
      }
      if (!content.includes('description:')) {
        feedback.push('Add a description to your Chart.yaml for better documentation');
      }
    }

    if (scores.templating < 60) {
      if (!content.includes('{{- include')) {
        feedback.push('Use template includes for better code reuse');
      }
      if (!content.includes('{{- if')) {
        feedback.push('Add conditional logic to make templates more flexible');
      }
    }

    if (scores.values < 60) {
      if (!content.includes('# -- ') && !content.includes('# Default values')) {
        feedback.push('Document your values.yaml with comments for better usability');
      }
      if (!content.includes('resources:')) {
        feedback.push('Include resource configuration in values.yaml');
      }
    }

    if (scores.bestPractices < 60) {
      if (!content.includes('securityContext:')) {
        feedback.push('Add security context configuration for better pod security');
      }
      if (!content.includes('livenessProbe:')) {
        feedback.push('Include health probes for better reliability');
      }
    }

    return feedback;
  }
}

/**
 * Convenience function for scoring Helm charts
 * Maintains backward compatibility with existing simple scoring functions
 */
export function scoreHelmChart(content: string, context?: ScoringContext): Record<string, number> {
  const scorer = new HelmScorer();
  const result = scorer.score(content, context);
  return result.breakdown;
}

/**
 * Convenience function for detailed Helm chart scoring
 * Returns complete scoring result with feedback
 */
export function scoreHelmChartDetailed(content: string, context?: ScoringContext): ScoringResult {
  const scorer = new HelmScorer();
  return scorer.score(content, context);
}
