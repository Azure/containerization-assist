/**
 * Chain Hints Registry
 * Central configuration for tool workflow guidance at application level
 */

/**
 * Chain hints for tool workflow guidance
 */
export interface ChainHints {
  /** Guidance message shown after successful execution */
  success: string;
  /** Guidance message shown after failed execution */
  failure: string;
}

/**
 * Chain hints registry mapping tool names to their workflow hints
 */
export type ChainHintsRegistry = Partial<Record<string, ChainHints>>;

/**
 * Default chain hints for all containerization tools
 * Provides workflow guidance for successful tool chaining
 */
export const DEFAULT_CHAIN_HINTS: ChainHintsRegistry = {
  'analyze-repo': {
    success:
      'Repository analysis completed successfully. Continue by calling the generate-dockerfile or fix-dockerfile tools to create or fix your Dockerfile.',
    failure: 'Repository analysis failed. Please check the logs for details.',
  },

  'build-image': {
    success:
      'Image built successfully. Next: Call scan-image to check for security vulnerabilities.',
    failure: 'Image build failed. Use fix-dockerfile to resolve issues, then retry build-image.',
  },

  'deploy': {
    success:
      'Application deployed successfully. Use verify-deploy to check deployment health and status.',
    failure:
      'Deployment failed. Check cluster connectivity, manifests validity, and resource availability.',
  },

  'fix-dockerfile': {
    success:
      'Dockerfile fixes applied successfully. Next: Call build-image to test the fixed Dockerfile.',
    failure: 'Dockerfile fix failed. Review validation errors and try manual fixes.',
  },

  'generate-k8s-manifests': {
    success:
      'Kubernetes manifests generated successfully. Next: Call prepare-cluster to create a kind cluster to deploy to.',
    failure: 'Manifest generation failed. Ensure you have a valid image and try again.',
  },

  'prepare-cluster': {
    success: 'Cluster preparation successful. Next: Call deploy to deploy to the kind cluster.',
    failure:
      'Cluster preparation found issues. Check connectivity, permissions, and namespace configuration.',
  },

  'push-image': {
    success: 'Image pushed successfully. Review AI optimization insights for push improvements.',
    failure:
      'Image push failed. Check registry credentials, network connectivity, and image tag format.',
  },

  'scan-image': {
    success:
      'Security scan passed! Proceed with push-image to push to a registry, or continue with deployment preparation.',
    failure:
      'Security vulnerabilities detected. Review scan results and apply fixes before proceeding.',
  },

  'validate-dockerfile': {
    success:
      'Dockerfile validated successfully. Continue by building this Dockerfile. Continue trying to build and fix until it builds successfully. Then call the generate-k8s-manifests tool to proceed with containerization.',
    failure:
      'Dockerfile validation failed. Review errors and use fix-dockerfile to address issues.',
  },
};
