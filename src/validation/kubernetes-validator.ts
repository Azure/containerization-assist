/**
 * Kubernetes validation using YAML parser (Functional)
 *
 * Invariant: All validation rules maintain backwards compatibility
 * Trade-off: Runtime YAML parsing cost over build-time validation for flexibility
 */

import { parse as parseYaml } from 'yaml';
import { extractErrorMessage } from '@/lib/error-utils';
import {
  KubernetesValidationRule,
  KubernetesManifest,
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
} from './core-types';
import { AIValidator } from './ai-validator';
import type { ToolContext } from '@/mcp/context';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';

// Type definitions for Kubernetes resources
interface PodSpec {
  containers?: Container[];
  initContainers?: Container[];
  volumes?: Volume[];
  securityContext?: SecurityContext;
  hostNetwork?: boolean;
}

interface Container {
  name?: string;
  resources?: {
    limits?: Record<string, string>;
    requests?: Record<string, string>;
  };
  securityContext?: SecurityContext;
  image?: string;
  imagePullPolicy?: string;
  livenessProbe?: Probe;
  readinessProbe?: Probe;
}

interface SecurityContext {
  runAsNonRoot?: boolean;
  readOnlyRootFilesystem?: boolean;
  capabilities?: {
    drop?: string[];
  };
  privileged?: boolean;
  runAsUser?: number;
  fsGroup?: number;
}

interface Volume {
  emptyDir?: Record<string, unknown>;
  [key: string]: unknown;
}

interface Probe {
  httpGet?: unknown;
  tcpSocket?: unknown;
  exec?: unknown;
}

interface WorkloadSpec {
  template?: {
    spec?: PodSpec;
  };
  jobTemplate?: {
    spec?: {
      template?: {
        spec?: PodSpec;
      };
    };
  };
  strategy?: {
    type?: string;
  };
}

export interface KubernetesValidatorInstance {
  validate(yamlContent: string): ValidationReport;
  getRules(): KubernetesValidationRule[];
  getCategory(category: ValidationCategory): KubernetesValidationRule[];
}

/**
 * Check if manifest is a workload type
 */
const isWorkload = (manifest: KubernetesManifest): boolean => {
  const workloadKinds = ['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'Pod'];
  return manifest.kind ? workloadKinds.includes(manifest.kind) : false;
};

/**
 * Get pod spec from different workload types
 */
const getPodSpec = (manifest: KubernetesManifest): PodSpec | undefined => {
  if (manifest.kind === 'Pod') {
    return manifest.spec as PodSpec;
  }
  if (manifest.kind === 'Job' || manifest.kind === 'CronJob') {
    const spec = manifest.spec as WorkloadSpec;
    return spec?.jobTemplate?.spec?.template?.spec || spec?.template?.spec;
  }
  return (manifest.spec as WorkloadSpec)?.template?.spec;
};

/**
 * Get all containers from manifest
 */
const getContainers = (manifest: KubernetesManifest): Container[] => {
  const podSpec = getPodSpec(manifest);
  return [...(podSpec?.containers || []), ...(podSpec?.initContainers || [])];
};

/**
 * Kubernetes validation rules
 */
const KUBERNETES_RULES: KubernetesValidationRule[] = [
  {
    id: 'has-resource-limits',
    name: 'Resource limits defined',
    description: 'Containers must have CPU and memory limits defined',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      return containers.every((container) => {
        const resources = container.resources;
        return resources?.limits?.cpu && resources?.limits?.memory;
      });
    },
    message: 'Define CPU and memory limits for all containers',
    severity: ValidationSeverity.ERROR,
    fix: 'Add resources.limits.cpu and resources.limits.memory',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'has-resource-requests',
    name: 'Resource requests defined',
    description: 'Containers should have resource requests for proper scheduling',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      return containers.every((container) => {
        const resources = container.resources;
        return resources?.requests?.cpu && resources?.requests?.memory;
      });
    },
    message: 'Define CPU and memory requests for proper scheduling',
    severity: ValidationSeverity.WARNING,
    fix: 'Add resources.requests.cpu and resources.requests.memory',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'has-readiness-probe',
    name: 'Readiness probe configured',
    description: 'Containers should have readiness probes for traffic management',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      return containers.every((container) => container.readinessProbe);
    },
    message: 'Add readiness probe for traffic management',
    severity: ValidationSeverity.WARNING,
    fix: 'Add readinessProbe with httpGet, tcpSocket, or exec',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'has-liveness-probe',
    name: 'Liveness probe configured',
    description: 'Containers should have liveness probes for auto-restart',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      // Liveness probes prevent zombie containers in production workloads
      return containers.some((container) => container.livenessProbe);
    },
    message: 'Consider adding liveness probe for auto-restart',
    severity: ValidationSeverity.INFO,
    fix: 'Add livenessProbe with httpGet, tcpSocket, or exec',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'no-privileged-containers',
    name: 'No privileged containers',
    description: 'Containers should not run in privileged mode',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      return containers.every((container) => {
        const securityContext = container.securityContext;
        return !securityContext?.privileged;
      });
    },
    message: 'Containers should not run in privileged mode',
    severity: ValidationSeverity.ERROR,
    fix: 'Remove privileged: true or set to false',
    category: ValidationCategory.SECURITY,
  },

  {
    id: 'security-context-defined',
    name: 'Security context configured',
    description: 'Pods should define appropriate security context',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const podSpec = getPodSpec(manifest);
      const securityContext = podSpec?.securityContext;

      return !!(
        securityContext?.runAsNonRoot ||
        securityContext?.runAsUser ||
        securityContext?.fsGroup
      );
    },
    message: 'Define pod security context for better security',
    severity: ValidationSeverity.WARNING,
    fix: 'Add securityContext with runAsNonRoot: true and runAsUser',
    category: ValidationCategory.SECURITY,
  },

  {
    id: 'has-labels',
    name: 'Proper labeling',
    description: 'Resources should have meaningful labels',
    check: (manifest: KubernetesManifest) => {
      const labels = manifest.metadata?.labels;
      const hasAppLabel = labels?.app || labels?.['app.kubernetes.io/name'];
      return !!hasAppLabel;
    },
    message: 'Add meaningful labels for resource organization',
    severity: ValidationSeverity.INFO,
    fix: 'Add labels like app, version, component under metadata.labels',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'image-pull-policy',
    name: 'Appropriate image pull policy',
    description: 'Image pull policy should match image tag strategy',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const containers = getContainers(manifest);
      return containers.every((container) => {
        const c = container;
        // :latest tags change content, requiring Always pull policy
        if (c.image?.includes(':latest')) {
          return c.imagePullPolicy === 'Always';
        }
        // Immutable tags don't need Always, reducing network overhead
        return true;
      });
    },
    message: 'Set appropriate imagePullPolicy for image tag strategy',
    severity: ValidationSeverity.INFO,
    fix: 'Use imagePullPolicy: Always for :latest, IfNotPresent for specific tags',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'no-host-network',
    name: 'Avoid host networking',
    description: 'Pods should not use host networking unless necessary',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const podSpec = getPodSpec(manifest);
      return !podSpec?.hostNetwork;
    },
    message: 'Avoid hostNetwork unless absolutely necessary',
    severity: ValidationSeverity.WARNING,
    fix: 'Remove hostNetwork: true or use proper service exposure',
    category: ValidationCategory.SECURITY,
  },

  {
    id: 'no-host-path-volumes',
    name: 'Avoid hostPath volumes',
    description: 'Avoid mounting host directories unless necessary',
    check: (manifest: KubernetesManifest) => {
      if (!isWorkload(manifest)) return true;

      const podSpec = getPodSpec(manifest);
      const volumes = podSpec?.volumes || [];

      return !volumes.some((volume: Record<string, unknown>) => volume.hostPath);
    },
    message: 'Avoid hostPath volumes for better security',
    severity: ValidationSeverity.WARNING,
    fix: 'Use configMaps, secrets, or persistent volumes instead',
    category: ValidationCategory.SECURITY,
  },

  {
    id: 'service-has-selector',
    name: 'Service has proper selector',
    description: 'Services should have selectors to target pods',
    check: (manifest: KubernetesManifest) => {
      if (manifest.kind !== 'Service') return true;

      return !!(manifest.spec?.selector && Object.keys(manifest.spec.selector).length > 0);
    },
    message: 'Service should have selector to target pods',
    severity: ValidationSeverity.ERROR,
    fix: 'Add spec.selector with labels matching your pods',
    category: ValidationCategory.BEST_PRACTICE,
  },

  {
    id: 'deployment-has-strategy',
    name: 'Deployment strategy defined',
    description: 'Deployments should define update strategy',
    check: (manifest: KubernetesManifest) => {
      if (manifest.kind !== 'Deployment') return true;

      return !!(manifest.spec as WorkloadSpec)?.strategy?.type;
    },
    message: 'Define deployment strategy for updates',
    severity: ValidationSeverity.INFO,
    fix: 'Add strategy.type (RollingUpdate or Recreate)',
    category: ValidationCategory.BEST_PRACTICE,
  },
];

/**
 * Parse YAML documents from content
 */
const parseDocuments = (yamlContent: string): KubernetesManifest[] => {
  // YAML allows multiple K8s resources separated by ---
  const parts = yamlContent.split(/^---\s*$/m);
  const documents: KubernetesManifest[] = [];

  for (const part of parts) {
    const trimmed = part.trim();
    if (trimmed) {
      try {
        const doc = parseYaml(trimmed);
        if (doc && typeof doc === 'object') {
          documents.push(doc);
        }
      } catch {
        // Ignore parsing errors for individual documents
      }
    }
  }

  return documents;
};

/**
 * Calculate validation grade from score
 */
const calculateGrade = (score: number): ValidationGrade => {
  if (score >= 90) return 'A';
  if (score >= 80) return 'B';
  if (score >= 70) return 'C';
  if (score >= 60) return 'D';
  return 'F';
};

/**
 * Create validation report from results
 */
const createReport = (results: ValidationResult[]): ValidationReport => {
  const errors = results.filter(
    (r) => !r.passed && r.metadata?.severity === ValidationSeverity.ERROR,
  ).length;
  const warnings = results.filter(
    (r) => !r.passed && r.metadata?.severity === ValidationSeverity.WARNING,
  ).length;
  const info = results.filter(
    (r) => !r.passed && r.metadata?.severity === ValidationSeverity.INFO,
  ).length;
  const passed = results.filter((r) => r.passed).length;
  const total = results.length;

  // Weighted scoring prioritizes security and correctness over style
  const score = Math.max(0, 100 - errors * 15 - warnings * 5 - info * 2);
  const grade = calculateGrade(score);

  return {
    results,
    score,
    grade,
    passed,
    failed: total - passed,
    errors,
    warnings,
    info,
    timestamp: new Date().toISOString(),
  };
};

/**
 * Validate Kubernetes YAML content
 */
export const validateKubernetesContent = (yamlContent: string): ValidationReport => {
  try {
    try {
      parseYaml(yamlContent);
    } catch (parseError) {
      return {
        results: [
          {
            ruleId: 'parse-error',
            isValid: false,
            passed: false,
            errors: [`Failed to parse YAML: ${extractErrorMessage(parseError)}`],
            warnings: [],
            message: `Failed to parse YAML: ${extractErrorMessage(parseError)}`,
            metadata: {
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
        score: 0,
        grade: 'F',
        passed: 0,
        failed: 1,
        errors: 1,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };
    }

    // Extract all K8s resources from potentially multi-document YAML
    const documents = parseDocuments(yamlContent);

    if (documents.length === 0) {
      return {
        results: [
          {
            ruleId: 'no-documents',
            isValid: false,
            passed: false,
            errors: ['No valid Kubernetes documents found'],
            warnings: [],
            message: 'No valid Kubernetes documents found',
            metadata: {
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
        score: 0,
        grade: 'F',
        passed: 0,
        failed: 1,
        errors: 1,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };
    }

    const allResults: ValidationResult[] = [];
    let validDocumentCount = 0;

    for (const doc of documents) {
      if (!doc.apiVersion || !doc.kind) continue;

      validDocumentCount++;

      for (const rule of KUBERNETES_RULES) {
        const passed = rule.check(doc);
        const resourceName = doc.metadata?.name || doc.kind;

        allResults.push({
          ruleId: `${resourceName}-${rule.id}`,
          isValid: passed,
          passed,
          errors: passed ? [] : [`[${resourceName}] ${rule.name}: ${rule.message}`],
          warnings: [],
          message: passed
            ? `✓ [${resourceName}] ${rule.name}`
            : `✗ [${resourceName}] ${rule.name}: ${rule.message}`,
          suggestions: !passed && rule.fix ? [rule.fix] : [],
          metadata: {
            severity: rule.severity,
            location: `${doc.kind}/${resourceName}`,
          },
        });
      }
    }

    // Fail-fast if content parses as YAML but contains no K8s resources
    if (validDocumentCount === 0) {
      return {
        results: [
          {
            ruleId: 'no-documents',
            isValid: false,
            passed: false,
            errors: ['No valid Kubernetes documents found'],
            warnings: [],
            message: 'No valid Kubernetes documents found',
            metadata: {
              severity: ValidationSeverity.ERROR,
            },
          },
        ],
        score: 0,
        grade: 'F',
        passed: 0,
        failed: 1,
        errors: 1,
        warnings: 0,
        info: 0,
        timestamp: new Date().toISOString(),
      };
    }

    return createReport(allResults);
  } catch (error) {
    return {
      results: [
        {
          ruleId: 'parse-error',
          isValid: false,
          passed: false,
          errors: [`Failed to parse YAML: ${extractErrorMessage(error)}`],
          warnings: [],
          message: `Failed to parse YAML: ${extractErrorMessage(error)}`,
          metadata: {
            severity: ValidationSeverity.ERROR,
          },
        },
      ],
      score: 0,
      grade: 'F',
      passed: 0,
      failed: 1,
      errors: 1,
      warnings: 0,
      info: 0,
      timestamp: new Date().toISOString(),
    };
  }
};

/**
 * Enhanced validation options for AI integration
 */
export interface KubernetesValidationOptions {
  enableAI?: boolean;
  aiOptions?: {
    focus?: 'security' | 'performance' | 'best-practices' | 'all';
    confidence?: number;
    maxIssues?: number;
    includeFixes?: boolean;
  };
}

/**
 * Validate Kubernetes content with optional AI enhancement
 */
export async function validateKubernetesContentWithAI(
  content: string,
  ctx: ToolContext,
  options: KubernetesValidationOptions = {},
): Promise<ValidationReport> {
  // Run standard validation first
  const baseReport = validateKubernetesContent(content);

  // If AI enhancement is disabled or no context, return base report
  if (options.enableAI === false || !ctx) {
    return baseReport;
  }

  try {
    // Run AI validation for additional insights
    const aiValidator = new AIValidator();
    const aiValidationResult = await aiValidator.validateWithAI(
      content,
      {
        contentType: 'kubernetes',
        focus: options.aiOptions?.focus || 'all',
        confidence: options.aiOptions?.confidence || 0.7,
        maxIssues: options.aiOptions?.maxIssues || 10,
        includeFixes: options.aiOptions?.includeFixes ?? true,
      },
      ctx,
    );

    if (aiValidationResult.ok) {
      const aiReport = aiValidationResult.value;

      // Filter out AI results that duplicate existing rule-based results
      // to avoid noise while preserving AI-unique insights
      const existingRuleIds = new Set(baseReport.results.map((r) => r.ruleId));
      const uniqueAIResults = aiReport.results.filter((result) => {
        // Keep AI results that don't duplicate existing rules
        // or provide additional insights with high confidence
        return (
          !existingRuleIds.has(result.ruleId) || (result.confidence && result.confidence > 0.8)
        );
      });

      // Merge reports, prioritizing rule-based results but adding AI insights
      const combinedResults = [
        ...baseReport.results,
        ...uniqueAIResults.map((aiResult) => ({
          ...aiResult,
          metadata: {
            ...aiResult.metadata,
            aiEnhanced: true,
            aiModel: aiReport.aiMetadata.model,
            aiConfidence: aiReport.aiMetadata.confidence,
          },
        })),
      ];

      // Calculate new counts with combined results
      const errorCount = combinedResults.filter(
        (r) => !r.isValid && r.metadata?.severity === ValidationSeverity.ERROR,
      ).length;
      const warningCount = combinedResults.filter(
        (r) =>
          (r.warnings && r.warnings.length > 0) ||
          r.metadata?.severity === ValidationSeverity.WARNING,
      ).length;
      const infoCount = combinedResults.filter(
        (r) => r.metadata?.severity === ValidationSeverity.INFO,
      ).length;

      return {
        ...baseReport,
        results: combinedResults,
        passed: combinedResults.filter((r) => r.isValid).length,
        failed: combinedResults.filter((r) => !r.isValid).length,
        errors: errorCount,
        warnings: warningCount,
        info: infoCount,
      };
    } else {
      ctx.logger.warn(
        { error: aiValidationResult.error },
        'AI validation failed, using standard validation only',
      );
      return baseReport;
    }
  } catch (error) {
    ctx.logger.debug({ error }, 'AI validation threw exception, using standard validation only');
    return baseReport;
  }
}

/**
 * Get all Kubernetes validation rules
 */
export const getKubernetesRules = (): KubernetesValidationRule[] => {
  return [...KUBERNETES_RULES];
};

/**
 * Get Kubernetes rules by category
 */
export const getKubernetesRulesByCategory = (
  category: ValidationCategory,
): KubernetesValidationRule[] => {
  return KUBERNETES_RULES.filter((rule) => rule.category === category);
};

/**
 * Create a Kubernetes validator factory
 */
export const createKubernetesValidator = (): KubernetesValidatorInstance => {
  return {
    validate: validateKubernetesContent,
    getRules: getKubernetesRules,
    getCategory: getKubernetesRulesByCategory,
  };
};

/**
 * Extended validation options for knowledge enhancement
 */
export interface KubernetesValidationKnowledgeOptions extends KubernetesValidationOptions {
  enableKnowledge?: boolean;
  knowledgeOptions?: {
    targetImprovement?: 'security' | 'performance' | 'best-practices' | 'enhancement' | 'all';
    userQuery?: string;
  };
}

/**
 * Validate Kubernetes manifests with knowledge enhancement integration
 */
export async function validateKubernetesManifestsWithKnowledge(
  manifests: string,
  ctx: ToolContext,
  options: KubernetesValidationKnowledgeOptions = {},
): Promise<ValidationReport & { knowledgeEnhancement?: KnowledgeEnhancementResult }> {
  // Run standard validation first (including AI if enabled)
  const baseReport = await validateKubernetesContentWithAI(manifests, ctx, options);

  // If knowledge enhancement is disabled or no issues to address, return base report
  if (options.enableKnowledge === false || !ctx) {
    return baseReport;
  }

  // Only apply knowledge enhancement if there are validation issues
  const hasIssues = !baseReport.results.every((r) => r.passed);

  if (hasIssues) {
    try {
      // Import knowledge enhancement function
      const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
        '@/mcp/ai/knowledge-enhancement'
      );

      // Create knowledge enhancement request from validation results
      const enhancementRequest = createEnhancementFromValidation(
        manifests,
        'kubernetes',
        baseReport.results
          .filter((r) => !r.passed)
          .map((r) => ({
            message: r.message || 'Validation issue',
            severity: r.metadata?.severity === ValidationSeverity.ERROR ? 'error' : 'warning',
            category: r.ruleId?.split('-')[1] || 'general',
          })),
        options.knowledgeOptions?.targetImprovement || 'security',
      );

      // Add user query if provided
      if (options.knowledgeOptions?.userQuery) {
        enhancementRequest.userQuery = options.knowledgeOptions.userQuery;
      }

      const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

      if (enhancementResult.ok) {
        return {
          ...baseReport,
          knowledgeEnhancement: enhancementResult.value,
        };
      } else {
        ctx.logger.warn(
          { error: enhancementResult.error },
          'Knowledge enhancement failed, using validation results only',
        );
      }
    } catch (error) {
      ctx.logger.debug(
        { error },
        'Knowledge enhancement threw exception, using validation results only',
      );
    }
  }

  return baseReport;
}

/**
 * Standalone validation function for simple use cases
 */
export const validateKubernetes = (yamlContent: string): ValidationReport => {
  return validateKubernetesContent(yamlContent);
};
