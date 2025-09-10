/**
 * Analysis Scoring System - Configurable criteria-based evaluation for analysis variants
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type {
  AnalysisVariant,
  ScoredAnalysisVariant,
  AnalysisScoringCriteria,
  AnalysisScoreDetails,
  AnalysisSelectionConstraints,
} from './analysis-types';

/**
 * Default balanced analysis scoring criteria
 */
export const DEFAULT_ANALYSIS_SCORING_CRITERIA: AnalysisScoringCriteria = {
  accuracy: { weight: 0.3, minScore: 0.6 },
  completeness: { weight: 0.3, minScore: 0.5 },
  relevance: { weight: 0.25, minScore: 0.5 },
  actionability: { weight: 0.15, minScore: 0.4 },
};

/**
 * Focus-specific scoring criteria presets
 */
const ANALYSIS_SCORING_PRESETS: Record<string, AnalysisScoringCriteria> = {
  comprehensive: {
    accuracy: { weight: 0.25, minScore: 0.5 },
    completeness: { weight: 0.4, minScore: 0.6 },
    relevance: { weight: 0.2, minScore: 0.4 },
    actionability: { weight: 0.15, minScore: 0.3 },
  },
  security: {
    accuracy: { weight: 0.35, minScore: 0.7 },
    completeness: { weight: 0.2, minScore: 0.5 },
    relevance: { weight: 0.3, minScore: 0.6 },
    actionability: { weight: 0.15, minScore: 0.4 },
  },
  performance: {
    accuracy: { weight: 0.3, minScore: 0.6 },
    completeness: { weight: 0.25, minScore: 0.5 },
    relevance: { weight: 0.25, minScore: 0.6 },
    actionability: { weight: 0.2, minScore: 0.4 },
  },
  architecture: {
    accuracy: { weight: 0.3, minScore: 0.5 },
    completeness: { weight: 0.35, minScore: 0.6 },
    relevance: { weight: 0.25, minScore: 0.5 },
    actionability: { weight: 0.1, minScore: 0.4 },
  },
  deployment: {
    accuracy: { weight: 0.25, minScore: 0.5 },
    completeness: { weight: 0.25, minScore: 0.4 },
    relevance: { weight: 0.2, minScore: 0.5 },
    actionability: { weight: 0.3, minScore: 0.6 },
  },
};

/**
 * Advanced analysis evaluator for detailed scoring
 */
class AnalysisEvaluator {
  constructor(private logger: Logger) {}

  /**
   * Comprehensive evaluation of an analysis variant
   */
  async evaluateAnalysis(variant: AnalysisVariant): Promise<
    Result<{
      accuracy: AnalysisAccuracyEval;
      completeness: AnalysisCompletenessEval;
      relevance: AnalysisRelevanceEval;
      actionability: AnalysisActionabilityEval;
    }>
  > {
    try {
      this.logger.debug({ variant: variant.id }, 'Starting analysis evaluation');

      const evaluation = {
        accuracy: this.evaluateAccuracy(variant),
        completeness: this.evaluateCompleteness(variant),
        relevance: this.evaluateRelevance(variant),
        actionability: this.evaluateActionability(variant),
      };

      this.logger.debug({ variant: variant.id }, 'Analysis evaluation completed');
      return Success(evaluation);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error({ error: message, variant: variant.id }, 'Analysis evaluation failed');
      return Failure(`Analysis evaluation failed: ${message}`);
    }
  }

  private evaluateAccuracy(variant: AnalysisVariant): AnalysisAccuracyEval {
    let score = 40;
    const strengths: string[] = [];
    const weaknesses: string[] = [];

    // Language detection accuracy (20 points)
    if (variant.language && variant.language !== 'unknown') {
      score += 20;
      strengths.push('Accurate language detection');
    } else {
      weaknesses.push('Failed to detect programming language');
    }

    // Framework detection accuracy (15 points)
    if (variant.framework) {
      score += 15;
      strengths.push('Framework identified');
    }

    // Build system detection accuracy (15 points)
    if (variant.buildSystem) {
      score += 15;
      strengths.push('Build system identified');
    }

    // Dependencies analysis accuracy (10 points)
    if (variant.dependencies.length > 0) {
      score += 10;
      strengths.push('Dependencies analyzed');

      // Penalty for too many dependencies (might indicate inaccurate parsing)
      if (variant.dependencies.length > 200) {
        score -= 5;
        weaknesses.push('Suspiciously high dependency count');
      }
    }

    // Confidence alignment with findings
    const expectedConfidence = this.calculateExpectedConfidence(variant);
    const confidenceDiff = Math.abs(variant.confidence - expectedConfidence);
    if (confidenceDiff < 10) {
      score += 5;
      strengths.push('Confidence aligned with findings');
    } else if (confidenceDiff > 30) {
      score -= 5;
      weaknesses.push('Confidence not aligned with findings');
    }

    return {
      score: Math.min(100, Math.max(0, score)),
      strengths,
      weaknesses,
      details: {
        languageDetection: !!variant.language && variant.language !== 'unknown',
        frameworkDetection: !!variant.framework,
        buildSystemDetection: !!variant.buildSystem,
        dependencyAnalysis: variant.dependencies.length > 0,
        confidenceAlignment: confidenceDiff < 20,
      },
    };
  }

  private evaluateCompleteness(variant: AnalysisVariant): AnalysisCompletenessEval {
    let score = 30;
    const coverageAreas: string[] = [];
    const missingAreas: string[] = [];

    // Core analysis completeness (40 points)
    if (variant.language) {
      score += 10;
      coverageAreas.push('Language identification');
    } else {
      missingAreas.push('Language identification');
    }

    if (variant.framework) {
      score += 8;
      coverageAreas.push('Framework detection');
    }

    if (variant.buildSystem) {
      score += 8;
      coverageAreas.push('Build system analysis');
    }

    if (variant.dependencies.length > 0) {
      score += 10;
      coverageAreas.push('Dependency analysis');
    } else {
      missingAreas.push('Dependency analysis');
    }

    if (variant.ports.length > 0) {
      score += 4;
      coverageAreas.push('Port detection');
    }

    // Infrastructure analysis (20 points)
    if (variant.hasDockerfile || variant.hasDockerCompose || variant.hasKubernetes) {
      score += 10;
      coverageAreas.push('Containerization status');
    }

    if (variant.recommendations && Object.keys(variant.recommendations).length > 0) {
      score += 10;
      coverageAreas.push('Recommendations provided');
    } else {
      missingAreas.push('Actionable recommendations');
    }

    // Insights depth (25 points)
    const totalInsights =
      variant.insights.keyFindings.length +
      variant.insights.riskAssessments.length +
      variant.insights.optimizationOpportunities.length +
      variant.insights.architecturalPatterns.length +
      variant.insights.deploymentReadiness.length;

    if (totalInsights > 15) {
      score += 25;
      coverageAreas.push('Rich insights provided');
    } else if (totalInsights > 10) {
      score += 20;
      coverageAreas.push('Good insights depth');
    } else if (totalInsights > 5) {
      score += 15;
      coverageAreas.push('Basic insights provided');
    } else {
      score += 5;
      missingAreas.push('Insufficient insights');
    }

    // Files analyzed (15 points)
    if (variant.filesAnalyzed > 50) {
      score += 15;
      coverageAreas.push('Comprehensive file analysis');
    } else if (variant.filesAnalyzed > 20) {
      score += 10;
      coverageAreas.push('Good file coverage');
    } else if (variant.filesAnalyzed > 0) {
      score += 5;
      coverageAreas.push('Basic file analysis');
    }

    return {
      score: Math.min(100, score),
      coverageAreas,
      missingAreas,
      metrics: {
        totalInsights,
        filesAnalyzed: variant.filesAnalyzed,
        completeness: variant.completeness,
        coveragePercentage: Math.min(100, (coverageAreas.length / 10) * 100),
      },
    };
  }

  private evaluateRelevance(variant: AnalysisVariant): AnalysisRelevanceEval {
    let score = 50;
    const relevantFindings: string[] = [];
    const irrelevantFindings: string[] = [];

    // Containerization relevance (30 points)
    const containerizationTerms = [
      'docker',
      'container',
      'image',
      'kubernetes',
      'k8s',
      'deployment',
    ];
    const containerizationRelevant = [
      ...variant.insights.keyFindings,
      ...variant.insights.optimizationOpportunities,
      ...variant.insights.deploymentReadiness,
    ].filter((finding) =>
      containerizationTerms.some((term) => finding.toLowerCase().includes(term)),
    ).length;

    score += Math.min(30, containerizationRelevant * 5);
    if (containerizationRelevant > 0) {
      relevantFindings.push('Containerization-focused insights');
    }

    // Build and deployment relevance (25 points)
    if (variant.buildSystem) {
      score += 15;
      relevantFindings.push('Build system analysis');
    }

    if (variant.insights.deploymentReadiness.length > 0) {
      score += 10;
      relevantFindings.push('Deployment readiness insights');
    }

    // Security relevance (20 points)
    const securityInsights = [
      ...variant.insights.riskAssessments,
      ...variant.insights.optimizationOpportunities,
    ].filter((insight) =>
      ['security', 'vulnerable', 'risk', 'auth', 'credential'].some((term) =>
        insight.toLowerCase().includes(term),
      ),
    ).length;

    score += Math.min(20, securityInsights * 4);
    if (securityInsights > 0) {
      relevantFindings.push('Security-focused insights');
    }

    // Performance relevance (15 points)
    const performanceInsights = [
      ...variant.insights.optimizationOpportunities,
      ...variant.insights.riskAssessments,
    ].filter((insight) =>
      ['performance', 'optimization', 'cache', 'memory', 'cpu', 'resource'].some((term) =>
        insight.toLowerCase().includes(term),
      ),
    ).length;

    score += Math.min(15, performanceInsights * 3);
    if (performanceInsights > 0) {
      relevantFindings.push('Performance insights');
    }

    // Architectural relevance (10 points)
    if (variant.insights.architecturalPatterns.length > 0) {
      score += 10;
      relevantFindings.push('Architectural patterns identified');
    }

    // Check for irrelevant findings
    const genericFindings = [
      ...variant.insights.keyFindings,
      ...variant.insights.optimizationOpportunities,
    ].filter(
      (finding) =>
        finding.toLowerCase().includes('file') && !finding.toLowerCase().includes('docker'),
    ).length;

    if (genericFindings > 5) {
      score -= 5;
      irrelevantFindings.push('Too many generic file-based findings');
    }

    return {
      score: Math.min(100, Math.max(0, score)),
      relevantFindings,
      irrelevantFindings,
      focus: {
        containerization: containerizationRelevant,
        security: securityInsights,
        performance: performanceInsights,
        architecture: variant.insights.architecturalPatterns.length,
      },
    };
  }

  private evaluateActionability(variant: AnalysisVariant): AnalysisActionabilityEval {
    let score = 30;
    const actionableItems: string[] = [];
    const vagueItems: string[] = [];

    // Optimization opportunities actionability (35 points)
    const specificOptimizations = variant.insights.optimizationOpportunities.filter((opt) =>
      this.isSpecificRecommendation(opt),
    ).length;

    score += Math.min(35, specificOptimizations * 7);
    if (specificOptimizations > 0) {
      actionableItems.push(`${specificOptimizations} specific optimization recommendations`);
    }

    const vagueOptimizations =
      variant.insights.optimizationOpportunities.length - specificOptimizations;
    if (vagueOptimizations > 2) {
      vagueItems.push('Some vague optimization suggestions');
    }

    // Risk assessment actionability (25 points)
    const actionableRisks = variant.insights.riskAssessments.filter((risk) =>
      this.isActionableRisk(risk),
    ).length;

    score += Math.min(25, actionableRisks * 5);
    if (actionableRisks > 0) {
      actionableItems.push(`${actionableRisks} actionable risk assessments`);
    }

    // Recommendations quality (20 points)
    if (variant.recommendations) {
      const recCount = Object.keys(variant.recommendations).length;
      score += Math.min(20, recCount * 5);
      if (recCount > 0) {
        actionableItems.push('Structured recommendations provided');
      }
    }

    // Deployment readiness actionability (20 points)
    const deploymentActions = variant.insights.deploymentReadiness.filter((item) =>
      ['add', 'configure', 'implement', 'create', 'setup'].some((action) =>
        item.toLowerCase().includes(action),
      ),
    ).length;

    score += Math.min(20, deploymentActions * 4);
    if (deploymentActions > 0) {
      actionableItems.push('Actionable deployment steps');
    }

    // Check for vague findings
    const vagueFindingsCount = variant.insights.keyFindings.filter((finding) =>
      ['detected', 'found', 'present', 'exists'].some(
        (vague) => finding.toLowerCase().includes(vague) && !this.hasSpecificDetails(finding),
      ),
    ).length;

    if (vagueFindingsCount > 3) {
      score -= 5;
      vagueItems.push('Too many vague findings without actionable details');
    }

    return {
      score: Math.min(100, Math.max(0, score)),
      actionableItems,
      vagueItems,
      metrics: {
        specificOptimizations,
        actionableRisks,
        deploymentActions,
        totalActionableItems: actionableItems.length,
      },
    };
  }

  private calculateExpectedConfidence(variant: AnalysisVariant): number {
    let expected = 40;

    if (variant.language && variant.language !== 'unknown') expected += 15;
    if (variant.framework) expected += 10;
    if (variant.buildSystem) expected += 10;
    if (variant.dependencies.length > 0) expected += 10;
    if (variant.filesAnalyzed > 10) expected += 10;
    if (variant.insights.keyFindings.length > 3) expected += 5;

    return Math.min(100, expected);
  }

  private isSpecificRecommendation(recommendation: string): boolean {
    const specificMarkers = [
      'use',
      'add',
      'configure',
      'implement',
      'replace',
      'update',
      'remove',
      'set',
      'install',
      'enable',
      'disable',
      'create',
      'modify',
    ];

    return (
      specificMarkers.some((marker) => recommendation.toLowerCase().includes(marker)) &&
      recommendation.length > 20
    ); // Avoid too brief recommendations
  }

  private isActionableRisk(risk: string): boolean {
    // Actionable risks should suggest what to do about them
    const actionableMarkers = [
      'should',
      'need to',
      'consider',
      'recommend',
      'ensure',
      'must',
      'avoid',
    ];

    return (
      actionableMarkers.some((marker) => risk.toLowerCase().includes(marker)) ||
      this.isSpecificRecommendation(risk)
    );
  }

  private hasSpecificDetails(finding: string): boolean {
    // Check if finding has specific details like numbers, filenames, or technical terms
    const hasNumbers = /\d+/.test(finding);
    const hasFilename = /\.(js|ts|py|java|go|rs|rb|php|json|yaml|yml|toml|xml)/.test(finding);
    const hasTechnicalTerms = ['docker', 'kubernetes', 'api', 'database', 'cache', 'auth'].some(
      (term) => finding.toLowerCase().includes(term),
    );

    return hasNumbers || hasFilename || hasTechnicalTerms;
  }
}

/**
 * Analysis variant scoring and selection service
 */
export class AnalysisVariantScorer {
  private evaluator: AnalysisEvaluator;

  constructor(private logger: Logger) {
    this.evaluator = new AnalysisEvaluator(logger);
  }

  /**
   * Score multiple analysis variants with given criteria
   */
  async scoreAnalysisVariants(
    variants: AnalysisVariant[],
    criteria: AnalysisScoringCriteria = DEFAULT_ANALYSIS_SCORING_CRITERIA,
  ): Promise<Result<ScoredAnalysisVariant[]>> {
    try {
      const scoredVariants: ScoredAnalysisVariant[] = [];

      for (const variant of variants) {
        const evaluationResult = await this.evaluator.evaluateAnalysis(variant);
        if (!evaluationResult.ok) {
          this.logger.warn(
            {
              variant: variant.id,
              error: evaluationResult.error,
            },
            'Skipping variant due to evaluation failure',
          );
          continue;
        }

        const evaluation = evaluationResult.value;
        const weightedScore =
          evaluation.accuracy.score * criteria.accuracy.weight +
          evaluation.completeness.score * criteria.completeness.weight +
          evaluation.relevance.score * criteria.relevance.weight +
          evaluation.actionability.score * criteria.actionability.weight;

        const scoreDetails: AnalysisScoreDetails = {
          total: Math.round(weightedScore),
          breakdown: {
            accuracy: evaluation.accuracy.score,
            completeness: evaluation.completeness.score,
            relevance: evaluation.relevance.score,
            actionability: evaluation.actionability.score,
          },
          strengths: [
            ...evaluation.accuracy.strengths,
            ...evaluation.completeness.coverageAreas.slice(0, 3),
            ...evaluation.relevance.relevantFindings.slice(0, 2),
            ...evaluation.actionability.actionableItems.slice(0, 2),
          ],
          weaknesses: [
            ...evaluation.accuracy.weaknesses,
            ...evaluation.completeness.missingAreas.slice(0, 2),
            ...evaluation.relevance.irrelevantFindings.slice(0, 2),
            ...evaluation.actionability.vagueItems.slice(0, 2),
          ],
          recommendations: this.generateScoringRecommendations(evaluation),
          confidence: variant.confidence,
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
        'Analysis variants scored and ranked',
      );

      return Success(scoredVariants);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      this.logger.error({ error: message }, 'Analysis variant scoring failed');
      return Failure(`Analysis scoring failed: ${message}`);
    }
  }

  /**
   * Select best analysis variant based on constraints
   */
  selectBestAnalysisVariant(
    scoredVariants: ScoredAnalysisVariant[],
    constraints?: AnalysisSelectionConstraints,
  ): ScoredAnalysisVariant | null {
    if (scoredVariants.length === 0) {
      return null;
    }

    let candidates = [...scoredVariants];

    // Apply constraints
    if (constraints) {
      if (constraints.minConfidence !== undefined) {
        const minConfidence = constraints.minConfidence;
        candidates = candidates.filter((v) => v.confidence >= minConfidence);
      }

      if (constraints.minCompleteness !== undefined) {
        const minCompleteness = constraints.minCompleteness;
        candidates = candidates.filter((v) => v.completeness >= minCompleteness);
      }

      if (constraints.mustInclude) {
        const mustInclude = constraints.mustInclude;
        candidates = candidates.filter((v) =>
          mustInclude.every((item) => this.variantIncludesInsight(v, item)),
        );
      }

      if (constraints.mustNotInclude) {
        const mustNotInclude = constraints.mustNotInclude;
        candidates = candidates.filter(
          (v) => !mustNotInclude.some((item) => this.variantIncludesInsight(v, item)),
        );
      }

      if (constraints.preferredPerspective) {
        // Prefer variants with matching perspective, but don't exclude others
        const preferred = candidates.filter(
          (v) => v.perspective === constraints.preferredPerspective,
        );
        if (preferred.length > 0) {
          candidates = preferred;
        }
      }
    }

    if (candidates.length === 0) {
      this.logger.warn('No analysis variants meet selection constraints');
      return scoredVariants.length > 0 ? (scoredVariants[0] ?? null) : null; // Return best overall if constraints too strict
    }

    const selected = candidates[0]; // We know candidates.length > 0 from check above
    if (!selected) {
      return null;
    }

    this.logger.info(
      {
        variant: selected.id,
        score: selected.score.total,
        perspective: selected.perspective,
        confidence: selected.confidence,
      },
      'Best analysis variant selected',
    );

    return selected;
  }

  /**
   * Get analysis scoring criteria preset by focus
   */
  getAnalysisScoringPreset(focus: string): AnalysisScoringCriteria {
    return ANALYSIS_SCORING_PRESETS[focus] || DEFAULT_ANALYSIS_SCORING_CRITERIA;
  }

  private generateScoringRecommendations(evaluation: AnalysisEvaluation): string[] {
    const recommendations: string[] = [];

    if (evaluation.accuracy.score < 70) {
      recommendations.push('Improve language and framework detection accuracy');
    }

    if (evaluation.completeness.score < 60) {
      recommendations.push('Provide more comprehensive analysis coverage');
    }

    if (evaluation.relevance.score < 70) {
      recommendations.push('Focus more on containerization-relevant insights');
    }

    if (evaluation.actionability.score < 60) {
      recommendations.push('Provide more specific and actionable recommendations');
    }

    if (evaluation.completeness.missingAreas.length > 2) {
      recommendations.push('Address missing analysis areas');
    }

    return recommendations;
  }

  private variantIncludesInsight(variant: AnalysisVariant, searchTerm: string): boolean {
    const allInsights = [
      ...variant.insights.keyFindings,
      ...variant.insights.riskAssessments,
      ...variant.insights.optimizationOpportunities,
      ...variant.insights.architecturalPatterns,
      ...variant.insights.deploymentReadiness,
    ];

    return allInsights.some((insight) => insight.toLowerCase().includes(searchTerm.toLowerCase()));
  }
}

// Analysis evaluation result interfaces
interface AnalysisEvaluation {
  accuracy: AnalysisAccuracyEval;
  completeness: AnalysisCompletenessEval;
  relevance: AnalysisRelevanceEval;
  actionability: AnalysisActionabilityEval;
}

interface AnalysisAccuracyEval {
  score: number;
  strengths: string[];
  weaknesses: string[];
  details: {
    languageDetection: boolean;
    frameworkDetection: boolean;
    buildSystemDetection: boolean;
    dependencyAnalysis: boolean;
    confidenceAlignment: boolean;
  };
}

interface AnalysisCompletenessEval {
  score: number;
  coverageAreas: string[];
  missingAreas: string[];
  metrics: {
    totalInsights: number;
    filesAnalyzed: number;
    completeness: number;
    coveragePercentage: number;
  };
}

interface AnalysisRelevanceEval {
  score: number;
  relevantFindings: string[];
  irrelevantFindings: string[];
  focus: {
    containerization: number;
    security: number;
    performance: number;
    architecture: number;
  };
}

interface AnalysisActionabilityEval {
  score: number;
  actionableItems: string[];
  vagueItems: string[];
  metrics: {
    specificOptimizations: number;
    actionableRisks: number;
    deploymentActions: number;
    totalActionableItems: number;
  };
}
