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

import { resolvePath, joinPaths, getRelativePath, safeNormalizePath } from '@lib/path-utils';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import { promises as fs } from 'node:fs';
import { ensureSession, defineToolIO, useSessionSlice } from '@mcp/tool-session-helpers';
import { createStandardProgress } from '@mcp/progress-helper';
import type { ToolContext } from '../../mcp/context';
import { createDockerClient, type DockerBuildOptions } from '../../lib/docker';

import { type Result, Success, Failure } from '../../types';
import { extractErrorMessage } from '../../lib/error-utils';
import { fileExists } from '@lib/file-utils';
import { buildImageSchema, type BuildImageParams } from './schema';
import { z } from 'zod';
import type { SessionData } from '../session-types';

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

// Define the result schema for type safety
const BuildImageResultSchema = z.object({
  success: z.boolean(),
  sessionId: z.string(),
  imageId: z.string(),
  tags: z.array(z.string()),
  size: z.number(),
  layers: z.number().optional(),
  buildTime: z.number(),
  logs: z.array(z.string()),
  securityWarnings: z.array(z.string()).optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(buildImageSchema, BuildImageResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastBuiltAt: z.date().optional(),
  lastBuiltImageId: z.string().optional(),
  lastBuiltTags: z.array(z.string()).optional(),
  totalBuilds: z.number().optional(),
  lastBuildTime: z.number().optional(),
  lastSecurityWarningCount: z.number().optional(),
});

/**
 * Prepare build arguments with defaults
 */
function prepareBuildArgs(
  buildArgs: Record<string, string> = {},
  session: SessionData | null | undefined,
): Record<string, string> {
  const defaults: Record<string, string> = {
    NODE_ENV: process.env.NODE_ENV ?? 'production',
    BUILD_DATE: new Date().toISOString(),
    VCS_REF: process.env.GIT_COMMIT ?? 'unknown',
  };

  // Add session-specific args if available
  const analysisResult = session?.results?.['analyze-repo'] as any;
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
  const logger = getToolLogger(context, 'build-image');
  const timer = createToolTimer(logger, 'build-image');

  try {
    const {
      path: rawBuildPath = '.',
      dockerfile = 'Dockerfile',
      dockerfilePath: rawDockerfilePath,
      imageName,
      tags = [],
      buildArgs = {},
      platform: _platform,
    } = params;

    // Normalize paths to handle Windows separators
    const buildContext = safeNormalizePath(rawBuildPath);
    const dockerfilePath = rawDockerfilePath ? safeNormalizePath(rawDockerfilePath) : undefined;

    logger.info({ path: buildContext, dockerfile, tags }, 'Starting Docker image build');

    // Progress: Validating build parameters and environment
    if (progress) await progress('VALIDATING');

    const startTime = Date.now();

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId, state: session } = sessionResult.value;
    const slice = useSessionSlice('build-image', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId }, 'Starting Docker image build with session');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    const dockerClient = createDockerClient(logger);

    // Determine paths
    // Use build context path directly - no legacy session fields
    const sessionData = session as SessionData;
    const repoPath = buildContext;
    let finalDockerfilePath = dockerfilePath
      ? resolvePath(repoPath, dockerfilePath)
      : resolvePath(repoPath, dockerfile);

    // Check if we should use a generated Dockerfile
    const dockerfileResult = sessionData?.results?.['generate-dockerfile'] as
      | { path?: string; content?: string }
      | undefined;
    const generatedPath = dockerfileResult?.path;

    if (!(await fileExists(finalDockerfilePath))) {
      // If the specified Dockerfile doesn't exist, check for generated one
      if (generatedPath) {
        const resolvedGeneratedPath = resolvePath(repoPath, generatedPath);
        if (await fileExists(resolvedGeneratedPath)) {
          finalDockerfilePath = resolvedGeneratedPath;
          logger.info(
            { generatedPath: resolvedGeneratedPath, originalPath: dockerfile },
            'Using generated Dockerfile',
          );
        } else {
          /**
           * Failure Mode: Generated path exists in session but file missing
           * Recovery: Write content from session if available
           */
          const dockerfileContent = dockerfileResult?.content;
          if (dockerfileContent) {
            // Use the user-specified dockerfile name (defaults to 'Dockerfile')
            finalDockerfilePath = joinPaths(repoPath, dockerfile);
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
        const dockerfileContent = dockerfileResult?.content;
        if (dockerfileContent) {
          // Use the user-specified dockerfile name (defaults to 'Dockerfile')
          finalDockerfilePath = joinPaths(repoPath, dockerfile);
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
    const finalBuildArgs = prepareBuildArgs(buildArgs, sessionData);

    // Analyze security
    const securityWarnings = analyzeBuildSecurity(dockerfileContent, finalBuildArgs);
    if (securityWarnings.length > 0) {
      logger.warn({ warnings: securityWarnings }, 'Security warnings found in build');
    }

    // Prepare Docker build options
    const buildOptions: DockerBuildOptions = {
      context: repoPath, // Build context is the path parameter
      dockerfile: getRelativePath(repoPath, finalDockerfilePath), // Dockerfile path relative to context
      buildargs: finalBuildArgs,
      ...(_platform !== undefined && { platform: _platform }),
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
      return Failure(`Failed to build image: ${errorMessage}`);
    }

    const buildTime = Date.now() - startTime;

    // Progress: Finalizing build results and updating session
    if (progress) await progress('FINALIZING');

    // Prepare the result
    const finalTags = tags.length > 0 ? tags : imageName ? [imageName] : [];
    const result: BuildImageResult = {
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
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastBuiltAt: new Date(),
        lastBuiltImageId: buildResult.value.imageId,
        lastBuiltTags: finalTags,
        totalBuilds:
          (session?.completed_steps || []).filter((s: string) => s === 'build-image').length + 1,
        lastBuildTime: buildTime,
        lastSecurityWarningCount: securityWarnings.length,
      },
    });

    timer.end({ imageId: buildResult.value.imageId, buildTime });
    logger.info({ imageId: buildResult.value.imageId, buildTime }, 'Docker image build completed');

    // Progress: Complete
    if (progress) await progress('COMPLETE');

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Docker image build failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Build image tool with selective progress reporting
 */
export const buildImage = buildImageImpl;
