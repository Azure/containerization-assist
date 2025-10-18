/**
 * Platform and path utilities
 *
 * Consolidates cross-platform system detection and path normalization
 * for consistent behavior across different operating systems.
 */

import * as path from 'path';

// ============================================================================
// Platform Detection
// ============================================================================

export interface SystemInfo {
  isWindows: boolean;
  isMac: boolean;
  isLinux: boolean;
}

/**
 * Get system information for cross-platform logic
 */
export function getSystemInfo(): SystemInfo {
  return {
    isWindows: process.platform === 'win32',
    isMac: process.platform === 'darwin',
    isLinux: process.platform === 'linux',
  };
}

/**
 * Get OS string for download URLs
 */
export function getDownloadOS(): string {
  const system = getSystemInfo();
  if (system.isWindows) return 'windows';
  if (system.isMac) return 'darwin';
  return 'linux';
}

/**
 * Get architecture string for download URLs
 */
export function getDownloadArch(): string {
  switch (process.arch) {
    case 'x64':
      return 'amd64';
    case 'arm64':
      return 'arm64';
    default:
      return 'amd64';
  }
}

// ============================================================================
// Path Utilities
// ============================================================================

/**
 * Using native Node.js path.posix.normalize for consistent forward slash behavior
 * Normalizes paths to use forward slashes on all platforms for consistency.
 *
 * @param inputPath The path to normalize
 * @returns The normalized path with forward slashes, or the original value if null/undefined
 */
export function normalizePath(inputPath: string): string;
export function normalizePath(inputPath: null): null;
export function normalizePath(inputPath: undefined): undefined;
export function normalizePath(inputPath: string | null | undefined): string | null | undefined;
export function normalizePath(inputPath: string | null | undefined): string | null | undefined {
  if (inputPath == null) return inputPath; // handles both null and undefined
  if (inputPath === '') return inputPath;
  // Convert all backslashes to forward slashes, then normalize
  return path.posix.normalize(inputPath.replace(/\\/g, '/'));
}
