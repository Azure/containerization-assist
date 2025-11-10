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
import type { ToolNextAction } from '../shared/schemas';
import { CATEGORY } from '@/knowledge/types';
import { createKnowledgeTool, createSimpleCategorizer } from '../shared/knowledge-tool-pattern';
import type { z } from 'zod';
import yaml from 'js-yaml';
import { extractErrorMessage } from '@/lib/errors';
import { pluralize } from '@/lib/summary-helpers';
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

/**
 * Extended input parameters that include optional policy configuration.
 * This is used internally to pass policy configuration from the handler to buildPlan.
 */
interface ExtendedK8sManifestParams extends GenerateK8sManifestsParams {
  k8sConfig?: import('@/config/policy-generation-config').K8sGenerationConfig;
}

// Manifest type to topic mapping
const MANIFEST_TYPE_TO_TOPIC = {
  kubernetes: TOPICS.KUBERNETES,
  helm: TOPICS.GENERATE_HELM_CHARTS,
  aca: TOPICS.KUBERNETES,
  kustomize: TOPICS.KUBERNETES,
} as const;

/**
 * Default resource limits for policy validation pseudo-manifests
 *
 * These values are used when converting ManifestPlan to YAML for policy validation.
 * They represent reasonable defaults for a typical microservice application:
 * - CPU: 500m (0.5 cores) - Sufficient for most API services under moderate load
 * - Memory: 512Mi - Adequate for typical application runtime and data structures
 *
 * These are NOT production recommendations - actual resource limits should be
 * determined through profiling and load testing. These values are only used
 * to ensure policy rules that check for resource limit presence can evaluate
 * the pseudo-manifest structure.
 */
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
  ExtendedK8sManifestParams,
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

        // Determine manifest files for ACA conversion
        const manifestFiles: Array<{ path: string; purpose: string }> = [
          { path: './k8s/deployment.yaml', purpose: 'Application deployment' },
        ];

        // Add service if ingress is configured
        const hasIngress = analysis.containerApps.some((app) => app.hasIngress);
        if (hasIngress) {
          manifestFiles.push({ path: './k8s/service.yaml', purpose: 'Service exposure' });
        }

        // Add configmap/secrets if configured
        const hasSecrets = analysis.containerApps.some((app) => app.hasSecrets);
        if (hasSecrets) {
          manifestFiles.push({ path: './k8s/secret.yaml', purpose: 'Secret management' });
        }

        const nextAction: ToolNextAction = {
          action: 'create-files',
          instruction: `Create Kubernetes manifests in ./k8s directory by converting the ACA manifest using field mappings from recommendations.fieldMappings. Apply security considerations from recommendations.securityConsiderations and best practices from recommendations.bestPractices. Reference the acaAnalysis for container app structure.`,
          files: manifestFiles,
        };

        const totalContainers = analysis.containerApps.reduce((sum, app) => sum + app.containers, 0);
        const summary =
          `🔨 ACTION REQUIRED: Convert ACA manifest to Kubernetes\n` +
          `Container Apps: ${pluralize(analysis.containerApps.length, 'app')} (${pluralize(totalContainers, 'container')})\n` +
          `Manifests: ${manifestFiles.map((f) => f.path.split('/').pop()).join(', ')}\n` +
          `Field Mappings: ${fieldMappings.length} items\n` +
          `Recommendations: ${knowledgeMatches.length} total (${fieldMappings.length} mappings, ${securityMatches.length} security, ${bestPracticeMatches.length} best practices)\n\n` +
          `✅ Ready to create Kubernetes manifests from ACA config.`;

        return {
          nextAction,
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

      // Determine manifest files for repository mode
      const manifestFiles: Array<{ path: string; purpose: string }> = [
        { path: './k8s/deployment.yaml', purpose: 'Application deployment' },
        { path: './k8s/service.yaml', purpose: 'Service exposure' },
      ];

      // Add configmap if there are ports or environment variables
      if (input.ports && input.ports.length > 0) {
        manifestFiles.push({ path: './k8s/configmap.yaml', purpose: 'Configuration management' });
      }

      // Build policy config instruction if available
      const policyInstruction = input.k8sConfig
        ? ` Apply policy-driven configuration: ${
            input.k8sConfig.resourceDefaults
              ? `resource limits (CPU: ${input.k8sConfig.resourceDefaults.cpuLimit}, Memory: ${input.k8sConfig.resourceDefaults.memoryLimit}), `
              : ''
          }${input.k8sConfig.replicas ? `replicas: ${input.k8sConfig.replicas}, ` : ''}${
            input.k8sConfig.orgStandards?.namespace
              ? `namespace: ${input.k8sConfig.orgStandards.namespace}, `
              : ''
          }${
            input.k8sConfig.orgStandards?.requiredLabels
              ? `labels: ${Object.keys(input.k8sConfig.orgStandards.requiredLabels).join(', ')}.`
              : ''
          }`
        : '';

      const nextAction: ToolNextAction = {
        action: 'create-files',
        instruction: `Create ${input.manifestType} manifests in ./k8s directory for ${input.name}. Use security considerations from recommendations.securityConsiderations, resource management from recommendations.resourceManagement, and best practices from recommendations.bestPractices. Reference repositoryInfo for application details like language, ports, and dependencies.${policyInstruction}`,
        files: manifestFiles,
      };

      const frameworksStr =
        input.frameworks && input.frameworks.length > 0
          ? ` (${input.frameworks.map((f) => f.name).join(', ')})`
          : '';

      const policyConfigInfo = input.k8sConfig
        ? `Policy Config: ${input.k8sConfig.replicas || 1} replicas, ${
            input.k8sConfig.orgStandards?.namespace || 'default'
          } namespace\n`
        : '';

      const summary =
        `🔨 ACTION REQUIRED: Create ${input.manifestType} manifests\n` +
        `Application: ${input.name || input.language || 'application'}${frameworksStr}\n` +
        `Manifests: ${manifestFiles.map((f) => f.path.split('/').pop()).join(', ')}\n${
        policyConfigInfo
        }Recommendations: ${knowledgeMatches.length} total (${securityMatches.length} security, ${resourceMatches.length} resources, ${bestPracticeMatches.length} best practices)\n\n` +
        `✅ Ready to create manifests in ./k8s directory.`;

      return {
        nextAction,
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

  // Query policy for generation configuration (if policy is available)
  let k8sConfig: import('@/config/policy-generation-config').K8sGenerationConfig | null = null;
  if (ctx.policy) {
    const configQuery = await ctx.queryConfig<{ kubernetes?: import('@/config/policy-generation-config').K8sGenerationConfig }>(
      'containerization.generation_config',
      {
        language: input.language || 'auto-detect',
        framework: input.frameworks?.[0]?.name,
        environment: input.environment || 'production',
        appName: input.name || 'app',
      },
    );

    k8sConfig = configQuery?.kubernetes || null;

    if (k8sConfig) {
      logger.info(
        {
          resourceDefaults: k8sConfig.resourceDefaults,
          replicas: k8sConfig.replicas,
          features: k8sConfig.features,
          orgStandards: k8sConfig.orgStandards,
        },
        'Loaded Kubernetes generation config from policy',
      );
    }
  }

  // Add K8s config to input
  const extendedInput = {
    ...input,
    ...(k8sConfig && { k8sConfig }),
  };

  // Run the knowledge-based plan generation
  const result = await runPattern(extendedInput, ctx);

  if (!result.ok) return result;

  const plan = result.value;

  // Query policy for template additions and dynamic defaults (Sprint 3)
  if (ctx.policy) {
    // Query for template additions
    const templateQuery = await ctx.queryConfig<import('@/config/policy-generation-config').TemplateAdditions>(
      'containerization.templates.templates',
      {
        language: input.language || 'auto-detect',
        framework: input.frameworks?.[0]?.name,
        environment: input.environment || 'production',
        appName: input.name || 'app',
      },
    );

    if (templateQuery) {
      logger.info(
        {
          k8sTemplates: templateQuery.kubernetes?.length || 0,
        },
        'Loaded template additions from policy',
      );

      // Merge templates into plan using template merger
      const { mergeTemplatesIntoPlan } = await import('@/lib/template-merger');
      const updatedPlan = mergeTemplatesIntoPlan(
        plan,
        templateQuery,
        {
          language: input.language,
          environment: input.environment,
          framework: input.frameworks?.[0]?.name,
        },
      );
      Object.assign(plan, updatedPlan);
    }

    // Query for dynamic defaults (replicas, health checks, HPA)
    const dynamicDefaultsQuery = await ctx.queryConfig<import('@/config/policy-generation-config').DynamicDefaults>(
      'containerization.dynamic_defaults.defaults',
      {
        language: input.language || 'auto-detect',
        environment: input.environment || 'production',
        trafficLevel: input.trafficLevel,
        criticalityTier: input.criticalityTier,
      },
    );

    if (dynamicDefaultsQuery) {
      logger.info(
        {
          replicas: dynamicDefaultsQuery.replicas,
          hasHealthChecks: !!dynamicDefaultsQuery.healthChecks,
          hasAutoscaling: !!dynamicDefaultsQuery.autoscaling,
        },
        'Loaded dynamic defaults from policy',
      );

      // Add dynamic defaults as policy-driven recommendations
      if (dynamicDefaultsQuery.replicas) {
        const replicaInfo = {
          id: 'policy-replica-count',
          category: 'resource-management',
          recommendation: `Replica count (from policy): ${dynamicDefaultsQuery.replicas}`,
          tags: ['policy-driven', 'replicas'],
          matchScore: 100,
          policyDriven: true,
        };
        plan.recommendations.resourceManagement = [replicaInfo, ...(plan.recommendations.resourceManagement || [])];
      }

      if (dynamicDefaultsQuery.healthChecks) {
        const healthCheckInfo = {
          id: 'policy-health-check-config',
          category: 'best-practice',
          recommendation: `Health check configuration (from policy): initialDelay=${dynamicDefaultsQuery.healthChecks.initialDelaySeconds}s, period=${dynamicDefaultsQuery.healthChecks.periodSeconds}s, timeout=${dynamicDefaultsQuery.healthChecks.timeoutSeconds}s, failureThreshold=${dynamicDefaultsQuery.healthChecks.failureThreshold}`,
          tags: ['policy-driven', 'health-check'],
          matchScore: 100,
          policyDriven: true,
        };
        plan.recommendations.bestPractices = [healthCheckInfo, ...plan.recommendations.bestPractices];
      }

      if (dynamicDefaultsQuery.autoscaling) {
        const hpaInfo = {
          id: 'policy-hpa-config',
          category: 'resource-management',
          recommendation: `HPA configuration (from policy): min=${dynamicDefaultsQuery.autoscaling.minReplicas}, max=${dynamicDefaultsQuery.autoscaling.maxReplicas}, targetCPU=${dynamicDefaultsQuery.autoscaling.targetCPUUtilization || 'N/A'}%, targetMemory=${dynamicDefaultsQuery.autoscaling.targetMemoryUtilization || 'N/A'}%`,
          tags: ['policy-driven', 'autoscaling', 'hpa'],
          matchScore: 100,
          policyDriven: true,
        };
        plan.recommendations.resourceManagement = [...(plan.recommendations.resourceManagement || []), hpaInfo];
      }
    }
  }

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
