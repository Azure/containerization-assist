/**
 * Platform and path utilities
 *
 * Consolidates cross-platform system detection and path normalization
 * for consistent behavior across different operating systems.
 */

import * as path from 'path';
import type { DockerPlatform } from '@/tools/shared/schemas';

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

/**
 * Map Kubernetes node architecture to Docker platform format
 */
export function mapNodeArchToPlatform(arch: string, os: string = 'linux'): DockerPlatform | null {
  const normalized = arch.toLowerCase();

  // Map common Kubernetes arch values to Docker platform format
  const archMap: Record<string, string> = {
    'amd64': 'amd64',
    'x86_64': 'amd64',
    'arm64': 'arm64',
    'aarch64': 'arm64',
    'armv7l': 'arm/v7',
    'armv6l': 'arm/v6',
    '386': '386',
    'i386': '386',
    'i686': '386',
    'ppc64le': 'ppc64le',
    's390x': 's390x',
    'riscv64': 'riscv64',
  };

  const mappedArch = archMap[normalized];
  if (!mappedArch) {
    return null;
  }

  const platform = `${os}/${mappedArch}`;

  // Validate against known platforms
  const validPlatforms: DockerPlatform[] = [
    'linux/amd64',
    'linux/arm64',
    'linux/arm/v7',
    'linux/arm/v6',
    'linux/386',
    'linux/ppc64le',
    'linux/s390x',
    'linux/riscv64',
    'windows/amd64',
  ];

  return validPlatforms.includes(platform as DockerPlatform) ? (platform as DockerPlatform) : null;
}

/**
 * Check if a target platform is compatible with a cluster platform.
 * Returns true if images built for targetPlatform can run on clusterPlatform.
 *
 * @param targetPlatform - Platform images are built for (e.g., "linux/amd64")
 * @param clusterPlatform - Platform cluster nodes support (e.g., "linux/amd64")
 * @returns true if compatible, false otherwise
 */
export function isPlatformCompatible(
  targetPlatform: DockerPlatform,
  clusterPlatform: DockerPlatform,
): boolean {
  // Exact match is always compatible
  if (targetPlatform === clusterPlatform) {
    return true;
  }

  // ARM64 can run ARM/v7 and ARM/v6 binaries
  if (clusterPlatform === 'linux/arm64') {
    return targetPlatform === 'linux/arm/v7' || targetPlatform === 'linux/arm/v6';
  }

  // AMD64 can run 386 binaries
  if (clusterPlatform === 'linux/amd64' && targetPlatform === 'linux/386') {
    return true;
  }

  // Different platforms are generally not compatible without emulation
  return false;
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
export function normalizePath(inputPath: string | null | undefined): string | null | undefined;
export function normalizePath(inputPath: string | null | undefined): string | null | undefined {
  if (inputPath == null) return inputPath; // handles both null and undefined
  if (inputPath === '') return inputPath;
  // Convert all backslashes to forward slashes, then normalize
  return path.posix.normalize(inputPath.replace(/\\/g, '/'));
}
