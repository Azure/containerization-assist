/**
 * Centralized regex patterns for the codebase
 *
 * Simple, flat structure - no nested objects or abstractions
 * Each pattern is a direct export for easy import and use
 */

// Multi-stage patterns
export const AS_CLAUSE = /\s+AS\s+/i;

// Version and tag patterns
export const LATEST_TAG = /:latest(?:\s|$)/;

// Package manager patterns
export const SUDO_INSTALL = /install.*sudo|apt-get.*sudo|yum.*sudo|apk.*sudo/;
export const PACKAGE_FILES = /package.*\.json|requirements\.txt|go\.mod|pom\.xml/;

// Secret detection patterns (for validation, not extraction)
export const PASSWORD_PATTERN = /\w*password\w*\s*=\s*["']?.+["']?/i;
export const API_KEY_PATTERN = /\w*api[_-]?key\w*\s*=\s*["']?.+["']?/i;
export const SECRET_PATTERN = /\w*secret\w*\s*=\s*["']?.+["']?/i;
export const TOKEN_PATTERN = /\w*token\w*\s*=\s*["']?.+["']?/i;
