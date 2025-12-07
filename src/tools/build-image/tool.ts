/**
 * Build Docker images from Dockerfiles.
 * Handles multi-stage builds, build arguments, and platform-specific builds.
 *
 * @example
 * ```typescript
 * const result = await buildImage({
 *   path: '/path/to/app',
 *   tags: ['myapp:latest', 'myapp:v1.0.0'],
 *   buildArgs: { NODE_ENV: 'production' }
 * }, context);
 * ```
 */

import path from 'path';
import { normalizePath } from '@/lib/platform';
import { setupToolContext } from '@/lib/tool-context-helpers';
import type { ToolContext } from '@/core/context';
import { createDockerClient, type DockerBuildOptions } from '@/infra/docker/client';
import { validatePathOrFail, parseImageName } from '@/lib/validation-helpers';
import { readDockerfile } from '@/lib/file-utils';

import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/errors';
import { type BuildImageParams, buildImageSchema } from './schema';
import { formatSize, formatDuration } from '@/lib/summary-helpers';

export interface BuildImageResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the build outcome, image details, and next steps.
   * @example "✅ Built image successfully. Image: myapp:latest (245MB). Build completed in 45s."
   */
  summary?: string;
  /** Whether the build completed successfully */
  success: boolean;
  /** Generated Docker image ID (SHA256 hash) */
  imageId: string;
  /** All tags that were requested/attempted to be created */
  requestedTags: string[];
  /** Successfully created tags with full image names */
  createdTags: string[];
  /** Tags that failed to be created (if any) */
  failedTags?: string[];
  /** Final image size in bytes */
  size: number;
  /** Number of layers in the image */
  layers?: number;
  /** Total build time in milliseconds */
  buildTime: number;
  /** Complete build output logs */
  logs: string[];
  /** Security-related warnings discovered during build */
  securityWarnings?: string[];
}


/**
 * Prepare build arguments by merging user-provided args with default build metadata
 */
async function prepareBuildArgs(
  buildArgs: Record<string, string> = {},
): Promise<Record<string, string>> {
  const defaults: Record<string, string> = {
    NODE_ENV: process.env.NODE_ENV ?? 'production',
    BUILD_DATE: new Date().toISOString(),
    VCS_REF: process.env.GIT_COMMIT ?? 'unknown',
  };

  return { ...defaults, ...buildArgs };
}

/**
 * Combine image name with tag to create a full image reference
 * If tag already contains a colon or slash (indicating it's a full reference), return as-is
 * Otherwise, combine imageName with tag
 */
function combineImageNameAndTag(imageName: string | undefined, tag: string): string {
  // If tag already looks like a full reference (contains : or /), use as-is
  if (tag.includes(':') || tag.includes('/')) {
    return tag;
  }

  // If no imageName provided, use tag as-is (will get 'latest' appended by Docker if needed)
  if (!imageName) {
    return tag;
  }

  // Combine imageName with tag
  return `${imageName}:${tag}`;
}

/**
 * Apply additional tags to a built image, returning any tags that failed to apply
 */

async function applyAdditionalTags(
  imageId: string,
  tags: string[],
  dockerClient: ReturnType<typeof createDockerClient>,
  logger: ReturnType<typeof setupToolContext>['logger'],
): Promise<string[]> {
  const failedTags: string[] = [];
  for (const tag of tags) {
    const parsedTag = parseImageName(tag);
    if (!parsedTag.ok) {
      logger.warn({ imageId, tag, error: parsedTag.error }, 'Failed to parse tag - skipping');
      failedTags.push(tag);
      continue;
    }

    const { repository, tag: tagName, registry } = parsedTag.value;
    const fullRepository = registry ? `${registry}/${repository}` : repository;

    const tagResult = await dockerClient.tagImage(imageId, fullRepository, tagName);
    if (!tagResult.ok) {
      failedTags.push(tag);
      logger.warn({ imageId, tag, error: tagResult.error }, 'Failed to apply tag');
    }
  }
  return failedTags;
}

/**
 * Analyze build for security issues
 */
function analyzeBuildSecurity(dockerfile: string, buildArgs: Record<string, string>): string[] {
  const warnings: string[] = [];

  // Check for secrets in build args
  const sensitiveKeys = ['password', 'token', 'key', 'secret', 'api_key', 'apikey'];
  for (const key of Object.keys(buildArgs)) {
    if (sensitiveKeys.some((sensitive) => key.toLowerCase().includes(sensitive))) {
      warnings.push(`Potential secret in build arg: ${key}`);
    }
  }

  // Check for sudo in Dockerfile
  if (dockerfile.includes('sudo ')) {
    warnings.push('Using sudo in Dockerfile - consider running as non-root');
  }

  // Check for latest tags
  if (dockerfile.includes(':latest')) {
    warnings.push('Using :latest tag - consider pinning versions for reproducibility');
  }

  // Check for root user
  if (!dockerfile.includes('USER ') || dockerfile.includes('USER root')) {
    warnings.push('Container may run as root - consider adding a non-root USER');
  }

  return warnings;
}

/**
 * Build Docker image handler
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

  const { logger, timer } = setupToolContext(context, 'build-image');

  const {
    path: rawBuildPath = '.',
    dockerfile = 'Dockerfile',
    dockerfilePath: rawDockerfilePath,
    imageName = 'app:latest',
    tags = [],
    buildArgs = {},
    platform,
  } = params;

  try {
    // Validate build context path
    const buildContextResult = await validatePathOrFail(rawBuildPath, {
      mustExist: true,
      mustBeDirectory: true,
    });
    if (!buildContextResult.ok) return buildContextResult;

    // Normalize paths
    const buildContext = normalizePath(buildContextResult.value);
    const dockerfilePath = rawDockerfilePath ? normalizePath(rawDockerfilePath) : undefined;
    const dockerfileRelativePath = dockerfilePath || dockerfile;
    const finalDockerfilePath = path.resolve(buildContext, dockerfileRelativePath);

    const dockerClient = createDockerClient(logger);

    // Read Dockerfile for security analysis
    const dockerfileContentResult = await readDockerfile({
      path: finalDockerfilePath,
    });

    if (!dockerfileContentResult.ok) {
      return dockerfileContentResult;
    }

    const dockerfileContent = dockerfileContentResult.value;

    // Prepare build arguments
    const finalBuildArgs = await prepareBuildArgs(buildArgs);

    // Analyze security
    const securityWarnings = analyzeBuildSecurity(dockerfileContent, finalBuildArgs);
    if (securityWarnings.length > 0) {
      logger.warn({ warnings: securityWarnings }, 'Security warnings found in build');
    }

    // Determine final tags to apply to the image
    let finalTags: string[] = [];
    if (tags.length > 0) {
      // Combine each tag with imageName (if tag is not already a full reference)
      finalTags = tags.map((tag) => combineImageNameAndTag(imageName, tag));
    } else if (imageName) {
      // No tags provided, use imageName as-is (Docker will default to 'latest' if no tag)
      finalTags = [imageName];
    }

    // Log the final tags that will be applied
    logger.info(
      { finalTags, originalTags: tags, imageName },
      'Determined final tags for build',
    );

    // Prepare Docker build options
    const buildOptions: DockerBuildOptions = {
      context: buildContext,
      dockerfile: path.relative(buildContext, finalDockerfilePath),
      buildargs: finalBuildArgs,
      ...(platform !== undefined && { platform }),
      ...(finalTags.length > 0 && finalTags[0] && { t: finalTags[0] }),
    };

    // Build the image
    logger.info({ buildOptions, finalDockerfilePath }, 'About to call Docker buildImage');
    const buildResult = await dockerClient.buildImage(buildOptions);

    if (!buildResult.ok) {
      const errorMessage = buildResult.error ?? 'Unknown error';

      // Propagate Docker error guidance from infrastructure layer
      const guidance = buildResult.guidance;
      const buildLogs = (guidance?.details?.buildLogs as string[]) || [];

      let detailedError = `Failed to build image: ${errorMessage}`;
      if (buildLogs.length > 0) {
        detailedError += `\n\nBuild logs:\n${buildLogs.join('\n')}`;
      }

      return Failure(detailedError, guidance);
    }

    // Apply additional tags to the built image
    let failedTags: string[] = [];
    if (finalTags.length > 1 && buildResult.value.imageId) {
      const additionalTags = finalTags.slice(1);
      logger.info(
        { imageId: buildResult.value.imageId, additionalTags },
        'Applying additional tags',
      );
      failedTags = await applyAdditionalTags(
        buildResult.value.imageId,
        additionalTags,
        dockerClient,
        logger,
      );
      if (failedTags.length > 0) {
        logger.warn({ failedTags }, 'Some tags failed to apply');
      }
    }

    // Generate summary
    const successfulTags = finalTags.filter((tag) => !failedTags.includes(tag));
    const imageTag = successfulTags[0] || buildResult.value.imageId;
    const sizeText = buildResult.value.size ? ` (${formatSize(buildResult.value.size)})` : '';
    const timeText = buildResult.value.buildTime
      ? ` Build completed in ${formatDuration(Math.round(buildResult.value.buildTime / 1000))}.`
      : '';

    const failedTagsText =
      failedTags.length > 0
        ? ` ⚠️  Failed to apply ${failedTags.length} tag(s): ${failedTags.join(', ')}`
        : '';

    const summary = `✅ Built image successfully. Image: ${imageTag}${sizeText}.${timeText}${failedTagsText}`;

    const result: BuildImageResult = {
      summary,
      success: true,
      imageId: buildResult.value.imageId,
      requestedTags: finalTags,
      createdTags: successfulTags,
      size: buildResult.value.size,
      ...(buildResult.value.layers !== undefined && { layers: buildResult.value.layers }),
      buildTime: buildResult.value.buildTime,
      logs: buildResult.value.logs,
      ...(securityWarnings.length > 0 && { securityWarnings }),
      ...(failedTags.length > 0 && { failedTags }),
    };

    timer.end({ imageId: buildResult.value.imageId, buildTime: buildResult.value.buildTime });
    return Success(result);
  } catch (error) {
    timer.error(error);

    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred during the Docker build process',
      resolution:
        'Check the error message above for details. Common issues include Docker daemon not running, insufficient permissions, or invalid build context',
    });
  }
}

export const buildImage = handleBuildImage;

import { tool } from '@/types/tool';

export default tool({
  name: 'build-image',
  description: 'Build Docker images from Dockerfiles with security analysis',
  version: '2.0.0',
  schema: buildImageSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  handler: handleBuildImage,
});
