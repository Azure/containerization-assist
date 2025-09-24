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
const FROM_INSTRUCTION = /^FROM\s+[\w\-./:]+/im;

// Multi-stage patterns
export const AS_CLAUSE = /\s+AS\s+/i;

// Version and tag patterns
export const LATEST_TAG = /:latest(?:\s|$)/;

// Package manager patterns
export const SUDO_INSTALL = /install.*sudo|apt-get.*sudo|yum.*sudo|apk.*sudo/;
export const PACKAGE_FILES = /package.*\.json|requirements\.txt|go\.mod|pom\.xml/;

// ============================================
// Security Patterns
// ============================================

// Secret detection patterns (for validation, not extraction)
export const PASSWORD_PATTERN = /password\s*=\s*["'].+["']/i;
export const API_KEY_PATTERN = /api[_-]?key\s*=\s*["'].+["']/i;
export const SECRET_PATTERN = /secret\s*=\s*["'].+["']/i;
export const TOKEN_PATTERN = /token\s*=\s*["'].+["']/i;

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
// Extraction functions
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
