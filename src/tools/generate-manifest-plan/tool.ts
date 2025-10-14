/**
 * Generate Manifest Plan Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating Kubernetes/Helm/ACA/Kustomize manifests.
 * This tool helps users understand best practices and recommendations before
 * actual manifest generation.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 *
 * @category kubernetes
 * @version 1.0.0
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import {
  generateManifestPlanSchema,
  type ManifestPlan,
  type ManifestRequirement,
  type GenerateManifestPlanParams,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';

const name = 'generate-manifest-plan';
const description =
  'Gather insights from knowledgebase and return requirements for Kubernetes/Helm/ACA/Kustomize manifest creation';
const version = '1.0.0';

// Manifest type to topic mapping
const MANIFEST_TYPE_TO_TOPIC = {
  kubernetes: TOPICS.KUBERNETES,
  helm: TOPICS.GENERATE_HELM_CHARTS,
  aca: TOPICS.KUBERNETES,
  kustomize: TOPICS.KUBERNETES,
} as const;

// Define category types for better type safety
type ManifestCategory = 'security' | 'resourceManagement' | 'bestPractices';

// Create the tool runner using the shared pattern
const runPattern = createKnowledgeTool<
  GenerateManifestPlanParams,
  ManifestPlan,
  ManifestCategory,
  Record<string, never> // No additional rules for manifest plan
>({
  name,
  query: {
    topic: (input) => MANIFEST_TYPE_TO_TOPIC[input.manifestType],
    category: CATEGORY.KUBERNETES,
    maxChars: 8000,
    maxSnippets: 20,
    extractFilters: (input) => ({
      environment: input.environment || 'production',
      language: input.language,
      framework: input.frameworks?.[0]?.name, // Use first framework if available
    }),
  },
  categorization: {
    categoryNames: ['security', 'resourceManagement', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<ManifestCategory>({
      security: (s) => s.category === 'security' || Boolean(s.tags?.includes('security')),
      resourceManagement: (s) =>
        Boolean(
          s.tags?.includes('resources') ||
            s.tags?.includes('limits') ||
            s.tags?.includes('requests') ||
            s.tags?.includes('optimization'),
        ),
      bestPractices: () => true, // Catch remaining snippets as best practices
    }),
  },
  rules: {
    applyRules: () => ({}), // No additional rules for manifest plan
  },
  plan: {
    buildPlan: (input, knowledge, _rules, confidence) => {
      // Map knowledge snippets to ManifestRequirements
      const knowledgeMatches: ManifestRequirement[] = knowledge.all.map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      const securityMatches: ManifestRequirement[] = (knowledge.categories.security || []).map(
        (snippet) => ({
          id: snippet.id,
          category: snippet.category || 'security',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }),
      );

      const resourceMatches: ManifestRequirement[] = (
        knowledge.categories.resourceManagement || []
      ).map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

      const bestPracticeMatches: ManifestRequirement[] = (knowledge.categories.bestPractices || [])
        .filter((snippet) => {
          // Exclude snippets already in security or resource management
          const isInSecurity = (knowledge.categories.security || []).some(
            (s) => s.id === snippet.id,
          );
          const isInResource = (knowledge.categories.resourceManagement || []).some(
            (s) => s.id === snippet.id,
          );
          return !isInSecurity && !isInResource;
        })
        .map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'generic',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }));

      const frameworksStr =
        input.frameworks && input.frameworks.length > 0
          ? ` (${input.frameworks.map((f) => f.name).join(', ')})`
          : '';

      const nextStepTool = {
        kubernetes: 'k8s-manifests',
        helm: 'helm-charts',
        aca: 'aca-manifests',
        kustomize: 'kustomize',
      }[input.manifestType];

      const summary = `
Manifest Planning Summary:
- Manifest Type: ${input.manifestType}
- Language: ${input.language || 'not specified'}${frameworksStr}
- Environment: ${input.environment || 'production'}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Security: ${securityMatches.length}
  - Resource Management: ${resourceMatches.length}
  - Best Practices: ${bestPracticeMatches.length}

Next Step: Use generate-${nextStepTool} with sessionId to create manifests using these recommendations.
      `.trim();

      return {
        repositoryInfo: {
          name: input.name,
          modulePathAbsoluteUnix: input.path,
          language: input.language,
          languageVersion: input.languageVersion,
          frameworks: input.frameworks,
          buildSystem: input.buildSystem,
          dependencies: input.dependencies,
          ports: input.ports,
          entryPoint: input.entryPoint,
        },
        manifestType: input.manifestType,
        recommendations: {
          securityConsiderations: securityMatches,
          resourceManagement: resourceMatches,
          bestPractices: bestPracticeMatches,
        },
        knowledgeMatches,
        confidence,
        summary,
      };
    },
  },
});

// Wrapper function to add validation
async function run(
  input: z.infer<typeof generateManifestPlanSchema>,
  ctx: ToolContext,
): Promise<Result<ManifestPlan>> {
  const path = input.path;

  if (!path) {
    return Failure('Path is required to generate manifest plan.');
  }

  return runPattern(input, ctx);
}

const tool: MCPTool<typeof generateManifestPlanSchema, ManifestPlan> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateManifestPlanSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
