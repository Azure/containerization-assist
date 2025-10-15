/**
 * Unified scoring framework for containerization content
 *
 * This module provides a centralized scoring system that consolidates
 * duplicated scoring logic from across the codebase.
 */

// Base interfaces and types
export { type ScoringResult, type ScoringCriteria, type ScoringContext } from './base-scorer';

// Dockerfile scoring
export {
  DockerfileScorer,
  scoreDockerfile,
  scoreDockerfileDetailed,
  createDockerfileScoringFunction,
} from './dockerfile-scorer';

// Kubernetes manifest scoring
export {
  KubernetesScorer,
  scoreKubernetesManifest,
  createKubernetesScoringFunction,
} from './kubernetes-scorer';

// Helm chart scoring
export { HelmScorer, scoreHelmChart } from './helm-scorer';

// Additional specialized scorers
export {
  scoreACAManifest,
  scoreBaseImageRecommendation,
  scoreRepositoryAnalysis,
  scoreACAConversion,
} from './legacy-scorers';

// Import types and classes for internal use
import { type ScoringResult, type ScoringContext } from './base-scorer';
import { DockerfileScorer } from './dockerfile-scorer';
import { KubernetesScorer } from './kubernetes-scorer';
import { HelmScorer } from './helm-scorer';

/**
 * Universal scoring function that detects content type and applies appropriate scorer
 * @param content Content to score
 * @param context Optional scoring context
 * @returns Scoring result
 */
export function scoreContent(content: string, context?: ScoringContext): ScoringResult {
  const contentType = detectContentType(content, context?.contentType);

  switch (contentType.toLowerCase()) {
    case 'dockerfile':
      return new DockerfileScorer().score(content, context);
    case 'kubernetes':
    case 'k8s':
    case 'yaml':
      return new KubernetesScorer().score(content, context);
    case 'helm':
    case 'chart':
      return new HelmScorer().score(content, context);
    default:
      throw new Error(`Unknown content type: ${contentType}`);
  }
}

/**
 * Detect content type from content analysis
 * @param content Content to analyze
 * @param hint Optional content type hint
 * @returns Detected content type
 */
function detectContentType(content: string, hint?: string): string {
  if (hint) {
    return hint;
  }

  // Dockerfile detection
  if (/^FROM\s+/m.test(content) || content.includes('RUN ') || content.includes('COPY ')) {
    return 'dockerfile';
  }

  // Helm chart detection
  if (content.includes('{{') && content.includes('}}') && content.includes('Values')) {
    return 'helm';
  }

  // Kubernetes manifest detection (fallback for YAML)
  if (content.includes('apiVersion:') && content.includes('kind:')) {
    return 'kubernetes';
  }

  // Default to Kubernetes for generic YAML
  return 'kubernetes';
}
