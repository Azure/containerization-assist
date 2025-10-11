/**
 * Generate Dockerfile Plan Tool
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
import type { MCPTool } from '@/types/tool';
import {
  generateDockerfilePlanSchema,
  type DockerfilePlan,
  type DockerfileRequirement,
} from './schema';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import type { z } from 'zod';
import { CATEGORY } from '@/knowledge/types';

const name = 'generate-dockerfile-plan';
const description =
  'Gather insights from knowledgebase and return requirements for Dockerfile creation';
const version = '1.0.0';

async function run(
  input: z.infer<typeof generateDockerfilePlanSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfilePlan>> {
  const {
    language: inputLanguage,
    framework: inputFramework,
    environment,
    modulePathAbsoluteUnix,
  } = input;

  const path = input.repositoryPathAbsoluteUnix || '';
  const modulePath = modulePathAbsoluteUnix || path;

  const language = inputLanguage || 'auto-detect';
  const framework = inputFramework;

  if (!path) {
    return Failure('Path is required. Provide a path parameter.');
  }

  const repositoryInfo = {
    path: modulePath,
    language,
    framework,
    languageVersion: undefined,
    frameworkVersion: undefined,
    buildSystem: undefined,
    dependencies: undefined,
    ports: undefined,
    entryPoint: undefined,
  };

  ctx.logger.info(
    { language, framework, environment },
    'Querying knowledgebase for Dockerfile recommendations',
  );

  const knowledgeSnippets = await getKnowledgeSnippets(TOPICS.DOCKERFILE, {
    environment: environment || 'production',
    tool: name,
    language,
    ...(framework && { framework }),
    maxChars: 8000,
    maxSnippets: 20,
    category: CATEGORY.DOCKERFILE,
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

  const buildSystemType = undefined;
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
- Path: ${modulePath}${modulePathAbsoluteUnix ? ' (module)' : ''}
- Language: ${language}${framework ? ` (${framework})` : ''}
- Environment: ${environment || 'production'}
- Build Strategy: ${buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Optimizations: ${optimizationMatches.length}
  - Best Practices: ${bestPracticeMatches.length}
  `.trim();

  const plan: DockerfilePlan = {
    repositoryInfo: {
      name: modulePath.split('/').pop() || 'unknown',
      modulePathAbsoluteUnix: modulePath,
      ...(repositoryInfo.language && {
        language:
          repositoryInfo.language === 'java' || repositoryInfo.language === 'dotnet'
            ? repositoryInfo.language
            : 'other',
      }),
      ...(repositoryInfo.framework &&
        repositoryInfo.framework !== 'auto-detect' && {
          frameworks: [{ name: repositoryInfo.framework }],
        }),
    },
    recommendations: {
      buildStrategy,
      securityConsiderations: securityMatches,
      optimizations: optimizationMatches,
      bestPractices: bestPracticeMatches,
    },
    knowledgeMatches,
    confidence,
    summary,
  };

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

const tool: MCPTool<typeof generateDockerfilePlanSchema, DockerfilePlan> = {
  name,
  description,
  category: 'docker',
  version,
  schema: generateDockerfilePlanSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
