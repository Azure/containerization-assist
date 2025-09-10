/**
 * Analysis Perspectives - Simple Enhancement for Repository Analysis
 *
 * Provides different analysis perspectives (security, performance, comprehensive)
 * that integrate seamlessly with the existing analyze-repo tool without complex types.
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type { AnalyzeRepoResult, AnalysisPerspective, PerspectiveConfig } from './types';

/**
 * Analysis perspective configurations
 */
export const ANALYSIS_PERSPECTIVES: Record<AnalysisPerspective, PerspectiveConfig> = {
  comprehensive: {
    perspective: 'comprehensive',
    emphasis: ['complete coverage', 'detailed analysis', 'thorough dependency review'],
    additionalChecks: [
      'architecture patterns',
      'deployment readiness',
      'scalability considerations',
      'monitoring hooks',
    ],
  },
  'security-focused': {
    perspective: 'security-focused',
    emphasis: ['security vulnerabilities', 'compliance requirements', 'access controls'],
    additionalChecks: [
      'vulnerable dependencies',
      'hardcoded secrets',
      'insecure configurations',
      'privilege escalation risks',
      'network security',
    ],
  },
  'performance-focused': {
    perspective: 'performance-focused',
    emphasis: ['performance bottlenecks', 'resource optimization', 'scalability'],
    additionalChecks: [
      'resource usage patterns',
      'caching opportunities',
      'database query optimization',
      'memory management',
      'CPU intensive operations',
    ],
  },
};

/**
 * Apply perspective-specific insights to analysis result
 */
export function applyAnalysisPerspective(
  baseAnalysis: AnalyzeRepoResult,
  perspective: AnalysisPerspective,
  logger: Logger,
): Result<AnalyzeRepoResult> {
  try {
    const config = ANALYSIS_PERSPECTIVES[perspective];

    // Create perspective-based recommendations
    const perspectiveRecommendations = {
      ...baseAnalysis.recommendations,
      perspective,
      perspectiveInsights: config.emphasis,
      additionalRecommendations: generatePerspectiveRecommendations(baseAnalysis, config, logger),
    };

    // Create result with perspective applied
    const analysisWithPerspective: AnalyzeRepoResult = {
      ...baseAnalysis,
      recommendations: perspectiveRecommendations,
      metadata: baseAnalysis.metadata
        ? {
            repoPath: baseAnalysis.metadata.repoPath,
            depth: baseAnalysis.metadata.depth,
            includeTests: baseAnalysis.metadata.includeTests,
            timestamp: baseAnalysis.metadata.timestamp,
            ...(baseAnalysis.metadata.aiInsights && {
              aiInsights: `${baseAnalysis.metadata.aiInsights} (${perspective} perspective)`,
            }),
          }
        : {
            repoPath: 'unknown',
            depth: 1,
            includeTests: false,
            timestamp: new Date().toISOString(),
          },
    };

    logger.info(
      {
        perspective,
        insights: config.emphasis.length,
        recommendations: perspectiveRecommendations.additionalRecommendations.length,
      },
      'Analysis with perspective applied',
    );

    return Success(analysisWithPerspective);
  } catch (error) {
    logger.error({ error, perspective }, 'Failed to apply analysis perspective');
    return Failure(
      `Failed to apply perspective: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}

/**
 * Generate perspective-specific recommendations
 */
function generatePerspectiveRecommendations(
  analysis: AnalyzeRepoResult,
  config: PerspectiveConfig,
  logger: Logger,
): string[] {
  const recommendations: string[] = [];

  switch (config.perspective) {
    case 'security-focused':
      recommendations.push(...generateSecurityRecommendations(analysis, logger));
      break;

    case 'performance-focused':
      recommendations.push(...generatePerformanceRecommendations(analysis, logger));
      break;

    case 'comprehensive':
      recommendations.push(...generateComprehensiveRecommendations(analysis, logger));
      break;
  }

  return recommendations;
}

/**
 * Generate security-focused recommendations
 */
function generateSecurityRecommendations(analysis: AnalyzeRepoResult, logger: Logger): string[] {
  const recommendations: string[] = [];

  // Check for security-sensitive dependencies
  const securitySensitiveDeps = analysis.dependencies.filter((dep) =>
    ['express', 'cors', 'helmet', 'jsonwebtoken', 'bcrypt'].includes(dep.name),
  );

  if (securitySensitiveDeps.length > 0) {
    recommendations.push('Review security configurations for web framework dependencies');
  }

  // Dockerfile security recommendations
  if (!analysis.hasDockerfile) {
    recommendations.push('Create Dockerfile with non-root user and minimal base image');
  }

  // Port security
  if (analysis.ports.some((port) => port < 1024)) {
    recommendations.push('Avoid using privileged ports (< 1024) in containers');
  }

  // Language-specific security
  if (analysis.language === 'javascript' || analysis.language === 'typescript') {
    recommendations.push('Run npm audit to check for known vulnerabilities');
    recommendations.push('Consider using --production flag to exclude dev dependencies');
  }

  logger.debug({ count: recommendations.length }, 'Generated security recommendations');
  return recommendations;
}

/**
 * Generate performance-focused recommendations
 */
function generatePerformanceRecommendations(analysis: AnalyzeRepoResult, logger: Logger): string[] {
  const recommendations: string[] = [];

  // Build system optimization
  if (analysis.buildSystem?.type === 'npm') {
    recommendations.push('Consider using npm ci for faster, reliable builds');
    recommendations.push('Use multi-stage builds to reduce final image size');
  }

  // Dependency optimization
  const devDeps = analysis.dependencies.filter((dep) => dep.type === 'development');
  if (devDeps.length > 0) {
    recommendations.push('Exclude development dependencies from production builds');
  }

  // Language-specific performance
  if (analysis.language === 'javascript' || analysis.language === 'typescript') {
    recommendations.push('Enable production optimizations (NODE_ENV=production)');
    recommendations.push('Consider using Alpine Linux base image for smaller size');
  }

  // Port and networking
  if (analysis.ports.length > 3) {
    recommendations.push('Review if all exposed ports are necessary for performance');
  }

  logger.debug({ count: recommendations.length }, 'Generated performance recommendations');
  return recommendations;
}

/**
 * Generate comprehensive recommendations
 */
function generateComprehensiveRecommendations(
  analysis: AnalyzeRepoResult,
  logger: Logger,
): string[] {
  const recommendations: string[] = [];

  // Documentation
  recommendations.push('Document containerization decisions in README');
  recommendations.push('Add healthcheck endpoint for container monitoring');

  // Testing
  if (!analysis.buildSystem?.testCommand) {
    recommendations.push('Add automated testing to build pipeline');
  }

  // Deployment readiness
  if (!analysis.hasKubernetes) {
    recommendations.push('Consider creating Kubernetes manifests for orchestration');
  }

  // Monitoring and observability
  recommendations.push('Add logging configuration for structured logs');
  recommendations.push('Consider adding metrics collection endpoint');

  // Framework-specific
  if (analysis.framework) {
    recommendations.push(`Follow ${analysis.framework} best practices for containerization`);
  }

  logger.debug({ count: recommendations.length }, 'Generated comprehensive recommendations');
  return recommendations;
}

/**
 * Select best perspective based on analysis context
 */
export function selectBestPerspective(
  analysis: AnalyzeRepoResult,
  preferences?: {
    securityFocus?: boolean;
    performanceFocus?: boolean;
  },
): AnalysisPerspective {
  // Use preferences if specified
  if (preferences?.securityFocus) {
    return 'security-focused';
  }
  if (preferences?.performanceFocus) {
    return 'performance-focused';
  }

  // Auto-select based on analysis characteristics
  const hasSecurityDeps = analysis.dependencies.some((dep) =>
    ['express', 'cors', 'helmet', 'jsonwebtoken', 'bcrypt', 'passport'].includes(dep.name),
  );

  const hasPerformanceDeps = analysis.dependencies.some((dep) =>
    ['redis', 'memcached', 'cluster', 'worker_threads'].includes(dep.name),
  );

  if (hasSecurityDeps && !hasPerformanceDeps) {
    return 'security-focused';
  }

  if (hasPerformanceDeps && !hasSecurityDeps) {
    return 'performance-focused';
  }

  // Default to comprehensive for balanced analysis
  return 'comprehensive';
}
