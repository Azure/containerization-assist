/**
 * VS Code Integration Utilities
 *
 * Provides adapter functions for VS Code extension integration.
 * These utilities help bridge VS Code APIs with SDK expectations.
 *
 * Note: This module has NO direct vscode dependency - it works with
 * the interface shape, allowing it to be used in non-VS Code contexts too.
 *
 * @example
 * ```typescript
 * import {
 *   createAbortSignalFromToken,
 *   formatErrorForLLM,
 *   resolveWorkspacePath,
 * } from 'containerization-assist-mcp/sdk';
 *
 * // In VS Code tool invoke method
 * async invoke(options, token: vscode.CancellationToken) {
 *   const { signal, dispose } = createAbortSignalFromToken(token);
 *   try {
 *     const result = await analyzeRepo({ repositoryPath: options.input.path }, { signal });
 *     if (!result.ok) {
 *       throw new Error(formatErrorForLLM(result.error, result.guidance));
 *     }
 *     return formatResult(result.value);
 *   } finally {
 *     dispose();
 *   }
 * }
 * ```
 */

import * as path from 'node:path';

import type { ErrorGuidance } from '@/types/core';

// Re-export ErrorGuidance for vscode-utils consumers
export type { ErrorGuidance };

// ===== CANCELLATION ADAPTER =====

/**
 * Interface matching VS Code's CancellationToken.
 * Using an interface allows this to work without vscode dependency.
 */
export interface CancellationTokenLike {
  /** Whether cancellation has been requested */
  isCancellationRequested: boolean;
  /** Register a listener for cancellation */
  onCancellationRequested: (listener: () => void) => { dispose: () => void };
}

/**
 * Result of creating an AbortSignal from a CancellationToken.
 */
export interface AbortSignalResult {
  /** The AbortSignal to pass to SDK functions */
  signal: AbortSignal;
  /** The underlying AbortController (for advanced use) */
  controller: AbortController;
  /** Clean up function - call when done to remove event listener */
  dispose: () => void;
}

/**
 * Creates an AbortSignal from a VS Code CancellationToken.
 *
 * Use this to convert VS Code's cancellation mechanism to the
 * AbortSignal expected by SDK functions.
 *
 * @param token - VS Code CancellationToken (or compatible interface)
 * @returns Object containing AbortSignal and cleanup function
 *
 * @example
 * ```typescript
 * // In VS Code extension tool
 * import { createAbortSignalFromToken } from 'containerization-assist-mcp/sdk';
 *
 * async invoke(options, token: vscode.CancellationToken) {
 *   const { signal, dispose } = createAbortSignalFromToken(token);
 *
 *   try {
 *     const result = await analyzeRepo(
 *       { repositoryPath: options.input.path },
 *       { signal }
 *     );
 *     return formatResult(result);
 *   } finally {
 *     dispose(); // Clean up event listener
 *   }
 * }
 * ```
 */
export function createAbortSignalFromToken(token: CancellationTokenLike): AbortSignalResult {
  const controller = new AbortController();

  // Check if already cancelled
  if (token.isCancellationRequested) {
    controller.abort();
  }

  // Listen for future cancellation
  const subscription = token.onCancellationRequested(() => {
    controller.abort();
  });

  return {
    signal: controller.signal,
    controller,
    dispose: () => subscription.dispose(),
  };
}

// ===== ERROR FORMATTING =====

/**
 * Format a Result failure for VS Code tool error display.
 *
 * Creates an error message that is informative for the LLM,
 * including guidance on how to recover.
 *
 * @param error - Error message from Result
 * @param guidance - Optional guidance object from Result
 * @returns Formatted error message string
 *
 * @example
 * ```typescript
 * if (!result.ok) {
 *   throw new Error(formatErrorForLLM(result.error, result.guidance));
 * }
 * ```
 */
export function formatErrorForLLM(error: string, guidance?: ErrorGuidance): string {
  const parts = [error];

  if (guidance?.hint) {
    parts.push(`Hint: ${guidance.hint}`);
  }

  if (guidance?.resolution) {
    parts.push(`Resolution: ${guidance.resolution}`);
  }

  return parts.join('\n\n');
}

// ===== PATH UTILITIES =====

/**
 * Resolve a path relative to the workspace root.
 *
 * VS Code tools often receive relative paths - this helps resolve them
 * to absolute paths that the SDK functions expect.
 *
 * @param inputPath - Path from tool input (may be relative)
 * @param workspaceRoot - Workspace root path
 * @returns Resolved absolute path
 *
 * @example
 * ```typescript
 * const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
 * if (workspaceRoot) {
 *   input.repositoryPath = resolveWorkspacePath(input.repositoryPath, workspaceRoot);
 * }
 * ```
 */
export function resolveWorkspacePath(inputPath: string, workspaceRoot: string): string {
  // Already absolute - return as-is
  if (path.isAbsolute(inputPath)) {
    return inputPath;
  }

  // Resolve relative path against workspace root
  // path.resolve handles ., .., and normalizes separators
  return path.resolve(workspaceRoot, inputPath);
}

// ===== INPUT VALIDATION HELPERS =====

/**
 * Result of field validation - discriminated union for type narrowing.
 *
 * When `valid` is true, TypeScript knows `missing` is an empty array.
 * When `valid` is false, TypeScript knows `missing` contains the field names.
 */
export type ValidationResult<T extends string = string> =
  | { readonly valid: true; readonly missing: readonly [] }
  | { readonly valid: false; readonly missing: readonly T[] };

/**
 * Check if an input object has all required fields.
 *
 * Returns a discriminated union that enables TypeScript type narrowing.
 *
 * @param input - Input object to validate
 * @param requiredFields - Array of required field names (use `as const` for best type inference)
 * @returns Discriminated union enabling type narrowing
 *
 * @example
 * ```typescript
 * const validation = validateRequiredFields(input, ['repositoryPath', 'imageName'] as const);
 * if (!validation.valid) {
 *   // TypeScript knows validation.missing is ('repositoryPath' | 'imageName')[]
 *   throw new Error(`Missing required fields: ${validation.missing.join(', ')}`);
 * }
 * // TypeScript knows validation.valid is true here
 * ```
 */
export function validateRequiredFields<const T extends string>(
  input: Record<string, unknown>,
  requiredFields: readonly T[],
): ValidationResult<T> {
  const missing = requiredFields.filter((field): field is T => {
    const value = input[field];
    return value === undefined || value === null || value === '';
  });

  if (missing.length === 0) {
    return { valid: true, missing: [] as const };
  }

  return { valid: false, missing };
}

/**
 * Character escape mappings for markdown sanitization.
 * Maps special characters to their escaped equivalents.
 */
const MARKDOWN_ESCAPE_MAP: Readonly<Record<string, string>> = {
  '\\': '\\\\',
  '`': '\\`',
  '*': '\\*',
  _: '\\_',
  '[': '\\[',
  ']': '\\]',
  '(': '\\(',
  ')': '\\)',
  '<': '&lt;',
  '>': '&gt;',
};

/**
 * Regex matching all markdown special characters that need escaping.
 */
const MARKDOWN_SPECIAL_CHARS = /[\\`*_[\]()<>]/g;

/**
 * Sanitize a string for use in confirmation messages.
 *
 * Escapes markdown special characters to prevent injection.
 * Uses single-pass regex replacement for O(n) performance.
 *
 * @param input - String to sanitize
 * @returns Sanitized string safe for markdown display
 */
export function sanitizeForMarkdown(input: string): string {
  // The regex only matches characters that exist in MARKDOWN_ESCAPE_MAP,
  // so the lookup is guaranteed to succeed
  return input.replace(
    MARKDOWN_SPECIAL_CHARS,
    (char) => MARKDOWN_ESCAPE_MAP[char] ?? char,
  );
}
