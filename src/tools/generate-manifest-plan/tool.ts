/**
 * Generate Manifest Plan Tool
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
import { generateManifestPlanSchema, type ManifestPlan, type ManifestRequirement } from './schema';
import { getKnowledgeSnippets } from '@/knowledge/matcher';
import type { z } from 'zod';

const name = 'generate-manifest-plan';
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
  input: z.infer<typeof generateManifestPlanSchema>,
  ctx: ToolContext,
): Promise<Result<ManifestPlan>> {
  const { manifestType, environment } = input;

  const path = input.path;

  if (!path) {
    return Failure('Path is required to generate manifest plan.');
  }

  const language = input.language;
  const frameworks = input.frameworks;

  ctx.logger.info(
    { manifestType, language, frameworks, environment },
    'Querying knowledgebase for manifest recommendations',
  );

  const topic = MANIFEST_TYPE_TO_TOPIC[manifestType];
  const knowledgeSnippets = await getKnowledgeSnippets(topic, {
    environment: environment || 'production',
    tool: name,
    ...(language && { language }),
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

  const frameworksStr =
    frameworks && frameworks.length > 0 ? ` (${frameworks.map((f) => f.name).join(', ')})` : '';

  const summary = `
Manifest Planning Summary:
- Manifest Type: ${manifestType}
- Language: ${language || 'not specified'}${frameworksStr}
- Environment: ${environment || 'production'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Resource Management: ${resourceMatches.length}
  - Best Practices: ${bestPracticeMatches.length}

Next Step: Use generate-${manifestType === 'kubernetes' ? 'k8s-manifests' : manifestType === 'helm' ? 'helm-charts' : manifestType === 'aca' ? 'aca-manifests' : 'kustomize'} with sessionId to create manifests using these recommendations.
  `.trim();

  const plan: ManifestPlan = {
    repositoryInfo: {
      name: input.name,
      modulePathAbsoluteUnix: path,
      language,
      languageVersion: input.languageVersion,
      frameworks,
      buildSystem: input.buildSystem,
      dependencies: input.dependencies,
      ports: input.ports,
      entryPoint: input.entryPoint,
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

const tool: MCPTool<typeof generateManifestPlanSchema, ManifestPlan> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateManifestPlanSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
