/**
 * Topic constants for AI prompt categorization and knowledge matching
 */
export const TOPICS = {
  // Repository analysis
  ANALYZE_REPOSITORY: 'analyze_repository',

  // Dockerfile generation and management
  DOCKERFILE_GENERATION: 'dockerfile_generation',
  DOCKERFILE_BASE: 'dockerfile_base',
  DOCKERFILE_DEPENDENCIES: 'dockerfile_dependencies',
  DOCKERFILE_BUILD: 'dockerfile_build',
  DOCKERFILE_RUNTIME: 'dockerfile_runtime',
  FIX_DOCKERFILE: 'fix_dockerfile',

  // Base image resolution
  RESOLVE_BASE_IMAGES: 'resolve_base_images',

  // Kubernetes manifests
  GENERATE_K8S_MANIFESTS: 'generate_k8s_manifests',

  // Helm charts
  GENERATE_HELM_CHARTS: 'generate_helm_charts',

  // Azure Container Apps
  GENERATE_ACA_MANIFESTS: 'generate_aca_manifests',
  CONVERT_ACA_TO_K8S: 'convert_aca_to_k8s',
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
