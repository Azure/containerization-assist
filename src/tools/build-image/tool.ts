/**
 * Build Docker images from Dockerfiles.
 * Handles multi-stage builds, build arguments, and platform-specific builds.
 *
 * @example
 * ```typescript
 * const result = await buildImage({
 *   sessionId: 'session-123',
 *   context: '/path/to/app',
 *   tags: ['myapp:latest', 'myapp:v1.0.0'],
 *   buildArgs: { NODE_ENV: 'production' }
 * }, context);
 * ```
 */

import path from 'node:path';
import { promises as fs } from 'node:fs';
import { getSession, updateSession } from '@mcp/tools/session-helpers';
import { createStandardProgress } from '@mcp/utils/progress-helper';
import type { ToolContext } from '../../mcp/context/types';
import { createDockerClient, type DockerBuildOptions } from '../../lib/docker';
import { createTimer, createLogger } from '../../lib/logger';
import { type Result, Success, Failure } from '../../domain/types';
import type { BuildImageParams } from './schema';

export interface BuildImageResult {
  /** Whether the build completed successfully */
  success: boolean;
  /** Session identifier used for this build */
  sessionId: string;
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
 * Check if file exists and is actually a file (not a directory)
 */
async function fileExists(filePath: string): Promise<boolean> {
  try {
    const stats = await fs.stat(filePath);
    return stats.isFile();
  } catch {
    return false;
  }
}

/**
 * Prepare build arguments with defaults
 */
interface SessionWithAnalysis {
  workflow_state?: {
    analysis_result?: {
      language?: string;
      framework?: string;
    };
  };
}

function prepareBuildArgs(
  buildArgs: Record<string, string> = {},
  session: SessionWithAnalysis | null | undefined,
): Record<string, string> {
  const defaults: Record<string, string> = {
    NODE_ENV: process.env.NODE_ENV ?? 'production',
    BUILD_DATE: new Date().toISOString(),
    VCS_REF: process.env.GIT_COMMIT ?? 'unknown',
  };

  // Add session-specific args if available
  const analysisResult = session?.workflow_state?.analysis_result;
  if (analysisResult) {
    if (analysisResult.language) {
      defaults.LANGUAGE = analysisResult.language;
    }
    if (analysisResult.framework) {
      defaults.FRAMEWORK = analysisResult.framework;
    }
  }

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
 * Build image implementation - direct execution with selective progress
 */
async function buildImageImpl(
  params: BuildImageParams,
  context: ToolContext,
): Promise<Result<BuildImageResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Optional progress reporting for complex operations (Docker build process)
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger || createLogger({ name: 'build-image' });
  const timer = createTimer(logger, 'build-image');

  try {
    const {
      context: buildContext = '.',
      dockerfile = 'Dockerfile',
      dockerfilePath,
      imageName,
      tags = [],
      buildArgs = {},
      platform,
    } = params;

    logger.info({ context: buildContext, dockerfile, tags }, 'Starting Docker image build');

    // Progress: Validating build parameters and environment
    if (progress) await progress('VALIDATING');

    const startTime = Date.now();

    // Get or create session
    const sessionResult = await getSession(params.sessionId, context);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    logger.info({ sessionId }, 'Starting Docker image build');

    const dockerClient = createDockerClient(logger);

    // Determine paths
    const sessionState = session as SessionWithAnalysis & { repo_path?: string };
    const repoPath = sessionState.repo_path ?? buildContext;
    let finalDockerfilePath = dockerfilePath
      ? path.resolve(repoPath, dockerfilePath)
      : path.resolve(repoPath, dockerfile);

    // Check if we should use a generated Dockerfile
    const dockerfileResult = (sessionState as Record<string, unknown>).dockerfile_result as
      | Record<string, unknown>
      | undefined;
    const generatedPath = dockerfileResult?.path as string | undefined;

    if (!(await fileExists(finalDockerfilePath))) {
      // If the specified Dockerfile doesn't exist, check for generated one
      if (generatedPath) {
        const resolvedGeneratedPath = path.resolve(repoPath, generatedPath);
        if (await fileExists(resolvedGeneratedPath)) {
          finalDockerfilePath = resolvedGeneratedPath;
          logger.info(
            { generatedPath: resolvedGeneratedPath, originalPath: dockerfile },
            'Using generated Dockerfile',
          );
        } else {
          // Generated path exists but file not found, check for content
          const dockerfileContent = dockerfileResult?.content as string | undefined;
          if (dockerfileContent) {
            // Write the Dockerfile content to generated file
            finalDockerfilePath = path.join(repoPath, 'Dockerfile.generated');
            await fs.writeFile(finalDockerfilePath, dockerfileContent, 'utf-8');
            logger.info(
              { dockerfilePath: finalDockerfilePath },
              'Created Dockerfile from session content',
            );
          } else {
            return Failure(
              `Dockerfile not found at: ${finalDockerfilePath} or ${resolvedGeneratedPath}`,
            );
          }
        }
      } else {
        // Check if we have Dockerfile content in session
        const dockerfileContent = dockerfileResult?.content as string | undefined;
        if (dockerfileContent) {
          // Write the Dockerfile content to generated file
          finalDockerfilePath = path.join(repoPath, 'Dockerfile.generated');
          await fs.writeFile(finalDockerfilePath, dockerfileContent, 'utf-8');
          logger.info(
            { dockerfilePath: finalDockerfilePath },
            'Created Dockerfile from session content',
          );
        } else {
          return Failure(
            `Dockerfile not found at ${finalDockerfilePath}. Provide dockerfilePath parameter or ensure session has Dockerfile from generate-dockerfile tool.`,
          );
        }
      }
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
    const finalBuildArgs = prepareBuildArgs(buildArgs, session as SessionWithAnalysis);

    // Analyze security
    const securityWarnings = analyzeBuildSecurity(dockerfileContent, finalBuildArgs);
    if (securityWarnings.length > 0) {
      logger.warn({ warnings: securityWarnings }, 'Security warnings found in build');
    }

    // Prepare Docker build options
    const buildOptions: DockerBuildOptions = {
      context: repoPath, // Build context is the repository path
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

    // Progress: Main build phase (Docker build execution)
    if (progress) await progress('EXECUTING');

    // Build the image
    logger.info({ buildOptions, finalDockerfilePath }, 'About to call Docker buildImage');
    const buildResult = await dockerClient.buildImage(buildOptions);

    if (!buildResult.ok) {
      return Failure(`Failed to build image: ${buildResult.error ?? 'Unknown error'}`);
    }

    const buildTime = Date.now() - startTime;

    // Progress: Finalizing build results and updating session
    if (progress) await progress('FINALIZING');

    // Update session with build result using simplified helper
    const finalTags = tags.length > 0 ? tags : imageName ? [imageName] : [];
    const updateResult = await updateSession(
      sessionId,
      {
        build_result: {
          success: true,
          imageId: buildResult.value.imageId ?? '',
          tags: finalTags,
          size: (buildResult.value as unknown as { size?: number }).size ?? 0,
          metadata: {
            layers: (buildResult.value as unknown as { layers?: number }).layers,
            buildTime,
            logs: buildResult.value.logs,
            securityWarnings,
          },
        },
        completed_steps: [...(session.completed_steps || []), 'build-image'],
      },
      context,
    );

    if (!updateResult.ok) {
      logger.warn({ error: updateResult.error }, 'Failed to update session, but build succeeded');
    }

    timer.end({ imageId: buildResult.value.imageId, buildTime });
    logger.info({ imageId: buildResult.value.imageId, buildTime }, 'Docker image build completed');

    // Progress: Complete
    if (progress) await progress('COMPLETE');

    return Success({
      success: true,
      sessionId,
      imageId: buildResult.value.imageId,
      tags: finalTags,
      size: (buildResult.value as unknown as { size?: number }).size ?? 0,
      ...((buildResult.value as unknown as { layers?: number }).layers !== undefined && {
        layers: (buildResult.value as unknown as { layers: number }).layers,
      }),
      buildTime,
      logs: buildResult.value.logs,
      ...(securityWarnings.length > 0 && { securityWarnings }),
      _chainHint: 'Next: scan_image, tag_image, or push_image',
    });
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Docker image build failed');

    return Failure(error instanceof Error ? error.message : String(error));
  }
}

/**
 * Build image tool with selective progress reporting
 */
export const buildImage = buildImageImpl;
