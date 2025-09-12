/**
 * Generates AI-powered or template-based Dockerfiles from repository analysis.
 * Trade-off: AI quality over speed; fallback templates ensure availability.
 */

// import { extractErrorMessage } from '../../lib/error-utils'; // Not currently used
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { getSession, updateSession } from '@mcp/tool-session-helpers';
import { createStandardProgress } from '@mcp/progress-helper';
import { aiGenerateWithSampling, aiGenerate } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import type { SamplingOptions } from '@lib/sampling';
import { createTimer, createLogger } from '@lib/logger';
import type { SessionAnalysisResult } from '../session-types';
import type { ToolContext } from '../../mcp/context';
import {
  Success,
  Failure,
  type Result,
  type WorkflowState,
  type AnalyzeRepoResult,
} from '../../types';
import { getDefaultPort } from '@config/defaults';
import { getRecommendedBaseImage } from '@lib/base-images';
import {
  stripFencesAndNoise,
  isValidDockerfileContent,
  extractBaseImage,
} from '@lib/text-processing';
import {
  getSuccessChainHint,
  getFailureHint,
  formatChainHint,
  type SessionContext,
} from '../../lib/chain-hints';
import { TOOL_NAMES } from '../../exports/tools.js';
import type { GenerateDockerfileParams } from './schema';

/**
 * Single module Dockerfile generation result with optional sampling metadata
 */
interface SingleDockerfileResult {
  /** Generated Dockerfile content */
  content: string;
  /** Path where Dockerfile was written */
  path: string;
  /** Module root path this dockerfile corresponds to */
  moduleRoot: string;
  /** Base image used */
  baseImage: string;
  /** Whether optimization was applied */
  optimization: boolean;
  /** Whether multi-stage build was used */
  multistage: boolean;
  /** Warnings about potential issues */
  warnings?: string[];
  /** Sampling metadata if sampling was used */
  samplingMetadata?: {
    stoppedEarly?: boolean;
    candidatesGenerated: number;
    winnerScore: number;
    samplingDuration?: number;
  };
  /** Winner score if sampling was used */
  winnerScore?: number;
  /** Score breakdown if requested */
  scoreBreakdown?: Record<string, number>;
  /** All candidates if requested */
  allCandidates?: Array<{
    id: string;
    content: string;
    score: number;
    scoreBreakdown: Record<string, number>;
    rank?: number;
  }>;
  /** Validation score and report */
  validationScore?: number;
  validationGrade?: string;
  validationReport?: string;
}

/**
 * Multi-module Dockerfile generation result maintaining backwards compatibility
 */
export interface GenerateDockerfileResult {
  /** Generated Dockerfile content (for single module compatibility) */
  content?: string;
  /** Path where Dockerfile was written (for single module compatibility) */
  path?: string;
  /** Base image used (for single module compatibility) */
  baseImage?: string;
  /** Whether optimization was applied */
  optimization: boolean;
  /** Whether multi-stage build was used */
  multistage: boolean;
  /** Warnings about potential issues */
  warnings?: string[];
  /** Session ID for reference */
  sessionId?: string;
  /** Array of dockerfile results for each module root */
  dockerfiles: SingleDockerfileResult[];
  /** Total number of dockerfiles generated */
  count: number;
  /** Sampling metadata if sampling was used */
  samplingMetadata?: {
    stoppedEarly?: boolean;
    candidatesGenerated: number;
    winnerScore: number;
    samplingDuration?: number;
  };
  /** Winner score if sampling was used */
  winnerScore?: number;
  /** Score breakdown if requested */
  scoreBreakdown?: Record<string, number>;
  /** All candidates if requested */
  allCandidates?: Array<{
    id: string;
    content: string;
    score: number;
    scoreBreakdown: Record<string, number>;
    rank?: number;
  }>;
  /** Scoring details for test compatibility */
  scoringDetails?: {
    candidates?: Array<{
      score: number;
      [key: string]: any;
    }>;
  };
}

/**
 * Template-based Dockerfile generation fallback.
 * Invariant: Always produces valid Dockerfile syntax even without AI.
 */
function generateTemplateDockerfile(
  analysisResult: AnalyzeRepoResult,
  params: GenerateDockerfileParams,
  moduleRoot?: string,
): Result<Pick<SingleDockerfileResult, 'content' | 'baseImage'>> {
  const { language, framework, dependencies = [], ports = [], buildSystem } = analysisResult;
  const { baseImage, multistage = true, securityHardening = true } = params;
  const effectiveBase = baseImage || getRecommendedBaseImage(language || 'unknown');
  const mainPort = ports[0] || getDefaultPort(language || framework || 'generic');
  let dockerfile = `# Generated Dockerfile for ${language} ${framework ? `(${framework})` : ''}\n`;
  if (moduleRoot) {
    dockerfile += `# Module root: ${moduleRoot}\n`;
  }
  dockerfile += `FROM ${effectiveBase}\n\n`;
  dockerfile += `# Metadata\n`;
  dockerfile += `LABEL maintainer="generated"\n`;
  dockerfile += `LABEL language="${language || 'unknown'}"\n`;
  if (framework) dockerfile += `LABEL framework="${framework}"\n`;
  if (moduleRoot) dockerfile += `LABEL module.root="${moduleRoot}"\n`;
  dockerfile += '\n';
  // Working directory follows container best practices
  dockerfile += `WORKDIR /app\n\n`;
  // Language-specific setup
  switch (language) {
    case 'javascript':
    case 'typescript': {
      // Handle Node.js projects - detect package manager
      const hasYarn = dependencies.some((d) => d.name === 'yarn');
      const hasPnpm = dependencies.some((d) => d.name === 'pnpm');
      const packageManager = hasPnpm ? 'pnpm' : hasYarn ? 'yarn' : 'npm';
      dockerfile += `# Copy package files\n`;
      if (packageManager === 'pnpm') {
        dockerfile += `COPY package.json pnpm-lock.yaml* ./\n`;
        dockerfile += `RUN corepack enable && pnpm install --frozen-lockfile\n\n`;
      } else if (packageManager === 'yarn') {
        dockerfile += `COPY package.json yarn.lock* ./\n`;
        dockerfile += `RUN yarn install --frozen-lockfile\n\n`;
      } else {
        dockerfile += `COPY package*.json ./\n`;
        dockerfile += `RUN npm ci --only=production\n\n`;
      }
      dockerfile += `# Copy application files\n`;
      dockerfile += `COPY . .\n\n`;
      if (language === 'typescript') {
        dockerfile += `# Build TypeScript\n`;
        dockerfile += `RUN ${packageManager} run build\n\n`;
      }
      break;
    }
    case 'python':
      // Handle Python projects
      dockerfile += `# Install dependencies\n`;
      dockerfile += `COPY requirements.txt ./\n`;
      dockerfile += `RUN pip install --no-cache-dir -r requirements.txt\n\n`;
      break;
    case 'java': {
      // Handle Java projects - detect build system
      const javaBuildSystem =
        buildSystem?.type || (dependencies.some((d) => d.name === 'gradle') ? 'gradle' : 'maven');
      // Use system commands if no wrapper detected
      const mavenCmd = buildSystem?.buildCommand?.includes('mvnw') ? './mvnw' : 'mvn';
      const gradleCmd = buildSystem?.buildCommand?.includes('gradlew') ? './gradlew' : 'gradle';
      if (multistage) {
        dockerfile = `# Multi-stage build for Java\n`;
        if (javaBuildSystem === 'gradle') {
          dockerfile += `FROM gradle:8-jdk17 AS builder\n`;
          dockerfile += `WORKDIR /build\n`;
          dockerfile += `COPY build.gradle* settings.gradle* ./\n`;
          if (gradleCmd === './gradlew') {
            dockerfile += `COPY gradlew gradlew.bat ./\n`;
            dockerfile += `COPY gradle/ gradle/\n`;
          }
          dockerfile += `RUN ${gradleCmd} dependencies --no-daemon || true\n`;
          dockerfile += `COPY src ./src\n`;
          dockerfile += `RUN ${gradleCmd} build --no-daemon -x test\n\n`;
          dockerfile += `FROM ${effectiveBase}\n`;
          dockerfile += `WORKDIR /app\n`;
          dockerfile += `COPY --from=builder /build/build/libs/*.jar app.jar\n`;
        } else {
          // Default to Maven
          dockerfile += `FROM maven:3-amazoncorretto-17 AS builder\n`;
          dockerfile += `COPY pom.xml .\n`;
          if (mavenCmd === './mvnw') {
            dockerfile += `COPY mvnw mvnw.cmd ./\n`;
            dockerfile += `COPY .mvn/ .mvn/\n`;
          }
          dockerfile += `RUN ${mavenCmd} dependency:go-offline\n`;
          dockerfile += `RUN ${mavenCmd} package -DskipTests\n\n`;
          dockerfile += `COPY --from=builder /build/target/*.jar app.jar\n`;
        }
        dockerfile += `# Copy JAR file\n`;
        if (javaBuildSystem === 'gradle') {
          dockerfile += `COPY build/libs/*.jar app.jar\n\n`;
        } else {
          dockerfile += `COPY target/*.jar app.jar\n\n`;
        }
      }
      break;
    }
    case 'go':
      // Handle Go projects
      dockerfile = `# Multi-stage build for Go\n`;
      dockerfile += `FROM golang:1.21-alpine AS builder\n`;
      dockerfile += `WORKDIR /build\n`;
      dockerfile += `COPY go.* ./\n`;
      dockerfile += `RUN go mod download\n`;
      dockerfile += `COPY . .\n`;
      dockerfile += `RUN CGO_ENABLED=0 go build -o app\n\n`;
      dockerfile += `FROM alpine:latest\n`;
      dockerfile += `RUN apk --no-cache add ca-certificates\n`;
      dockerfile += `WORKDIR /app\n`;
      dockerfile += `COPY --from=builder /build/app .\n`;
      dockerfile += `# Copy binary\n`;
      dockerfile += `COPY app /app/\n\n`;
      break;
    default:
    // Generic Dockerfile
  }
  // Security hardening
  if (securityHardening) {
    dockerfile += `# Security hardening\n`;
    dockerfile += `RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup\n`;
    dockerfile += `USER appuser\n\n`;
  }
  // Expose port
  if (mainPort) {
    dockerfile += `# Expose application port\n`;
    dockerfile += `EXPOSE ${mainPort}\n\n`;
  }
  // Set entrypoint based on language
  dockerfile += `# Start application\n`;
  switch (language) {
    case 'javascript':
    case 'typescript':
      dockerfile += `CMD ["node", "${language === 'typescript' ? 'dist/' : ''}index.js"]\n`;
      break;
    case 'python':
      dockerfile += `CMD ["python", "app.py"]\n`;
      break;
    case 'java':
      dockerfile += `CMD ["java", "-jar", "app.jar"]\n`;
      break;
    case 'go':
      dockerfile += `CMD ["./app"]\n`;
      break;
    default:
      dockerfile += `CMD ["sh", "-c", "echo 'Please configure your application startup command'"]\n`;
      break;
  }
  return Success({ content: dockerfile, baseImage: effectiveBase });
}

/**
 * Convert SessionAnalysisResult to AnalyzeRepoResult for compatibility
 */
function sessionToAnalyzeRepoResult(sessionResult: SessionAnalysisResult): AnalyzeRepoResult {
  return {
    ok: true,
    sessionId: 'session-converted', // This won't be used in template generation
    language: sessionResult.language || 'unknown',
    ...(sessionResult.framework && { framework: sessionResult.framework }),
    dependencies:
      sessionResult.dependencies?.map((d) => {
        const dep: { name: string; version?: string; type: string } = {
          name: d.name,
          type: 'dependency',
        };
        if (d.version) {
          dep.version = d.version;
        }
        return dep;
      }) || [],
    ports: sessionResult.ports || [],
    ...(sessionResult.build_system && {
      buildSystem: {
        type: sessionResult.build_system.type || 'unknown',
        file: sessionResult.build_system.build_file || '',
        buildCommand: sessionResult.build_system.build_command || '',
      },
    }),
    hasDockerfile: false, // These won't be used in template generation
    hasDockerCompose: false,
    hasKubernetes: false,
    recommendations: {
      baseImage: 'node:18-alpine', // Default recommendation
      buildStrategy: 'multi-stage' as const,
      securityNotes: [],
    },
    metadata: {
      repoPath: '',
      depth: 0,
      timestamp: Date.now(),
    },
  };
}

/**
 * Build arguments for AI prompt from analysis result
 */
function buildArgsFromAnalysis(
  analysisResult: SessionAnalysisResult,
  optimization?: boolean | string,
): Record<string, unknown> {
  const {
    language = 'unknown',
    framework = '',
    dependencies = [],
    ports = [],
    build_system,
    summary = '',
  } = analysisResult;
  // Infer package manager from build system
  const packageManager =
    build_system?.type === 'maven' || build_system?.type === 'gradle'
      ? build_system.type
      : language === 'javascript' || language === 'typescript'
        ? 'npm'
        : 'unknown';
  // Get build file information
  const buildFile = build_system?.build_file || '';
  const hasWrapper =
    buildFile.includes('mvnw') || build_system?.build_command?.includes('mvnw') || false;
  // Determine appropriate build command
  let recommendedBuildCommand = '';
  if (build_system?.type === 'maven') {
    recommendedBuildCommand = hasWrapper ? './mvnw' : 'mvn';
  } else if (build_system?.type === 'gradle') {
    recommendedBuildCommand = hasWrapper ? './gradlew' : 'gradle';
  } else if (build_system?.build_command) {
    recommendedBuildCommand = build_system.build_command;
  }
  return {
    language,
    framework,
    dependencies: dependencies?.map((d) => d.name || d).join(', ') || '',
    ports: ports?.join(', ') || '',
    summary: summary || `${language} ${framework ? `${framework} ` : ''}application`,
    packageManager,
    buildSystem: build_system?.type || 'none',
    buildCommand: recommendedBuildCommand,
    buildFile,
    hasWrapper,
    ...(optimization && {
      optimization: typeof optimization === 'string' ? optimization : 'performance',
    }),
  };
}

// computeHash function removed - was unused after tool wrapper elimination
/**
 * Generate Dockerfile for a single module root
 */
async function generateSingleDockerfile(
  analysisResult: SessionAnalysisResult,
  params: GenerateDockerfileParams,
  moduleRoot: string,
  context: ToolContext,
  logger: ReturnType<typeof createLogger>,
): Promise<Result<SingleDockerfileResult>> {
  const { multistage = true, securityHardening = true } = params;
  // Normalize optimization to boolean - any string value means optimization is enabled
  const optimization = params.optimization === false ? false : true;

  // Prepare sampling options (filter out undefined values)
  const samplingOptions: SamplingOptions = {};
  // Sampling is enabled by default unless explicitly disabled
  samplingOptions.enableSampling = !params.disableSampling;
  if (params.maxCandidates !== undefined) samplingOptions.maxCandidates = params.maxCandidates;
  if (params.earlyStopThreshold !== undefined)
    samplingOptions.earlyStopThreshold = params.earlyStopThreshold;
  if (params.includeScoreBreakdown !== undefined)
    samplingOptions.includeScoreBreakdown = params.includeScoreBreakdown;
  if (params.returnAllCandidates !== undefined)
    samplingOptions.returnAllCandidates = params.returnAllCandidates;
  if (params.useCache !== undefined) samplingOptions.useCache = params.useCache;

  let dockerfileContent: string;
  let baseImageUsed: string;
  let samplingMetadata: SingleDockerfileResult['samplingMetadata'];
  let winnerScore: number | undefined;
  let scoreBreakdown: Record<string, number> | undefined;
  let allCandidates: SingleDockerfileResult['allCandidates'];

  // Build AI prompt args with module-specific context
  let promptArgs = buildArgsFromAnalysis(analysisResult, optimization);
  promptArgs.moduleRoot = moduleRoot;
  promptArgs.moduleContext = `Generating Dockerfile for module at path: ${moduleRoot}`;

  // Enhance prompt with knowledge context
  try {
    const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
      operation: 'generate_dockerfile',
      ...(analysisResult.language && { language: analysisResult.language }),
      ...(analysisResult.framework && { framework: analysisResult.framework }),
      environment: params.environment || 'production',
      tags: ['dockerfile', 'generation', analysisResult.language, analysisResult.framework].filter(
        Boolean,
      ) as string[],
    });

    if (knowledgeResult.bestPractices && knowledgeResult.bestPractices.length > 0) {
      promptArgs = knowledgeResult;
      logger.info(
        {
          practicesCount: knowledgeResult.bestPractices.length,
          examplesCount: knowledgeResult.examples ? knowledgeResult.examples.length : 0,
          moduleRoot,
        },
        'Enhanced Dockerfile generation with knowledge for module',
      );
    }
  } catch (error) {
    logger.debug(
      { error, moduleRoot },
      'Knowledge enhancement failed for module, using base prompt',
    );
  }

  // Use sampling-aware generation (default) unless explicitly disabled
  if (!params.disableSampling) {
    const aiResult = await aiGenerateWithSampling(logger, context, {
      promptName: 'dockerfile-generation',
      promptArgs,
      expectation: 'dockerfile',
      maxRetries: 3,
      fallbackBehavior: 'default',
      ...samplingOptions,
    });

    if (aiResult.ok) {
      const cleaned = stripFencesAndNoise(aiResult.value.winner.content, 'dockerfile');
      if (!isValidDockerfileContent(cleaned)) {
        // Fall back to template if AI output is invalid
        const fallbackResult = generateTemplateDockerfile(
          sessionToAnalyzeRepoResult(analysisResult),
          params,
          moduleRoot,
        );
        if (!fallbackResult.ok) {
          return Failure(fallbackResult.error);
        }
        dockerfileContent = fallbackResult.value.content;
        baseImageUsed = fallbackResult.value.baseImage;
      } else {
        dockerfileContent = cleaned;
        baseImageUsed =
          extractBaseImage(cleaned) ||
          params.baseImage ||
          getRecommendedBaseImage(analysisResult.language ?? 'unknown');

        // Capture sampling metadata
        samplingMetadata = aiResult.value.samplingMetadata;
        winnerScore = aiResult.value.winner.score;
        scoreBreakdown = aiResult.value.winner.scoreBreakdown;
        allCandidates = aiResult.value.allCandidates;
      }
    } else {
      // Use template fallback
      const fallbackResult = generateTemplateDockerfile(
        sessionToAnalyzeRepoResult(analysisResult),
        params,
        moduleRoot,
      );
      if (!fallbackResult.ok) {
        return Failure(fallbackResult.error);
      }
      dockerfileContent = fallbackResult.value.content;
      baseImageUsed = fallbackResult.value.baseImage;
    }
  } else {
    // Standard generation without sampling
    const aiResult = await aiGenerate(logger, context, {
      promptName: 'dockerfile-generation',
      promptArgs,
      expectation: 'dockerfile',
      maxRetries: 3,
      fallbackBehavior: 'default',
    });

    if (aiResult.ok) {
      // Use AI-generated content
      const cleaned = stripFencesAndNoise(aiResult.value.content, 'dockerfile');
      if (!isValidDockerfileContent(cleaned)) {
        // Fall back to template if AI output is invalid
        const fallbackResult = generateTemplateDockerfile(
          sessionToAnalyzeRepoResult(analysisResult),
          params,
          moduleRoot,
        );
        if (!fallbackResult.ok) {
          return Failure(fallbackResult.error);
        }
        dockerfileContent = fallbackResult.value.content;
        baseImageUsed = fallbackResult.value.baseImage;
      } else {
        dockerfileContent = cleaned;
        baseImageUsed =
          extractBaseImage(cleaned) ||
          params.baseImage ||
          getRecommendedBaseImage(analysisResult.language ?? 'unknown');
      }
    } else {
      // Use template fallback
      const fallbackResult = generateTemplateDockerfile(
        sessionToAnalyzeRepoResult(analysisResult),
        params,
        moduleRoot,
      );
      if (!fallbackResult.ok) {
        return Failure(fallbackResult.error);
      }
      dockerfileContent = fallbackResult.value.content;
      baseImageUsed = fallbackResult.value.baseImage;
    }
  }

  // Determine output path - write to each module root directory
  const repoPath = params.repoPath || '.';
  const dockerfilePath = path.resolve(path.join(repoPath, moduleRoot, 'Dockerfile'));

  // Write Dockerfile to disk
  await fs.writeFile(dockerfilePath, dockerfileContent, 'utf-8');

  // Check for warnings
  const warnings: string[] = [];
  if (!securityHardening) {
    warnings.push('Security hardening is disabled - consider enabling for production');
  }
  if (dockerfileContent.includes('root')) {
    warnings.push('Container may run as root user');
  }
  if (dockerfileContent.includes(':latest')) {
    warnings.push('Using :latest tags - consider pinning versions');
  }

  const result: SingleDockerfileResult = {
    content: dockerfileContent,
    path: dockerfilePath,
    moduleRoot,
    baseImage: baseImageUsed,
    optimization,
    multistage,
    ...(warnings.length > 0 && { warnings }),
  };

  // Add sampling metadata if sampling was used
  if (!params.disableSampling) {
    if (samplingMetadata) {
      result.samplingMetadata = samplingMetadata;
    }
    if (winnerScore !== undefined) {
      result.winnerScore = winnerScore;
    }
    if (scoreBreakdown && params.includeScoreBreakdown) {
      result.scoreBreakdown = scoreBreakdown;
    }
    if (allCandidates && params.returnAllCandidates) {
      result.allCandidates = allCandidates;
    }
  }

  return Success(result);
}

/**
 * Generate Dockerfile implementation - direct execution with selective progress
 */
async function generateDockerfileImpl(
  params: GenerateDockerfileParams,
  context: ToolContext,
): Promise<Result<GenerateDockerfileResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Validate moduleRoots parameter - use default if not provided
  const moduleRoots =
    params.moduleRoots && params.moduleRoots.length > 0 ? params.moduleRoots : ['.'];

  // Optional progress reporting for complex operations (AI generation)
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = context.logger || createLogger({ name: 'generate-dockerfile' });
  const timer = createTimer(logger, 'generate-dockerfile');
  try {
    const { multistage = true } = params;
    // Normalize optimization to boolean - any string value means optimization is enabled
    const optimization = params.optimization === false ? false : true;
    // Progress: Starting validation and analysis
    if (progress) await progress('VALIDATING');
    // Get or create session
    const sessionResult = await getSession(params.sessionId, context);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }
    const { id: sessionId, state: session } = sessionResult.value;
    // Type the session properly with our extended properties
    interface ExtendedWorkflowState extends WorkflowState {
      repo_path?: string;
      analysis_result?: SessionAnalysisResult;
      dockerfile_result?: { content?: string };
    }
    const typedSession = session as ExtendedWorkflowState;
    // Get analysis result from session - it should be directly on the session
    const analysisResult = typedSession.analysis_result;
    if (!analysisResult) {
      return Failure(
        `Repository must be analyzed first. Please run 'analyze-repo' before 'generate-dockerfile'.`,
      );
    }
    if (progress) await progress('EXECUTING');

    // Generate dockerfile for each module root
    const dockerfileResults: SingleDockerfileResult[] = [];
    const errors: string[] = [];

    for (const moduleRoot of moduleRoots) {
      try {
        logger.info({ moduleRoot }, 'Generating Dockerfile for module');
        const moduleResult = await generateSingleDockerfile(
          analysisResult,
          params,
          moduleRoot,
          context,
          logger,
        );
        if (moduleResult.ok) {
          dockerfileResults.push(moduleResult.value);
        } else {
          errors.push(
            `Failed to generate Dockerfile for module '${moduleRoot}': ${moduleResult.error}`,
          );
        }
      } catch (error) {
        errors.push(
          `Error generating Dockerfile for module '${moduleRoot}': ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    }

    // Check if any dockerfiles were generated successfully
    if (dockerfileResults.length === 0) {
      return Failure(`Failed to generate any Dockerfiles. Errors: ${errors.join('; ')}`);
    }

    // Progress: Finalizing
    if (progress) await progress('FINALIZING');
    const dockerfileResult = {
      dockerfiles: dockerfileResults,
      multistage,
      fixed: false,
      fixes: [],
    };
    // Update session with Dockerfile result using simplified helper
    const updateResult = await updateSession(
      sessionId,
      {
        dockerfile_result: dockerfileResult,
        completed_steps: [...(typedSession.completed_steps || []), 'dockerfile'],
        metadata: {
          ...(typedSession.metadata || {}),
          dockerfile_count: dockerfileResults.length,
          dockerfile_moduleRoots: moduleRoots,
          dockerfile_optimization: optimization,
          ai_enhancement_used: dockerfileResults.some((d) => d.samplingMetadata || d.winnerScore),
        },
      },
      context,
    );
    if (!updateResult.ok) {
      logger.warn(
        { error: updateResult.error },
        'Failed to update session, but Dockerfile generation succeeded',
      );
    }
    // Progress: Complete
    if (progress) await progress('COMPLETE');

    timer.end({ count: dockerfileResults.length });

    // Aggregate warnings from all dockerfiles
    const allWarnings: string[] = [];
    dockerfileResults.forEach((result) => {
      if (result.warnings) {
        allWarnings.push(...result.warnings);
      }
    });

    // Add errors as warnings if some dockerfiles failed but others succeeded
    if (errors.length > 0) {
      allWarnings.push(...errors);
    }
    // Return result with file write indicator and chain hint
    // Prepare session context for dynamic chain hints
    const dockerfileContent =
      dockerfileResults.length === 1 ? dockerfileResults[0]?.content : undefined;
    const sessionContext: SessionContext = {
      completed_steps: typedSession.completed_steps || [],
      dockerfile_result: dockerfileContent ? { content: dockerfileContent } : {},
      ...(typedSession.analysis_result && {
        analysis_result: typedSession.analysis_result,
      }),
    };

    const result: GenerateDockerfileResult & {
      _fileWritten?: boolean;
      _fileWrittenPath?: string;
      NextStep?: string;
    } = {
      dockerfiles: dockerfileResults,
      count: dockerfileResults.length,
      optimization,
      multistage,
      ...(allWarnings.length > 0 && { warnings: allWarnings }),
      sessionId,
      _fileWritten: true,
      _fileWrittenPath: dockerfileResults.map((d) => d.path).join(', '),
      NextStep: getSuccessChainHint(TOOL_NAMES.GENERATE_DOCKERFILE, sessionContext),
    };

    // Set compatibility fields for single module (backwards compatibility)
    if (dockerfileResults.length === 1) {
      const firstResult = dockerfileResults[0];
      if (firstResult) {
        result.content = firstResult.content;
        result.path = firstResult.path;
        result.baseImage = firstResult.baseImage;

        // Add sampling metadata from first result
        if (firstResult.samplingMetadata) {
          result.samplingMetadata = firstResult.samplingMetadata;
        }
        if (firstResult.winnerScore !== undefined) {
          result.winnerScore = firstResult.winnerScore;
        }
        if (firstResult.scoreBreakdown) {
          result.scoreBreakdown = firstResult.scoreBreakdown;
        }
        if (firstResult.allCandidates) {
          result.allCandidates = firstResult.allCandidates;
          // Add scoringDetails for test compatibility
          result.scoringDetails = {
            candidates: firstResult.allCandidates.map((c) => ({
              score: c.score,
              id: c.id,
              scoreBreakdown: c.scoreBreakdown,
            })),
          };
        }
      }
    }
    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Dockerfile generation failed');

    // Add failure chain hint
    const sessionContext = {
      completed_steps: [],
    };
    const errorMessage = error instanceof Error ? error.message : String(error);
    const hint = getFailureHint(TOOL_NAMES.GENERATE_DOCKERFILE, errorMessage, sessionContext);
    const chainHint = formatChainHint(hint);

    return Failure(`${errorMessage}\n${chainHint}`);
  }
}

/**
 * Generate Dockerfile tool with selective progress reporting
 */
export const generateDockerfile = generateDockerfileImpl;
