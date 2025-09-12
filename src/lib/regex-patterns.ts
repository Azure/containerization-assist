/**
 * Centralized regex patterns for the codebase
 *
 * Simple, flat structure - no nested objects or abstractions
 * Each pattern is a direct export for easy import and use
 */

// ============================================
// Dockerfile Patterns
// ============================================

// Dockerfile instructions
export const FROM_INSTRUCTION = /^FROM\s+[\w\-./:]+/im;
export const WORKDIR_INSTRUCTION = /^WORKDIR\s+/im;
export const COPY_INSTRUCTION = /^COPY\s+/im;
export const RUN_INSTRUCTION = /^RUN\s+/im;
export const CMD_INSTRUCTION = /^CMD\s+/im;
export const ENTRYPOINT_INSTRUCTION = /^ENTRYPOINT\s+/im;
export const EXPOSE_INSTRUCTION = /^EXPOSE\s+\d+/im;
export const ENV_INSTRUCTION = /^ENV\s+\w+/im;
export const USER_INSTRUCTION = /^USER\s+/im;
export const ARG_INSTRUCTION = /^ARG\s+/im;
export const HEALTHCHECK_INSTRUCTION = /^HEALTHCHECK\s+/im;

// Multi-stage patterns
export const MULTI_STAGE_FROM = /FROM\s+[\w\-./:]+\s+AS\s+\w+/im;
export const AS_CLAUSE = /\s+AS\s+/i;

// Version and tag patterns
export const LATEST_TAG = /:latest(?:\s|$)/g;
export const PINNED_VERSION = /:\d+\.\d+/;
export const SEMVER_VERSION = /:\d+\.\d+\.\d+/;

// User patterns
export const ROOT_USER = /USER\s+root/i;

// Package manager patterns
export const SUDO_INSTALL = /install.*sudo|apt-get.*sudo|yum.*sudo|apk.*sudo/;
export const PACKAGE_FILES = /package.*\.json|requirements\.txt|go\.mod|pom\.xml/;
export const APT_UPDATE = /apt-get\s+update/;
export const APT_CLEAN = /apt-get\s+clean|rm\s+-rf\s+\/var\/lib\/apt\/lists/;

// ============================================
// Security Patterns
// ============================================

// Secret detection patterns (for validation, not extraction)
export const PASSWORD_PATTERN = /password\s*=\s*["'].+["']/i;
export const API_KEY_PATTERN = /api[_-]?key\s*=\s*["'].+["']/i;
export const SECRET_PATTERN = /secret\s*=\s*["'].+["']/i;
export const TOKEN_PATTERN = /token\s*=\s*["'].+["']/i;

// Insecure patterns
export const HTTP_URL = /http:\/\/[^\s"']+/g;
export const CHMOD_777 = /chmod\s+(777|a\+rwx)/g;

// ============================================
// Kubernetes Patterns
// ============================================

// Resource identification
export const K8S_API_VERSION = /^apiVersion:\s*[\w/]+/m;
export const K8S_KIND = /^kind:\s*\w+/m;
export const K8S_DEPLOYMENT = /^kind:\s*Deployment/m;
export const K8S_SERVICE = /^kind:\s*Service/m;
export const K8S_INGRESS = /^kind:\s*Ingress/m;
export const K8S_CONFIGMAP = /^kind:\s*ConfigMap/m;
export const K8S_SECRET = /^kind:\s*Secret/m;

// YAML document separator
export const YAML_DOC_SEPARATOR = /^---\s*$/m;

// Security context patterns
export const SECURITY_CONTEXT = /securityContext:/;
export const RUN_AS_NON_ROOT = /runAsNonRoot:\s*true/;
export const READ_ONLY_ROOT = /readOnlyRootFilesystem:\s*true/;

// Resource patterns
export const RESOURCE_LIMITS = /resources:\s*\n\s*limits:/;
export const RESOURCE_REQUESTS = /resources:\s*\n\s*requests:/;
export const CPU_LIMIT = /cpu:\s*["']?\d+m?["']?/;
export const MEMORY_LIMIT = /memory:\s*["']?\d+[GMK]i?["']?/;

// ============================================
// Code Fence Patterns (for AI response processing)
// ============================================

export const DOCKERFILE_FENCE = /```(?:docker|dockerfile|Dockerfile|DOCKERFILE)?\s*\n([\s\S]*?)```/;
export const YAML_FENCE = /```(?:yaml|yml|YAML|YML)?\s*\n([\s\S]*?)```/;
export const GENERIC_FENCE = /```[a-zA-Z0-9]*\s*\n?([\s\S]*?)```/;

// ============================================
// Docker Image Patterns
// ============================================

export const IMAGE_TAG = /^(.+?)(?::([^/]+))?$/;
export const ENV_VAR_PATTERN = /^[A-Z_][A-Z0-9_]*=.*$/;

// ============================================
// Simple extraction functions - only where actually needed
// ============================================

/**
 * Extract base image from Dockerfile FROM instruction
 */
export function extractBaseImage(dockerfile: string): string | null {
  const match = dockerfile.match(FROM_INSTRUCTION);
  if (!match) return null;

  // Remove FROM keyword and clean up
  let baseImage = match[0].replace(/^FROM\s+/i, '').trim();

  // Remove AS clause if present (multi-stage)
  const asIndex = baseImage.search(AS_CLAUSE);
  if (asIndex > -1) {
    baseImage = baseImage.substring(0, asIndex).trim();
  }

  return baseImage;
}

/**
 * Check if a Dockerfile uses latest tags
 */
export function hasLatestTag(content: string): boolean {
  return LATEST_TAG.test(content);
}

/**
 * Check if running as root user
 */
export function isRunningAsRoot(dockerfile: string): boolean {
  const userMatches = dockerfile.match(/USER\s+(\w+)/gi);
  if (!userMatches || userMatches.length === 0) {
    return true; // No USER instruction means running as root
  }

  // Check the last USER instruction
  const lastUser = userMatches[userMatches.length - 1];
  const user = lastUser ? lastUser.replace(/USER\s+/i, '').trim() : 'root';
  return user === 'root' || user === '0';
}

/**
 * Extract exposed ports from Dockerfile
 */
export function extractExposedPorts(dockerfile: string): number[] {
  const ports: number[] = [];
  const matches = dockerfile.match(/EXPOSE\s+(\d+)/gim);

  if (matches) {
    matches.forEach((match) => {
      const port = parseInt(match.replace(/EXPOSE\s+/i, '').trim(), 10);
      if (!isNaN(port)) {
        ports.push(port);
      }
    });
  }

  return ports;
}

/**
 * Check if content contains hardcoded secrets
 */
export function hasHardcodedSecrets(content: string): boolean {
  return (
    PASSWORD_PATTERN.test(content) ||
    API_KEY_PATTERN.test(content) ||
    SECRET_PATTERN.test(content) ||
    TOKEN_PATTERN.test(content)
  );
}

/**
 * Check if Dockerfile has multi-stage build
 */
export function isMultiStage(dockerfile: string): boolean {
  return MULTI_STAGE_FROM.test(dockerfile);
}

/**
 * Extract Kubernetes resource kind
 */
export function extractK8sKind(yaml: string): string | null {
  const match = yaml.match(K8S_KIND);
  return match ? match[0].replace(/^kind:\s*/i, '').trim() : null;
}
