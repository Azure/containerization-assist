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

import path from 'path';
import { normalizePath } from '@/lib/path-utils';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { getPostBuildHint } from '@/lib/workflow-hints';
import { promises as fs } from 'node:fs';
import { createStandardProgress } from '@/mcp/progress-helper';
import type { ToolContext } from '@/mcp/context';
import { createDockerClient, type DockerBuildOptions } from '@/lib/docker';

import { type Result, Success, Failure, TOPICS } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { fileExists } from '@/lib/file-utils';
import { type BuildImageParams, buildImageSchema } from './schema';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages, type MCPMessage } from '@/mcp/ai/message-converter';

// Additional result interface for AI suggestions
export interface BuildOptimizationSuggestions {
  suggestions: string[];
  optimizations: string[];
  nextSteps: string[];
  confidence: number;
}

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
  /** AI-powered optimization suggestions */
  optimizationSuggestions?: BuildOptimizationSuggestions;
  /** Workflow hints for the next step */
  workflowHints?: {
    nextStep: string;
    message: string;
  };
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
 * Score build optimization suggestions based on content quality and relevance
 */
function scoreBuildOptimization(text: string): number {
  let score = 0;

  // Basic content quality (30 points)
  if (text.length > 100) score += 10;
  if (text.includes('\n')) score += 10;
  if (!text.toLowerCase().includes('error')) score += 10;

  // Optimization indicators (40 points)
  if (/layer|cache|size|performance|speed/i.test(text)) score += 15;
  if (/multi-stage|alpine|distroless/i.test(text)) score += 10;
  if (/security|vulnerability|best.practice/i.test(text)) score += 15;

  // Structure and actionability (30 points)
  if (/\d+\.|-|\*/.test(text)) score += 10; // Has list structure
  if (/optimize|improve|reduce|enhance/i.test(text)) score += 10;
  if (text.split('\n').length >= 3) score += 10; // Multi-line content

  return Math.min(score, 100);
}

/**
 * Build optimization prompt for AI enhancement
 */
async function buildOptimizationPrompt(
  buildContext: string,
  dockerfileContent: string,
  buildLogs: string[],
  imageSize: number,
  buildTime: number,
): Promise<{ messages: MCPMessage[]; maxTokens: number }> {
  const basePrompt = `You are a Docker optimization expert. Analyze build results and provide specific optimization suggestions.

Focus on:
1. Image size reduction techniques
2. Build time optimization
3. Layer caching strategies
4. Security improvements
5. Performance enhancements

Provide concrete, actionable recommendations.

Analyze this Docker build and provide optimization suggestions:

**Build Context:** ${buildContext}

**Dockerfile Content:**
\`\`\`dockerfile
${dockerfileContent}
\`\`\`

**Build Metrics:**
- Image size: ${(imageSize / (1024 * 1024)).toFixed(2)} MB
- Build time: ${(buildTime / 1000).toFixed(2)} seconds

**Build Logs (last 10 lines):**
${buildLogs.slice(-10).join('\n')}

Please provide:
1. **Size Optimization:** Specific techniques to reduce image size
2. **Build Performance:** Ways to improve build speed and caching
3. **Security Enhancements:** Security-focused improvements
4. **Best Practices:** Docker best practices not currently followed

Format your response as clear, actionable recommendations.`;

  const messages = await buildMessages({
    basePrompt,
    topic: TOPICS.DOCKER_OPTIMIZATION,
    tool: 'build-image',
    environment: 'docker',
  });

  return { messages: toMCPMessages(messages).messages, maxTokens: 2048 };
}

/**
 * Generate AI-powered build optimization suggestions
 */
async function generateOptimizationSuggestions(
  buildContext: string,
  dockerfileContent: string,
  buildResult: { logs?: string[]; size?: number; imageId: string },
  ctx: ToolContext,
  buildStartTime: number,
): Promise<Result<BuildOptimizationSuggestions>> {
  try {
    const optimizationResult = await sampleWithRerank(
      ctx,
      async () =>
        buildOptimizationPrompt(
          buildContext,
          dockerfileContent,
          buildResult.logs || [],
          buildResult.size || 0,
          Date.now() - buildStartTime || 0,
        ),
      scoreBuildOptimization,
      {},
    );

    if (!optimizationResult.ok) {
      return Failure(`Failed to generate optimization suggestions: ${optimizationResult.error}`);
    }

    const text = optimizationResult.value.text;

    // Parse the AI response to extract structured suggestions
    const suggestions: string[] = [];
    const optimizations: string[] = [];
    const nextSteps: string[] = [];

    const lines = text
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line.length > 0);

    let currentSection = '';
    for (const line of lines) {
      if (line.includes('Size Optimization') || line.includes('size') || line.includes('Size')) {
        currentSection = 'size';
        continue;
      }
      if (
        line.includes('Build Performance') ||
        line.includes('performance') ||
        line.includes('Performance')
      ) {
        currentSection = 'performance';
        continue;
      }
      if (line.includes('Security') || line.includes('security')) {
        currentSection = 'security';
        continue;
      }
      if (line.includes('Best Practices') || line.includes('practices')) {
        currentSection = 'practices';
        continue;
      }

      if (line.startsWith('-') || line.startsWith('*') || line.match(/^\d+\./)) {
        const cleanLine = line.replace(/^[-*\d.]\s*/, '');
        if (cleanLine.length > 10) {
          if (currentSection === 'size') {
            optimizations.push(`Size: ${cleanLine}`);
          } else if (currentSection === 'performance') {
            optimizations.push(`Performance: ${cleanLine}`);
          } else if (currentSection === 'security') {
            suggestions.push(`Security: ${cleanLine}`);
          } else {
            suggestions.push(cleanLine);
          }
        }
      }
    }

    // Add general next steps
    nextSteps.push('Consider implementing multi-stage builds if not already used');
    nextSteps.push('Review and optimize layer caching strategy');
    nextSteps.push('Scan image for security vulnerabilities');

    return Success({
      suggestions,
      optimizations,
      nextSteps,
      confidence: optimizationResult.value.score ?? 0,
    });
  } catch (error) {
    return Failure(`Failed to generate optimization suggestions: ${extractErrorMessage(error)}`);
  }
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

  // Normalize paths to handle Windows separators
  const buildContext = normalizePath(rawBuildPath);
  const dockerfilePath = rawDockerfilePath ? normalizePath(rawDockerfilePath) : undefined;

  // Use session facade directly
  const sessionId = params.sessionId || context.session?.id;
  if (!sessionId) {
    return Failure('Session ID is required for build operations');
  }

  try {
    // Progress: Validating build parameters and environment
    if (progress) await progress('VALIDATING');

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

      // Session storage is handled by orchestrator (for both success and failure)
      // Propagate Docker error guidance from infrastructure layer
      return Failure(`Failed to build image: ${errorMessage}`, buildResult.guidance);
    }

    const buildTime = Date.now() - startTime;

    // Progress: Finalizing build results and updating session
    if (progress) await progress('FINALIZING');

    // Generate AI-powered optimization suggestions for successful builds
    let optimizationSuggestions: BuildOptimizationSuggestions | undefined;
    try {
      // Generate optimization suggestions
      const suggestionResult = await generateOptimizationSuggestions(
        buildContext,
        dockerfileContent,
        buildResult.value,
        context,
        startTime,
      );

      if (suggestionResult.ok) {
        optimizationSuggestions = suggestionResult.value;
        logger.info(
          {
            suggestions: optimizationSuggestions.suggestions.length,
            confidence: optimizationSuggestions.confidence,
          },
          'Generated AI optimization suggestions',
        );
      } else {
        logger.warn(
          { error: suggestionResult.error },
          'Failed to generate optimization suggestions',
        );
      }
    } catch (error) {
      logger.warn(
        { error: extractErrorMessage(error) },
        'Error generating optimization suggestions',
      );
    }

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
      ...(optimizationSuggestions && { optimizationSuggestions }),
      workflowHints: getPostBuildHint(
        context.session,
        sessionId,
        optimizationSuggestions ? String(optimizationSuggestions) : undefined,
      ),
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
export const buildImage = buildImageImpl;

// New Tool interface export
import type { Tool } from '@/types/tool';

const tool: Tool<typeof buildImageSchema, BuildImageResult> = {
  name: 'build-image',
  description: 'Build Docker images from Dockerfiles with AI-powered optimization suggestions',
  version: '2.0.0',
  schema: buildImageSchema,
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'single',
    enhancementCapabilities: ['optimization-suggestions', 'build-analysis', 'performance-insights'],
  },
  run: buildImageImpl,
};

export default tool;
