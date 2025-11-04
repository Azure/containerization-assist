/**
 * Simple .dockerignore parser
 * Implements basic Docker ignore pattern matching without external dependencies
 */

import { promises as fs } from 'fs';
import path from 'path';

/**
 * Parse .dockerignore file content into patterns
 */
export function parseDockerignore(content: string): { patterns: string[]; exceptions: string[] } {
  const lines = content.split('\n');
  const patterns: string[] = [];
  const exceptions: string[] = [];

  for (const line of lines) {
    const trimmed = line.trim();

    if (!trimmed || trimmed.startsWith('#')) {
      continue;
    }

    if (trimmed.startsWith('!')) {
      exceptions.push(trimmed.substring(1).trim());
    } else {
      patterns.push(trimmed);
    }
  }

  return { patterns, exceptions };
}

/**
 * Check if a path matches a dockerignore pattern
 * Implements simplified Docker pattern matching:
 * - * matches any sequence of non-separator characters
 * - ** matches any sequence including separators
 * - ? matches any single non-separator character
 */
export function matchPattern(filePath: string, pattern: string): boolean {
  const normalizedPath = filePath.replace(/\\/g, '/');
  const normalizedPattern = pattern.replace(/\\/g, '/');
  const hasLeadingSlash = normalizedPattern.startsWith('/');

  const cleanPattern = normalizedPattern.replace(/^\/+|\/+$/g, '');
  const cleanPath = normalizedPath.replace(/^\/+|\/+$/g, '');

  let regexPattern = cleanPattern
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*/g, '<!DOUBLESTAR!>')
    .replace(/\*/g, '[^/]*')
    .replace(/<!DOUBLESTAR!>/g, '.*?')
    .replace(/\?/g, '[^/]');

  const hasSlash = cleanPattern.includes('/');
  const startsWithDoubleStar = cleanPattern.startsWith('**');

  if (startsWithDoubleStar) {
    regexPattern = `^${regexPattern}`;
  } else if (hasLeadingSlash || hasSlash) {
    regexPattern = `^${regexPattern}(/|$)`;
  } else {
    regexPattern = `(^|/)${regexPattern}(/|$)`;
  }

  const regex = new RegExp(regexPattern);
  return regex.test(cleanPath);
}

/**
 * Check if a file should be ignored based on dockerignore rules
 */
export function shouldIgnore(filePath: string, patterns: string[], exceptions: string[]): boolean {
  const isIgnored = patterns.some((pattern) => matchPattern(filePath, pattern));

  if (!isIgnored) {
    return false;
  }

  const isException = exceptions.some((exception) => matchPattern(filePath, exception));
  return !isException;
}

/**
 * Read and parse .dockerignore file from a directory
 */
export async function readDockerignore(
  contextPath: string,
): Promise<{ patterns: string[]; exceptions: string[] } | null> {
  const dockerignorePath = path.join(contextPath, '.dockerignore');

  try {
    const content = await fs.readFile(dockerignorePath, 'utf-8');
    return parseDockerignore(content);
  } catch {
    return null;
  }
}

/**
 * Create an ignore filter function for tar-fs
 * @param patterns - Ignore patterns from .dockerignore
 * @param exceptions - Exception patterns (starting with !)
 * @param alwaysInclude - Files that should never be ignored (e.g., Dockerfile)
 */
export function createIgnoreFunction(
  patterns: string[],
  exceptions: string[],
  alwaysInclude: string[] = [],
): (name: string) => boolean {
  return (name: string) => {
    const normalizedName = name.replace(/\\/g, '/');

    for (const includePath of alwaysInclude) {
      const normalizedInclude = includePath.replace(/\\/g, '/');
      if (
        normalizedName === normalizedInclude ||
        normalizedName.endsWith(`/${normalizedInclude}`)
      ) {
        return false;
      }
    }

    return shouldIgnore(name, patterns, exceptions);
  };
}
