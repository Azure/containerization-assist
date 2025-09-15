/**
 * Kubernetes validation using YAML parser (Functional)
 *
 * Invariant: All validation rules maintain backwards compatibility
 * Trade-off: Runtime YAML parsing cost over build-time validation for flexibility
 */

import { parse as parseYaml } from 'yaml';
import { extractErrorMessage } from '../lib/error-utils';
import {
  KubernetesValidationRule,
  KubernetesManifest,
  ValidationResult,
  ValidationReport,
  ValidationSeverity,
  ValidationCategory,
  ValidationGrade,
} from './core-types';

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
const getPodSpec = (manifest: KubernetesManifest): Record<string, unknown> | undefined => {
  if (manifest.kind === 'Pod') {
    return manifest.spec;
  }
  if (manifest.kind === 'Job' || manifest.kind === 'CronJob') {
    const spec = manifest.spec as any;
    return spec?.jobTemplate?.spec?.template?.spec || spec?.template?.spec;
  }
  return (manifest.spec as any)?.template?.spec;
};

/**
 * Get all containers from manifest
 */
const getContainers = (manifest: KubernetesManifest): Array<Record<string, unknown>> => {
  const podSpec = getPodSpec(manifest);
  const spec = podSpec as any;
  return [...(spec?.containers || []), ...(spec?.initContainers || [])];
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
        const resources = (container as any).resources;
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
        const resources = (container as any).resources;
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
        const securityContext = (container as any).securityContext;
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
      const securityContext = (podSpec as any)?.securityContext;

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
        const c = container as any;
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
      const volumes = (podSpec as any)?.volumes || [];

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

      return !!(manifest.spec as any)?.strategy?.type;
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
 * Standalone validation function for simple use cases
 */
export const validateKubernetes = (yamlContent: string): ValidationReport => {
  return validateKubernetesContent(yamlContent);
};
