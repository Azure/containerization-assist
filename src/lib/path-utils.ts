import * as path from 'path';

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
  return path.posix.normalize(inputPath.replace(/\\/g, '/'));
}
