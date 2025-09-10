/**
 * Advanced Scoring System - Configurable criteria-based evaluation
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type {
  DockerfileVariant,
  ScoredVariant,
  ScoringCriteria,
  ScoreDetails,
  SelectionConstraints,
} from './types';

/**
 * Default balanced scoring criteria
 */
export const DEFAULT_SCORING_CRITERIA: ScoringCriteria = {
  security: 0.3,
  performance: 0.25,
  size: 0.25,
  maintainability: 0.2,
};

/**
 * Environment-specific scoring criteria presets
 */
export const SCORING_PRESETS: Record<string, ScoringCriteria> = {
  production: {
    security: 0.4,
    performance: 0.3,
    size: 0.2,
    maintainability: 0.1,
  },
  development: {
    security: 0.1,
    performance: 0.2,
    size: 0.2,
    maintainability: 0.5,
  },
  staging: {
    security: 0.3,
    performance: 0.3,
    size: 0.2,
    maintainability: 0.2,
  },
};

/**
 * Advanced Dockerfile analyzer for detailed scoring
 */
export class DockerfileAnalyzer {
  constructor(private logger: Logger) {}

  /**
   * Comprehensive analysis of a Dockerfile variant
   */
  async analyzeDockerfile(variant: DockerfileVariant): Promise<
    Result<{
      security: SecurityAnalysis;
      performance: PerformanceAnalysis;
      size: SizeAnalysis;
      maintainability: MaintainabilityAnalysis;
    }>
  > {
    try {
      const content = variant.content;
      const lines = content
        .split('\n')
        .map((line) => line.trim())
        .filter((line) => line);

      const analysis = {
        security: this.analyzeSecurityFeatures(content, lines),
        performance: this.analyzePerformanceFeatures(content, lines),
        size: this.analyzeSizeOptimization(content, lines, variant),
        maintainability: this.analyzeMaintainabilityFeatures(content, lines),
      };

      this.logger.debug({ variant: variant.id }, 'Dockerfile analysis completed');
      return Success(analysis);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error({ error: message, variant: variant.id }, 'Analysis failed');
      return Failure(`Dockerfile analysis failed: ${message}`);
    }
  }

  /**
   * Analyzes security aspects of a Dockerfile.
   * Evaluates base image choice, user privileges, package management,
   * security tools, and potential credential leaks.
   *
   * Scoring breakdown (100 points total):
   * - Base image security: 25 points (alpine/distroless preferred)
   * - User management: 25 points (non-root user required)
   * - Package management: 20 points (proper cleanup practices)
   * - Security tools: 20 points (healthcheck, init system)
   * - Secrets handling: 10 points (no hardcoded credentials)
   *
   * @param content - Full Dockerfile content
   * @param lines - Dockerfile lines for line-by-line analysis
   * @returns SecurityAnalysis with score, features, issues, and recommendations
   */
  private analyzeSecurityFeatures(content: string, lines: string[]): SecurityAnalysis {
    const lowerContent = content.toLowerCase();
    let score = 0;
    const features: string[] = [];
    const issues: string[] = [];

    if (lowerContent.includes('alpine') || lowerContent.includes('distroless')) {
      score += 25;
      features.push('Secure base image');
    } else if (lowerContent.includes('slim')) {
      score += 15;
      features.push('Minimal base image');
    } else if (lowerContent.includes(':latest')) {
      issues.push('Using latest tag - potential security risk');
    }

    const userLines = lines.filter((line) => line.toLowerCase().startsWith('user '));
    if (userLines.some((line) => !line.toLowerCase().includes('user root'))) {
      score += 25;
      features.push('Non-root user');
    } else {
      issues.push('Running as root user');
    }

    if (lowerContent.includes('apt-get update') && lowerContent.includes('apt-get install')) {
      if (lowerContent.includes('rm -rf /var/lib/apt') || lowerContent.includes('apt-get clean')) {
        score += 20;
        features.push('Proper package cleanup');
      } else {
        score += 10;
        issues.push('Package cache not cleaned');
      }
    }

    if (lowerContent.includes('healthcheck')) {
      score += 10;
      features.push('Health check configured');
    }
    if (lowerContent.includes('dumb-init') || lowerContent.includes('tini')) {
      score += 10;
      features.push('Init system for proper signal handling');
    }

    if (
      !lowerContent.includes('password') &&
      !lowerContent.includes('secret') &&
      !lowerContent.includes('key=')
    ) {
      score += 10;
      features.push('No hardcoded secrets detected');
    } else {
      issues.push('Potential hardcoded secrets');
    }

    return {
      score: Math.min(100, score),
      features,
      issues,
      recommendations: this.generateSecurityRecommendations(issues, features),
    };
  }

  /**
   * Evaluates performance optimization techniques in a Dockerfile.
   * Focuses on build speed, layer caching, and runtime efficiency.
   *
   * Scoring breakdown (100 points total):
   * - Multi-stage builds: 30 points (reduces final image size)
   * - Layer optimization: 25 points (command chaining)
   * - Build cache mounts: 20 points (faster rebuilds)
   * - Dependency caching: 15 points (separate dependency copy)
   * - Build tool optimization: 10 points (deterministic installs)
   *
   * @param content - Full Dockerfile content
   * @param lines - Dockerfile lines for analysis
   * @returns PerformanceAnalysis with optimizations, bottlenecks, and recommendations
   */
  private analyzePerformanceFeatures(content: string, lines: string[]): PerformanceAnalysis {
    const lowerContent = content.toLowerCase();
    let score = 0;
    const optimizations: string[] = [];
    const bottlenecks: string[] = [];

    const fromCount = lines.filter((line) => line.toLowerCase().startsWith('from ')).length;
    if (fromCount > 1) {
      score += 30;
      optimizations.push('Multi-stage build');
    }

    if (lowerContent.includes('&&')) {
      const chainedCommands = content.split('&&').length - 1;
      if (chainedCommands > 3) {
        score += 25;
        optimizations.push('Command chaining for layer optimization');
      } else {
        score += 15;
        optimizations.push('Some command chaining');
      }
    }

    if (lowerContent.includes('--mount=type=cache')) {
      score += 20;
      optimizations.push('Build cache mounts');
    }

    const copyPackageFirst = lines.findIndex(
      (line) =>
        line.toLowerCase().includes('copy package') ||
        line.toLowerCase().includes('copy requirements') ||
        line.toLowerCase().includes('copy go.mod'),
    );
    const copyAllIndex = lines.findIndex(
      (line) => line.toLowerCase().includes('copy . ') && !line.toLowerCase().includes('package'),
    );

    if (copyPackageFirst !== -1 && copyAllIndex !== -1 && copyPackageFirst < copyAllIndex) {
      score += 15;
      optimizations.push('Dependency caching optimization');
    } else {
      bottlenecks.push('Suboptimal layer caching - copy dependencies separately');
    }

    if (
      lowerContent.includes('npm ci') ||
      lowerContent.includes('yarn install --frozen-lockfile')
    ) {
      score += 10;
      optimizations.push('Deterministic dependency installation');
    }

    return {
      score: Math.min(100, score),
      optimizations,
      bottlenecks,
      buildComplexity: this.assessBuildComplexity(lines),
      recommendations: this.generatePerformanceRecommendations(bottlenecks, optimizations),
    };
  }

  /**
   * Analyzes size optimization strategies in a Dockerfile.
   * Evaluates base image choice, cleanup practices, and layer efficiency.
   *
   * @param content - Full Dockerfile content
   * @param lines - Dockerfile lines for analysis
   * @param variant - Dockerfile variant with metadata
   * @returns SizeAnalysis with optimizations, wasteful practices, and estimated size
   */
  private analyzeSizeOptimization(
    content: string,
    lines: string[],
    variant: DockerfileVariant,
  ): SizeAnalysis {
    const lowerContent = content.toLowerCase();
    let score = 0;
    const optimizations: string[] = [];
    const wastefulPractices: string[] = [];

    // Base image efficiency (30 points)
    if (lowerContent.includes('distroless')) {
      score += 30;
      optimizations.push('Distroless base image');
    } else if (lowerContent.includes('alpine')) {
      score += 25;
      optimizations.push('Alpine Linux base');
    } else if (lowerContent.includes('slim')) {
      score += 20;
      optimizations.push('Slim base image');
    } else if (lowerContent.includes(':latest')) {
      wastefulPractices.push('Using latest tag may pull larger images');
    }

    // Multi-stage build benefits (25 points)
    const fromCount = lines.filter((line) => line.toLowerCase().startsWith('from ')).length;
    if (fromCount > 1) {
      if (lowerContent.includes('copy --from=')) {
        score += 25;
        optimizations.push('Multi-stage build with selective copying');
      } else {
        score += 15;
        optimizations.push('Multi-stage build');
      }
    }

    // Cleanup practices (20 points)
    if (lowerContent.includes('rm -rf')) {
      score += 10;
      optimizations.push('Manual cleanup');
    }
    if (lowerContent.includes('apt-get clean') || lowerContent.includes('rm -rf /var/lib/apt')) {
      score += 10;
      optimizations.push('Package manager cleanup');
    }

    // Layer reduction (15 points)
    const runCommands = lines.filter((line) => line.toLowerCase().startsWith('run ')).length;
    if (runCommands <= 3) {
      score += 15;
      optimizations.push('Minimal RUN layers');
    } else if (runCommands > 6) {
      wastefulPractices.push('Too many RUN layers');
    }

    // Dependency optimization (10 points)
    if (lowerContent.includes('--only=production') || lowerContent.includes('--prod')) {
      score += 10;
      optimizations.push('Production-only dependencies');
    }

    const estimatedSize = this.estimateImageSize(variant, optimizations);

    return {
      score: Math.min(100, score),
      optimizations,
      wastefulPractices,
      estimatedSize,
      recommendations: this.generateSizeRecommendations(wastefulPractices, optimizations),
    };
  }

  private analyzeMaintainabilityFeatures(
    content: string,
    lines: string[],
  ): MaintainabilityAnalysis {
    let score = 0;
    const goodPractices: string[] = [];
    const improvements: string[] = [];

    // Documentation (25 points)
    const commentLines = lines.filter((line) => line.startsWith('#')).length;
    if (commentLines >= 5) {
      score += 25;
      goodPractices.push('Well documented');
    } else if (commentLines >= 2) {
      score += 15;
      goodPractices.push('Some documentation');
    } else {
      improvements.push('Add more documentation comments');
    }

    // Labels and metadata (20 points)
    const labelCount = lines.filter((line) => line.toLowerCase().startsWith('label ')).length;
    if (labelCount >= 3) {
      score += 20;
      goodPractices.push('Rich metadata labels');
    } else if (labelCount >= 1) {
      score += 10;
      goodPractices.push('Basic labeling');
    }

    // Environment variables (15 points)
    const envCount = lines.filter((line) => line.toLowerCase().startsWith('env ')).length;
    if (envCount > 0) {
      score += 15;
      goodPractices.push('Environment variable configuration');
    }

    // Build arguments (15 points)
    const argCount = lines.filter((line) => line.toLowerCase().startsWith('arg ')).length;
    if (argCount > 0) {
      score += 15;
      goodPractices.push('Configurable build arguments');
    }

    // Readability (15 points)
    if (content.includes('\\') && content.includes('&&')) {
      score += 15;
      goodPractices.push('Readable multi-line commands');
    }

    // Structure (10 points)
    const hasWorkdir = lines.some((line) => line.toLowerCase().startsWith('workdir '));
    if (hasWorkdir) {
      score += 10;
      goodPractices.push('Explicit working directory');
    }

    return {
      score: Math.min(100, score),
      goodPractices,
      improvements,
      readabilityScore: this.assessReadability(content, lines),
      recommendations: this.generateMaintainabilityRecommendations(improvements, goodPractices),
    };
  }

  private generateSecurityRecommendations(issues: string[], features: string[]): string[] {
    const recommendations: string[] = [];

    if (!features.some((f) => f.includes('Non-root'))) {
      recommendations.push('Add non-root user for better security');
    }
    if (!features.some((f) => f.includes('Secure base'))) {
      recommendations.push('Consider using Alpine or distroless base images');
    }
    if (issues.some((i) => i.includes('latest'))) {
      recommendations.push('Pin base image to specific version');
    }
    if (!features.some((f) => f.includes('Health check'))) {
      recommendations.push('Add HEALTHCHECK instruction');
    }

    return recommendations;
  }

  private generatePerformanceRecommendations(
    bottlenecks: string[],
    optimizations: string[],
  ): string[] {
    const recommendations: string[] = [];

    if (!optimizations.some((o) => o.includes('Multi-stage'))) {
      recommendations.push('Consider multi-stage build for better performance');
    }
    if (!optimizations.some((o) => o.includes('caching'))) {
      recommendations.push('Optimize dependency caching by copying package files first');
    }
    if (bottlenecks.length > 0) {
      recommendations.push('Address identified performance bottlenecks');
    }

    return recommendations;
  }

  private generateSizeRecommendations(wasteful: string[], optimizations: string[]): string[] {
    const recommendations: string[] = [];

    if (!optimizations.some((o) => o.includes('Alpine') || o.includes('Distroless'))) {
      recommendations.push('Use smaller base images like Alpine or distroless');
    }
    if (!optimizations.some((o) => o.includes('cleanup'))) {
      recommendations.push('Add package manager cleanup commands');
    }
    if (wasteful.some((w) => w.includes('RUN layers'))) {
      recommendations.push('Combine RUN commands to reduce layers');
    }

    return recommendations;
  }

  private generateMaintainabilityRecommendations(
    improvements: string[],
    practices: string[],
  ): string[] {
    const recommendations: string[] = [];

    if (improvements.some((i) => i.includes('documentation'))) {
      recommendations.push('Add more descriptive comments');
    }
    if (!practices.some((p) => p.includes('labels'))) {
      recommendations.push('Add metadata labels for better maintenance');
    }
    if (!practices.some((p) => p.includes('arguments'))) {
      recommendations.push('Use ARG for configurable build parameters');
    }

    return recommendations;
  }

  private assessBuildComplexity(lines: string[]): 'low' | 'medium' | 'high' {
    const fromCount = lines.filter((line) => line.toLowerCase().startsWith('from ')).length;
    const runCount = lines.filter((line) => line.toLowerCase().startsWith('run ')).length;

    if (fromCount > 1 || runCount > 5) return 'high';
    if (runCount > 2) return 'medium';
    return 'low';
  }

  private assessReadability(content: string, lines: string[]): number {
    let score = 50;

    // Comment ratio
    const commentRatio = lines.filter((l) => l.startsWith('#')).length / lines.length;
    score += commentRatio * 30;

    // Line length (prefer shorter lines)
    const avgLineLength =
      content.split('\n').reduce((sum, line) => sum + line.length, 0) / lines.length;
    if (avgLineLength < 80) score += 20;

    return Math.min(100, score);
  }

  private estimateImageSize(variant: DockerfileVariant, optimizations: string[]): string {
    const baseEstimate = variant.metadata.baseImage.includes('alpine')
      ? 50
      : variant.metadata.baseImage.includes('distroless')
        ? 30
        : variant.metadata.baseImage.includes('slim')
          ? 80
          : 150;

    const reductionFactor = optimizations.length * 10;
    const estimated = Math.max(30, baseEstimate - reductionFactor);

    return `~${estimated}MB`;
  }
}

/**
 * Variant scoring and selection service
 */
export class VariantScorer {
  private analyzer: DockerfileAnalyzer;

  constructor(private logger: Logger) {
    this.analyzer = new DockerfileAnalyzer(logger);
  }

  /**
   * Score multiple variants with given criteria
   */
  async scoreVariants(
    variants: DockerfileVariant[],
    criteria: ScoringCriteria = DEFAULT_SCORING_CRITERIA,
  ): Promise<Result<ScoredVariant[]>> {
    try {
      const scoredVariants: ScoredVariant[] = [];

      for (const variant of variants) {
        const analysisResult = await this.analyzer.analyzeDockerfile(variant);
        if (!analysisResult.ok) {
          this.logger.warn(
            { variant: variant.id, error: analysisResult.error },
            'Skipping variant due to analysis failure',
          );
          continue;
        }

        const analysis = analysisResult.value;
        const weightedScore =
          analysis.security.score * criteria.security +
          analysis.performance.score * criteria.performance +
          analysis.size.score * criteria.size +
          analysis.maintainability.score * criteria.maintainability;

        const scoreDetails: ScoreDetails = {
          total: Math.round(weightedScore),
          breakdown: {
            security: analysis.security.score,
            performance: analysis.performance.score,
            size: analysis.size.score,
            maintainability: analysis.maintainability.score,
          },
          reasons: [
            ...analysis.security.features,
            ...analysis.performance.optimizations,
            ...analysis.size.optimizations,
            ...analysis.maintainability.goodPractices,
          ],
          warnings: [
            ...analysis.security.issues,
            ...analysis.performance.bottlenecks,
            ...analysis.size.wastefulPractices,
            ...analysis.maintainability.improvements,
          ],
          recommendations: [
            ...analysis.security.recommendations,
            ...analysis.performance.recommendations,
            ...analysis.size.recommendations,
            ...analysis.maintainability.recommendations,
          ],
        };

        scoredVariants.push({
          ...variant,
          score: scoreDetails,
          rank: 0, // Will be set after sorting
        });
      }

      // Sort by score and assign ranks
      scoredVariants.sort((a, b) => b.score.total - a.score.total);
      scoredVariants.forEach((variant, index) => {
        variant.rank = index + 1;
      });

      this.logger.info(
        {
          variantCount: scoredVariants.length,
          topScore: scoredVariants[0]?.score.total,
        },
        'Variants scored and ranked',
      );

      return Success(scoredVariants);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error({ error: message }, 'Variant scoring failed');
      return Failure(`Scoring failed: ${message}`);
    }
  }

  /**
   * Select best variant based on constraints
   */
  selectBestVariant(
    scoredVariants: ScoredVariant[],
    constraints?: SelectionConstraints,
  ): ScoredVariant | null {
    if (scoredVariants.length === 0) {
      return null;
    }

    let candidates = [...scoredVariants];

    // Apply constraints
    if (constraints) {
      if (constraints.minScore !== undefined) {
        const minScore = constraints.minScore;
        candidates = candidates.filter((v) => v.score.total >= minScore);
      }

      if (constraints.mustHave && constraints.mustHave.length > 0) {
        const mustHave = constraints.mustHave;
        candidates = candidates.filter((v) =>
          mustHave.every(
            (feature) =>
              v.metadata.features.includes(feature) ||
              v.score.reasons.some((reason) =>
                reason.toLowerCase().includes(feature.toLowerCase()),
              ),
          ),
        );
      }

      if (constraints.mustNotHave?.length) {
        const mustNotHave = constraints.mustNotHave;
        candidates = candidates.filter(
          (v) =>
            !mustNotHave.some(
              (feature) =>
                v.metadata.features.includes(feature) ||
                v.score.warnings.some((warning) =>
                  warning.toLowerCase().includes(feature.toLowerCase()),
                ),
            ),
        );
      }

      if (constraints.preferredOptimization) {
        // Prefer variants with matching optimization, but don't exclude others
        const preferred = candidates.filter(
          (v) => v.metadata.optimization === constraints.preferredOptimization,
        );
        if (preferred.length > 0) {
          candidates = preferred;
        }
      }
    }

    if (candidates.length === 0) {
      this.logger.warn('No variants meet selection constraints');
      return scoredVariants[0] || null; // Return best overall if constraints too strict
    }

    const selected = candidates[0];
    if (!selected) {
      return null;
    }

    this.logger.info(
      {
        variant: selected.id,
        score: selected.score.total,
        strategy: selected.strategy,
      },
      'Best variant selected',
    );

    return selected;
  }

  /**
   * Get scoring criteria preset by environment
   */
  getScoringPreset(environment: string): ScoringCriteria {
    return SCORING_PRESETS[environment] || DEFAULT_SCORING_CRITERIA;
  }
}

// Analysis result interfaces
interface SecurityAnalysis {
  score: number;
  features: string[];
  issues: string[];
  recommendations: string[];
}

interface PerformanceAnalysis {
  score: number;
  optimizations: string[];
  bottlenecks: string[];
  buildComplexity: 'low' | 'medium' | 'high';
  recommendations: string[];
}

interface SizeAnalysis {
  score: number;
  optimizations: string[];
  wastefulPractices: string[];
  estimatedSize: string;
  recommendations: string[];
}

interface MaintainabilityAnalysis {
  score: number;
  goodPractices: string[];
  improvements: string[];
  readabilityScore: number;
  recommendations: string[];
}
