/**
 * Knowledge Pack Loader
 * Loads and manages static knowledge packs for AI enhancement
 *
 * @see {@link ../../docs/adr/003-knowledge-enhancement.md ADR-003: Knowledge Enhancement System}
 */

import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { resolve, join, dirname } from 'node:path';
import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from './schemas';
import { z } from 'zod';

const logger = createLogger().child({ module: 'knowledge-loader' });

interface KnowledgeState {
  entries: Map<string, LoadedEntry>;
  byCategory: Map<string, LoadedEntry[]>;
  byTag: Map<string, LoadedEntry[]>;
  loaded: boolean;
}

const knowledgeState: KnowledgeState = {
  entries: new Map(),
  byCategory: new Map(),
  byTag: new Map(),
  loaded: false,
};

/**
 * Discover built-in knowledge pack files from the knowledge/packs directory
 * Returns paths to all .json files
 *
 * Searches relative to the module's installation location first,
 * then falls back to searching upward from process.cwd().
 * Works in both ESM (dist/) and CJS (dist-cjs/) builds, and when installed via npm.
 */
function discoverBuiltInKnowledgePacks(): string[] {
  try {
    const searchPaths: string[] = [];

    // 1. First, try relative to the installed module location
    // This ensures knowledge packs are found when the package is installed via npm

    let modulePathResolved = false;

    // Try CJS approach first (for when imported via require())
    try {
      const dirName = new Function('return typeof __dirname !== "undefined" ? __dirname : undefined')();
      if (typeof dirName === 'string') {
        const moduleRelativePath = resolve(dirName, '../../../knowledge/packs');
        searchPaths.push(moduleRelativePath);
        modulePathResolved = true;
        logger.debug({ dirName, moduleRelativePath, method: 'CJS __dirname' }, 'Resolved module path for knowledge pack discovery');
      }
    } catch (error) {
      logger.debug({ error }, 'CJS __dirname not available');
    }

    // ESM approach: try to infer from process.argv[1] (the script being executed)
    // When running the CLI, process.argv[1] points to the cli.js file
    if (!modulePathResolved && process.argv && process.argv[1]) {
      try {
        // If argv[1] is something like /path/to/node_modules/package/dist/src/cli/cli.js
        // We can infer the package root
        const scriptPath = process.argv[1];
        logger.info({ scriptPath, argv: process.argv }, 'Attempting to resolve package root from process.argv');

        const distIndex = scriptPath.indexOf('/dist/src/');
        if (distIndex !== -1) {
          const packageRoot = scriptPath.substring(0, distIndex);
          const moduleRelativePath = join(packageRoot, 'knowledge/packs');
          searchPaths.push(moduleRelativePath);
          modulePathResolved = true;
          logger.info({ scriptPath, packageRoot, moduleRelativePath, method: 'process.argv[1]' }, 'Resolved module path for knowledge pack discovery');
        } else {
          logger.warn({ scriptPath, searched: '/dist/src/' }, 'Could not find /dist/src/ in script path');
        }
      } catch (error) {
        logger.warn({ error }, 'Failed to resolve module path from process.argv[1]');
      }
    }

    if (!modulePathResolved) {
      logger.warn('Could not resolve module path for built-in knowledge packs - will search from cwd');
    }

    // 2. Then search upward from current working directory (for development)
    let currentDir = process.cwd();
    searchPaths.push(join(currentDir, 'knowledge/packs'));

    let attempts = 0;
    const maxAttempts = 5;
    while (attempts < maxAttempts) {
      const parentDir = dirname(currentDir);
      if (parentDir === currentDir) {
        // Reached filesystem root
        break;
      }
      currentDir = parentDir;
      searchPaths.push(join(currentDir, 'knowledge/packs'));
      attempts++;
    }

    // Try each search path until we find one that exists
    logger.debug({ searchPaths, totalPaths: searchPaths.length }, 'Searching for knowledge packs in paths');

    for (const packsDir of searchPaths) {
      logger.debug({ path: packsDir, exists: existsSync(packsDir) }, 'Checking knowledge pack path');

      if (existsSync(packsDir)) {
        // Find all .json files
        const files = readdirSync(packsDir)
          .filter((file) => file.endsWith('.json'))
          .map((file) => resolve(join(packsDir, file)));

        logger.debug({ count: files.length, dir: packsDir, files: files.slice(0, 3) }, 'Found files in knowledge pack directory');

        if (files.length > 0) {
          logger.info({ count: files.length, dir: packsDir }, 'Discovered built-in knowledge packs');
          return files;
        }
      }
    }

    logger.error({ searchPaths, cwd: process.cwd() }, 'FATAL: No knowledge packs found in any search path');
    return [];
  } catch (error) {
    logger.warn({ error }, 'Failed to discover built-in knowledge packs');
    return [];
  }
}

/**
 * Validate and normalize pack structure
 * Handles both array and object-wrapped pack formats
 */
const validateAndNormalizePack = (
  packFile: string,
  data: unknown,
): { valid: boolean; entries?: KnowledgeEntry[] } => {
  try {
    const validated = KnowledgePackSchema.parse(data);

    // Extract entries based on format
    // Cast to KnowledgeEntry[] since Zod validation ensures compatibility
    let entries: KnowledgeEntry[];
    if (Array.isArray(validated)) {
      // Format 1: Flat array of entries
      entries = validated as KnowledgeEntry[];
    } else {
      // Format 2: Object with metadata and rules array
      entries = validated.rules as KnowledgeEntry[];
    }

    return { valid: true, entries };
  } catch (error) {
    if (error instanceof z.ZodError) {
      logger.warn(
        {
          pack: packFile,
          errors: error.issues.slice(0, 5).map((e: z.ZodIssue) => ({
            path: e.path.join('.'),
            message: e.message,
          })),
          totalErrors: error.issues.length,
        },
        'Pack validation failed',
      );
    }
    return { valid: false };
  }
};

const validateEntry = (entry: unknown): entry is KnowledgeEntry => {
  try {
    KnowledgeEntrySchema.parse(entry);
    return true;
  } catch (error) {
    if (error instanceof z.ZodError) {
      logger.warn(
        {
          entryId: (entry as { id?: string })?.id || 'unknown',
          errors: error.issues.map((e: z.ZodIssue) => ({
            path: e.path.join('.'),
            message: e.message,
          })),
        },
        'Entry validation failed',
      );
    }
    return false;
  }
};

const addEntry = (entry: KnowledgeEntry): void => {
  // No pattern compilation - patterns are compiled on-demand during matching
  knowledgeState.entries.set(entry.id, entry);
};

const buildIndices = (): void => {
  // Clear existing indices
  knowledgeState.byCategory.clear();
  knowledgeState.byTag.clear();

  for (const entry of knowledgeState.entries.values()) {
    // Index by category
    if (!knowledgeState.byCategory.has(entry.category)) {
      knowledgeState.byCategory.set(entry.category, []);
    }
    knowledgeState.byCategory.get(entry.category)?.push(entry);

    // Index by tags
    if (entry.tags) {
      for (const tag of entry.tags) {
        if (!knowledgeState.byTag.has(tag)) {
          knowledgeState.byTag.set(tag, []);
        }
        knowledgeState.byTag.get(tag)?.push(entry);
      }
    }
  }
};

const getTopTags = (limit: number): Array<{ tag: string; count: number }> => {
  const tagCounts: Record<string, number> = {};

  for (const entry of knowledgeState.entries.values()) {
    if (entry.tags) {
      for (const tag of entry.tags) {
        tagCounts[tag] = (tagCounts[tag] || 0) + 1;
      }
    }
  }

  return Object.entries(tagCounts)
    .sort(([, a], [, b]) => b - a)
    .slice(0, limit)
    .map(([tag, count]) => ({ tag, count }));
};

/**
 * Load knowledge entries from built-in knowledge packs
 * Throws an error if any built-in pack fails to load
 */
export const loadKnowledgeBase = (): void => {
  if (knowledgeState.loaded) {
    return;
  }

  const stats = {
    packsAttempted: 0,
    packsLoaded: 0,
    packsFailed: 0,
    entriesValid: 0,
    entriesInvalid: 0,
    failures: [] as Array<{ file: string; error: string }>,
  };

  try {
    // Discover built-in knowledge packs at runtime
    const packPaths = discoverBuiltInKnowledgePacks();
    stats.packsAttempted = packPaths.length;

    if (packPaths.length === 0) {
      throw new Error('No knowledge packs discovered - server cannot start without knowledge base');
    }

    logger.info({ totalPacks: packPaths.length }, 'Loading built-in knowledge packs');

    // Load each discovered pack
    for (const packPath of packPaths) {
      try {
        // Read and parse JSON file
        const fileContent = readFileSync(packPath, 'utf-8');
        const data = JSON.parse(fileContent);

        // Validate and normalize pack structure
        const result = validateAndNormalizePack(packPath, data);
        if (!result.valid || !result.entries) {
          const error = 'Pack validation failed (see previous log)';
          stats.packsFailed++;
          stats.failures.push({
            file: packPath,
            error,
          });
          // Throw error for built-in packs - they must all load successfully
          throw new Error(`Failed to load built-in knowledge pack ${packPath}: ${error}`);
        }

        const entries = result.entries;
        logger.debug({ pack: packPath, count: entries.length }, 'Loading knowledge pack');

        // Validate and add individual entries
        for (const entry of entries) {
          if (validateEntry(entry)) {
            addEntry(entry);
            stats.entriesValid++;
          } else {
            stats.entriesInvalid++;
          }
        }

        stats.packsLoaded++;
      } catch (packError) {
        stats.packsFailed++;
        const errorMessage = String(packError);
        stats.failures.push({
          file: packPath,
          error: errorMessage,
        });
        logger.error({ pack: packPath, error: packError }, 'Failed to load knowledge pack');
        // Re-throw the error to ensure server startup fails
        throw new Error(`Failed to load built-in knowledge pack ${packPath}: ${errorMessage}`);
      }
    }

    buildIndices();
    knowledgeState.loaded = true;

    logger.info(
      {
        packsAttempted: stats.packsAttempted,
        packsLoaded: stats.packsLoaded,
        packsFailed: stats.packsFailed,
        entriesValid: stats.entriesValid,
        entriesInvalid: stats.entriesInvalid,
        totalEntries: knowledgeState.entries.size,
        categories: Array.from(knowledgeState.byCategory.keys()),
        topTags: getTopTags(5),
      },
      'Knowledge base loaded',
    );
  } catch (error) {
    logger.error({ error }, 'Failed to load knowledge base');
    // Re-throw to ensure server startup fails
    throw error;
  }
};

/**
 * Get all entries
 */
export const getAllEntries = (): LoadedEntry[] => {
  return Array.from(knowledgeState.entries.values());
};

/**
 * Check if knowledge base is loaded
 */
export const isKnowledgeLoaded = (): boolean => {
  return knowledgeState.loaded;
};

/**
 * Load knowledge data and return entries.
 * Used by prompt engine for knowledge selection.
 */
export const loadKnowledgeData = (): { entries: LoadedEntry[] } => {
  if (!isKnowledgeLoaded()) {
    loadKnowledgeBase();
  }
  return {
    entries: getAllEntries(),
  };
};
