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
  type RepositoryInfo,
} from './schema';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import yaml from 'js-yaml';
import { extractErrorMessage } from '@/lib/errors';
import type { RegoEvaluator } from '@/config/policy-rego';
import type { Logger } from 'pino';
import { getToolLogger } from '@/lib/tool-helpers';
import {
  validateContentAgainstPolicy,
  type PolicyViolation,
  type PolicyValidationResult,
} from '@/lib/policy-helpers';

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

// Default resource limits for policy validation pseudo-manifests
const DEFAULT_POLICY_VALIDATION_CPU_LIMIT = '500m';
const DEFAULT_POLICY_VALIDATION_MEMORY_LIMIT = '512Mi';

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

/**
 * Convert ManifestPlan to pseudo-YAML text for policy validation
 * This allows policy rules to match against the planned manifest structure
 */
function planToManifestText(plan: ManifestPlan, manifestType: string): string {
  const lines: string[] = [];

  // Extract security and resource management recommendations
  const securityReqs = plan.recommendations.securityConsiderations || [];
  const resourceReqs = plan.recommendations.resourceManagement || [];

  if (manifestType === 'kubernetes') {
    lines.push('apiVersion: apps/v1');
    lines.push('kind: Deployment');
    lines.push('metadata:');
    lines.push(`  name: ${plan.repositoryInfo?.name || 'app'}`);
    lines.push('spec:');
    lines.push('  template:');
    lines.push('    spec:');

    // Check for privileged mode recommendation
    const hasPrivileged = securityReqs.some((r) =>
      r.recommendation.toLowerCase().includes('privileged'),
    );
    if (hasPrivileged) {
      lines.push('      containers:');
      lines.push('      - securityContext:');
      lines.push('          privileged: true');
    }

    // Check for host network
    const hasHostNetwork = securityReqs.some(
      (r) =>
        r.recommendation.toLowerCase().includes('hostnetwork') ||
        r.recommendation.toLowerCase().includes('host network'),
    );
    if (hasHostNetwork) {
      lines.push('      hostNetwork: true');
    }

    // Check for non-root user
    const hasNonRootUser = securityReqs.some(
      (r) =>
        r.recommendation.toLowerCase().includes('non-root') ||
        r.recommendation.toLowerCase().includes('runasnonroot'),
    );
    if (hasNonRootUser) {
      lines.push('      securityContext:');
      lines.push('        runAsNonRoot: true');
    }

    // Check for resource limits
    const hasResourceLimits = resourceReqs.some(
      (r) =>
        r.recommendation.toLowerCase().includes('resource') &&
        r.recommendation.toLowerCase().includes('limit'),
    );
    if (hasResourceLimits) {
      lines.push('      containers:');
      lines.push('      - resources:');
      lines.push('          limits:');
      lines.push(`            cpu: ${DEFAULT_POLICY_VALIDATION_CPU_LIMIT}`);
      lines.push(`            memory: ${DEFAULT_POLICY_VALIDATION_MEMORY_LIMIT}`);
    }
  }

  return lines.join('\n');
}

/**
 * Validate ManifestPlan against Rego policies
 * Uses shared validateContentAgainstPolicy utility
 */
async function validatePlanAgainstPolicy(
  plan: ManifestPlan,
  manifestType: string,
  policyEvaluator: RegoEvaluator,
  logger: Logger,
): Promise<PolicyValidationResult> {
  // Convert plan to manifest text for policy validation
  const manifestText = planToManifestText(plan, manifestType);

  logger.debug({ manifestText }, 'Generated manifest text from plan for policy validation');

  // Use shared validation utility
  return validateContentAgainstPolicy(
    manifestText,
    policyEvaluator,
    logger,
    'manifest plan',
  );
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
        } as RepositoryInfo,
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
  const logger = getToolLogger(ctx, name);

  // If acaManifest is provided, validate it can be parsed
  if (input.acaManifest) {
    try {
      parseAcaManifest(input.acaManifest);
    } catch (error) {
      return Failure(`Invalid ACA manifest: ${extractErrorMessage(error)}`, {
        message: `Invalid ACA manifest: ${extractErrorMessage(error)}`,
        hint: 'The provided Azure Container Apps manifest could not be parsed',
        resolution:
          'Ensure the acaManifest parameter contains valid YAML or JSON content representing an ACA manifest',
      });
    }
  }

  // Run the knowledge-based plan generation
  const result = await runPattern(input, ctx);

  if (!result.ok) return result;

  const plan = result.value;

  // Validate against policy if available
  if (ctx.policy) {
    const policyValidation = await validatePlanAgainstPolicy(
      plan,
      input.manifestType,
      ctx.policy,
      logger,
    );

    plan.policyValidation = policyValidation;

    // Block if there are violations
    if (!policyValidation.passed) {
      const violationMessages = policyValidation.violations
        .map((v: PolicyViolation) => `  - ${v.ruleId}: ${v.message}`)
        .join('\n');

      return Failure(
        `Generated manifest plan violates organizational policies:\n${violationMessages}`,
        {
          message: 'Policy violations detected in manifest plan',
          hint: `${policyValidation.violations.length} blocking policy rule(s) failed`,
          resolution: 'Adjust recommendations or update policy configuration',
        },
      );
    }

    // Log warnings/suggestions even if plan passes
    if (policyValidation.warnings.length > 0) {
      logger.warn(
        { warnings: policyValidation.warnings.map((w: PolicyViolation) => w.ruleId) },
        'Policy warnings in manifest plan',
      );
    }
  }

  return result;
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
  },
  chainHints: {
    success:
      'Manifest plan generated successfully and passed policy validation. Next: Call prepare-cluster to create a kind cluster to deploy to.',
    failure:
      'Manifest generation failed or plan violates policies. Review manifest requirements and policy violations.',
  },
  handler: handleGenerateK8sManifests,
});
