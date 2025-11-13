/**
 * Module Path Resolver
 * Shared utility for resolving module-relative paths in both ESM and CJS builds
 * Used by knowledge pack loader and policy discovery
 */

import { existsSync, realpathSync } from 'node:fs';
import { resolve, join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Logger } from 'pino';

// ===== Constants =====

const DIST_SRC_MARKER = '/dist/src/';
const MAX_PARENT_DIR_TRAVERSALS = 5;

// ===== Types =====

interface PathResolutionResult {
  resolved: true;
  path: string;
  method: string;
}

interface PathResolutionFailure {
  resolved: false;
}

type PathResolution = PathResolutionResult | PathResolutionFailure;

export interface ModulePathResolverOptions {
  /** Relative path from package root (e.g., 'policies', 'knowledge/packs') */
  relativePath: string;
  /** Logger instance for debug/info messages */
  logger: Logger;
  /** Module URL captured at module scope (import.meta.url) */
  moduleUrl?: string;
}

// ===== Path Resolution Strategies =====

/**
 * Attempt to resolve module path using CJS __dirname
 */
function tryResolveCJSPath(
  relativePathFromDistSrc: string,
  logger: Logger,
): PathResolution {
  try {
    const dirName = new Function(
      'return typeof __dirname !== "undefined" ? __dirname : undefined',
    )() as string | undefined;

    if (typeof dirName === 'string') {
      // From dist/src/knowledge or dist-cjs/src/app:
      //   Go up to package root, then down to target directory
      const moduleRelativePath = resolve(dirName, relativePathFromDistSrc);
      logger.debug(
        { moduleRelativePath, method: 'CJS __dirname' },
        'Resolved module path',
      );
      return { resolved: true, path: moduleRelativePath, method: 'CJS __dirname' };
    }
  } catch (error) {
    logger.debug({ error }, 'Failed to resolve module path from __dirname');
  }

  return { resolved: false };
}

/**
 * Attempt to resolve module path using ESM import.meta.url
 */
function tryResolveESMPath(
  moduleUrl: string | undefined,
  relativePathFromDistSrc: string,
  logger: Logger,
): PathResolution {
  if (!moduleUrl) {
    return { resolved: false };
  }

  try {
    const __filename = fileURLToPath(moduleUrl);
    const __dirname = dirname(__filename);
    const moduleRelativePath = resolve(__dirname, relativePathFromDistSrc);

    logger.debug(
      { moduleRelativePath, method: 'ESM import.meta.url' },
      'Resolved module path',
    );
    return { resolved: true, path: moduleRelativePath, method: 'ESM import.meta.url' };
  } catch (error) {
    logger.debug({ error }, 'Failed to resolve module path from import.meta.url');
  }

  return { resolved: false };
}

/**
 * Resolve symlinks to get the actual file path (important for npm bin wrappers)
 */
function resolveSymlink(path: string, logger: Logger): string {
  try {
    return realpathSync(path);
  } catch (error) {
    logger.debug({ path, error }, 'Could not resolve symlink, using original path');
    return path;
  }
}

/**
 * Attempt to resolve module path from process.argv[1] (CLI entrypoint)
 */
function tryResolveFromArgv(
  targetSubdir: string,
  logger: Logger,
): PathResolution {
  if (!process.argv[1]) {
    return { resolved: false };
  }

  try {
    // Resolve symlinks to get the actual file path (important for npm bin wrappers)
    const scriptPath = resolveSymlink(resolve(process.argv[1]), logger);
    const distIndex = scriptPath.indexOf(DIST_SRC_MARKER);

    if (distIndex !== -1) {
      const packageRoot = scriptPath.substring(0, distIndex);
      const moduleRelativePath = join(packageRoot, targetSubdir);

      logger.info(
        { scriptPath, packageRoot, moduleRelativePath, method: 'process.argv[1]' },
        'Resolved module path',
      );
      return { resolved: true, path: moduleRelativePath, method: 'process.argv[1]' };
    }

    logger.warn(
      { scriptPath, searched: DIST_SRC_MARKER },
      'Could not find marker in script path',
    );
  } catch (error) {
    logger.warn({ error }, 'Failed to resolve module path from process.argv[1]');
  }

  return { resolved: false };
}

/**
 * Generate search paths by walking up from current working directory
 */
function generateCwdSearchPaths(targetSubdir: string): string[] {
  const paths: string[] = [];
  let currentDir = process.cwd();
  paths.push(join(currentDir, targetSubdir));

  for (let i = 0; i < MAX_PARENT_DIR_TRAVERSALS; i++) {
    const parentDir = dirname(currentDir);
    if (parentDir === currentDir) {
      // Reached filesystem root
      break;
    }
    currentDir = parentDir;
    paths.push(join(currentDir, targetSubdir));
  }

  return paths;
}

// ===== Public API =====

/**
 * Resolve module-relative paths using a chain of strategies
 *
 * This function attempts multiple strategies to find the correct path to a
 * resource directory (like 'policies' or 'knowledge/packs') that ships with
 * the package.
 *
 * Search priority:
 *  1. Relative to the installed module location (CJS __dirname)
 *  2. Relative to the installed module location (ESM import.meta.url)
 *  3. Heuristic based on process.argv[1] (CLI entrypoint with symlink resolution)
 *  4. Walk upward from process.cwd() (dev / repo root)
 *
 * Works in all deployment scenarios:
 *  - Local development (ts-node / tsx / node from repo root)
 *  - Installed globally via npm and run via CLI
 *  - Used as a dependency inside another project
 *  - Executed through npm bin symlinks
 *
 * @returns Array of search paths in priority order
 */
export function resolveModulePaths(options: ModulePathResolverOptions): string[] {
  const { relativePath, logger, moduleUrl } = options;
  const searchPaths: string[] = [];

  // Calculate the relative path from dist/src to the target
  // Examples:
  //   'policies' -> '../../../policies'
  //   'knowledge/packs' -> '../../../knowledge/packs'
  const relativePathFromDistSrc = `../../../${relativePath}`;

  // Try resolution strategies in priority order
  const strategies = [
    () => tryResolveCJSPath(relativePathFromDistSrc, logger),
    () => tryResolveESMPath(moduleUrl, relativePathFromDistSrc, logger),
    () => tryResolveFromArgv(relativePath, logger),
  ];

  for (const strategy of strategies) {
    const result = strategy();
    if (result.resolved) {
      searchPaths.push(result.path);
      break;
    }
  }

  if (searchPaths.length === 0) {
    logger.debug('Could not resolve module path - will search from cwd');
  }

  // Add fallback paths from cwd
  searchPaths.push(...generateCwdSearchPaths(relativePath));

  logger.debug(
    { searchPaths, totalPaths: searchPaths.length, targetDir: relativePath },
    'Generated module search paths',
  );

  return searchPaths;
}

/**
 * Find the first existing directory from a list of search paths
 *
 * @param searchPaths - Array of paths to check
 * @returns The first path that exists as a directory, or undefined
 */
export function findExistingDirectory(searchPaths: string[]): string | undefined {
  for (const path of searchPaths) {
    if (existsSync(path)) {
      return path;
    }
  }
  return undefined;
}
