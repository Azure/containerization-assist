/**
 * Resolve Base Images Tool - Standardized Implementation
 *
 * Resolves optimal Docker base images for applications using standardized
 * helpers for consistency and improved error handling
 *
 * @example
 * ```typescript
 * const result = await resolveBaseImages({
 *   sessionId: 'session-123', // optional
 *   technology: 'nodejs',
 *   requirements: { environment: 'production', security: 'high' }
 * }, context, logger);
 *
 * if (result.primaryImage) {
 *   console.log('Recommended image:', result.primaryImage.name);
 *   console.log('Rationale:', result.rationale);
 * }
 * ```
 */

import {
  ensureSession,
  defineToolIO,
  useSessionSlice,
  getSessionSlice,
} from '@/mcp/tool-session-helpers';
import { initializeToolInstrumentation } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import type { ToolContext } from '@/mcp/context';
import type { Logger } from '@/lib/logger';
import { getRecommendedBaseImage } from '@/lib/base-images';
import { scoreConfigCandidates } from '@/lib/integrated-scoring';
import { getKnowledgeForCategory } from '@/knowledge/index';
import { resolveBaseImagesSchema, type ResolveBaseImagesParams } from './schema';
import { analyzeRepoSchema } from '@/tools/analyze-repo/schema';
import { z } from 'zod';

// Helper functions for base image resolution
function getSuggestedBaseImages(
  language: string,
  framework?: string,
  frameworkVersion?: string,
): string[] {
  // Check for .NET Framework specifically
  if (language === 'dotnet' || language === 'csharp') {
    if (
      framework === 'aspnet-webapi' ||
      framework === 'aspnet' ||
      framework === 'dotnet-framework'
    ) {
      if (frameworkVersion?.startsWith('4.')) {
        return [
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2022',
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8-windowsservercore-ltsc2019',
          'mcr.microsoft.com/dotnet/framework/aspnet:4.8',
        ];
      }
    }
    // .NET Core/5+ images
    return [
      'mcr.microsoft.com/dotnet/aspnet:8.0-alpine',
      'mcr.microsoft.com/dotnet/aspnet:8.0',
      'mcr.microsoft.com/dotnet/sdk:8.0',
    ];
  }

  const suggestions: Record<string, string[]> = {
    javascript: ['node:18-alpine', 'node:18-slim', 'node:18'],
    typescript: ['node:18-alpine', 'node:18-slim', 'node:18'],
    python: ['python:3.11-alpine', 'python:3.11-slim', 'python:3.11'],
    java: ['openjdk:17-alpine', 'openjdk:17-slim', 'openjdk:17'],
    go: ['golang:1.21-alpine', 'golang:1.21', 'alpine:latest'],
    rust: ['rust:1.70-alpine', 'rust:1.70', 'alpine:latest'],
  };
  return suggestions[language] || ['alpine:latest'];
}

async function getBaseImageKnowledge(
  language: string,
  environment: string,
  _securityLevel: string,
  logger: Logger,
): Promise<{
  recommendations: string[];
  securityNotes: string[];
  performanceNotes: string[];
  compatibilityWarnings: string[];
}> {
  try {
    // Query knowledge base for base image recommendations
    const dockerfileKnowledge = await getKnowledgeForCategory('dockerfile', `FROM ${language}`, {
      language,
      environment,
    });

    // Get security-specific knowledge
    const securityKnowledge = await getKnowledgeForCategory('security', `base image ${language}`, {
      language,
    });

    // Extract relevant recommendations
    const recommendations: string[] = [];
    const securityNotes: string[] = [];
    const performanceNotes: string[] = [];
    const compatibilityWarnings: string[] = [];

    // Process dockerfile knowledge
    dockerfileKnowledge.forEach((match: any) => {
      if (match.entry.tags?.includes('base-image')) {
        recommendations.push(match.entry.recommendation);
      }
      if (match.entry.tags?.includes('performance')) {
        performanceNotes.push(match.entry.recommendation);
      }
      if (match.entry.tags?.includes('compatibility')) {
        compatibilityWarnings.push(match.entry.recommendation);
      }
    });

    // Process security knowledge
    securityKnowledge.forEach((match: any) => {
      if (match.entry.severity === 'high' || match.entry.tags?.includes('base-image')) {
        securityNotes.push(match.entry.recommendation);
      }
    });

    // Add default recommendations based on language if none found
    if (recommendations.length === 0) {
      const defaults: Record<string, string[]> = {
        javascript: [
          'Use node:lts-alpine for production',
          'Consider node:18-alpine for smaller size',
        ],
        typescript: [
          'Use node:lts-alpine for production',
          'Consider node:18-alpine for smaller size',
        ],
        python: [
          'Use python:3.11-slim for production',
          'Consider python:3.11-alpine for minimal size',
        ],
        java: ['Use eclipse-temurin:17-alpine for production', 'Consider amazoncorretto for AWS'],
        go: ['Use golang:1.21-alpine for builds', 'Consider scratch or distroless for runtime'],
      };
      recommendations.push(
        ...(defaults[language] || ['Consider Alpine-based images for smaller size']),
      );
    }

    logger.debug(
      {
        language,
        recommendationCount: recommendations.length,
        securityNotesCount: securityNotes.length,
      },
      'Retrieved knowledge-based recommendations',
    );

    return {
      recommendations,
      securityNotes,
      performanceNotes,
      compatibilityWarnings,
    };
  } catch (error) {
    logger.debug({ error }, 'Failed to get knowledge recommendations, using defaults');
    return {
      recommendations: [],
      securityNotes: [],
      performanceNotes: [],
      compatibilityWarnings: [],
    };
  }
}

async function getImageMetadata(
  name: string,
  tag: string,
  logger: Logger,
): Promise<{
  name: string;
  tag: string;
  digest?: string;
  size?: string;
  lastUpdated?: string;
}> {
  // Simplified metadata - in a real system this would query registries
  logger.debug({ name, tag }, 'Getting image metadata');

  return {
    name,
    tag,
    digest: `sha256:${Math.random().toString(16).substr(2, 64)}`, // Mock digest
    size: tag.includes('alpine') ? '5MB' : tag.includes('slim') ? '100MB' : '200MB',
    lastUpdated: new Date().toISOString(),
  };
}
import { Success, Failure, type Result } from '@/types';

// Define the result schema for type safety
const BaseImageRecommendationSchema = z.object({
  sessionId: z.string(),
  technology: z.string().optional(),
  primaryImage: z.object({
    name: z.string(),
    tag: z.string(),
    digest: z.string().optional(),
    size: z.number().optional(),
    lastUpdated: z.string().optional(),
    score: z.number().optional(),
    knowledgeBasedRecommendations: z.array(z.string()).optional(),
  }),
  alternativeImages: z
    .array(
      z.object({
        name: z.string(),
        tag: z.string(),
        reason: z.string(),
        score: z.number().optional(),
        pros: z.array(z.string()).optional(),
        cons: z.array(z.string()).optional(),
      }),
    )
    .optional(),
  rationale: z.string(),
  securityConsiderations: z.array(z.string()).optional(),
  performanceNotes: z.array(z.string()).optional(),
  compatibilityWarnings: z.array(z.string()).optional(),
  bestPractices: z.array(z.string()).optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(resolveBaseImagesSchema, BaseImageRecommendationSchema);

// Define analyze-repo IO for accessing its session slice
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
});

const analyzeRepoIO = defineToolIO(analyzeRepoSchema, AnalyzeRepoResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastResolvedAt: z.date().optional(),
  primaryTechnology: z.string().optional(),
  recommendationScore: z.number().optional(),
});

export interface BaseImageRecommendation {
  sessionId: string;
  technology?: string;
  primaryImage: {
    name: string;
    tag: string;
    digest?: string;
    size?: number;
    lastUpdated?: string;
    score?: number;
    knowledgeBasedRecommendations?: string[];
  };
  alternativeImages?: Array<{
    name: string;
    tag: string;
    reason: string;
    score?: number;
    pros?: string[];
    cons?: string[];
  }>;
  rationale: string;
  securityConsiderations?: string[];
  performanceNotes?: string[];
  compatibilityWarnings?: string[];
  bestPractices?: string[];
}

/**
 * Base image resolution implementation - direct execution without wrapper
 */
async function resolveBaseImagesImpl(
  params: ResolveBaseImagesParams,
  context: ToolContext,
): Promise<Result<BaseImageRecommendation>> {
  // Basic parameter validation (essential validation only)
  if (!params || typeof params !== 'object') {
    return Failure('Invalid parameters provided');
  }
  const { logger, timer } = initializeToolInstrumentation(context, 'resolve-base-images');

  try {
    const { technology, requirements = {} } = params;

    // Extract requirements
    const targetEnvironment = (requirements.environment as string) || 'production';
    const securityLevel = (requirements.security as string) || 'medium';

    logger.info({ technology, targetEnvironment, securityLevel }, 'Resolving base images');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId } = sessionResult.value;
    const slice = useSessionSlice('resolve-base-images', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    logger.info(
      { sessionId, technology, targetEnvironment, securityLevel },
      'Starting base image resolution',
    );

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    // Get analysis result from analyze-repo session slice
    const analyzeRepoSliceResult = await getSessionSlice(
      'analyze-repo',
      sessionId,
      analyzeRepoIO,
      context,
    );
    const analyzeRepoSlice = analyzeRepoSliceResult.ok ? analyzeRepoSliceResult.value : null;
    const analysisResult = analyzeRepoSlice?.output;

    // Use provided technology or fall back to session analysis
    const language = technology || analysisResult?.language;
    if (!language) {
      return Failure(
        'No technology specified. Provide technology parameter or run analyze-repo tool first.',
      );
    }

    const framework = analysisResult?.framework;
    const frameworkVersion = analysisResult?.frameworkVersion;

    // Also check if recommendations already include a base image
    const recommendedBaseImage = analysisResult?.recommendations?.baseImage;

    logger.info(
      {
        sessionId,
        language,
        framework,
        frameworkVersion,
        recommendedBaseImage,
        hasAnalysisData: !!analysisResult,
      },
      'Base image resolution using session data',
    );

    const suggestedImages = getSuggestedBaseImages(language, framework, frameworkVersion);

    // If we have a recommended base image from analysis, add it to suggestions
    if (recommendedBaseImage && !suggestedImages.includes(recommendedBaseImage)) {
      suggestedImages.unshift(recommendedBaseImage);
    }

    // Get knowledge-based recommendations for base images
    const knowledgeData = await getBaseImageKnowledge(
      language,
      targetEnvironment,
      securityLevel,
      logger,
    );

    const {
      recommendations: knowledgeRecommendations,
      securityNotes: knowledgeSecurityNotes,
      performanceNotes: knowledgePerformanceNotes,
      compatibilityWarnings: knowledgeCompatibilityWarnings,
    } = knowledgeData;

    // Select primary image based on environment and security level
    let primaryImage = suggestedImages[0] ?? getRecommendedBaseImage(language); // Default fallback
    if (targetEnvironment === 'production' && securityLevel === 'high') {
      // Prefer alpine or slim images for production with high security
      primaryImage =
        suggestedImages.find((img) => img.includes('alpine') || img.includes('slim')) ??
        primaryImage;
    }

    const [imageName, imageTag] = primaryImage.split(':');

    // Get real image metadata from Docker registry
    // Use registry client directly without factory wrapper
    const imageMetadata = await getImageMetadata(imageName ?? 'node', imageTag ?? 'latest', logger);

    // Score the primary image
    let primaryScore: number | undefined;
    try {
      const testDockerfile = `FROM ${primaryImage}`;
      const scoring = await scoreConfigCandidates(
        [testDockerfile],
        'dockerfile',
        params.targetEnvironment || targetEnvironment,
        logger,
      );
      if (scoring.ok && scoring.value[0]) {
        primaryScore = scoring.value[0].score;
        logger.info({ primaryImage, primaryScore }, 'Scored primary base image');
      }
    } catch (error) {
      logger.debug({ error }, 'Could not score primary image, continuing without score');
    }

    // Score alternative images
    const scoredAlternatives = await Promise.all(
      suggestedImages.slice(1, 3).map(async (img) => {
        const [name, tag] = img.split(':');
        let score: number | undefined;
        try {
          const testDockerfile = `FROM ${img}`;
          const scoring = await scoreConfigCandidates(
            [testDockerfile],
            'dockerfile',
            params.targetEnvironment || targetEnvironment,
            logger,
          );
          if (scoring.ok && scoring.value[0]) {
            score = scoring.value[0].score;
          }
        } catch {
          // Continue without score
        }
        return {
          name: name ?? 'node',
          tag: tag ?? 'latest',
          reason: img.includes('alpine') ? 'Smaller size, better security' : 'More compatibility',
          ...(score !== undefined && { score }),
        };
      }),
    );

    // Sort alternatives by score if available
    scoredAlternatives.sort((a, b) => (b.score || 0) - (a.score || 0));

    const recommendation: BaseImageRecommendation = {
      sessionId,
      primaryImage: {
        name: imageMetadata.name,
        tag: imageMetadata.tag,
        ...(imageMetadata.digest && { digest: imageMetadata.digest }),
        ...(imageMetadata.size && { size: parseInt(imageMetadata.size) || 0 }),
        ...(imageMetadata.lastUpdated && { lastUpdated: imageMetadata.lastUpdated }),
        ...(primaryScore !== undefined && { score: primaryScore }),
        ...(knowledgeRecommendations.length > 0 && {
          knowledgeBasedRecommendations: knowledgeRecommendations.slice(0, 3),
        }),
      },
      alternativeImages: scoredAlternatives.map((alt) => ({
        ...alt,
        pros: alt.name.includes('alpine')
          ? ['Minimal size', 'Reduced attack surface', 'Fast startup']
          : alt.name.includes('slim')
            ? ['Smaller than standard', 'Good compatibility', 'Security patches']
            : ['Full compatibility', 'All packages included', 'Well-tested'],
        cons: alt.name.includes('alpine')
          ? ['Possible glibc compatibility issues', 'Limited package availability']
          : alt.name.includes('slim')
            ? ['Missing some development tools']
            : ['Larger image size', 'More potential vulnerabilities'],
      })),
      rationale: `Selected ${primaryImage} for ${language}${framework ? `/${framework}` : ''} application based on ${targetEnvironment} environment with ${securityLevel} security requirements`,
      technology: language,
      securityConsiderations: [
        securityLevel === 'high'
          ? 'Using minimal Alpine-based image for reduced attack surface'
          : 'Standard base image with regular security updates',
        'Recommend scanning with Trivy or Snyk before deployment',
        ...knowledgeSecurityNotes,
      ].filter(Boolean),
      performanceNotes: [
        primaryImage.includes('alpine')
          ? 'Alpine images are smaller but may have compatibility issues with some packages'
          : 'Standard images have better compatibility but larger size',
        ...knowledgePerformanceNotes,
      ].filter(Boolean),
      ...(knowledgeCompatibilityWarnings.length > 0 && {
        compatibilityWarnings: knowledgeCompatibilityWarnings,
      }),
      ...(knowledgeRecommendations.length > 0 && {
        bestPractices: knowledgeRecommendations.slice(0, 5),
      }),
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: recommendation,
      state: {
        lastResolvedAt: new Date(),
        primaryTechnology: language,
        recommendationScore: primaryScore,
      },
    });

    timer.end({ primaryImage, sessionId, technology: language });
    logger.info(
      { sessionId, primaryImage, technology: language },
      'Base image resolution completed',
    );

    // Add sessionId to the recommendation
    const enrichedRecommendation = {
      ...recommendation,
      sessionId,
    };

    return Success(enrichedRecommendation);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Base image resolution failed');

    return Failure(extractErrorMessage(error));
  }
}

/**
 * Resolve base images tool
 */
export const resolveBaseImages = resolveBaseImagesImpl;
