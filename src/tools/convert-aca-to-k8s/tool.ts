/**
 * Convert ACA to K8s Tool - Plan-Based Recommendations
 *
 * Analyzes Azure Container Apps manifest and returns structured recommendations
 * for converting to Kubernetes, including field mappings and best practices.
 *
 * Uses the knowledge-tool-pattern for consistent, deterministic behavior.
 * Returns a plan for the MCP client AI to use when generating the actual K8s manifests.
 *
 * @category azure
 * @version 3.0.0
 * @knowledgeEnhanced true
 * @samplingStrategy none
 */

import { Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { convertAcaToK8sSchema } from './schema';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import yaml from 'js-yaml';

const name = 'convert-aca-to-k8s';
const description =
  'Analyze ACA manifest and return structured recommendations for Kubernetes conversion';
const version = '3.0.0';

// Define result types
export interface AcaToK8sConversionPlan {
  acaAnalysis: {
    containerApps: Array<{
      name: string;
      containers: number;
      hasIngress: boolean;
      hasScaling: boolean;
      hasSecrets: boolean;
    }>;
    warnings: string[];
  };
  recommendations: {
    fieldMappings: ConversionRecommendation[];
    securityConsiderations: ConversionRecommendation[];
    bestPractices: ConversionRecommendation[];
  };
  knowledgeMatches: Array<{
    id: string;
    category: string;
    recommendation: string;
    tags?: string[];
    matchScore: number;
  }>;
  confidence: number;
  summary: string;
}

export interface ConversionRecommendation {
  id: string;
  category: string;
  recommendation: string;
  tags?: string[];
  matchScore: number;
}

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
type ConversionCategory = 'fieldMappings' | 'security' | 'bestPractices';

// Define ACA to K8s conversion input interface
interface AcaConversionInput {
  acaManifest: string;
  namespace?: string;
  includeComments?: boolean;
}

// Create the tool runner using the shared pattern
const runPattern = createKnowledgeTool<
  AcaConversionInput,
  AcaToK8sConversionPlan,
  ConversionCategory,
  Record<string, never> // No additional rules for conversion plan
>({
  name,
  query: {
    topic: TOPICS.CONVERT_ACA_TO_K8S,
    category: CATEGORY.KUBERNETES,
    maxChars: 3000,
    maxSnippets: 15,
    extractFilters: () => ({
      environment: 'production',
    }),
  },
  categorization: {
    categoryNames: ['fieldMappings', 'security', 'bestPractices'] as const,
    categorize: createSimpleCategorizer<ConversionCategory>({
      fieldMappings: (s) =>
        Boolean(
          s.tags?.includes('mapping') ||
            s.tags?.includes('conversion') ||
            s.text.toLowerCase().includes('map') ||
            s.text.toLowerCase().includes('convert'),
        ),
      security: (s) => s.category === 'security' || Boolean(s.tags?.includes('security')),
      bestPractices: (s) =>
        !(
          Boolean(
            s.tags?.includes('mapping') ||
              s.tags?.includes('conversion') ||
              s.text.toLowerCase().includes('map') ||
              s.text.toLowerCase().includes('convert'),
          ) ||
          s.category === 'security' ||
          Boolean(s.tags?.includes('security'))
        ),
    }),
  },
  rules: {
    applyRules: () => ({}), // No additional rules for conversion plan
  },
  plan: {
    buildPlan: (input, knowledge, _rules, confidence) => {
      // Parse the ACA manifest to analyze it
      const parsedManifest = parseAcaManifest(input.acaManifest);
      const analysis = analyzeAcaManifest(parsedManifest);

      // Map knowledge snippets to ConversionRecommendations
      const knowledgeMatches = knowledge.all.map((snippet) => ({
        id: snippet.id,
        category: snippet.category || 'generic',
        recommendation: snippet.text,
        ...(snippet.tags && { tags: snippet.tags }),
        matchScore: snippet.weight,
      }));

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
          // Exclude snippets already in field mappings or security
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
        recommendations: {
          fieldMappings,
          securityConsiderations: securityMatches,
          bestPractices: bestPracticeMatches,
        },
        knowledgeMatches,
        confidence,
        summary,
      };
    },
  },
});

/**
 * Wrapper function to add validation and manifest parsing
 */
async function run(
  input: z.infer<typeof convertAcaToK8sSchema>,
  ctx: ToolContext,
): Promise<Result<AcaToK8sConversionPlan>> {
  const logger = getToolLogger(ctx, name);
  const timer = createToolTimer(logger, name);

  try {
    if (!input.acaManifest) {
      return Failure('ACA manifest is required');
    }

    // Validate manifest can be parsed
    try {
      parseAcaManifest(input.acaManifest);
    } catch (error) {
      return Failure(`Invalid ACA manifest: ${extractErrorMessage(error)}`);
    }

    const result = await runPattern(input, ctx);

    timer.end({ success: result.ok });
    return result;
  } catch (error) {
    timer.error(error);
    return Failure(`Conversion planning failed: ${extractErrorMessage(error)}`);
  }
}

const tool: MCPTool<typeof convertAcaToK8sSchema, AcaToK8sConversionPlan> = {
  name,
  description,
  category: 'azure',
  version,
  schema: convertAcaToK8sSchema,
  metadata: {
    knowledgeEnhanced: true,
    samplingStrategy: 'none',
    enhancementCapabilities: ['recommendations'],
  },
  run,
};

export default tool;
