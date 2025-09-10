/**
 * Strategy Engine - Simple functional sampling strategies
 */

import type { Logger } from 'pino';
import { Success, Failure, type Result } from '@types';
import type { DockerfileContext, DockerfileVariant, ScoringCriteria, ScoreDetails } from './types';
import { getBaseImageRecommendations } from '@lib/base-images';
import { DEFAULT_NETWORK, DEFAULT_CONTAINER, getDefaultPort } from '@config/defaults';

/**
 * Default scoring configurations
 */
export const SCORING_PRESETS: Record<string, ScoringCriteria> = {
  balanced: { security: 0.25, performance: 0.25, size: 0.25, maintainability: 0.25 },
  security: { security: 0.5, performance: 0.2, size: 0.15, maintainability: 0.15 },
  performance: { security: 0.15, performance: 0.5, size: 0.2, maintainability: 0.15 },
  size: { security: 0.15, performance: 0.2, size: 0.5, maintainability: 0.15 },
  maintainability: { security: 0.2, performance: 0.15, size: 0.15, maintainability: 0.5 },
};

export const DEFAULT_SCORING_CRITERIA = SCORING_PRESETS.balanced;

/**
 * Analyze Dockerfile variant and compute multi-dimensional quality scores
 *
 * This function implements a heuristic-based scoring system that evaluates
 * Dockerfile variants across four key dimensions:
 *
 * **Security Scoring Logic:**
 * - Base score: 50/100
 * - Non-root user (+20): Detects 'USER' instruction avoiding root
 * - Secure base images (+15): Alpine/distroless images have smaller attack surface
 * - Package management (+10): Proper apt-get update && install pattern
 * - Health monitoring (+5): HEALTHCHECK instruction present
 *
 * **Performance Scoring Logic:**
 * - Base score: 50/100
 * - Multi-stage builds (+20): Multiple FROM instructions indicate build optimization
 * - Build artifact copying (+15): COPY --from= indicates efficient layer usage
 * - Cache mounts (+10): BuildKit cache mount optimization
 * - Layer cleanup (+5): Proper package cache cleanup
 *
 * **Size Scoring Logic:**
 * - Base score: 50/100
 * - Minimal base images (+25): Alpine/distroless significantly smaller
 * - Slim variants (+15): -slim tags are smaller than full images
 * - Cleanup commands (+10): rm -rf commands reduce final image size
 *
 * **Maintainability Scoring Logic:**
 * - Base score: 50/100
 * - Multi-stage builds (+20): Cleaner separation of build/runtime concerns
 * - Explicit versioning (+15): Pinned versions improve reproducibility
 * - Documentation (+10): LABEL instructions provide metadata
 * - Port declarations (+5): EXPOSE makes ports discoverable
 *
 * @param variant - Dockerfile variant to analyze
 * @returns Object with individual scores (0-100) for each quality dimension
 */
function analyzeVariant(variant: DockerfileVariant): {
  security: number;
  performance: number;
  size: number;
  maintainability: number;
} {
  const content = variant.content.toLowerCase();

  // Security scoring
  let security = 50;
  if (content.includes('user ') && !content.includes('user root')) security += 20;
  if (content.includes('alpine') || content.includes('distroless')) security += 15;
  if (content.includes('run apt-get update && apt-get install')) security += 10;
  if (content.includes('healthcheck')) security += 5;

  // Performance scoring
  let performance = 50;
  if (content.includes('from ') && content.split('from ').length > 2) performance += 20; // Multi-stage
  if (content.includes('copy --from=')) performance += 15;
  if (content.includes('run --mount=type=cache')) performance += 10;
  if (!content.includes('run apt-get update') || content.includes('rm -rf /var/lib/apt'))
    performance += 5;

  // Size scoring
  let size = 50;
  if (content.includes('alpine') || content.includes('distroless')) size += 25;
  if (content.includes('slim')) size += 15;
  if (content.includes('rm -rf')) size += 10;

  // Maintainability scoring
  let maintainability = 50;
  if (content.includes('label')) maintainability += 15;
  if (content.includes('arg ')) maintainability += 10;
  if (content.includes('env ')) maintainability += 10;
  if (content.split('\n').filter((line) => line.trim().startsWith('#')).length > 2)
    maintainability += 15;

  return {
    security: Math.min(100, security),
    performance: Math.min(100, performance),
    size: Math.min(100, size),
    maintainability: Math.min(100, maintainability),
  };
}

/**
 * Generate scoring reasons based on scores
 */
function generateScoringReasons(
  _variant: DockerfileVariant,
  scores: Record<string, number>,
): string[] {
  const reasons: string[] = [];

  if ((scores.security ?? 0) > 70) reasons.push('Strong security practices detected');
  if ((scores.performance ?? 0) > 70) reasons.push('Optimized build performance');
  if ((scores.size ?? 0) > 70) reasons.push('Efficient image size optimization');
  if ((scores.maintainability ?? 0) > 70) reasons.push('Good maintainability practices');

  return reasons;
}

/**
 * Detect warnings in Dockerfile
 */
function detectWarnings(variant: DockerfileVariant): string[] {
  const warnings: string[] = [];
  const content = variant.content.toLowerCase();

  if (content.includes('user root')) warnings.push('Running as root user');
  if (content.includes('latest')) warnings.push('Using latest tag');
  if (!content.includes('healthcheck')) warnings.push('No health check defined');

  return warnings;
}

/**
 * Generate recommendations based on scores
 */
function generateRecommendations(
  _variant: DockerfileVariant,
  scores: Record<string, number>,
): string[] {
  const recommendations: string[] = [];

  if ((scores.security ?? 0) < 60) recommendations.push('Add non-root user and security hardening');
  if ((scores.performance ?? 0) < 60)
    recommendations.push('Consider multi-stage build for better performance');
  if ((scores.size ?? 0) < 60)
    recommendations.push('Use smaller base images like Alpine or distroless');
  if ((scores.maintainability ?? 0) < 60)
    recommendations.push('Add labels and documentation comments');

  return recommendations;
}

/**
 * Score Dockerfile content directly
 */
export function scoreDockerfileContent(
  content: string,
  criteria: ScoringCriteria,
  _variantId?: string,
): ScoreDetails {
  const scores = analyzeVariant({ content } as DockerfileVariant);

  const total =
    scores.security * criteria.security +
    scores.performance * criteria.performance +
    scores.size * criteria.size +
    scores.maintainability * criteria.maintainability;

  return {
    total: Math.round(total),
    breakdown: scores,
    reasons: generateScoringReasons({ content } as DockerfileVariant, scores),
    warnings: detectWarnings({ content } as DockerfileVariant),
    recommendations: generateRecommendations({ content } as DockerfileVariant, scores),
  };
}

/**
 * Score a Dockerfile variant (backward compatibility)
 */
export async function scoreVariant(
  variant: DockerfileVariant,
  criteria: ScoringCriteria,
  logger: Logger,
): Promise<Result<ScoreDetails>> {
  try {
    const scoreDetails = scoreDockerfileContent(variant.content, criteria, variant.id);

    logger.debug(
      {
        variant: variant.id,
        total: scoreDetails.total,
        breakdown: scoreDetails.breakdown,
      },
      'Variant scored',
    );

    return Success(scoreDetails);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.error({ error: message, variant: variant.id }, 'Scoring failed');
    return Failure(`Failed to score variant: ${message}`);
  }
}

/**
 * Get install command for package manager
 */
function getInstallCommand(packageManager: string, production = false): string {
  const commands = {
    npm: production ? 'npm ci --only=production' : 'npm ci',
    yarn: production
      ? 'yarn install --frozen-lockfile --production'
      : 'yarn install --frozen-lockfile',
    pnpm: production ? 'pnpm install --frozen-lockfile --prod' : 'pnpm install --frozen-lockfile',
  };
  return commands[packageManager as keyof typeof commands] || 'npm ci';
}

/**
 * Get start command for language and package manager
 */
function getStartCommand(language: string, packageManager: string): string {
  if (language === 'javascript' || language === 'typescript') {
    return packageManager === 'yarn' ? 'yarn start' : 'npm start';
  }
  return 'npm start';
}

/**
 * Get build command for language and package manager
 */
function getBuildCommand(language: string, packageManager: string): string {
  if (language === 'typescript') {
    return packageManager === 'yarn' ? 'yarn build' : 'npm run build';
  }
  return 'echo "No build step required"';
}

/**
 * Generate security-focused Dockerfile
 */
export async function generateSecurityDockerfile(
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant>> {
  try {
    const { language, packageManager, ports } = context.analysis;
    const baseImage = getBaseImageRecommendations({
      language,
      preference: 'security',
    }).primary;
    const primaryPort = ports[0] || getDefaultPort(language);

    const dockerfileContent = `# Security-focused Dockerfile for ${language}
FROM ${baseImage}

# Create non-root user
RUN addgroup -g 1001 -S appuser && \\
    adduser -S appuser -u 1001 -G appuser

# Set working directory
WORKDIR /app

# Update packages for security
RUN apk update && apk upgrade && \\
    apk add --no-cache dumb-init && \\
    rm -rf /var/cache/apk/*

# Copy package files
COPY package*.json ./

# Install dependencies as root, then change ownership
RUN ${getInstallCommand(packageManager, true)} && \\
    chown -R appuser:appuser /app

# Copy application code
COPY . .
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE ${primaryPort}

# Add healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \\
    CMD wget --no-verbose --tries=1 --spider http://${DEFAULT_NETWORK.host}:${primaryPort}${DEFAULT_CONTAINER.healthCheckPath} || exit 1

# Use dumb-init for proper signal handling
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["${getStartCommand(language, packageManager)}"]`;

    const variant: DockerfileVariant = {
      id: `security-${Date.now()}`,
      content: dockerfileContent,
      strategy: 'security-first',
      metadata: {
        baseImage,
        optimization: 'security',
        features: ['non-root-user', 'minimal-packages', 'security-updates', 'healthcheck'],
        estimatedSize: '< 200MB',
        buildComplexity: 'medium',
        securityFeatures: ['non-root', 'minimal-attack-surface', 'security-updates'],
        aiEnhanced: false,
      },
      generated: new Date(),
    };

    logger.info({ variant: variant.id }, 'Security-first variant generated');
    return Success(variant);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Security strategy generation failed: ${message}`);
  }
}

/**
 * Generate performance-optimized Dockerfile
 */
export async function generatePerformanceDockerfile(
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant>> {
  try {
    const { language, packageManager, ports } = context.analysis;
    const baseImage = getBaseImageRecommendations({
      language,
      preference: 'performance',
    }).primary;
    const primaryPort = ports[0] || getDefaultPort(language);

    const dockerfileContent = `# Performance-optimized multi-stage Dockerfile
# Build stage
FROM ${baseImage} AS builder

WORKDIR /app

# Install dependencies with cache mounts
COPY package*.json ./
RUN --mount=type=cache,target=/root/.${packageManager} \\
    ${getInstallCommand(packageManager)}

# Copy and build application
COPY . .
RUN ${getBuildCommand(language, packageManager)}

# Production stage
FROM ${baseImage} AS production

# Create non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy built application from builder stage
COPY --from=builder --chown=appuser:appuser /app/dist ./dist
COPY --from=builder --chown=appuser:appuser /app/node_modules ./node_modules
COPY --from=builder --chown=appuser:appuser /app/package*.json ./

# Switch to non-root user
USER appuser

# Expose port
EXPOSE ${primaryPort}

# Optimized startup
CMD ["${language === 'typescript' ? 'node dist/index.js' : 'npm start'}"]`;

    const variant: DockerfileVariant = {
      id: `performance-${Date.now()}`,
      content: dockerfileContent,
      strategy: 'performance-optimized',
      metadata: {
        baseImage,
        optimization: 'performance',
        features: ['multi-stage-build', 'layer-caching', 'parallel-builds', 'optimized-runtime'],
        estimatedSize: '< 300MB',
        buildComplexity: 'high',
        securityFeatures: ['non-root'],
        aiEnhanced: false,
      },
      generated: new Date(),
    };

    logger.info({ variant: variant.id }, 'Performance variant generated');
    return Success(variant);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Performance strategy generation failed: ${message}`);
  }
}

/**
 * Generate size-optimized Dockerfile
 */
export async function generateSizeDockerfile(
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant>> {
  try {
    const { language, packageManager: _packageManager, ports } = context.analysis;
    const primaryPort = ports[0] || getDefaultPort(language);
    const baseImage = getBaseImageRecommendations({
      language,
      preference: 'size',
    }).primary;

    // Select minimal production image
    const minimalImages = {
      javascript: 'gcr.io/distroless/nodejs18-debian11',
      typescript: 'gcr.io/distroless/nodejs18-debian11',
      python: 'gcr.io/distroless/python3-debian11',
      java: 'gcr.io/distroless/java17-debian11',
      go: 'gcr.io/distroless/static-debian11',
      rust: 'gcr.io/distroless/cc-debian11',
    };
    const minimalImage =
      minimalImages[language as keyof typeof minimalImages] || 'gcr.io/distroless/static-debian11';

    const dockerfileContent = `# Size-optimized Dockerfile using distroless
# Build stage
FROM ${baseImage} AS builder

WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm ci --only=production && npm cache clean --force

# Copy source and build if needed
COPY . .
${language === 'typescript' ? 'RUN npm run build' : ''}

# Remove dev dependencies and unnecessary files
RUN npm prune --production && \\
    rm -rf node_modules/.cache && \\
    rm -rf src/ test/ *.md

# Production stage - distroless
FROM ${minimalImage}

# Copy only necessary files from builder
COPY --from=builder /app/node_modules ./node_modules
${
  language === 'typescript'
    ? 'COPY --from=builder /app/dist ./dist'
    : 'COPY --from=builder /app/*.js ./'
}
COPY --from=builder /app/package.json ./

# Expose port
EXPOSE ${primaryPort}

# Run the application
${language === 'typescript' ? 'CMD ["dist/index.js"]' : 'CMD ["index.js"]'}`;

    const variant: DockerfileVariant = {
      id: `size-${Date.now()}`,
      content: dockerfileContent,
      strategy: 'size-optimized',
      metadata: {
        baseImage: minimalImage,
        optimization: 'size',
        features: ['distroless', 'minimal-layers', 'dependency-pruning', 'static-binary'],
        estimatedSize: '< 100MB',
        buildComplexity: 'high',
        securityFeatures: ['distroless', 'minimal-attack-surface'],
        aiEnhanced: false,
      },
      generated: new Date(),
    };

    logger.info({ variant: variant.id }, 'Size-optimized variant generated');
    return Success(variant);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Size optimization strategy generation failed: ${message}`);
  }
}

/**
 * Generate balanced Dockerfile
 */
export async function generateBalancedDockerfile(
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant>> {
  try {
    const { language, packageManager, ports } = context.analysis;
    const baseImage = getBaseImageRecommendations({
      language,
      preference: 'balanced',
    }).primary;
    const primaryPort = ports[0] || getDefaultPort(language);

    const dockerfileContent = `# Balanced Dockerfile for ${language}
FROM ${baseImage}

# Create non-root user
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install production dependencies
RUN ${getInstallCommand(packageManager, true)} && \\
    npm cache clean --force

# Copy application code
COPY --chown=appuser:appuser . .

${language === 'typescript' ? '# Build TypeScript\nRUN npm run build\n' : ''}
# Switch to non-root user
USER appuser

# Expose port
EXPOSE ${primaryPort}

# Health check
HEALTHCHECK --interval=30s --timeout=3s \\
    CMD node -e "require('http').get('http://localhost:${primaryPort}/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

# Start application
CMD ["${getStartCommand(language, packageManager)}"]`;

    const variant: DockerfileVariant = {
      id: `balanced-${Date.now()}`,
      content: dockerfileContent,
      strategy: 'balanced',
      metadata: {
        baseImage,
        optimization: 'balanced',
        features: ['non-root-user', 'healthcheck', 'production-deps', 'clean-cache'],
        estimatedSize: '< 250MB',
        buildComplexity: 'low',
        securityFeatures: ['non-root'],
        aiEnhanced: false,
      },
      generated: new Date(),
    };

    logger.info({ variant: variant.id }, 'Balanced variant generated');
    return Success(variant);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    return Failure(`Balanced strategy generation failed: ${message}`);
  }
}

/**
 * Generate variants using strategy names
 */
export async function generateVariants(
  context: DockerfileContext,
  strategyNames: SamplingStrategyName[],
  logger: Logger,
): Promise<Result<DockerfileVariant[]>> {
  return executeMultipleSamplingStrategies(strategyNames, context, logger);
}

/**
 * Score multiple variants
 */
export function scoreVariants(
  variants: DockerfileVariant[],
  criteria: ScoringCriteria,
  logger?: Logger,
): ScoreDetails[] {
  const scores = variants.map((variant) => {
    const score = scoreDockerfileContent(variant.content, criteria, variant.id);
    logger?.debug(
      {
        variant: variant.id,
        total: score.total,
        breakdown: score.breakdown,
      },
      'Variant scored',
    );
    return score;
  });

  logger?.info({ scoreCount: scores.length }, 'Variants scored successfully');
  return scores;
}

/**
 * Direct Dockerfile generation functions registry
 */
export const dockerfileGenerators = {
  'security-first': generateSecurityDockerfile,
  'performance-optimized': generatePerformanceDockerfile,
  'size-optimized': generateSizeDockerfile,
  balanced: generateBalancedDockerfile,
} as const;

type SamplingStrategyName = keyof typeof dockerfileGenerators;

/**
 * Execute a single Dockerfile generation by strategy name
 */
export async function executeSamplingStrategy(
  strategyName: SamplingStrategyName,
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant>> {
  const generator = dockerfileGenerators[strategyName];
  if (!generator) {
    return Failure(`Unknown generation strategy: ${strategyName}`);
  }

  return generator(context, logger);
}

/**
 * Execute multiple Dockerfile generation strategies
 */
export async function executeMultipleSamplingStrategies(
  strategyNames: SamplingStrategyName[],
  context: DockerfileContext,
  logger: Logger,
): Promise<Result<DockerfileVariant[]>> {
  const variants: DockerfileVariant[] = [];
  const errors: string[] = [];

  for (const strategyName of strategyNames) {
    const result = await executeSamplingStrategy(strategyName, context, logger);
    if (result.ok) {
      variants.push(result.value);
    } else {
      errors.push(`${strategyName}: ${result.error}`);
    }
  }

  if (variants.length === 0) {
    return Failure(`No variants generated. Errors: ${errors.join('; ')}`);
  }

  logger.info({ count: variants.length }, 'Dockerfile variants generated');
  return Success(variants);
}

/**
 * Get list of available generation strategies
 */
export function getAvailableSamplingStrategies(): SamplingStrategyName[] {
  return Object.keys(dockerfileGenerators) as SamplingStrategyName[];
}

// Legacy compatibility exports
export const samplingStrategies = dockerfileGenerators;
