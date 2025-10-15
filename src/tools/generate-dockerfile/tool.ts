/**
 * Generate Dockerfile Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating a Dockerfile. This tool helps users
 * understand best practices and recommendations before actual Dockerfile generation.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 */

import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import {
  generateDockerfileSchema,
  type DockerfilePlan,
  type DockerfileRequirement,
  type GenerateDockerfileParams,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';

const name = 'generate-dockerfile';
const description =
  'Gather insights from knowledgebase and return requirements for Dockerfile creation';
const version = '2.0.0';

type DockerfileCategory = 'security' | 'optimization' | 'bestPractices';

interface DockerfileBuildRules {
  buildStrategy: {
    multistage: boolean;
    reason: string;
  };
}

const runPattern = createKnowledgeTool<
  GenerateDockerfileParams,
  DockerfilePlan,
  DockerfileCategory,
  DockerfileBuildRules
>({
  name,
  query: {
    topic: TOPICS.DOCKERFILE,
    category: CATEGORY.DOCKERFILE,
    maxChars: 8000,
    maxSnippets: 20,
    extractFilters: (input) => ({
      environment: input.environment || 'production',
      language: input.language || 'auto-detect',
      framework: input.framework,
    }),
  },
  categorization: {
    categoryNames: ['security', 'optimization', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<DockerfileCategory>({
      security: (s) => s.category === 'security' || Boolean(s.tags?.includes('security')),
      optimization: (s) =>
        Boolean(
          s.tags?.includes('optimization') ||
            s.tags?.includes('caching') ||
            s.tags?.includes('size'),
        ),
      bestPractices: () => true, // Catch remaining snippets as best practices
    }),
  },
  rules: {
    applyRules: (input) => {
      const language = input.language || 'auto-detect';
      const buildSystemType = undefined;

      const shouldUseMultistage =
        language === 'java' ||
        language === 'go' ||
        language === 'rust' ||
        language === 'dotnet' ||
        language === 'c#' ||
        (typeof buildSystemType === 'string' && ['maven', 'gradle'].includes(buildSystemType));

      return {
        buildStrategy: {
          multistage: shouldUseMultistage,
          reason: shouldUseMultistage
            ? 'Multi-stage build recommended to separate build tools from runtime, reducing image size by 70-90%'
            : 'Single-stage build sufficient for interpreted languages',
        },
      };
    },
  },
  plan: {
    buildPlan: (input, knowledge, rules, confidence) => {
      const path = input.repositoryPath || '';
      const modulePath = input.modulePath || path;
      const language = input.language || 'auto-detect';
      const framework = input.framework;

      const knowledgeMatches: DockerfileRequirement[] = knowledge.all.map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      const securityMatches: DockerfileRequirement[] = (knowledge.categories.security || []).map(
        (snippet) => ({
          id: snippet.id,
          category: snippet.category || 'security',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }),
      );

      const optimizationMatches: DockerfileRequirement[] = (
        knowledge.categories.optimization || []
      ).map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'optimization',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      const bestPracticeMatches: DockerfileRequirement[] = (
        knowledge.categories.bestPractices || []
      )
        .filter((snippet) => {
          // Exclude snippets already in security or optimization
          const isInSecurity = (knowledge.categories.security || []).some(
            (s) => s.id === snippet.id,
          );
          const isInOptimization = (knowledge.categories.optimization || []).some(
            (s) => s.id === snippet.id,
          );
          return !isInSecurity && !isInOptimization;
        })
        .map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'generic',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }));

      const summary = `
Dockerfile Planning Summary:
- Path: ${modulePath}${input.modulePath ? ' (module)' : ''}
- Language: ${language}${framework ? ` (${framework})` : ''}
- Environment: ${input.environment || 'production'}
- Build Strategy: ${rules.buildStrategy.multistage ? 'Multi-stage' : 'Single-stage'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Optimizations: ${optimizationMatches.length}
  - Best Practices: ${bestPracticeMatches.length}
      `.trim();

      return {
        repositoryInfo: {
          name: modulePath.split('/').pop() || 'unknown',
          modulePath,
          ...(language &&
            language !== 'auto-detect' && {
              language: language === 'java' || language === 'dotnet' ? language : 'other',
            }),
          ...(framework &&
            framework !== 'auto-detect' && {
              frameworks: [{ name: framework }],
            }),
        },
        recommendations: {
          buildStrategy: rules.buildStrategy,
          securityConsiderations: securityMatches,
          optimizations: optimizationMatches,
          bestPractices: bestPracticeMatches,
        },
        knowledgeMatches,
        confidence,
        summary,
      };
    },
  },
});

async function run(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<DockerfilePlan>> {
  const path = input.repositoryPath || '';

  if (!path) {
    return Failure('Path is required. Provide a path parameter.');
  }

  return runPattern(input, ctx);
}

const tool: MCPTool<typeof generateDockerfileSchema, DockerfilePlan> = {
  name,
  description,
  category: 'docker',
  version,
  schema: generateDockerfileSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
