/**
 * Generate Kubernetes Manifests Tool
 *
 * Analyzes repository and queries knowledgebase to gather insights and return
 * structured requirements for creating Kubernetes/Helm/ACA/Kustomize manifests.
 * This tool helps users understand best practices and recommendations before
 * actual manifest generation.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 *
 * @category kubernetes
 * @version 2.0.0
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import {
  generateK8sManifestsSchema,
  type ManifestPlan,
  type ManifestRequirement,
  type GenerateK8sManifestsParams,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import yaml from 'js-yaml';
import { extractErrorMessage } from '@/lib/error-utils';

const name = 'generate-k8s-manifests';
const description =
  'Gather insights from knowledgebase and return requirements for Kubernetes/Helm/ACA/Kustomize manifest creation. Supports repository analysis or ACA manifest conversion.';
const version = '2.0.0';

// Manifest type to topic mapping
const MANIFEST_TYPE_TO_TOPIC = {
  kubernetes: TOPICS.KUBERNETES,
  helm: TOPICS.GENERATE_HELM_CHARTS,
  aca: TOPICS.KUBERNETES,
  kustomize: TOPICS.KUBERNETES,
} as const;

/**
 * Parse ACA manifest from YAML or JSON string
 */
function parseAcaManifest(manifestStr: string): Record<string, unknown> {
  try {
    // Try YAML first (most common for manifests)
    return yaml.load(manifestStr) as Record<string, unknown>;
  } catch {
    try {
      // Fallback to JSON
      return JSON.parse(manifestStr) as Record<string, unknown>;
    } catch {
      throw new Error('Invalid manifest format: must be valid YAML or JSON');
    }
  }
}

/**
 * Analyze ACA manifest to extract key information
 */
function analyzeAcaManifest(acaManifest: Record<string, unknown>): {
  containerApps: Array<{
    name: string;
    containers: number;
    hasIngress: boolean;
    hasScaling: boolean;
    hasSecrets: boolean;
  }>;
  warnings: string[];
} {
  const warnings: string[] = [];
  const containerApps: Array<{
    name: string;
    containers: number;
    hasIngress: boolean;
    hasScaling: boolean;
    hasSecrets: boolean;
  }> = [];

  // Extract ACA properties
  const properties = (acaManifest.properties || acaManifest) as Record<string, unknown>;
  const configuration = (properties.configuration || {}) as Record<string, unknown>;
  const template = (properties.template || {}) as Record<string, unknown>;
  const containers = (template.containers || []) as Array<Record<string, unknown>>;
  const scale = (template.scale || {}) as Record<string, unknown>;
  const ingress = (configuration.ingress || {}) as Record<string, unknown>;
  const secrets = (configuration.secrets || []) as Array<Record<string, unknown>>;

  const appName = (acaManifest.name as string) || 'aca-app';

  containerApps.push({
    name: appName,
    containers: containers.length,
    hasIngress: Boolean(ingress.external || ingress.targetPort),
    hasScaling: Boolean(scale.minReplicas || scale.maxReplicas),
    hasSecrets: secrets.length > 0,
  });

  if (containers.length === 0) {
    warnings.push('No containers found in ACA manifest');
  }

  if (!ingress.external && !ingress.targetPort) {
    warnings.push('No ingress configuration found - Service may not be created');
  }

  return { containerApps, warnings };
}

// Define category types for better type safety
type ManifestCategory = 'fieldMappings' | 'security' | 'resourceManagement' | 'bestPractices';

// Create the tool runner using the shared pattern
const runPattern = createKnowledgeTool<
  GenerateK8sManifestsParams,
  ManifestPlan,
  ManifestCategory,
  Record<string, never> // No additional rules for manifest plan
>({
  name,
  query: {
    topic: (input) => {
      // Use ACA conversion topic if acaManifest is provided
      if (input.acaManifest) {
        return TOPICS.CONVERT_ACA_TO_K8S;
      }
      return MANIFEST_TYPE_TO_TOPIC[input.manifestType];
    },
    category: CATEGORY.KUBERNETES,
    maxChars: 8000,
    maxSnippets: 20,
    extractFilters: (input) => ({
      environment: input.environment || 'production',
      language: input.language,
      framework: input.frameworks?.[0]?.name, // Use first framework if available
      detectedDependencies: input.detectedDependencies,
    }),
  },
  categorization: {
    categoryNames: ['fieldMappings', 'security', 'resourceManagement', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<ManifestCategory>({
      fieldMappings: (s) =>
        Boolean(
          s.tags?.includes('mapping') ||
            s.tags?.includes('conversion') ||
            s.text.toLowerCase().includes('map') ||
            s.text.toLowerCase().includes('convert'),
        ),
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

      // Handle ACA conversion mode
      if (input.acaManifest) {
        const parsedManifest = parseAcaManifest(input.acaManifest);
        const analysis = analyzeAcaManifest(parsedManifest);

        const fieldMappings = (knowledge.categories.fieldMappings || []).map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'field-mapping',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }));

        const securityMatches = (knowledge.categories.security || []).map((snippet) => ({
          id: snippet.id,
          category: snippet.category || 'security',
          recommendation: snippet.text,
          ...(snippet.tags && { tags: snippet.tags }),
          matchScore: snippet.weight,
        }));

        const bestPracticeMatches = (knowledge.categories.bestPractices || [])
          .filter((snippet) => {
            const isInMappings = (knowledge.categories.fieldMappings || []).some(
              (s) => s.id === snippet.id,
            );
            const isInSecurity = (knowledge.categories.security || []).some(
              (s) => s.id === snippet.id,
            );
            return !isInMappings && !isInSecurity;
          })
          .map((snippet) => ({
            id: snippet.id,
            category: snippet.category || 'best-practice',
            recommendation: snippet.text,
            ...(snippet.tags && { tags: snippet.tags }),
            matchScore: snippet.weight,
          }));

        const summary = `
ACA to K8s Conversion Planning Summary:
- Container Apps: ${analysis.containerApps.length}
- Total Containers: ${analysis.containerApps.reduce((sum, app) => sum + app.containers, 0)}
- Knowledge Matches: ${knowledgeMatches.length} recommendations found
  - Field Mappings: ${fieldMappings.length}
  - Security Considerations: ${securityMatches.length}
  - Best Practices: ${bestPracticeMatches.length}
- Warnings: ${analysis.warnings.length}

Use this plan to guide the conversion from Azure Container Apps to Kubernetes manifests.
        `.trim();

        return {
          acaAnalysis: analysis,
          manifestType: 'kubernetes',
          recommendations: {
            fieldMappings,
            securityConsiderations: securityMatches,
            bestPractices: bestPracticeMatches,
          },
          knowledgeMatches,
          confidence,
          summary,
        };
      }

      // Handle repository-based generation mode
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

Next Step: Use generate-${nextStepTool} to create manifests using these recommendations.
      `.trim();

      return {
        repositoryInfo: {
          name: input.name,
          modulePath: input.modulePath,
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
async function handleGenerateK8sManifests(
  input: z.infer<typeof generateK8sManifestsSchema>,
  ctx: ToolContext,
): Promise<Result<ManifestPlan>> {
  // If acaManifest is provided, validate it can be parsed
  if (input.acaManifest) {
    try {
      parseAcaManifest(input.acaManifest);
    } catch (error) {
      return Failure(`Invalid ACA manifest: ${extractErrorMessage(error)}`);
    }
  }

  return runPattern(input, ctx);
}

import { tool } from '@/types/tool';

export default tool({
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateK8sManifestsSchema,
  metadata: {
    knowledgeEnhanced: true,
    enhancementCapabilities: ['recommendations'],
  },
  handler: handleGenerateK8sManifests,
});
