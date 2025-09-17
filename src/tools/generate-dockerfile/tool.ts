/**
 * Generates AI-powered or template-based Dockerfiles from repository analysis.
 * Trade-off: AI quality over speed; fallback templates ensure availability.
 */

// import { extractErrorMessage } from '@lib/error-utils'; // Not currently used
import { promises as fs } from 'node:fs';
import { getToolLogger, createToolTimer } from '@lib/tool-helpers';
import type { Logger } from '@lib/logger';
import path from 'path';
import {
  ensureSession,
  defineToolIO,
  useSessionSlice,
  getSessionSlice,
} from '@mcp/tool-session-helpers';
import { createStandardProgress } from '@mcp/progress-helper';
import { aiGenerateWithSampling } from '@mcp/tool-ai-helpers';
import { enhancePromptWithKnowledge } from '@lib/ai-knowledge-enhancer';
import type { SamplingOptions } from '@lib/sampling';
import { analyzeRepoSchema } from '@tools/analyze-repo/schema';

import type { SessionAnalysisResult } from '@tools/session-types';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { getDefaultPort, ANALYSIS_CONFIG } from '@config/defaults';
import { getRecommendedBaseImage } from '@lib/base-images';
import {
  stripFencesAndNoise,
  isValidDockerfileContent,
  extractBaseImage,
} from '@lib/text-processing';
import { generateDockerfileSchema, type GenerateDockerfileParams } from './schema';
import { z } from 'zod';
import { AnalyzeRepoResult } from '@tools/analyze-repo/tool';

// Define the result schema for type safety - complex nested structure
const SingleDockerfileResultSchema = z.object({
  content: z.string(),
  path: z.string(),
  moduleRoot: z.string(),
  baseImage: z.string(),
  optimization: z.boolean(),
  multistage: z.boolean(),
  warnings: z.array(z.string()).optional(),
  samplingMetadata: z
    .object({
      stoppedEarly: z.boolean().optional(),
      candidatesGenerated: z.number(),
      winnerScore: z.number(),
      samplingDuration: z.number().optional(),
    })
    .optional(),
  winnerScore: z.number().optional(),
  scoreBreakdown: z.record(z.number()).optional(),
  allCandidates: z
    .array(
      z.object({
        id: z.string(),
        content: z.string(),
        score: z.number(),
        scoreBreakdown: z.record(z.number()),
        rank: z.number().optional(),
      }),
    )
    .optional(),
  validationScore: z.number().optional(),
  validationGrade: z.string().optional(),
  validationReport: z.string().optional(),
});

const GenerateDockerfileResultSchema = z.object({
  content: z.string().optional(),
  path: z.string().optional(),
  baseImage: z.string().optional(),
  optimization: z.boolean(),
  multistage: z.boolean(),
  warnings: z.array(z.string()).optional(),
  sessionId: z.string().optional(),
  dockerfiles: z.array(SingleDockerfileResultSchema),
  count: z.number(),
  samplingMetadata: z
    .object({
      stoppedEarly: z.boolean().optional(),
      candidatesGenerated: z.number(),
      winnerScore: z.number(),
      samplingDuration: z.number().optional(),
    })
    .optional(),
  winnerScore: z.number().optional(),
  scoreBreakdown: z.record(z.number()).optional(),
  allCandidates: z
    .array(
      z.object({
        id: z.string(),
        content: z.string(),
        score: z.number(),
        scoreBreakdown: z.record(z.number()),
        rank: z.number().optional(),
      }),
    )
    .optional(),
  scoringDetails: z
    .object({
      candidates: z
        .array(
          z
            .object({
              score: z.number(),
            })
            .catchall(z.any()),
        )
        .optional(),
    })
    .optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(generateDockerfileSchema, GenerateDockerfileResultSchema);

// Define analyze-repo tool IO for accessing its session slice data
// Note: We need to import the result schema from analyze-repo
const AnalyzeRepoResultSchema = z.object({
  ok: z.boolean(),
  sessionId: z.string(),
  language: z.string(),
  languageVersion: z.string().optional(),
  framework: z.string().optional(),
  frameworkVersion: z.string().optional(),
  buildSystem: z
    .object({
      type: z.string(),
      file: z.string(),
      buildCommand: z.string(),
      testCommand: z.string().optional(),
    })
    .optional(),
  dependencies: z.array(
    z.object({
      name: z.string(),
      version: z.string().optional(),
      type: z.string(),
    }),
  ),
  ports: z.array(z.number()),
  hasDockerfile: z.boolean(),
  hasDockerCompose: z.boolean(),
  hasKubernetes: z.boolean(),
  recommendations: z.object({
    baseImage: z.string(),
    buildStrategy: z.enum(['multi-stage', 'single-stage']),
    securityNotes: z.array(z.string()),
  }),
  confidence: z.number(),
  detectionMethod: z.enum(['signature', 'extension', 'fallback', 'ai-enhanced']),
  detectionDetails: z.object({
    signatureMatches: z.number(),
    extensionMatches: z.number(),
    frameworkSignals: z.number(),
    buildSystemSignals: z.number(),
  }),
  metadata: z.object({
    path: z.string(),
    depth: z.number(),
    timestamp: z.number(),
    includeTests: z.boolean().optional(),
    aiInsights: z.unknown().optional(),
  }),
  modules: z.array(z.string()).optional(),
});

const analyzeRepoIO = defineToolIO(analyzeRepoSchema, AnalyzeRepoResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastGeneratedAt: z.date().optional(),
  dockerfileCount: z.number().optional(),
  primaryModule: z.string().optional(),
  generationStrategy: z.enum(['ai', 'template', 'hybrid']).optional(),
  lastOptimization: z.string().optional(),
});

/**
 * Single module Dockerfile generation result with optional sampling metadata
 */
interface SingleDockerfileResult {
  /** Generated Dockerfile content */
  content: string;
  /** Repository path (build context) */
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
  /** Repository path (build context) (for single module compatibility) */
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
 * Generates Dockerfile using templates when AI generation fails.
 *
 * Invariant: Always produces valid Dockerfile syntax for reliability
 * Trade-off: Template consistency over AI creativity for fallback scenarios
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
    case 'dotnet': {
      // Handle .NET projects - check framework type
      const isFramework =
        framework?.includes('framework') ||
        framework?.includes('aspnet-webapi') ||
        framework?.includes('aspnet-mvc');

      if (isFramework) {
        // .NET Framework - Windows containers only
        dockerfile = `# .NET Framework application - requires Windows containers\n`;
        dockerfile += `FROM mcr.microsoft.com/dotnet/framework/sdk:4.8-windowsservercore-ltsc2022 AS builder\n`;
        dockerfile += `WORKDIR /app\n\n`;
        dockerfile += `# Copy project files\n`;
        dockerfile += `COPY *.sln .\n`;
        dockerfile += `COPY **/*.csproj ./\n`;
        dockerfile += `RUN for /f %i in ('dir /b /s *.csproj') do mkdir %~dpi && move %i %~dpi\n\n`;

        dockerfile += `# Restore packages\n`;
        dockerfile += `RUN nuget restore\n\n`;

        dockerfile += `# Copy source code\n`;
        dockerfile += `COPY . .\n\n`;

        dockerfile += `# Build application\n`;
        dockerfile += `RUN msbuild /p:Configuration=Release\n\n`;

        dockerfile += `# Runtime stage\n`;
        dockerfile += `FROM mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022\n`;
        dockerfile += `WORKDIR /inetpub/wwwroot\n`;
        dockerfile += `COPY --from=builder /app/bin/Release/net48/publish .\n`;
      } else {
        // .NET Core/5+ - Linux containers
        dockerfile = `# Multi-stage build for .NET\n`;
        dockerfile += `FROM mcr.microsoft.com/dotnet/sdk:8.0 AS builder\n`;
        dockerfile += `WORKDIR /src\n\n`;

        dockerfile += `# Copy project files\n`;
        dockerfile += `COPY *.csproj ./\n`;
        dockerfile += `RUN dotnet restore\n\n`;

        dockerfile += `# Copy source code\n`;
        dockerfile += `COPY . .\n`;
        dockerfile += `RUN dotnet publish -c Release -o /app/publish\n\n`;

        dockerfile += `# Runtime stage\n`;
        dockerfile += `FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine\n`;
        dockerfile += `WORKDIR /app\n`;
        dockerfile += `COPY --from=builder /app/publish .\n`;
      }
      break;
    }
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
    case 'dotnet': {
      // Check if it's .NET Framework or Core
      const isFramework =
        framework?.includes('framework') ||
        framework?.includes('aspnet-webapi') ||
        framework?.includes('aspnet-mvc');
      if (isFramework) {
        // .NET Framework uses IIS
        dockerfile += `# Application runs under IIS\n`;
      } else {
        // .NET Core/5+ uses Kestrel
        const appName = buildSystem?.file?.replace('.csproj', '') || 'app';
        dockerfile += `CMD ["dotnet", "${appName}.dll"]\n`;
      }
      break;
    }
    default:
      dockerfile += `CMD ["sh", "-c", "echo 'Please configure your application startup command'"]\n`;
      break;
  }
  return Success({ content: dockerfile, baseImage: effectiveBase });
}

/**
 * Automatically detects module roots by finding directories containing build files.
 * Supports Java (pom.xml, build.gradle) and Node.js (package.json) projects.
 */
async function detectModuleRoots(
  repoPath: string,
  language?: string,
  logger?: Logger,
): Promise<string[]> {
  try {
    // For .NET projects, ALWAYS use the root directory
    // .NET solutions should have one Dockerfile at the solution root
    if (language === 'dotnet') {
      logger?.info({ repoPath, language }, 'Detected .NET project - using single root Dockerfile');
      return ['.'];
    }

    // For other single-module projects, also use root
    if (
      language === 'python' ||
      language === 'go' ||
      language === 'rust' ||
      language === 'php' ||
      language === 'ruby'
    ) {
      logger?.info(
        { repoPath, language },
        `Detected ${language} project - using single root Dockerfile`,
      );
      return ['.'];
    }

    // Only scan for multi-module in Java/Node monorepos
    const buildFiles =
      language === 'java'
        ? ['pom.xml', 'build.gradle', 'build.gradle.kts']
        : language === 'javascript' || language === 'typescript'
          ? ['package.json']
          : ['pom.xml', 'build.gradle', 'package.json'];

    logger?.info(
      { repoPath, language, buildFiles },
      'Detecting module roots for potential monorepo',
    );

    const moduleRoots: string[] = [];

    async function scanDirectory(dirPath: string, depth: number = 0): Promise<void> {
      // Limit depth to avoid infinite recursion and performance issues
      if (depth > 3) return;

      try {
        const entries = await fs.readdir(dirPath, { withFileTypes: true });

        // Check if current directory contains build files
        for (const buildFile of buildFiles) {
          const buildFilePath = path.join(dirPath, buildFile);
          try {
            // For Java projects, ensure this is an actual module, not just a parent pom
            if (buildFile === 'pom.xml') {
              const pomContent = await fs.readFile(buildFilePath, 'utf-8');
              // Check if this is a parent pom by looking for <modules> tag
              const isParentPom =
                pomContent.includes('<modules>') && pomContent.includes('<module>');
              // Check if it has source code (src directory)
              const srcDir = path.join(dirPath, 'src');
              const hasSrc = await fs
                .stat(srcDir)
                .then((stats) => stats.isDirectory())
                .catch(() => false);

              if (!isParentPom || hasSrc) {
                const relativePath = path.relative(repoPath, dirPath) || '.';
                if (!moduleRoots.includes(relativePath)) {
                  moduleRoots.push(relativePath);
                  logger?.info({ moduleRoot: relativePath, buildFile }, 'Found module root');
                }
              }
            } else {
              // For non-Maven build files, add the module
              const relativePath = path.relative(repoPath, dirPath) || '.';
              if (!moduleRoots.includes(relativePath)) {
                moduleRoots.push(relativePath);
                logger?.info({ moduleRoot: relativePath, buildFile }, 'Found module root');
              }
            }
            break; // Found a build file, no need to check others in this directory
          } catch {
            // Build file doesn't exist, continue
          }
        }

        // Recursively scan subdirectories, but skip some common directories
        for (const entry of entries) {
          if (
            entry.isDirectory() &&
            !entry.name.startsWith('.') &&
            !['node_modules', 'target', 'build', 'dist', 'out'].includes(entry.name)
          ) {
            await scanDirectory(path.join(dirPath, entry.name), depth + 1);
          }
        }
      } catch (error) {
        logger?.debug({ dirPath, error }, 'Error scanning directory');
      }
    }

    await scanDirectory(repoPath);

    // If no modules found, return root as default
    if (moduleRoots.length === 0) {
      moduleRoots.push('.');
      logger?.info('No modules detected, using root directory');
    }

    logger?.info({ moduleRoots, count: moduleRoots.length }, 'Module detection complete');
    return moduleRoots;
  } catch (error) {
    logger?.error({ error, repoPath }, 'Error detecting module roots');
    return ['.'];
  }
}

/**
 * Adapts session analysis format to tool requirements.
 * Provides backward compatibility during result format transition.
 */
function sessionToAnalyzeRepoResult(sessionResult: SessionAnalysisResult): AnalyzeRepoResult {
  return {
    ok: true,
    sessionId: 'session-converted', // This won't be used in template generation
    language: sessionResult.language || 'unknown',
    confidence: 0.8, // Default confidence for session-converted results
    detectionMethod: 'signature' as const,
    detectionDetails: {
      signatureMatches: 0,
      extensionMatches: 0,
      frameworkSignals: 0,
      buildSystemSignals: 0,
    },
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
    ...(sessionResult.buildSystem && {
      buildSystem: {
        type: sessionResult.buildSystem.type || 'unknown',
        file: sessionResult.buildSystem.buildFile || '',
        buildCommand: sessionResult.buildSystem.buildCommand || '',
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
      path: '',
      depth: 0,
      timestamp: Date.now(),
    },
  };
}

/**
 * Constructs AI prompt arguments from repository analysis.
 * Infers optimal build configuration from detected patterns.
 */
function buildArgsFromAnalysis(
  analysisResult: SessionAnalysisResult,
  optimization?: boolean | string,
): Record<string, unknown> {
  const {
    language = 'unknown',
    framework = '',
    frameworkVersion = '',
    dependencies = [],
    ports = [],
    buildSystem,
    summary = '',
    recommendations = {},
  } = analysisResult;
  // Infer package manager from build system
  const packageManager =
    buildSystem?.type === 'maven' || buildSystem?.type === 'gradle'
      ? buildSystem.type
      : language === 'javascript' || language === 'typescript'
        ? 'npm'
        : 'unknown';
  // Get build file information
  const buildFile = buildSystem?.buildFile || '';
  const hasWrapper =
    buildFile.includes('mvnw') || buildSystem?.buildCommand?.includes('mvnw') || false;
  // Determine appropriate build command
  let recommendedBuildCommand = '';
  if (buildSystem?.type === 'maven') {
    recommendedBuildCommand = hasWrapper ? './mvnw' : 'mvn';
  } else if (buildSystem?.type === 'gradle') {
    recommendedBuildCommand = hasWrapper ? './gradlew' : 'gradle';
  } else if (buildSystem?.buildCommand) {
    recommendedBuildCommand = buildSystem.buildCommand;
  }
  return {
    language,
    framework,
    frameworkVersion,
    dependencies: dependencies?.map((d) => d.name || d).join(', ') || '',
    ports: ports?.join(', ') || '',
    summary: summary || `${language} ${framework ? `${framework} ` : ''}application`,
    packageManager,
    buildSystem: buildSystem?.type || 'none',
    buildCommand: recommendedBuildCommand,
    buildFile,
    hasWrapper,
    // Include analysis recommendations - use 'baseImage' to match prompt template
    baseImage: recommendations?.baseImage || '',
    buildStrategy: recommendations?.buildStrategy || '',
    securityNotes: recommendations?.securityNotes?.join('; ') || '',
    ...(optimization && {
      optimization: typeof optimization === 'string' ? optimization : 'performance',
    }),
  };
}

// computeHash function removed - was unused after tool wrapper elimination

/**
 * Generates Dockerfile through direct AI analysis when initial detection fails.
 * Fallback strategy when confidence thresholds aren't met for guided generation.
 */
async function generateWithDirectAnalysis(
  analysisResult: SessionAnalysisResult,
  params: GenerateDockerfileParams,
  moduleRoot: string,
  context: ToolContext,
  logger: Logger,
): Promise<Result<SingleDockerfileResult>> {
  const rawPath = params.path || '.';
  const repoPath = path.normalize(rawPath);
  const normalizedModuleRoot = path.normalize(moduleRoot);

  // Build comprehensive prompt args using the same rich analysis as high-confidence path
  const promptArgs = {
    ...buildArgsFromAnalysis(analysisResult, params.optimization),
    repoPath: path.isAbsolute(normalizedModuleRoot)
      ? normalizedModuleRoot
      : path.resolve(path.join(repoPath, normalizedModuleRoot)),
    moduleRoot,
  };

  logger.info(
    {
      promptArgs,
      detectedLanguage: analysisResult.language,
      confidence: analysisResult.confidence,
      threshold: ANALYSIS_CONFIG.CONFIDENCE_THRESHOLD,
    },
    '🤖 AI Direct Analysis: Asking AI to examine repository files and generate Dockerfile without pre-analysis constraints',
  );

  // Enhance prompt with knowledge context (same as high-confidence path)
  let enhancedPromptArgs = promptArgs;
  try {
    const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
      operation: 'generate_dockerfile',
      ...(analysisResult.language && { language: analysisResult.language }),
      ...(analysisResult.framework && { framework: analysisResult.framework }),
      environment: params.environment ?? 'production',
      tags: ['dockerfile', 'generation', analysisResult.language, analysisResult.framework].filter(
        Boolean,
      ) as string[],
    });

    if (knowledgeResult.bestPractices && knowledgeResult.bestPractices.length > 0) {
      enhancedPromptArgs = { ...promptArgs, ...knowledgeResult };
      logger.info(
        {
          practicesCount: knowledgeResult.bestPractices.length,
          examplesCount: knowledgeResult.examples ? knowledgeResult.examples.length : 0,
          moduleRoot,
        },
        'Enhanced direct analysis with knowledge for module',
      );
    }
  } catch (error) {
    logger.debug(
      { error, moduleRoot },
      'Knowledge enhancement failed for direct analysis, using base prompt',
    );
  }

  // Use direct analysis prompt
  const aiResult = await aiGenerateWithSampling(logger, context, {
    promptName: 'dockerfile-direct-analysis',
    promptArgs: enhancedPromptArgs,
    expectation: 'dockerfile',
    maxRetries: 3,
    fallbackBehavior: 'default',
    maxTokens: ANALYSIS_CONFIG.DIRECT_ANALYSIS_MAX_TOKENS,
    // Sampling is always enabled for better quality
  });

  if (aiResult.ok) {
    const cleaned = stripFencesAndNoise(aiResult.value.winner.content, 'dockerfile');

    if (isValidDockerfileContent(cleaned)) {
      // Success - extract info from generated content
      const baseImageUsed = extractBaseImage(cleaned) || params.baseImage || 'node:18-alpine'; // fallback

      // Handle moduleRoot path resolution correctly
      let targetPath: string;
      if (path.isAbsolute(normalizedModuleRoot)) {
        // Absolute path - use as-is
        targetPath = normalizedModuleRoot;
      } else if (normalizedModuleRoot.startsWith(repoPath)) {
        // moduleRoot already includes repoPath - use as-is to avoid duplication
        targetPath = normalizedModuleRoot;
      } else {
        // Relative path - join with repoPath
        targetPath = path.resolve(path.join(repoPath, normalizedModuleRoot));
      }
      const dockerfilePath = path.join(targetPath, 'Dockerfile');

      await fs.writeFile(dockerfilePath, cleaned, 'utf-8');

      const result: SingleDockerfileResult = {
        content: cleaned,
        path: path.resolve(repoPath),
        moduleRoot,
        baseImage: baseImageUsed,
        optimization: params.optimization !== false,
        multistage: cleaned.includes('FROM ') && cleaned.split('FROM ').length > 2,
        warnings: [], // Could add analysis for warnings
      };

      // Add sampling metadata if available
      if (aiResult.value.samplingMetadata) {
        result.samplingMetadata = aiResult.value.samplingMetadata;
        result.winnerScore = aiResult.value.winner.score;
        if (params.includeScoreBreakdown && aiResult.value.winner.scoreBreakdown) {
          result.scoreBreakdown = aiResult.value.winner.scoreBreakdown;
        }
        if (params.returnAllCandidates && aiResult.value.allCandidates) {
          result.allCandidates = aiResult.value.allCandidates;
        }
      }

      logger.info(
        {
          baseImage: baseImageUsed,
          multistage: result.multistage,
          dockerfilePath,
          originalDetection: {
            language: analysisResult.language,
            confidence: analysisResult.confidence,
          },
        },
        '✅ DIRECT ANALYSIS SUCCESS: AI successfully analyzed repository and generated Dockerfile',
      );

      return Success(result);
    }
  }

  // Fallback to template if direct analysis fails
  logger.warn(
    {
      error: aiResult.ok ? 'Invalid content' : aiResult.error,
      originalDetection: {
        language: analysisResult.language,
        confidence: analysisResult.confidence,
      },
    },
    '⚠️ DIRECT ANALYSIS FAILED: AI could not generate valid Dockerfile, falling back to template generation',
  );

  const fallbackResult = generateTemplateDockerfile(
    sessionToAnalyzeRepoResult(analysisResult),
    params,
    moduleRoot,
  );

  if (!fallbackResult.ok) {
    return Failure(`Both direct analysis and template generation failed: ${fallbackResult.error}`);
  }

  // Handle template fallback similar to existing logic
  let targetPath: string;
  if (path.isAbsolute(normalizedModuleRoot)) {
    // Absolute path - use as-is
    targetPath = normalizedModuleRoot;
  } else if (normalizedModuleRoot.startsWith(repoPath)) {
    // moduleRoot already includes repoPath - use as-is to avoid duplication
    targetPath = normalizedModuleRoot;
  } else {
    // Relative path - join with repoPath
    targetPath = path.resolve(path.join(repoPath, normalizedModuleRoot));
  }
  const dockerfilePath = path.join(targetPath, 'Dockerfile');

  await fs.writeFile(dockerfilePath, fallbackResult.value.content, 'utf-8');

  logger.info(
    {
      baseImage: fallbackResult.value.baseImage,
      dockerfilePath,
      originalDetection: {
        language: analysisResult.language,
        confidence: analysisResult.confidence,
      },
    },
    '🔧 TEMPLATE FALLBACK SUCCESS: Generated basic Dockerfile using template after direct analysis failed',
  );

  return Success({
    content: fallbackResult.value.content,
    path: path.resolve(repoPath),
    moduleRoot,
    baseImage: fallbackResult.value.baseImage,
    optimization: params.optimization !== false,
    multistage: params.multistage !== false,
  });
}

/**
 * Generates Dockerfile for individual module within multi-module project.
 * Supports both AI-guided and template-based generation with sampling optimization.
 */
async function generateSingleDockerfile(
  analysisResult: SessionAnalysisResult,
  params: GenerateDockerfileParams,
  moduleRoot: string,
  context: ToolContext,
  logger: Logger,
): Promise<Result<SingleDockerfileResult>> {
  const { multistage = true, securityHardening = true } = params;
  const optimization = params.optimization === false ? false : true;

  // Prepare sampling options (filter out undefined values)
  const samplingOptions: SamplingOptions = {};
  // Sampling is always enabled for better quality
  samplingOptions.enableSampling = true;
  if (params.maxCandidates !== undefined) samplingOptions.maxCandidates = params.maxCandidates;
  if (params.earlyStopThreshold !== undefined)
    samplingOptions.earlyStopThreshold = params.earlyStopThreshold;
  if (params.includeScoreBreakdown !== undefined)
    samplingOptions.includeScoreBreakdown = params.includeScoreBreakdown;
  if (params.returnAllCandidates !== undefined)
    samplingOptions.returnAllCandidates = params.returnAllCandidates;
  if (params.useCache !== undefined) samplingOptions.useCache = params.useCache;

  // Use direct analysis only as fallback for very low confidence or missing data
  const shouldUseDirectAnalysis =
    !analysisResult.language ||
    analysisResult.language === 'unknown' ||
    !analysisResult.confidence ||
    analysisResult.confidence < 50; // Very low confidence threshold for direct analysis

  if (shouldUseDirectAnalysis) {
    const reason =
      analysisResult.language === 'unknown'
        ? 'language detection failed (unknown)'
        : !analysisResult.language
          ? 'no language detected'
          : `very low confidence (confidence: ${analysisResult.confidence}/50)`;

    logger.info(
      {
        confidence: analysisResult.confidence,
        language: analysisResult.language,
        detectionMethod: analysisResult.detectionMethod,
        threshold: ANALYSIS_CONFIG.CONFIDENCE_THRESHOLD,
        moduleRoot,
        reason,
      },
      `🔧 DIRECT ANALYSIS: ${reason} - AI will examine repository files directly`,
    );

    return await generateWithDirectAnalysis(analysisResult, params, moduleRoot, context, logger);
  }

  logger.info(
    {
      confidence: analysisResult.confidence,
      language: analysisResult.language,
      framework: analysisResult.framework,
      detectionMethod: analysisResult.detectionMethod,
      moduleRoot,
    },
    `🤖 AI SAMPLING: Using structured analysis data with AI sampling for optimal Dockerfile generation`,
  );

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

  // Log what we're sending to the AI
  logger.info(
    {
      language: promptArgs.language,
      framework: promptArgs.framework,
      frameworkVersion: analysisResult.frameworkVersion,
      baseImage: promptArgs.baseImage,
      buildStrategy: promptArgs.buildStrategy,
      buildSystem: promptArgs.buildSystem,
      buildFile: promptArgs.buildFile,
      buildCommand: promptArgs.buildCommand,
      packageManager: promptArgs.packageManager,
    },
    'Prompt arguments being sent to AI',
  );

  // Enhance prompt with knowledge context
  try {
    const knowledgeResult = await enhancePromptWithKnowledge(promptArgs, {
      operation: 'generate_dockerfile',
      ...(analysisResult.language && { language: analysisResult.language }),
      ...(analysisResult.framework && { framework: analysisResult.framework }),
      environment: params.environment ?? 'production',
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

  // Always use sampling for better quality
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

  // Determine output path - write to each module root directory
  const rawPath = params.path || '.';
  const repoPath = path.normalize(rawPath);
  const normalizedModuleRoot = path.normalize(moduleRoot);

  // Handle moduleRoot path resolution correctly
  let targetPath: string;
  if (path.isAbsolute(normalizedModuleRoot)) {
    // Absolute path - use as-is
    targetPath = normalizedModuleRoot;
  } else if (normalizedModuleRoot.startsWith(repoPath)) {
    // moduleRoot already includes repoPath - use as-is to avoid duplication
    targetPath = normalizedModuleRoot;
  } else {
    // Relative path - join with repoPath
    targetPath = path.resolve(path.join(repoPath, normalizedModuleRoot));
  }
  const dockerfilePath = path.join(targetPath, 'Dockerfile');

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
    path: path.resolve(repoPath), // Repository path (build context)
    moduleRoot,
    baseImage: baseImageUsed,
    optimization,
    multistage,
    ...(warnings.length > 0 && { warnings }),
  };

  // Add sampling metadata
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

  return Success(result);
}

/**
 * Main Dockerfile generation orchestrator supporting multi-module projects.
 * Coordinates analysis, AI generation, and template fallbacks across project modules.
 */
async function generateDockerfileImpl(
  params: GenerateDockerfileParams,
  context: ToolContext,
): Promise<Result<GenerateDockerfileResult>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }

  // Optional progress reporting for complex operations (AI generation)
  const progress = context.progress ? createStandardProgress(context.progress) : undefined;
  const logger = getToolLogger(context, 'generate-dockerfile');
  const timer = createToolTimer(logger, 'generate-dockerfile');
  try {
    const { multistage = true } = params;
    // Normalize optimization to boolean - any string value means optimization is enabled
    const optimization = params.optimization === false ? false : true;
    // Progress: Starting validation and analysis
    if (progress) await progress('VALIDATING');
    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId } = sessionResult.value;
    const slice = useSessionSlice('generate-dockerfile', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info({ sessionId }, 'Starting Dockerfile generation');

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    // Get analysis result from analyze-repo session slice
    const analyzeRepoSliceResult = await getSessionSlice(
      'analyze-repo',
      sessionId,
      analyzeRepoIO,
      context,
    );
    if (!analyzeRepoSliceResult.ok) {
      logger.debug(
        { sessionId, error: analyzeRepoSliceResult.error },
        'Failed to get analyze-repo session slice',
      );
      return Failure(
        `Repository must be analyzed first. Please run 'analyze-repo' before 'generate-dockerfile'.`,
      );
    }

    const analyzeRepoSlice = analyzeRepoSliceResult.value;
    const rawAnalysisResult = analyzeRepoSlice?.output;

    // Debug: Log what we found in session slice
    logger.info(
      {
        sessionId,
        hasSlice: !!analyzeRepoSlice,
        hasOutput: !!rawAnalysisResult,
        analyzeRepoSliceKeys: analyzeRepoSlice ? Object.keys(analyzeRepoSlice) : [],
        analysisLanguage: rawAnalysisResult?.language,
        analysisFramework: rawAnalysisResult?.framework,
        analysisFrameworkVersion: rawAnalysisResult?.frameworkVersion,
        analysisRecommendations: rawAnalysisResult?.recommendations,
      },
      'Session analysis slice lookup',
    );

    if (!rawAnalysisResult) {
      return Failure(
        `Repository analysis not found in session. Please run 'analyze-repo' before 'generate-dockerfile'.`,
      );
    }

    // Map the analyze-repo result to SessionAnalysisResult format
    const analysisResult: SessionAnalysisResult = {
      language: rawAnalysisResult.language,
      ...(rawAnalysisResult.framework && { framework: rawAnalysisResult.framework }),
      ...(rawAnalysisResult.frameworkVersion && {
        frameworkVersion: rawAnalysisResult.frameworkVersion,
      }),
      dependencies: rawAnalysisResult.dependencies.map((dep) => ({
        name: dep.name,
        ...(dep.version && { version: dep.version }),
      })),
      ports: rawAnalysisResult.ports,
      confidence: rawAnalysisResult.confidence,
      detectionMethod: rawAnalysisResult.detectionMethod,
      ...(rawAnalysisResult.buildSystem && {
        buildSystem: {
          type: rawAnalysisResult.buildSystem.type,
          buildFile: rawAnalysisResult.buildSystem.file,
          buildCommand: rawAnalysisResult.buildSystem.buildCommand,
        },
      }),
      summary: `${rawAnalysisResult.language} ${rawAnalysisResult.framework || ''} project`.trim(),
      // Include recommendations
      ...(rawAnalysisResult.recommendations && {
        recommendations: {
          baseImage: rawAnalysisResult.recommendations.baseImage,
          buildStrategy: rawAnalysisResult.recommendations.buildStrategy,
          securityNotes: rawAnalysisResult.recommendations.securityNotes,
        },
      }),
      // Include detected modules
      ...(rawAnalysisResult.modules && { modules: rawAnalysisResult.modules }),
    };

    // Determine module roots from analysis or provided paths
    let rawModuleRoots: string[];

    // First check if analyze-repo detected modules
    if (analysisResult.modules && analysisResult.modules.length > 0) {
      rawModuleRoots = analysisResult.modules;
      logger.info(
        { modulesFromAnalysis: rawModuleRoots },
        'Using module roots detected by analyze-repo',
      );
    } else if (params.dockerfileDirectoryPaths && params.dockerfileDirectoryPaths.length > 0) {
      rawModuleRoots = params.dockerfileDirectoryPaths;
      logger.info(
        { providedModuleRoots: params.dockerfileDirectoryPaths },
        'Using provided module roots',
      );
    } else {
      // Fallback: Auto-detect modules based on repository analysis
      const repoPath = path.normalize(params.path || '.');
      rawModuleRoots = await detectModuleRoots(repoPath, analysisResult.language, logger);
      logger.info({ detectedModuleRoots: rawModuleRoots }, 'Auto-detected module roots (fallback)');
    }
    const moduleRoots = rawModuleRoots.map((r) => path.normalize(r));

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
    // Prepare the result for session update
    const generationResult: GenerateDockerfileResult = {
      dockerfiles: dockerfileResults,
      count: dockerfileResults.length,
      optimization,
      multistage,
      sessionId,
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: generationResult,
      state: {
        lastGeneratedAt: new Date(),
        dockerfileCount: dockerfileResults.length,
        primaryModule: moduleRoots[0] || 'root',
        generationStrategy: dockerfileResults.some((d) => d.samplingMetadata || d.winnerScore)
          ? 'ai'
          : 'template',
        lastOptimization: optimization ? 'enabled' : 'disabled',
      },
    });
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
    // Return result with file write indicator
    const result: GenerateDockerfileResult & {
      _fileWritten?: boolean;
      _fileWrittenPath?: string;
    } = {
      dockerfiles: dockerfileResults,
      count: dockerfileResults.length,
      optimization,
      multistage,
      ...(allWarnings.length > 0 && { warnings: allWarnings }),
      sessionId,
      _fileWritten: true,
      _fileWrittenPath: dockerfileResults.map((d) => d.path).join(', '),
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

    const errorMessage = error instanceof Error ? error.message : String(error);
    return Failure(errorMessage);
  }
}

/**
 * Main entry point for Dockerfile generation tool.
 * Provides AI-powered containerization with intelligent fallbacks.
 */
export const generateDockerfile = generateDockerfileImpl;
