/**
 * Cross-platform system detection utilities
 */

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
