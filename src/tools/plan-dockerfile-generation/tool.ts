/**
 * Plan Dockerfile Generation Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating a Dockerfile. This tool helps users
 * understand best practices and recommendations before actual Dockerfile generation.
 *
 * @category docker
 * @version 1.0.0
 * @aiDriven false
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import {
  planDockerfileGenerationSchema,
  type DockerfilePlan,
  type DockerfileRequirement,
} from './schema';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import type { z } from 'zod';

const name = 'plan-dockerfile-generation';
const description =
  'Gather insights from knowledgebase and return requirements for Dockerfile creation';
const version = '1.0.0';

const BASE_IMAGE_RECOMMENDATIONS: Record<
  string,
  Array<{
    image: string;
    reason: string;
    priority: 'primary' | 'alternative';
  }>
> = {
  javascript: [
    {
      image: 'node:20-alpine',
      reason: 'Minimal size, official Node.js image',
      priority: 'primary' as const,
    },
    {
      image: 'node:20-slim',
      reason: 'Debian-based with more tools',
      priority: 'alternative' as const,
    },
  ],
  typescript: [
    {
      image: 'node:20-alpine',
      reason: 'Minimal size, official Node.js image',
      priority: 'primary' as const,
    },
    {
      image: 'node:20-slim',
      reason: 'Debian-based with more tools',
      priority: 'alternative' as const,
    },
  ],
  python: [
    { image: 'python:3.11-slim', reason: 'Minimal Python image', priority: 'primary' as const },
    {
      image: 'python:3.11-alpine',
      reason: 'Smallest size but may have compatibility issues',
      priority: 'alternative' as const,
    },
  ],
  java: [
    {
      image: 'mcr.microsoft.com/openjdk/jdk:21-mariner',
      reason: 'Microsoft Azure Linux with enhanced security',
      priority: 'primary' as const,
    },
    {
      image: 'eclipse-temurin:17-jre-alpine',
      reason: 'Minimal JRE for runtime',
      priority: 'alternative' as const,
    },
  ],
  go: [
    {
      image: 'golang:1.21-alpine',
      reason: 'Minimal Go build environment',
      priority: 'primary' as const,
    },
    {
      image: 'gcr.io/distroless/static-debian11',
      reason: 'Distroless for runtime (multi-stage)',
      priority: 'alternative' as const,
    },
  ],
  'c#': [
    {
      image: 'mcr.microsoft.com/dotnet/aspnet:8.0',
      reason: 'Official ASP.NET runtime',
      priority: 'primary' as const,
    },
    {
      image: 'mcr.microsoft.com/dotnet/sdk:8.0',
      reason: 'SDK for build stage',
      priority: 'alternative' as const,
    },
  ],
  dotnet: [
    {
      image: 'mcr.microsoft.com/dotnet/aspnet:8.0',
      reason: 'Official ASP.NET runtime',
      priority: 'primary' as const,
    },
    {
      image: 'mcr.microsoft.com/dotnet/sdk:8.0',
      reason: 'SDK for build stage',
      priority: 'alternative' as const,
    },
  ],
  php: [
    { image: 'php:8.2-fpm-alpine', reason: 'Minimal PHP-FPM image', priority: 'primary' as const },
    { image: 'php:8.2-apache', reason: 'With Apache web server', priority: 'alternative' as const },
  ],
  ruby: [
    { image: 'ruby:3.2-alpine', reason: 'Minimal Ruby image', priority: 'primary' as const },
    {
      image: 'ruby:3.2-slim',
      reason: 'Debian-based with more tools',
      priority: 'alternative' as const,
    },
  ],
  rust: [
    {
      image: 'rust:1.75-alpine',
      reason: 'Minimal Rust build environment',
      priority: 'primary' as const,
    },
    {
      image: 'gcr.io/distroless/static-debian11',
      reason: 'Distroless for runtime (multi-stage)',
      priority: 'alternative' as const,
    },
  ],
} as const;

async function run(
  input: z.infer<typeof planDockerfileGenerationSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfilePlan>> {
  const {
    sessionId,
    language: inputLanguage,
    framework: inputFramework,
    environment,
    baseImagePreference,
  } = input;

  let path = input.path;
  let analysis: RepositoryAnalysis | undefined;

  if (sessionId && ctx.sessionManager) {
    try {
      const workflowStateResult = await ctx.sessionManager.get(sessionId);
      if (workflowStateResult.ok && workflowStateResult.value) {
        const workflowState = workflowStateResult.value as Record<string, unknown>;

        const metadata = workflowState.metadata as Record<string, unknown> | undefined;
        if (metadata && !path && typeof metadata.analyzedPath === 'string') {
          path = metadata.analyzedPath;
        }

        const results = workflowState.results as Record<string, unknown> | undefined;
        const analyzeRepoResult = results?.['analyze-repo'];
        if (analyzeRepoResult && typeof analyzeRepoResult === 'object') {
          analysis = analyzeRepoResult as RepositoryAnalysis;
          ctx.logger.info(
            { sessionId, language: analysis.language, framework: analysis.framework },
            'Retrieved repository analysis from sessionManager',
          );
        }
      }
    } catch (sessionError) {
      ctx.logger.debug(
        {
          sessionId,
          error: sessionError instanceof Error ? sessionError.message : String(sessionError),
        },
        'Unable to load workflow session data',
      );
    }
  }

  const language = inputLanguage || analysis?.language || 'auto-detect';
  const framework = inputFramework || analysis?.framework;

  if (!path && !analysis) {
    return Failure(
      'Either path or sessionId with analysis data is required. Run analyze-repo first or provide a path.',
    );
  }

  const repositoryInfo = {
    path: path || (analysis as { analyzedPath?: string } | undefined)?.analyzedPath,
    language,
    framework,
    languageVersion: analysis?.languageVersion,
    frameworkVersion: analysis?.frameworkVersion,
    buildSystem: analysis?.buildSystem,
    dependencies: analysis?.dependencies,
    ports: analysis?.suggestedPorts || analysis?.ports,
    entryPoint: analysis?.entryPoint,
  };

  ctx.logger.info(
    { language, framework, environment },
    'Querying knowledgebase for Dockerfile recommendations',
  );

  const knowledgeSnippets = await getKnowledgeSnippets(TOPICS.DOCKERFILE_GENERATION, {
    environment: environment || 'production',
    tool: name,
    language,
    ...(framework && { framework }),
    maxChars: 8000,
    maxSnippets: 20,
  });

  const knowledgeMatches: DockerfileRequirement[] = knowledgeSnippets.map((snippet) => ({
    id: snippet.id,
    category: snippet.category || 'generic',
    recommendation: snippet.text,
    ...(snippet.tags && { tags: snippet.tags }),
    matchScore: snippet.weight,
  }));

  const securityMatches = knowledgeMatches.filter(
    (m) => m.category === 'security' || m.tags?.includes('security'),
  );
  const optimizationMatches = knowledgeMatches.filter(
    (m) =>
      m.tags?.includes('optimization') || m.tags?.includes('caching') || m.tags?.includes('size'),
  );
  const bestPracticeMatches = knowledgeMatches.filter(
    (m) => !securityMatches.includes(m) && !optimizationMatches.includes(m),
  );

  let baseImages = [
    ...(BASE_IMAGE_RECOMMENDATIONS[language] || [
      { image: 'alpine:latest', reason: 'Minimal base image', priority: 'primary' },
    ]),
  ];

  if (baseImagePreference) {
    const lowerPreference = baseImagePreference.toLowerCase();
    if (lowerPreference.includes('microsoft') || lowerPreference.includes('azure')) {
      if (language === 'java') {
        baseImages = [
          {
            image: 'mcr.microsoft.com/openjdk/jdk:21-mariner',
            reason: 'Microsoft Azure Linux with enhanced security',
            priority: 'primary',
          },
          ...baseImages,
        ];
      } else if (language === 'dotnet' || language === 'c#') {
        baseImages = [
          {
            image: 'mcr.microsoft.com/dotnet/aspnet:8.0',
            reason: 'Official ASP.NET runtime',
            priority: 'primary',
          },
          ...baseImages,
        ];
      }
    } else if (lowerPreference.includes('distroless')) {
      baseImages = [
        {
          image: 'gcr.io/distroless/static-debian11',
          reason: 'Distroless for runtime (multi-stage)',
          priority: 'primary',
        },
        ...baseImages,
      ];
    } else if (lowerPreference.includes('alpine') && baseImages.length > 0 && baseImages[0]) {
      const firstImage = baseImages[0];
      baseImages = [
        {
          ...firstImage,
          priority: 'primary',
        },
        ...baseImages.slice(1),
      ];
    }
  }

  const buildSystemType = (analysis?.buildSystem as { type?: string } | undefined)?.type;
  const shouldUseMultistage =
    language === 'java' ||
    language === 'go' ||
    language === 'rust' ||
    language === 'dotnet' ||
    language === 'c#' ||
    buildSystemType === 'maven' ||
    buildSystemType === 'gradle';

  const buildStrategy = {
    multistage: shouldUseMultistage,
    reason: shouldUseMultistage
      ? 'Multi-stage build recommended to separate build tools from runtime, reducing image size by 70-90%'
      : 'Single-stage build sufficient for interpreted languages',
  };

  const confidence =
    knowledgeMatches.length > 0 ? Math.min(0.95, 0.5 + knowledgeMatches.length * 0.05) : 0.5;

  const summary = `
Dockerfile Planning Summary:
- Language: ${language}${framework ? ` (${framework})` : ''}
- Environment: ${environment || 'production'}
- Build Strategy: ${buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'}
- Recommended Base: ${baseImages[0]?.image || 'No base image available'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Optimizations: ${optimizationMatches.length}
  - Best Practices: ${bestPracticeMatches.length}

Next Step: Use generate-dockerfile with sessionId to create the Dockerfile using these recommendations.
  `.trim();

  const plan: DockerfilePlan = {
    repositoryInfo: {
      ...(repositoryInfo.path && { path: repositoryInfo.path }),
      ...(repositoryInfo.language && { language: repositoryInfo.language }),
      ...(repositoryInfo.framework && { framework: repositoryInfo.framework }),
      ...(repositoryInfo.languageVersion && { languageVersion: repositoryInfo.languageVersion }),
      ...(repositoryInfo.frameworkVersion && { frameworkVersion: repositoryInfo.frameworkVersion }),
      ...(repositoryInfo.buildSystem && { buildSystem: repositoryInfo.buildSystem }),
      ...(repositoryInfo.dependencies && { dependencies: repositoryInfo.dependencies }),
      ...(repositoryInfo.ports && { ports: repositoryInfo.ports }),
      ...(repositoryInfo.entryPoint && { entryPoint: repositoryInfo.entryPoint }),
    },
    recommendations: {
      baseImages,
      buildStrategy,
      securityConsiderations: securityMatches,
      optimizations: optimizationMatches,
      bestPractices: bestPracticeMatches,
    },
    knowledgeMatches,
    confidence,
    summary,
  };

  if (sessionId && ctx.session) {
    ctx.session.storeResult('plan-dockerfile-generation', plan);
    ctx.session.set('dockerfilePlanGenerated', true);
    ctx.logger.info({ sessionId }, 'Stored Dockerfile plan in session');
  }

  ctx.logger.info(
    {
      knowledgeMatchCount: knowledgeMatches.length,
      securityCount: securityMatches.length,
      optimizationCount: optimizationMatches.length,
      confidence,
    },
    'Dockerfile planning completed',
  );

  return Success(plan);
}

const tool: Tool<typeof planDockerfileGenerationSchema, DockerfilePlan> = {
  name,
  description,
  category: 'docker',
  version,
  schema: planDockerfileGenerationSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
