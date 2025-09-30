import { TextContentScorer, type ScoringResult, type ScoringContext } from './base-scorer';
import { extractKubernetesContent } from '@/lib/content-extraction';

/**
 * Kubernetes manifest-specific scoring criteria
 */
interface KubernetesScoringBreakdown extends Record<string, number> {
  parseability: number;
  resources: number;
  security: number;
  reliability: number;
  observability: number;
}

/**
 * Scorer for Kubernetes manifest content
 *
 * Evaluates Kubernetes YAML manifests for:
 * - YAML parsing and structure validity
 * - Resource management practices
 * - Security configurations
 * - Reliability patterns (probes, replicas, strategies)
 * - Observability features (monitoring, logging)
 */
export class KubernetesScorer extends TextContentScorer {
  /**
   * Score a Kubernetes manifest with comprehensive analysis
   */
  score(content: string, _context?: ScoringContext): ScoringResult {
    const breakdown: KubernetesScoringBreakdown = {
      parseability: this.scoreParseability(content),
      resources: this.scoreResources(content),
      security: this.scoreSecurity(content),
      reliability: this.scoreReliability(content),
      observability: this.scoreObservability(content),
    };

    const feedback = this.generateFeedback(content, breakdown);

    // Weight different aspects of Kubernetes manifests
    const weights = {
      parseability: 0.25,
      resources: 0.2,
      security: 0.25,
      reliability: 0.2,
      observability: 0.1,
    };

    return this.createResult(breakdown, weights, feedback);
  }

  /**
   * Validate that the content is parseable YAML
   */
  protected validateParseable(content: string): boolean {
    try {
      // Basic YAML structure check
      const lines = content.trim().split('\n');
      return (
        lines.some((line) => line.includes('apiVersion:')) &&
        lines.some((line) => line.includes('kind:')) &&
        lines.some((line) => line.includes('metadata:'))
      );
    } catch {
      return false;
    }
  }

  /**
   * Score YAML parseability and Kubernetes structure
   */
  private scoreParseability(content: string): number {
    let score = 0;

    // Basic YAML structure (40 points)
    if (!this.validateParseable(content)) {
      return 0; // Invalid structure
    }
    score += 40;

    // Kubernetes-specific structure (60 points)
    if (content.includes('apiVersion:')) score += 15;
    if (content.includes('kind:')) score += 15;
    if (content.includes('metadata:')) score += 15;
    if (content.includes('spec:')) score += 10;
    if (/name:\s*\S+/m.test(content)) score += 5; // Has proper name

    return Math.min(score, 100);
  }

  /**
   * Score resource management practices
   */
  private scoreResources(content: string): number {
    let score = 0;

    // Resource requests and limits (60 points)
    if (content.includes('resources:')) {
      score += 20;
      if (content.includes('limits:')) score += 20;
      if (content.includes('requests:')) score += 20;
    }

    // CPU and memory best practices (25 points)
    if (/cpu:\s*[0-9]+m/.test(content)) score += 10; // Prefer millicores
    if (/memory:\s*[0-9]+Mi/.test(content)) score += 10; // Prefer Mi over Gi for smaller apps
    if (!/cpu:\s*[0-9]+[^m]/.test(content)) score += 5; // Avoid whole CPU units

    // Resource quotas and limits (15 points)
    if (content.includes('ResourceQuota')) score += 10;
    if (content.includes('LimitRange')) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score security configurations
   */
  private scoreSecurity(content: string): number {
    let score = 0;

    // Security context (40 points)
    if (content.includes('securityContext:')) {
      score += 15;
      if (content.includes('runAsNonRoot: true')) score += 10;
      if (content.includes('readOnlyRootFilesystem: true')) score += 10;
      if (content.includes('allowPrivilegeEscalation: false')) score += 5;
    }

    // Pod security context (20 points)
    if (content.includes('podSecurityContext:')) {
      score += 10;
      if (/runAsUser:\s*[1-9]/.test(content)) score += 10; // Non-root UID
    }

    // Network policies (15 points)
    if (content.includes('NetworkPolicy')) score += 15;

    // Service account (10 points)
    if (/serviceAccount(Name)?:/.test(content)) score += 10;

    // Image security (15 points)
    if (!/image:.*:latest/.test(content)) score += 10; // Avoid latest tags
    if (/imagePullPolicy:\s*Always/.test(content)) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score reliability patterns
   */
  private scoreReliability(content: string): number {
    let score = 0;

    // Replica management (25 points)
    if (/replicas:\s*[2-9]/.test(content)) score += 20; // Multiple replicas
    if (/replicas:\s*[5-9]/.test(content)) score += 5; // High availability

    // Health checks (35 points)
    if (content.includes('livenessProbe:')) score += 15;
    if (content.includes('readinessProbe:')) score += 15;
    if (content.includes('startupProbe:')) score += 5;

    // Deployment strategy (20 points)
    if (content.includes('strategy:')) {
      score += 10;
      if (content.includes('RollingUpdate')) score += 10;
    }

    // Resource management for reliability (20 points)
    if (content.includes('PodDisruptionBudget')) score += 15;
    if (content.includes('HorizontalPodAutoscaler')) score += 5;

    return Math.min(score, 100);
  }

  /**
   * Score observability features
   */
  private scoreObservability(content: string): number {
    let score = 0;

    // Prometheus monitoring (30 points)
    if (content.includes('prometheus.io/scrape')) score += 15;
    if (content.includes('prometheus.io/port')) score += 10;
    if (content.includes('prometheus.io/path')) score += 5;

    // Labels and selectors (25 points)
    if (content.includes('labels:')) score += 15;
    if (/app\.kubernetes\.io\/name/.test(content)) score += 5;
    if (/app\.kubernetes\.io\/version/.test(content)) score += 5;

    // Annotations (20 points)
    if (content.includes('annotations:')) score += 10;
    if (content.includes('deployment.kubernetes.io/revision')) score += 5;
    if (content.includes('kubernetes.io/change-cause')) score += 5;

    // Logging configuration (25 points)
    if (/env:[\s\S]*LOG_LEVEL/.test(content)) score += 15;
    if (content.includes('Fluentd') || content.includes('filebeat')) score += 10;

    return Math.min(score, 100);
  }

  /**
   * Generate helpful feedback messages based on scoring
   */
  private generateFeedback(content: string, scores: KubernetesScoringBreakdown): string[] {
    const feedback: string[] = [];

    if (scores.parseability < 50) {
      feedback.push('Manifest has structural issues that need to be resolved');
    }

    if (scores.resources < 60) {
      if (!content.includes('resources:')) {
        feedback.push('Add resource requests and limits for better resource management');
      }
      if (!content.includes('limits:')) {
        feedback.push('Set resource limits to prevent resource exhaustion');
      }
    }

    if (scores.security < 60) {
      if (!content.includes('securityContext:')) {
        feedback.push('Add security context to improve pod security');
      }
      if (!content.includes('runAsNonRoot: true')) {
        feedback.push('Run containers as non-root user when possible');
      }
      if (/image:.*:latest/.test(content)) {
        feedback.push('Pin image versions instead of using :latest tags');
      }
    }

    if (scores.reliability < 60) {
      if (!/replicas:\s*[2-9]/.test(content)) {
        feedback.push('Use multiple replicas for high availability');
      }
      if (!content.includes('livenessProbe:')) {
        feedback.push('Add liveness probes for automatic restart on failure');
      }
      if (!content.includes('readinessProbe:')) {
        feedback.push('Add readiness probes for proper traffic routing');
      }
    }

    if (scores.observability < 60) {
      if (!content.includes('prometheus.io/scrape')) {
        feedback.push('Add Prometheus annotations for metrics collection');
      }
      if (!content.includes('labels:')) {
        feedback.push('Add proper labels for better resource organization');
      }
    }

    return feedback;
  }
}

/**
 * Convenience function for scoring Kubernetes manifests
 * Maintains backward compatibility with existing simple scoring functions
 */
export function scoreKubernetesManifest(
  content: string,
  context?: ScoringContext,
): Record<string, number> {
  const scorer = new KubernetesScorer();
  const result = scorer.score(content, context);
  return result.breakdown;
}

/**
 * Convenience function for detailed Kubernetes manifest scoring
 * Returns complete scoring result with feedback
 */
export function scoreKubernetesManifestDetailed(
  content: string,
  context?: ScoringContext,
): ScoringResult {
  const scorer = new KubernetesScorer();
  return scorer.score(content, context);
}

/**
 * Creates a scoring function that extracts and scores Kubernetes manifest content
 *
 * This helper consolidates the common pattern of extracting Kubernetes manifests
 * from AI responses and scoring them. Used by sampling-based tools to avoid duplication.
 *
 * @returns A function that takes raw AI response text and returns a numeric score
 *
 * @example
 * ```typescript
 * const samplingResult = await sampleWithRerank(
 *   ctx,
 *   buildSamplingRequest,
 *   createKubernetesScoringFunction(),
 *   {}
 * );
 * ```
 */
export function createKubernetesScoringFunction(): (text: string) => number {
  return (text: string) => {
    const extraction = extractKubernetesContent(text);
    if (extraction.success && extraction.content) {
      // Score the first manifest or average all manifests
      const manifests = extraction.content;
      if (manifests.length > 0) {
        const scores = manifests.map((manifest) => {
          const yamlString =
            typeof manifest === 'string' ? manifest : JSON.stringify(manifest, null, 2);
          const result = scoreKubernetesManifest(yamlString);
          // Calculate weighted average from breakdown
          const weights = {
            parseability: 0.3,
            resources: 0.2,
            security: 0.25,
            reliability: 0.15,
            observability: 0.1,
          };
          return Object.entries(result).reduce((sum, [key, value]) => {
            const weight = weights[key as keyof typeof weights] || 0;
            return sum + value * weight;
          }, 0);
        });
        return scores.reduce((sum, score) => sum + score, 0) / scores.length;
      }
    }
    // Fallback to scoring raw text
    const result = scoreKubernetesManifest(text);
    const weights = {
      parseability: 0.3,
      resources: 0.2,
      security: 0.25,
      reliability: 0.15,
      observability: 0.1,
    };
    return Object.entries(result).reduce((sum, [key, value]) => {
      const weight = weights[key as keyof typeof weights] || 0;
      return sum + value * weight;
    }, 0);
  };
}
