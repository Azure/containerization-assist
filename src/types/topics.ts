/**
 * Topic constants for AI prompt categorization and knowledge matching
 */
export const TOPICS = {
  // Repository analysis
  ANALYZE_REPOSITORY: 'analyze_repository',

  // Dockerfile generation and management
  DOCKERFILE: 'dockerfile',
  DOCKERFILE_BASE: 'dockerfile_base',
  DOCKERFILE_DEPENDENCIES: 'dockerfile_dependencies',
  DOCKERFILE_BUILD: 'dockerfile_build',
  DOCKERFILE_RUNTIME: 'dockerfile_runtime',
  FIX_DOCKERFILE: 'fix_dockerfile',
  DOCKERFILE_REPAIR: 'dockerfile_repair',

  // Docker optimization
  DOCKER_OPTIMIZATION: 'docker_optimization',

  // Docker tagging
  DOCKER_TAGGING: 'docker_tagging',

  // Kubernetes manifests
  KUBERNETES: 'kubernetes',
  KUBERNETES_REPAIR: 'kubernetes_repair',

  // Helm charts
  GENERATE_HELM_CHARTS: 'generate_helm_charts',

  // Azure Container Apps
  GENERATE_ACA_MANIFESTS: 'generate_aca_manifests',
  CONVERT_ACA_TO_K8S: 'convert_aca_to_k8s',

  // AI services
  KNOWLEDGE_ENHANCEMENT: 'knowledge_enhancement',
} as const;

/**
 * Type representing all valid topic values
 */
export type Topic = (typeof TOPICS)[keyof typeof TOPICS];

/**
 * Type guard to check if a string is a valid topic
 */
export function isValidTopic(value: string): value is Topic {
  return Object.values(TOPICS).includes(value as Topic);
}

/**
 * Get all available topics as an array
 */
export function getAllTopics(): Topic[] {
  return Object.values(TOPICS);
}
