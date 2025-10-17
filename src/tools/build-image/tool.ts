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
import { normalizePath } from '@/lib/path-utils';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { promises as fs } from 'node:fs';
import { createStandardProgress } from '@/mcp/progress-helper';
import type { ToolContext } from '@/mcp/context';
import { createDockerClient, type DockerBuildOptions } from '@/infra/docker/client';
import { validatePath } from '@/lib/validation';

import { type Result, Success, Failure } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { fileExists } from '@/lib/file-utils';
import { type BuildImageParams, buildImageSchema } from './schema';

export interface BuildImageResult {
  /** Whether the build completed successfully */
  success: boolean;
  /** Generated Docker image ID (SHA256 hash) */
  imageId: string;
  /** Tags applied to the built image */
  tags: string[];
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
 * Build image handler - direct execution with selective progress
 */
async function handleBuildImage(
  params: BuildImageParams,
  context: ToolContext,
): Promise<Result<BuildImageResult>> {
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Optional progress reporting for complex operations (Docker build process)
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = getToolLogger(context, 'build-image');
  const timer = createToolTimer(logger, 'build-image');

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
    // Progress: Validating build parameters and environment
    if (progress) await progress('VALIDATING');

    // Validate build context path
    const buildContextResult = await validatePath(rawBuildPath, {
      mustExist: true,
      mustBeDirectory: true,
    });
    if (!buildContextResult.ok) {
      return buildContextResult;
    }

    // Normalize paths to handle Windows separators
    const buildContext = normalizePath(buildContextResult.value);
    const dockerfilePath = rawDockerfilePath ? normalizePath(rawDockerfilePath) : undefined;

    const startTime = Date.now();

    const dockerClient = createDockerClient(logger);

    // Determine paths
    const repoPath = buildContext;
    const finalDockerfilePath = dockerfilePath
      ? path.resolve(repoPath, dockerfilePath)
      : path.resolve(repoPath, dockerfile);

    if (!(await fileExists(finalDockerfilePath))) {
      return Failure(
        `Dockerfile not found at ${finalDockerfilePath}. Provide dockerfilePath parameter.`,
      );
    }

    // Read Dockerfile for security analysis
    let dockerfileContent: string;
    try {
      dockerfileContent = await fs.readFile(finalDockerfilePath, 'utf-8');
    } catch (error) {
      const err = error as { code?: string };
      if (err.code === 'EISDIR') {
        logger.error({ path: finalDockerfilePath }, 'Attempted to read directory as file');
        return Failure(`Dockerfile path points to a directory: ${finalDockerfilePath}`);
      }
      throw error;
    }

    // Prepare build arguments
    const finalBuildArgs = await prepareBuildArgs(buildArgs);

    // Analyze security
    const securityWarnings = analyzeBuildSecurity(dockerfileContent, finalBuildArgs);
    if (securityWarnings.length > 0) {
      logger.warn({ warnings: securityWarnings }, 'Security warnings found in build');
    }

    // Prepare Docker build options
    const buildOptions: DockerBuildOptions = {
      context: repoPath, // Build context is the path parameter
      dockerfile: path.relative(repoPath, finalDockerfilePath), // Dockerfile path relative to context
      buildargs: finalBuildArgs,
      ...(platform !== undefined && { platform }),
    };

    // Add tags if provided
    if (tags.length > 0 || imageName) {
      const finalTags = tags.length > 0 ? tags : imageName ? [imageName] : [];
      if (finalTags.length > 0) {
        const primaryTag = finalTags[0];
        if (primaryTag) {
          buildOptions.t = primaryTag; // Docker buildImage expects single tag
        }
      }
    }

    // Docker build process streams to provide real-time feedback
    if (progress) await progress('EXECUTING');

    // Build the image
    logger.info({ buildOptions, finalDockerfilePath }, 'About to call Docker buildImage');
    const buildResult = await dockerClient.buildImage(buildOptions);

    if (!buildResult.ok) {
      const errorMessage = buildResult.error ?? 'Unknown error';

      // Propagate Docker error guidance from infrastructure layer
      return Failure(`Failed to build image: ${errorMessage}`, buildResult.guidance);
    }

    const buildTime = Date.now() - startTime;

    if (progress) await progress('FINALIZING');

    // Prepare the result
    const finalTags = tags.length > 0 ? tags : imageName ? [imageName] : [];
    const result: BuildImageResult = {
      success: true,
      imageId: buildResult.value.imageId,
      tags: finalTags,
      size: (buildResult.value as unknown as { size?: number }).size ?? 0,
      ...((buildResult.value as unknown as { layers?: number }).layers !== undefined && {
        layers: (buildResult.value as unknown as { layers: number }).layers,
      }),
      buildTime,
      logs: buildResult.value.logs,
      ...(securityWarnings.length > 0 && { securityWarnings }),
    };

    timer.end({ imageId: buildResult.value.imageId, buildTime });

    if (progress) await progress('COMPLETE');

    return Success(result);
  } catch (error) {
    timer.error(error);

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Build image tool with selective progress reporting
 */
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
