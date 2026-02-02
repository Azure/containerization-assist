/**
 * Build Image Context Preparation Tool
 *
 * Analyzes Dockerfiles and build contexts, returning structured recommendations
 * and commands for an agent to execute the build.
 *
 * This tool does NOT execute Docker builds - it provides context and guidance
 * for agents to run builds with optimal settings.
 *
 * @example
 * ```typescript
 * const result = await buildImageContext({
 *   path: '/path/to/app',
 *   imageName: 'myapp',
 *   tags: ['latest', 'v1.0.0'],
 * }, context);
 *
 * // Agent executes: result.value.nextAction.buildCommand.command
 * ```
 */

import path from 'path';
import fs from 'fs/promises';
import { normalizePath } from '@/lib/platform';
import { setupToolContext } from '@/lib/tool-context-helpers';
import type { ToolContext } from '@/core/context';
import { validatePathOrFail } from '@/lib/validation-helpers';
import { readDockerfile } from '@/lib/file-utils';

import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/errors';
import {
  type BuildImageParams,
  type BuildImageResult,
  type SecurityWarning,
  type BuildKitFeatures,
  type BuildCommand,
  buildImageSchema,
} from './schema';

/**
 * Prepare build arguments by merging user-provided args with default build metadata
 */
function prepareBuildArgs(buildArgs: Record<string, string> = {}): Record<string, string> {
  const defaults: Record<string, string> = {
    NODE_ENV: process.env.NODE_ENV ?? 'production',
    BUILD_DATE: new Date().toISOString(),
    VCS_REF: process.env.GIT_COMMIT ?? 'unknown',
  };

  return { ...defaults, ...buildArgs };
}

/**
 * Analyze Dockerfile for security issues with structured output
 */
function analyzeSecurityIssues(
  dockerfile: string,
  buildArgs: Record<string, string>,
): SecurityWarning[] {
  const warnings: SecurityWarning[] = [];
  const lines = dockerfile.split('\n');

  // Check for secrets in build args
  const sensitivePatterns = [
    'password',
    'token',
    'key',
    'secret',
    'api_key',
    'apikey',
    'credential',
    'private',
  ];
  for (const [key, value] of Object.entries(buildArgs)) {
    const keyLower = key.toLowerCase();
    if (sensitivePatterns.some((pattern) => keyLower.includes(pattern))) {
      warnings.push({
        id: 'secret-in-build-arg',
        severity: 'high',
        message: `Potential secret in build arg: ${key}`,
        remediation:
          'Use --mount=type=secret for sensitive data instead of build args. Build args are visible in image history.',
      });
    }
    // Also check if value looks like a secret
    if (value && (value.length > 20 || /^[A-Za-z0-9+/=]{20,}$/.test(value))) {
      warnings.push({
        id: 'secret-value-in-build-arg',
        severity: 'medium',
        message: `Build arg "${key}" may contain a secret value`,
        remediation:
          'Review if this value should be passed via --mount=type=secret instead of build args.',
      });
    }
  }

  // Check for sudo usage
  lines.forEach((line, index) => {
    if (/\bsudo\s/.test(line) && !line.trim().startsWith('#')) {
      warnings.push({
        id: 'sudo-usage',
        severity: 'medium',
        message: 'Using sudo in Dockerfile',
        line: index + 1,
        remediation:
          'RUN commands execute as root by default. Remove sudo and add a non-root USER directive at the end.',
      });
    }
  });

  // Check for :latest tags in FROM
  lines.forEach((line, index) => {
    const fromMatch = line.match(/^FROM\s+(\S+)/i);
    if (fromMatch) {
      const image = fromMatch[1];
      if (image?.endsWith(':latest') || (!image?.includes(':') && !image?.includes('@'))) {
        warnings.push({
          id: 'unpinned-base-image',
          severity: 'medium',
          message: `Unpinned base image: ${image}`,
          line: index + 1,
          remediation:
            'Pin base images to specific versions (e.g., node:20-alpine) or digests for reproducible builds.',
        });
      }
    }
  });

  // Check for root user
  const userDirectives = lines.filter((line) => /^USER\s+/i.test(line.trim()));
  const lastUser = userDirectives[userDirectives.length - 1];
  if (!lastUser || /^USER\s+(root|0)\s*$/i.test(lastUser.trim())) {
    warnings.push({
      id: 'runs-as-root',
      severity: 'high',
      message: 'Container runs as root user',
      remediation:
        'Add a non-root USER directive (e.g., USER node or USER 1000). Running as root increases attack surface.',
    });
  }

  // Check for ADD instead of COPY (potential remote URL fetch)
  lines.forEach((line, index) => {
    if (/^ADD\s+https?:\/\//i.test(line.trim())) {
      warnings.push({
        id: 'add-remote-url',
        severity: 'medium',
        message: 'ADD used with remote URL',
        line: index + 1,
        remediation:
          'Use RUN curl/wget instead of ADD for remote URLs. ADD auto-extracts archives which can be unexpected.',
      });
    }
  });

  // Check for chmod 777
  lines.forEach((line, index) => {
    if (/chmod\s+777/i.test(line)) {
      warnings.push({
        id: 'overly-permissive-chmod',
        severity: 'high',
        message: 'chmod 777 grants excessive permissions',
        line: index + 1,
        remediation: 'Use more restrictive permissions (e.g., 755 for directories, 644 for files).',
      });
    }
  });

  // Check for apt-get without --no-install-recommends
  lines.forEach((line, index) => {
    if (/apt-get\s+install/i.test(line) && !/--no-install-recommends/i.test(line)) {
      warnings.push({
        id: 'apt-install-recommends',
        severity: 'low',
        message: 'apt-get install without --no-install-recommends',
        line: index + 1,
        remediation:
          'Add --no-install-recommends to reduce image size by skipping optional packages.',
      });
    }
  });

  return warnings;
}

/**
 * Compute overall risk level from warnings
 */
function computeRiskLevel(warnings: SecurityWarning[]): 'low' | 'medium' | 'high' {
  const hasCritical = warnings.some((w) => w.severity === 'critical');
  const highCount = warnings.filter((w) => w.severity === 'high').length;
  const mediumCount = warnings.filter((w) => w.severity === 'medium').length;

  if (hasCritical || highCount >= 2) return 'high';
  if (highCount >= 1 || mediumCount >= 3) return 'medium';
  return 'low';
}

/**
 * Generate security recommendations based on warnings
 */
function generateSecurityRecommendations(warnings: SecurityWarning[]): string[] {
  const recommendations: string[] = [];

  if (warnings.some((w) => w.id === 'runs-as-root')) {
    recommendations.push(
      'Add a non-root USER directive to improve container security and follow least-privilege principle.',
    );
  }

  if (warnings.some((w) => w.id === 'secret-in-build-arg')) {
    recommendations.push(
      'Use BuildKit secrets (--mount=type=secret) instead of build args for sensitive data.',
    );
  }

  if (warnings.some((w) => w.id === 'unpinned-base-image')) {
    recommendations.push(
      'Pin base images to specific versions or SHA digests for reproducible, secure builds.',
    );
  }

  if (recommendations.length === 0) {
    recommendations.push(
      'No critical security issues detected. Consider running a vulnerability scan after build.',
    );
  }

  return recommendations;
}

/**
 * Analyze Dockerfile for BuildKit features
 */
function analyzeBuildKitFeatures(dockerfile: string): BuildKitFeatures {
  const lines = dockerfile.split('\n');

  // Detect multi-stage builds
  const fromStatements = lines.filter((line) => /^FROM\s+/i.test(line.trim()));
  const multiStage = fromStatements.length > 1;
  const stageCount = fromStatements.length;

  // Detect BuildKit-specific features
  const cacheMount = /--mount=type=cache/i.test(dockerfile);
  const secretMount = /--mount=type=secret/i.test(dockerfile);
  const sshMount = /--mount=type=ssh/i.test(dockerfile);
  const copyFrom = /COPY\s+--from=/i.test(dockerfile);
  const heredoc = /<<[-~]?\s*\w+/.test(dockerfile) || /<<[-~]?EOF/i.test(dockerfile);

  return {
    cacheMount,
    secretMount,
    sshMount,
    multiStage,
    stageCount,
    copyFrom,
    heredoc,
  };
}

/**
 * Generate BuildKit recommendations
 */
function generateBuildKitRecommendations(features: BuildKitFeatures, dockerfile: string): string[] {
  const recommendations: string[] = [];

  // If already using BuildKit features
  if (features.cacheMount || features.secretMount || features.sshMount || features.heredoc) {
    recommendations.push('Dockerfile uses BuildKit features - ensure DOCKER_BUILDKIT=1 is set.');
  }

  // Suggest cache mounts for package managers
  if (!features.cacheMount) {
    if (/npm\s+(install|ci)/i.test(dockerfile)) {
      recommendations.push(
        'Consider using --mount=type=cache,target=/root/.npm for npm cache to speed up builds.',
      );
    }
    if (/pip\s+install/i.test(dockerfile)) {
      recommendations.push(
        'Consider using --mount=type=cache,target=/root/.cache/pip for pip cache.',
      );
    }
    if (/go\s+(build|mod)/i.test(dockerfile)) {
      recommendations.push(
        'Consider using --mount=type=cache,target=/go/pkg/mod for Go module cache.',
      );
    }
  }

  // Multi-stage build suggestions
  if (!features.multiStage && dockerfile.length > 500) {
    recommendations.push(
      'Consider using multi-stage builds to reduce final image size by separating build and runtime stages.',
    );
  }

  return recommendations;
}

/**
 * Analyze Dockerfile content for metadata
 */
function analyzeDockerfileContent(dockerfile: string): {
  baseImages: string[];
  exposedPorts: number[];
  finalUser?: string;
  hasHealthcheck: boolean;
  layerCount: number;
} {
  const lines = dockerfile.split('\n');

  // Extract base images
  const baseImages = lines
    .filter((line) => /^FROM\s+/i.test(line.trim()))
    .map((line) => {
      const match = line.match(/^FROM\s+(\S+)/i);
      return match?.[1] ?? '';
    })
    .filter(Boolean);

  // Extract exposed ports
  const exposedPorts: number[] = [];
  lines.forEach((line) => {
    const match = line.match(/^EXPOSE\s+(.+)/i);
    if (match?.[1]) {
      const ports = match[1].split(/\s+/).map((p) => {
        const portStr = p.split('/')[0];
        return portStr ? parseInt(portStr, 10) : NaN;
      });
      exposedPorts.push(...ports.filter((p) => !isNaN(p)));
    }
  });

  // Find final USER
  const userDirectives = lines.filter((line) => /^USER\s+/i.test(line.trim()));
  const lastUserLine = userDirectives[userDirectives.length - 1];
  const finalUser = lastUserLine?.match(/^USER\s+(\S+)/i)?.[1];

  // Check for HEALTHCHECK
  const hasHealthcheck = lines.some((line) => /^HEALTHCHECK\s+/i.test(line.trim()));

  // Estimate layer count (each RUN, COPY, ADD creates a layer)
  const layerCount = lines.filter((line) => {
    const trimmed = line.trim();
    return /^(RUN|COPY|ADD)\s+/i.test(trimmed) && !trimmed.startsWith('#');
  }).length;

  const result: {
    baseImages: string[];
    exposedPorts: number[];
    finalUser?: string;
    hasHealthcheck: boolean;
    layerCount: number;
  } = {
    baseImages,
    exposedPorts,
    hasHealthcheck,
    layerCount,
  };

  if (finalUser !== undefined) {
    result.finalUser = finalUser;
  }

  return result;
}

/**
 * Combine image name with tag to create a full image reference
 */
function combineImageNameAndTag(imageName: string | undefined, tag: string): string {
  if (tag.includes(':') || tag.includes('/')) {
    return tag;
  }
  if (!imageName) {
    return tag;
  }
  return `${imageName}:${tag}`;
}

/**
 * Compute final tags from imageName and tags array
 */
function computeFinalTags(imageName?: string, tags?: string[]): string[] {
  if (tags && tags.length > 0) {
    return tags.map((tag) => combineImageNameAndTag(imageName, tag));
  }
  if (imageName) {
    // Ensure imageName has a tag
    return [imageName.includes(':') ? imageName : `${imageName}:latest`];
  }
  return ['app:latest'];
}

/**
 * Generate Docker build command
 */
function generateBuildCommand(
  contextPath: string,
  dockerfileRelative: string,
  finalTags: string[],
  buildArgs: Record<string, string>,
  platform: string,
  useBuildKit: boolean,
): BuildCommand {
  const flags: string[] = [];

  // Dockerfile path
  if (dockerfileRelative !== 'Dockerfile') {
    flags.push(`-f ${dockerfileRelative}`);
  }

  // Tags
  for (const tag of finalTags) {
    flags.push(`-t ${tag}`);
  }

  // Build args
  for (const [key, value] of Object.entries(buildArgs)) {
    // Escape values with spaces or special characters
    const escapedValue = value.includes(' ') || value.includes('"') ? `"${value}"` : value;
    flags.push(`--build-arg ${key}=${escapedValue}`);
  }

  // Platform
  flags.push(`--platform ${platform}`);

  // BuildKit progress output
  if (useBuildKit) {
    flags.push('--progress=plain');
  }

  const command = `docker build ${flags.join(' ')} ${contextPath}`;

  return {
    command,
    parts: {
      executable: 'docker',
      subcommand: 'build',
      flags,
      context: contextPath,
    },
    environment: useBuildKit ? { DOCKER_BUILDKIT: '1' } : {},
  };
}

/**
 * Check if .dockerignore exists
 */
async function checkDockerignore(contextPath: string): Promise<boolean> {
  try {
    await fs.access(path.join(contextPath, '.dockerignore'));
    return true;
  } catch {
    return false;
  }
}

/**
 * Build image context preparation handler
 */
async function handleBuildImage(
  params: BuildImageParams,
  context: ToolContext,
): Promise<Result<BuildImageResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided', {
      message: 'Parameters must be a valid object',
      hint: 'Tool received invalid or missing parameters',
      resolution: 'Ensure parameters are provided as a JSON object',
    });
  }

  const { logger, timer } = setupToolContext(context, 'build-image-context');

  const {
    path: rawBuildPath = '.',
    dockerfile = 'Dockerfile',
    dockerfilePath: rawDockerfilePath,
    imageName,
    tags = [],
    buildArgs = {},
    platform = 'linux/amd64',
  } = params;

  try {
    // Validate build context path
    const buildContextResult = await validatePathOrFail(rawBuildPath, {
      mustExist: true,
      mustBeDirectory: true,
    });
    if (!buildContextResult.ok) return buildContextResult;

    // Normalize paths
    const buildContextPath = normalizePath(buildContextResult.value);
    const dockerfilePath = rawDockerfilePath ? normalizePath(rawDockerfilePath) : undefined;
    const dockerfileRelativePath = dockerfilePath || dockerfile;
    const finalDockerfilePath = path.resolve(buildContextPath, dockerfileRelativePath);

    // Read Dockerfile
    const dockerfileContentResult = await readDockerfile({
      path: finalDockerfilePath,
    });

    if (!dockerfileContentResult.ok) {
      return dockerfileContentResult;
    }

    const dockerfileContent = dockerfileContentResult.value;

    // Prepare build arguments
    const finalBuildArgs = prepareBuildArgs(buildArgs);

    // Security analysis
    const securityWarnings = analyzeSecurityIssues(dockerfileContent, finalBuildArgs);
    const riskLevel = computeRiskLevel(securityWarnings);
    const securityRecommendations = generateSecurityRecommendations(securityWarnings);

    logger.debug(
      { warningCount: securityWarnings.length, riskLevel },
      'Security analysis complete',
    );

    // BuildKit analysis
    const buildKitFeatures = analyzeBuildKitFeatures(dockerfileContent);
    const buildKitRecommendations = generateBuildKitRecommendations(
      buildKitFeatures,
      dockerfileContent,
    );
    const useBuildKit =
      buildKitFeatures.cacheMount ||
      buildKitFeatures.secretMount ||
      buildKitFeatures.sshMount ||
      buildKitFeatures.heredoc ||
      buildKitFeatures.multiStage;

    // Dockerfile analysis
    const dockerfileAnalysis = analyzeDockerfileContent(dockerfileContent);

    // Compute final tags
    const finalTags = computeFinalTags(imageName, tags);

    // Check for .dockerignore
    const hasDockerignore = await checkDockerignore(buildContextPath);

    // Generate build command
    const dockerfileRelative = path.relative(buildContextPath, finalDockerfilePath);
    const buildCommand = generateBuildCommand(
      buildContextPath,
      dockerfileRelative,
      finalTags,
      finalBuildArgs,
      platform,
      useBuildKit,
    );

    // Generate fallback command (without BuildKit)
    const fallbackCommand = useBuildKit
      ? generateBuildCommand(
          buildContextPath,
          dockerfileRelative,
          finalTags,
          finalBuildArgs,
          platform,
          false,
        )
      : undefined;

    // Pre-checks for agent
    const preChecks: string[] = ['Verify Docker daemon is running: docker info'];
    if (!hasDockerignore) {
      preChecks.push(
        'Consider creating .dockerignore to exclude unnecessary files from build context',
      );
    }
    if (riskLevel === 'high') {
      preChecks.push('Review security warnings before proceeding with build');
    }

    // Post-build suggestions
    const postBuildSteps: string[] = [
      `Verify image was created: docker images | grep ${finalTags[0]?.split(':')[0] || 'app'}`,
      'Run vulnerability scan: docker scout quickview or trivy image',
    ];
    if (!dockerfileAnalysis.hasHealthcheck) {
      postBuildSteps.push('Consider adding HEALTHCHECK to Dockerfile for production deployments');
    }

    // Generate summary
    const warningText =
      securityWarnings.length > 0
        ? ` Found ${securityWarnings.length} security warning(s) (${riskLevel} risk).`
        : ' No security issues detected.';
    const buildKitText = useBuildKit ? ' BuildKit recommended.' : '';
    const summary = `Build context prepared for ${finalTags[0] || 'image'}.${warningText}${buildKitText}`;

    const result: BuildImageResult = {
      summary,
      context: {
        buildContextPath,
        dockerfilePath: finalDockerfilePath,
        dockerfileRelative,
        hasDockerignore,
      },
      securityAnalysis: {
        warnings: securityWarnings,
        riskLevel,
        recommendations: securityRecommendations,
      },
      buildConfig: {
        finalTags,
        buildArgs: finalBuildArgs,
        platform,
      },
      buildKitAnalysis: {
        features: buildKitFeatures,
        recommended: useBuildKit,
        recommendations: buildKitRecommendations,
      },
      dockerfileAnalysis,
      nextAction: {
        action: 'execute-build',
        preChecks,
        buildCommand,
        ...(fallbackCommand && { fallbackCommand }),
        postBuildSteps,
      },
    };

    timer.end({ tags: finalTags, riskLevel });
    return Success(result);
  } catch (error) {
    timer.error(error);

    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred during build context preparation',
      resolution:
        'Check the error message above for details. Common issues include invalid paths or unreadable Dockerfile',
    });
  }
}

export const buildImageContext = handleBuildImage;

import { tool } from '@/types/tool';

export default tool({
  name: 'build-image-context',
  description:
    'Prepare Docker build context with security analysis and optimized build commands. Returns structured guidance for executing builds.',
  version: '3.0.0',
  schema: buildImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  handler: handleBuildImage,
});
