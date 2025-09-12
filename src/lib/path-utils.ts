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
