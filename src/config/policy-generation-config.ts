/**
 * Policy Generation Configuration Types and Schemas
 *
 * Defines TypeScript interfaces and Zod schemas for policy-driven
 * generation configuration. These types are used by tools to query
 * policies for pre-generation defaults and organizational standards.
 */

import { z } from 'zod';

// ===== DOCKERFILE GENERATION CONFIG =====

/**
 * Dockerfile generation configuration from policy
 */
export const dockerfileGenerationConfigSchema = z.object({
  /**
   * Build strategy override (multi-stage, single-stage, distroless)
   */
  buildStrategy: z.enum(['multi-stage', 'single-stage', 'distroless']).optional(),

  /**
   * Base image category preference (official, distroless, alpine, minimal)
   */
  baseImageCategory: z.enum(['official', 'distroless', 'alpine', 'minimal']).optional(),

  /**
   * Optimization priority (speed, size, security)
   */
  optimizationPriority: z.enum(['speed', 'size', 'security', 'balanced']).optional(),

  /**
   * Security features to enable
   */
  securityFeatures: z.object({
    nonRootUser: z.boolean().optional(),
    readOnlyRootFS: z.boolean().optional(),
    noNewPrivileges: z.boolean().optional(),
    dropCapabilities: z.boolean().optional(),
  }).optional(),

  /**
   * Build features to enable
   */
  buildFeatures: z.object({
    buildCache: z.boolean().optional(),
    layerOptimization: z.boolean().optional(),
    healthcheck: z.boolean().optional(),
  }).optional(),
}).strict();

export type DockerfileGenerationConfig = z.infer<typeof dockerfileGenerationConfigSchema>;

// ===== KUBERNETES MANIFEST GENERATION CONFIG =====

/**
 * Resource defaults for different languages and environments
 */
export const resourceDefaultsSchema = z.object({
  /**
   * CPU request (e.g., "100m", "500m", "1")
   */
  cpuRequest: z.string().optional(),

  /**
   * CPU limit (e.g., "500m", "1", "2")
   */
  cpuLimit: z.string().optional(),

  /**
   * Memory request (e.g., "128Mi", "256Mi", "512Mi")
   */
  memoryRequest: z.string().optional(),

  /**
   * Memory limit (e.g., "256Mi", "512Mi", "1Gi")
   */
  memoryLimit: z.string().optional(),
}).strict();

export type ResourceDefaults = z.infer<typeof resourceDefaultsSchema>;

/**
 * Organizational standards for Kubernetes resources
 */
export const orgStandardsSchema = z.object({
  /**
   * Required labels to apply to all resources
   */
  requiredLabels: z.record(z.string()).optional(),

  /**
   * Namespace naming pattern or override
   */
  namespace: z.string().optional(),

  /**
   * Allowed container registries by environment
   */
  allowedRegistries: z.array(z.string()).optional(),

  /**
   * Service account name to use
   */
  serviceAccount: z.string().optional(),

  /**
   * Image pull policy (Always, IfNotPresent, Never)
   */
  imagePullPolicy: z.enum(['Always', 'IfNotPresent', 'Never']).optional(),
}).strict();

export type OrgStandards = z.infer<typeof orgStandardsSchema>;

/**
 * Feature toggles for Kubernetes manifest generation
 */
export const featureTogglesSchema = z.object({
  /**
   * Enable health checks (liveness/readiness probes)
   */
  healthChecks: z.boolean().optional(),

  /**
   * Enable horizontal pod autoscaling
   */
  autoscaling: z.boolean().optional(),

  /**
   * Enable resource quotas
   */
  resourceQuotas: z.boolean().optional(),

  /**
   * Enable network policies
   */
  networkPolicies: z.boolean().optional(),

  /**
   * Enable pod security policies
   */
  podSecurityPolicies: z.boolean().optional(),

  /**
   * Enable ingress
   */
  ingress: z.boolean().optional(),
}).strict();

export type FeatureToggles = z.infer<typeof featureTogglesSchema>;

/**
 * Kubernetes manifest generation configuration from policy
 */
export const k8sGenerationConfigSchema = z.object({
  /**
   * Resource defaults based on language and environment
   */
  resourceDefaults: resourceDefaultsSchema.optional(),

  /**
   * Organizational standards to apply
   */
  orgStandards: orgStandardsSchema.optional(),

  /**
   * Feature toggles for manifest generation
   */
  features: featureTogglesSchema.optional(),

  /**
   * Replica count default
   */
  replicas: z.number().int().positive().optional(),

  /**
   * Deployment strategy (RollingUpdate, Recreate)
   */
  deploymentStrategy: z.enum(['RollingUpdate', 'Recreate']).optional(),
}).strict();

export type K8sGenerationConfig = z.infer<typeof k8sGenerationConfigSchema>;

// ===== TEMPLATE INJECTION TYPES =====

/**
 * Dockerfile template snippet
 * Represents a snippet of Dockerfile code to be injected into generated Dockerfiles
 */
export const dockerfileTemplateSnippetSchema = z.object({
  /**
   * Unique identifier for this template
   */
  id: z.string(),

  /**
   * Section where this template should be injected
   * - setup: Early in the build process (after FROM, before main build steps)
   * - security: Security-related configurations (CA certs, user setup)
   * - observability: Observability agents and instrumentation
   * - runtime: Runtime configurations (before CMD/ENTRYPOINT)
   */
  section: z.enum(['setup', 'security', 'observability', 'runtime']),

  /**
   * The Dockerfile code snippet to inject
   */
  content: z.string(),

  /**
   * Human-readable description of what this template does
   */
  description: z.string(),

  /**
   * Priority for ordering within the same section (higher = earlier)
   */
  priority: z.number().int().optional(),

  /**
   * Conditions under which this template should be applied
   */
  conditions: z.object({
    languages: z.array(z.string()).optional(),
    environments: z.array(z.string()).optional(),
    frameworks: z.array(z.string()).optional(),
  }).optional(),
}).strict();

export type DockerfileTemplateSnippet = z.infer<typeof dockerfileTemplateSnippetSchema>;

/**
 * Kubernetes template component
 * Represents a component to be injected into Kubernetes manifests
 */
export const k8sTemplateComponentSchema = z.object({
  /**
   * Unique identifier for this template
   */
  id: z.string(),

  /**
   * Component type
   */
  type: z.enum(['sidecar', 'initContainer', 'volume', 'volumeMount', 'envVar', 'configMap', 'secret']),

  /**
   * Human-readable description of what this component does
   */
  description: z.string(),

  /**
   * The component specification (varies by type)
   */
  spec: z.record(z.unknown()),

  /**
   * Priority for ordering (higher = higher priority)
   */
  priority: z.number().int().optional(),

  /**
   * Conditions under which this template should be applied
   */
  conditions: z.object({
    languages: z.array(z.string()).optional(),
    environments: z.array(z.string()).optional(),
    frameworks: z.array(z.string()).optional(),
  }).optional(),
}).strict();

export type K8sTemplateComponent = z.infer<typeof k8sTemplateComponentSchema>;

/**
 * Template additions returned from policy
 */
export const templateAdditionsSchema = z.object({
  /**
   * Dockerfile template snippets
   */
  dockerfile: z.array(dockerfileTemplateSnippetSchema).optional(),

  /**
   * Kubernetes template components
   */
  kubernetes: z.array(k8sTemplateComponentSchema).optional(),
}).strict();

export type TemplateAdditions = z.infer<typeof templateAdditionsSchema>;

// ===== DYNAMIC DEFAULTS TYPES =====

/**
 * Health check configuration calculated from context
 */
export const healthCheckConfigSchema = z.object({
  /**
   * Initial delay before first check (seconds)
   */
  initialDelaySeconds: z.number().int().positive(),

  /**
   * Interval between checks (seconds)
   */
  periodSeconds: z.number().int().positive(),

  /**
   * Timeout for each check (seconds)
   */
  timeoutSeconds: z.number().int().positive(),

  /**
   * Failure threshold before marking unhealthy
   */
  failureThreshold: z.number().int().positive(),

  /**
   * Success threshold before marking healthy
   */
  successThreshold: z.number().int().positive(),
}).strict();

export type HealthCheckConfig = z.infer<typeof healthCheckConfigSchema>;

/**
 * HPA (Horizontal Pod Autoscaler) configuration
 */
export const hpaConfigSchema = z.object({
  /**
   * Minimum number of replicas
   */
  minReplicas: z.number().int().positive(),

  /**
   * Maximum number of replicas
   */
  maxReplicas: z.number().int().positive(),

  /**
   * Target CPU utilization percentage
   */
  targetCPUUtilization: z.number().int().min(1).max(100).optional(),

  /**
   * Target memory utilization percentage
   */
  targetMemoryUtilization: z.number().int().min(1).max(100).optional(),
}).strict();

export type HPAConfig = z.infer<typeof hpaConfigSchema>;

/**
 * Dynamic defaults calculated from context
 */
export const dynamicDefaultsSchema = z.object({
  /**
   * Replica count based on environment and criticality
   */
  replicas: z.number().int().positive().optional(),

  /**
   * Health check configuration based on language and startup characteristics
   */
  healthChecks: healthCheckConfigSchema.optional(),

  /**
   * HPA configuration based on workload characteristics
   */
  autoscaling: hpaConfigSchema.optional(),
}).strict();

export type DynamicDefaults = z.infer<typeof dynamicDefaultsSchema>;

// ===== COMPLETE GENERATION CONFIG =====

/**
 * Complete generation configuration from policy
 * Combines Dockerfile and Kubernetes manifest configuration, templates, and dynamic defaults
 */
export const generationConfigSchema = z.object({
  /**
   * Dockerfile generation configuration
   */
  dockerfile: dockerfileGenerationConfigSchema.optional(),

  /**
   * Kubernetes manifest generation configuration
   */
  kubernetes: k8sGenerationConfigSchema.optional(),

  /**
   * Template additions to inject
   */
  templates: templateAdditionsSchema.optional(),

  /**
   * Dynamic defaults calculated from context
   */
  dynamicDefaults: dynamicDefaultsSchema.optional(),
}).strict();

export type GenerationConfig = z.infer<typeof generationConfigSchema>;

// ===== QUERY INPUT TYPES =====

/**
 * Input for querying generation configuration
 */
export const configQueryInputSchema = z.object({
  /**
   * Detected language (node, python, go, java, etc.)
   */
  language: z.string().optional(),

  /**
   * Detected framework (express, fastapi, gin, spring, etc.)
   */
  framework: z.string().optional(),

  /**
   * Environment (dev, staging, prod)
   */
  environment: z.string().optional(),

  /**
   * Application name
   */
  appName: z.string().optional(),

  /**
   * Additional context
   */
  context: z.record(z.unknown()).optional(),
}).strict();

export type ConfigQueryInput = z.infer<typeof configQueryInputSchema>;
