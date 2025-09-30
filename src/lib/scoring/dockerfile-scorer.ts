import { DockerfileParser } from 'dockerfile-ast';
import { TextContentScorer, type ScoringResult, type ScoringContext } from './base-scorer';
import { extractDockerfileContent } from '@/lib/content-extraction';

/**
 * Dockerfile-specific scoring criteria
 */
interface DockerfileScoringBreakdown extends Record<string, number> {
  parseability: number;
  security: number;
  bestPractices: number;
  performance: number;
  caching: number;
}

/**
 * Scorer for Dockerfile content that consolidates logic from multiple existing implementations
 *
 * This scorer combines:
 * - Basic parseability checks using dockerfile-ast
 * - Security best practices scoring
 * - Performance and optimization checks
 * - Layer caching optimization scoring
 */
export class DockerfileScorer extends TextContentScorer {
  /**
   * Score a Dockerfile with comprehensive analysis
   */
  score(content: string, _context?: ScoringContext): ScoringResult {
    const breakdown: DockerfileScoringBreakdown = {
      parseability: this.scoreParseability(content),
      security: this.scoreSecurity(content),
      bestPractices: this.scoreBestPractices(content),
      performance: this.scorePerformance(content),
      caching: this.scoreCaching(content),
    };

    const feedback = this.generateFeedback(content, breakdown);

    // Weight parseability heavily since invalid Dockerfiles are useless
    const weights = {
      parseability: 0.3,
      security: 0.25,
      bestPractices: 0.2,
      performance: 0.15,
      caching: 0.1,
    };

    return this.createResult(breakdown, weights, feedback);
  }

  /**
   * Validate that the Dockerfile can be parsed
   */
  protected validateParseable(content: string): boolean {
    try {
      DockerfileParser.parse(content);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Score Dockerfile parseability and basic structure
   */
  private scoreParseability(content: string): number {
    let score = 0;

    // Basic parsing check (60 points)
    if (!this.validateParseable(content)) {
      return 0; // If it doesn't parse, it's fundamentally broken
    }
    score += 60;

    // Additional structural checks (40 points)
    if (content.includes('FROM ')) score += 10; // Has base image
    if (!/^\s*$/.test(content)) score += 5; // Not empty
    if (content.split('\n').length > 3) score += 5; // Non-trivial
    if (/^FROM\s+\w/m.test(content)) score += 10; // Well-formed FROM
    if (content.includes('# ')) score += 5; // Has comments
    if (!/\t/g.test(content)) score += 5; // No tabs (consistent spacing)

    return Math.min(score, 100);
  }

  /**
   * Score security practices in the Dockerfile
   */
  private scoreSecurity(content: string): number {
    let score = 0;

    // Pinned versions (20 points)
    if (/FROM\s+[^\s:]+:[^:\s]+(?:\s|$)/.test(content) && !/FROM\s+[^\s:]+:latest/.test(content)) {
      score += 20; // Pinned version, not latest
    }

    // Non-root user (25 points)
    if (/USER\s+(?!root|0)\w+/.test(content)) {
      score += 25;
    } else if (/USER root/i.test(content)) {
      score -= 10; // Penalty for explicit root
    }

    // No hardcoded secrets (15 points)
    if (!/(password|secret|api_key|token)\s*[=:]\s*[^\s]+/i.test(content)) {
      score += 15;
    }

    // Security-related instructions (25 points)
    if (content.includes('--chown=')) score += 10;
    if (!content.includes('sudo')) score += 10;
    if (!/ADD .* \/$/m.test(content)) score += 5; // Avoid ADD to root

    // File permissions and ownership (15 points)
    if (/COPY --chown=/m.test(content)) score += 10;
    if (/RUN chmod/m.test(content)) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score best practices adherence
   */
  private scoreBestPractices(content: string): number {
    let score = 0;

    // Multi-stage builds (20 points)
    const fromCount = (content.match(/^FROM\s+/gm) || []).length;
    if (fromCount > 1 || /FROM\s+.*\sAS\s+\w+/i.test(content)) {
      score += 20;
    }

    // Health checks (15 points)
    if (/HEALTHCHECK/i.test(content)) {
      score += 15;
    }

    // Working directory (10 points)
    if (/WORKDIR/i.test(content)) {
      score += 10;
    }

    // Port exposure (10 points)
    if (/EXPOSE\s+\d+/.test(content)) {
      score += 10;
    }

    // Labels and metadata (10 points)
    if (content.includes('LABEL')) score += 10;

    // Command chaining optimization (15 points)
    if (!/RUN .* && .* && .* && .* &&/m.test(content)) {
      score += 15; // Reasonable RUN command chaining
    }

    // Multi-stage copy (10 points)
    if (content.includes('COPY --from=')) score += 10;

    // Environment variables best practices (10 points)
    if (/ENV\s+\w+=/m.test(content) && !/(password|secret|key|token)/i.test(content)) {
      score += 10;
    }

    return Math.min(score, 100);
  }

  /**
   * Score performance and size optimizations
   */
  private scorePerformance(content: string): number {
    let score = 0;

    // Base image optimization (30 points)
    if (/FROM .+:alpine/.test(content)) score += 15; // Alpine base
    if (/FROM .+:(slim|minimal)/i.test(content)) score += 10; // Slim variants
    if (content.includes('distroless')) score += 15; // Distroless images

    // Size optimization techniques (25 points)
    if (content.includes('--no-cache')) score += 10;
    if (content.includes('&& rm -rf')) score += 10;
    if (/apt-get.*--no-install-recommends/m.test(content)) score += 5;

    // Multi-stage optimization (25 points)
    const fromCount = (content.match(/^FROM\s+/gm) || []).length;
    if (content.includes('multi-stage') || fromCount > 1) score += 15;
    if (/COPY --from=\w+\s+\/.*\/app/m.test(content)) score += 10;

    // Package manager cleanup (20 points)
    if (/apt-get clean/m.test(content)) score += 5;
    if (/yum clean all/m.test(content)) score += 5;
    if (/apk del/m.test(content)) score += 5;
    if (/rm -rf \/var\/lib\/apt\/lists/m.test(content)) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score layer caching optimization
   */
  private scoreCaching(content: string): number {
    let score = 0;

    // Dependency file copying patterns (40 points)
    if (/COPY\s+package.*\.json/.test(content)) score += 20;
    if (/COPY\s+go\.mod\s+go\.sum/.test(content)) score += 20;
    if (/COPY\s+requirements\.txt/.test(content)) score += 15;
    if (/COPY\s+Cargo\.toml/.test(content)) score += 15;

    // Copy order optimization (30 points)
    const copyCommands = (content.match(/COPY/g) || []).length;
    const runCommands = (content.match(/RUN/g) || []).length;
    if (copyCommands > 0 && runCommands > 0) {
      const firstCopy = content.indexOf('COPY');
      const firstRun = content.indexOf('RUN');
      if (firstCopy < firstRun) score += 20;
    }

    // Layer separation (30 points)
    const lines = content.split('\n');
    const copyBeforeInstall = lines.some((line, index) => {
      if (/COPY.*package.*\.json/.test(line)) {
        const nextInstall = lines
          .slice(index)
          .findIndex((l) => /RUN.*install|npm i|yarn|pip install|go mod download/i.test(l));
        return nextInstall > 0 && nextInstall < 5; // Within 5 lines
      }
      return false;
    });
    if (copyBeforeInstall) score += 20;

    // Avoid cache invalidation patterns (bonus points)
    if (!/COPY \. \./.test(content) || content.includes('.dockerignore')) score += 10;

    return Math.min(score, 100);
  }

  /**
   * Generate helpful feedback messages based on scoring
   */
  private generateFeedback(content: string, scores: DockerfileScoringBreakdown): string[] {
    const feedback: string[] = [];

    if (scores.parseability < 50) {
      feedback.push('Dockerfile has parsing issues that need to be resolved');
    }

    if (scores.security < 60) {
      if (/USER root/i.test(content)) {
        feedback.push('Consider using a non-root user for better security');
      }
      if (/(password|secret|api_key|token)\s*[=:]/i.test(content)) {
        feedback.push('Avoid hardcoding secrets in the Dockerfile');
      }
      if (/FROM.*:latest/.test(content)) {
        feedback.push('Pin base image versions instead of using :latest');
      }
    }

    if (scores.bestPractices < 60) {
      if (!/HEALTHCHECK/i.test(content)) {
        feedback.push('Add a HEALTHCHECK instruction for better container monitoring');
      }
      if (!/WORKDIR/i.test(content)) {
        feedback.push('Use WORKDIR to set the working directory');
      }
    }

    if (scores.performance < 60) {
      if (!/alpine|slim|distroless/i.test(content)) {
        feedback.push('Consider using smaller base images like Alpine or distroless');
      }
      if (!/FROM.*AS/i.test(content) && (content.match(/FROM/g) || []).length === 1) {
        feedback.push('Consider multi-stage builds for smaller final images');
      }
    }

    if (scores.caching < 60) {
      if (!/COPY.*package.*\.json/i.test(content)) {
        feedback.push('Copy dependency files separately to improve layer caching');
      }
    }

    return feedback;
  }
}

/**
 * Convenience function for scoring Dockerfiles
 * Maintains backward compatibility with existing simple scoring functions
 */
export function scoreDockerfile(content: string, context?: ScoringContext): number {
  const scorer = new DockerfileScorer();
  return scorer.score(content, context).total;
}

/**
 * Convenience function for detailed Dockerfile scoring
 * Returns category breakdown for advanced use cases
 * Note: Returns breakdown directly for backward compatibility
 */
export function scoreDockerfileDetailed(
  content: string,
  context?: ScoringContext,
): Record<string, number> {
  const scorer = new DockerfileScorer();
  return scorer.score(content, context).breakdown;
}

/**
 * Creates a scoring function that extracts and scores Dockerfile content
 *
 * This helper consolidates the common pattern of extracting Dockerfile content
 * from AI responses and scoring it. Used by sampling-based tools to avoid duplication.
 *
 * @returns A function that takes raw AI response text and returns a numeric score
 *
 * @example
 * ```typescript
 * const samplingResult = await sampleWithRerank(
 *   ctx,
 *   buildSamplingRequest,
 *   createDockerfileScoringFunction(),
 *   {}
 * );
 * ```
 */
export function createDockerfileScoringFunction(): (text: string) => number {
  return (text: string) => {
    const extraction = extractDockerfileContent(text);
    return scoreDockerfile(extraction.success && extraction.content ? extraction.content : text);
  };
}
