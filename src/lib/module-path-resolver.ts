/**
 * Module Path Resolver
 * Shared utility for resolving module-relative paths in both ESM and CJS builds
 * Used by knowledge pack loader and policy discovery
 */

import { existsSync, realpathSync } from 'node:fs';
import { resolve, join, dirname, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Logger } from 'pino';

// ===== Constants =====

/**
 * Markers for detecting package root from built distribution paths
 *
 * We check for both Unix and Windows path separators to ensure
 * cross-platform compatibility:
 * - Unix/Linux/macOS: /dist/src/
 * - Windows: \dist\src\
 *
 * The first marker uses the platform-specific separator (path.sep),
 * while the second is a fallback for Unix-style paths on Windows
 * (some tools normalize to forward slashes even on Windows).
 */
const DIST_SRC_MARKERS = [
  `${sep}dist${sep}src${sep}`, // Platform-specific separator
  '/dist/src/', // Fallback for normalized paths
] as const;

/**
 * Maximum number of parent directories to traverse when searching
 * for resources (policies, knowledge packs, etc.)
 */
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
 *
 * Handles npm bin symlinks and works across platforms.
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

    // Try to find the dist/src marker in the script path
    // Check both platform-specific and Unix-style separators
    let distIndex = -1;
    let foundMarker: string | undefined;

    for (const marker of DIST_SRC_MARKERS) {
      const index = scriptPath.indexOf(marker);
      if (index !== -1) {
        distIndex = index;
        foundMarker = marker;
        break;
      }
    }

    if (distIndex !== -1 && foundMarker) {
      const packageRoot = scriptPath.substring(0, distIndex);
      const moduleRelativePath = join(packageRoot, targetSubdir);

      logger.debug(
        {
          scriptPath,
          packageRoot,
          moduleRelativePath,
          marker: foundMarker,
          platform: process.platform,
          method: 'process.argv[1]',
        },
        'Resolved module path from argv',
      );

      return {
        resolved: true,
        path: moduleRelativePath,
        method: 'process.argv[1]',
      };
    }

    logger.debug(
      {
        scriptPath,
        searchedMarkers: DIST_SRC_MARKERS,
        platform: process.platform,
      },
      'Could not find dist/src marker in script path',
    );
  } catch (error) {
    logger.debug(
      { error, argv1: process.argv[1] },
      'Failed to resolve module path from process.argv[1]',
    );
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
      logger.debug(
        {
          method: result.method,
          path: result.path,
          totalPaths: searchPaths.length + generateCwdSearchPaths(relativePath).length,
        },
        'Module path resolved via strategy',
      );
      break;
    }
  }

  if (searchPaths.length === 0) {
    logger.debug(
      {
        relativePath,
        triedStrategies: ['CJS __dirname', 'ESM import.meta.url', 'process.argv[1]'],
        platform: process.platform,
      },
      'Could not resolve module path via any strategy - will search from cwd',
    );
  }

  // Add fallback paths from cwd
  const cwdPaths = generateCwdSearchPaths(relativePath);
  searchPaths.push(...cwdPaths);

  logger.debug(
    {
      searchPaths,
      totalPaths: searchPaths.length,
      targetDir: relativePath,
      platform: process.platform,
      cwd: process.cwd(),
    },
    'Generated module search paths',
  );

  return searchPaths;
}

/**
 * Find the first existing directory from a list of search paths
 *
 * @param searchPaths - Array of paths to check
 * @param logger - Optional logger for debugging
 * @returns The first path that exists as a directory, or undefined
 */
export function findExistingDirectory(
  searchPaths: string[],
  logger?: Logger,
): string | undefined {
  for (const path of searchPaths) {
    if (existsSync(path)) {
      logger?.debug({ path, exists: true }, 'Found existing directory');
      return path;
    } else {
      logger?.debug({ path, exists: false }, 'Directory does not exist');
    }
  }

  logger?.debug(
    {
      searchPaths,
      totalSearched: searchPaths.length,
      platform: process.platform,
    },
    'No directories found in search paths',
  );

  return undefined;
}
