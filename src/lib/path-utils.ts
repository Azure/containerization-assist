import path from 'node:path';

// Re-export standard functions - no over-abstraction
export const joinPaths = path.join;
export const resolvePath = path.resolve;
export const getRelativePath = path.relative;
export const getFileName = path.basename;
export const getDirectory = path.dirname;
export const getExtension = path.extname;
export const parsePath = path.parse;

// Only add custom logic where actually needed
export function normalizePathSeparators(filePath: string): string {
  return filePath.replace(/\\/g, '/');
}

// Normalize Windows paths and handle potential escape sequence issues
export function safeNormalizePath(filePath: string): string {
  if (!filePath) return filePath;

  // First normalize backslashes to forward slashes to prevent escape sequence interpretation
  let normalized = filePath.replace(/\\/g, '/');

  // Handle Git Bash/MinGW style paths that might get duplicated
  // e.g., "/c/Users/..." or "c:/Users/..." should not get an extra "c/" prepended
  // This can happen when path.resolve() is called on Windows with certain shells

  // Remove duplicate drive letter patterns like "/c/c/..." or "c:/c/..."
  // Pattern 1: /c/c/ -> /c/
  normalized = normalized.replace(/^\/([a-zA-Z])\/\1\//i, '/$1/');
  // Pattern 2: c:/c/ -> c:/
  normalized = normalized.replace(/^([a-zA-Z]):\/\1\//i, '$1:/');
  // Pattern 3: C:\c\Users -> C:\Users (handling the exact case from the screenshot)
  normalized = normalized.replace(/^([a-zA-Z]):\/([a-zA-Z])\//i, (match, drive1, drive2) => {
    // If the drive letters match (case-insensitive), remove the duplicate
    if (drive1.toLowerCase() === drive2.toLowerCase()) {
      return `${drive1}:/`;
    }
    return match;
  });

  // Pattern 4: Handle full path duplication like C:/Users/.../path/C:/Users/.../path
  // This can happen when path.resolve is called on an already absolute path
  const pathPattern = /^([a-zA-Z]:\/[^:]+)\/\1/;
  const match = normalized.match(pathPattern);
  if (match?.[1]) {
    // Remove the duplicate path
    normalized = match[1];
  }

  // Also check for pattern where the path appears twice with the second occurrence starting with drive letter
  // e.g., "C:/Users/path/to/project/C:/Users/path/to/project"
  const driveLetterPattern = /^([a-zA-Z]):\/(.+)\/[a-zA-Z]:\//;
  const driveMatch = normalized.match(driveLetterPattern);
  if (driveMatch) {
    const firstPart = `${driveMatch[1]}:/${driveMatch[2]}`;
    // Check if the path after the second drive letter matches the beginning
    if (normalized.includes(`${firstPart}/${firstPart}`)) {
      normalized = firstPart;
    }
  }

  // Handle potential double slashes that might occur
  // Check if it's a UNC path (//server/share) - these start with exactly two slashes followed by non-slash
  const isUncPath = normalized.match(/^\/\/[^/]/);

  if (isUncPath) {
    // For UNC paths, preserve the initial // but collapse other multiple slashes
    const uncPrefix = normalized.substring(0, 2);
    const restOfPath = normalized.substring(2).replace(/\/+/g, '/');
    normalized = uncPrefix + restOfPath;
  } else {
    // For all other paths, collapse multiple slashes to single slash
    normalized = normalized.replace(/\/+/g, '/');
  }

  return normalized;
}
