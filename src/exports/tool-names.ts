/**
 * Tool names as constants for type-safe registration
 * Use these instead of raw strings when registering specific tools
 */
export const TOOL_NAMES = {
  ANALYZE_REPO: 'analyze_repo',
  GENERATE_DOCKERFILE: 'generate_dockerfile',
  BUILD_IMAGE: 'build_image',
  SCAN_IMAGE: 'scan_image',
  TAG_IMAGE: 'tag_image',
  PUSH_IMAGE: 'push_image',
  GENERATE_K8S_MANIFESTS: 'generate_k8s_manifests',
  PREPARE_CLUSTER: 'prepare_cluster',
  DEPLOY_APPLICATION: 'deploy_application',
  VERIFY_DEPLOYMENT: 'verify_deployment',
  FIX_DOCKERFILE: 'fix_dockerfile',
  RESOLVE_BASE_IMAGES: 'resolve_base_images',
  OPS: 'ops',
  WORKFLOW: 'workflow',
} as const;

/**
 * Type for valid tool names
 */
export type ToolName = (typeof TOOL_NAMES)[keyof typeof TOOL_NAMES];
