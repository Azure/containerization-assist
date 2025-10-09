/**
 * Plan Manifest Generation Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating Kubernetes/Helm/ACA/Kustomize manifests.
 * This tool helps users understand best practices and recommendations before
 * actual manifest generation.
 *
 * @category kubernetes
 * @version 1.0.0
 * @aiDriven false
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import {
  planManifestGenerationSchema,
  type ManifestPlan,
  type ManifestRequirement,
} from './schema';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import type { z } from 'zod';

const name = 'plan-manifest-generation';
const description =
  'Gather insights from knowledgebase and return requirements for Kubernetes/Helm/ACA/Kustomize manifest creation';
const version = '1.0.0';

const MANIFEST_TYPE_TO_TOPIC = {
  kubernetes: TOPICS.GENERATE_K8S_MANIFESTS,
  helm: TOPICS.GENERATE_HELM_CHARTS,
  aca: TOPICS.GENERATE_ACA_MANIFESTS,
  kustomize: TOPICS.GENERATE_K8S_MANIFESTS,
} as const;

async function run(
  input: z.infer<typeof planManifestGenerationSchema>,
  ctx: ToolContext,
): Promise<Result<ManifestPlan>> {
  const {
    sessionId,
    manifestType,
    language: inputLanguage,
    framework: inputFramework,
    environment,
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
    { manifestType, language, framework, environment },
    'Querying knowledgebase for manifest recommendations',
  );

  const topic = MANIFEST_TYPE_TO_TOPIC[manifestType];
  const knowledgeSnippets = await getKnowledgeSnippets(topic, {
    environment: environment || 'production',
    tool: name,
    language,
    ...(framework && { framework }),
    maxChars: 8000,
    maxSnippets: 20,
  });

  const knowledgeMatches: ManifestRequirement[] = knowledgeSnippets.map((snippet) => ({
    id: snippet.id,
    category: snippet.category || 'generic',
    recommendation: snippet.text,
    ...(snippet.tags && { tags: snippet.tags }),
    matchScore: snippet.weight,
  }));

  const securityMatches = knowledgeMatches.filter(
    (m) => m.category === 'security' || m.tags?.includes('security'),
  );
  const resourceMatches = knowledgeMatches.filter(
    (m) =>
      m.tags?.includes('resources') ||
      m.tags?.includes('limits') ||
      m.tags?.includes('requests') ||
      m.tags?.includes('optimization'),
  );
  const bestPracticeMatches = knowledgeMatches.filter(
    (m) => !securityMatches.includes(m) && !resourceMatches.includes(m),
  );

  const confidence =
    knowledgeMatches.length > 0 ? Math.min(0.95, 0.5 + knowledgeMatches.length * 0.05) : 0.5;

  const summary = `
Manifest Planning Summary:
- Manifest Type: ${manifestType}
- Language: ${language}${framework ? ` (${framework})` : ''}
- Environment: ${environment || 'production'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Resource Management: ${resourceMatches.length}
  - Best Practices: ${bestPracticeMatches.length}

Next Step: Use generate-${manifestType === 'kubernetes' ? 'k8s-manifests' : manifestType === 'helm' ? 'helm-charts' : manifestType === 'aca' ? 'aca-manifests' : 'kustomize'} with sessionId to create manifests using these recommendations.
  `.trim();

  const plan: ManifestPlan = {
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
    manifestType,
    recommendations: {
      securityConsiderations: securityMatches,
      resourceManagement: resourceMatches,
      bestPractices: bestPracticeMatches,
    },
    knowledgeMatches,
    confidence,
    summary,
  };

  ctx.logger.info(
    {
      manifestType,
      knowledgeMatchCount: knowledgeMatches.length,
      securityCount: securityMatches.length,
      resourceCount: resourceMatches.length,
      confidence,
    },
    'Manifest planning completed',
  );

  return Success(plan);
}

const tool: MCPTool<typeof planManifestGenerationSchema, ManifestPlan> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: planManifestGenerationSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
