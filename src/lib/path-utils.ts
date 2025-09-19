import { join, posix } from 'node:path';

/**
 * Using native Node.js path.posix.normalize for consistent forward slash behavior
 * Normalizes paths to use forward slashes on all platforms for consistency.
 *
 * @param inputPath The path to normalize
 * @returns The normalized path with forward slashes, or the original value if null/undefined
 */
export function normalizePath(inputPath: string): string {
  if (inputPath == null) return inputPath; // handles both null and undefined
  if (inputPath === '') return inputPath;
  // Convert all backslashes to forward slashes, then normalize
  return posix.normalize(inputPath.replace(/\\/g, '/'));
}

// Alias for backward compatibility
export const normalizePathSeparators = normalizePath;
export const safeNormalizePath = normalizePath;

// Join paths using Node.js path.join
export function joinPaths(...paths: string[]): string {
  return join(...paths);
}
