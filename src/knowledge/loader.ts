/**
 * Knowledge Pack Loader
 * Loads and manages static knowledge packs for AI enhancement
 *
 * @see {@link ../../docs/adr/003-knowledge-enhancement.md ADR-003: Knowledge Enhancement System}
 */

import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from './schemas';
import { z } from 'zod';
import { readFileSync, existsSync, readdirSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const logger = createLogger().child({ module: 'knowledge-loader' });

// Capture import.meta.url at module load time (ESM only)
// This will be undefined in CJS builds, which is expected
// Using eval to avoid TypeScript compilation errors in CJS target
let moduleUrl: string | undefined;
try {
  // eslint-disable-next-line @typescript-eslint/no-implied-eval, no-eval
  moduleUrl = eval('import.meta.url');
} catch {
  // CJS build or import.meta not available
  moduleUrl = undefined;
}

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

const findExistingPath = (paths: readonly string[]): string | null => {
  for (const path of paths) {
    if (existsSync(path)) {
      return path;
    }
  }
  return null;
};

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
 * Get the package root directory by resolving from this module's location
 * Works in both ESM and CJS builds
 *
 * Strategy priority:
 * 1. Use import.meta.url (captured at module load) in ESM builds
 * 2. Use require.main.path in CJS builds
 * 3. Fall back to process.cwd()
 */
const getPackageRoot = (): string => {
  // Strategy 1: ESM - use captured import.meta.url
  // When built to ESM: dist/src/knowledge/loader.js -> go up 3 levels to package root
  // When in dev TSX: src/knowledge/loader.ts -> go up 2 levels to repo root
  if (moduleUrl) {
    try {
      const currentFile = fileURLToPath(moduleUrl);
      const moduleDir = path.dirname(currentFile);
      const isBuilt = moduleDir.includes('/dist/src/');
      const levelsUp = isBuilt ? 3 : 2;
      return path.resolve(moduleDir, ...Array(levelsUp).fill('..'));
    } catch (err) {
      logger.debug({ moduleUrl, error: err }, 'Failed to resolve from import.meta.url');
    }
  }

  // Strategy 2: CJS - use require.main path
  // In packaged mode, entry point is usually dist/src/cli/cli.js
  try {
    // Use globalThis to avoid TypeScript errors
    const req = (globalThis as { require?: NodeRequire }).require;
    if (req?.main?.path) {
      // Go up from entry point to package root
      return path.resolve(req.main.path, '../..');
    }
  } catch (err) {
    logger.debug({ error: err }, 'Failed to resolve from require.main');
  }

  // Strategy 3: Last resort - assume we're in repo/package root
  logger.debug({ cwd: process.cwd() }, 'Using process.cwd() as package root');
  return process.cwd();
};

/**
 * Load knowledge entries from all knowledge packs
 */
export const loadKnowledgeBase = async (): Promise<void> => {
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
    // Get package root and search for knowledge packs
    const packageRoot = getPackageRoot();

    // Find packs directory with multiple fallback strategies
    const possiblePacksDirs = [
      // 1. Relative to package root (works for both dev and installed)
      path.join(packageRoot, 'knowledge/packs'),
      // 2. Development mode (from repo root)
      path.resolve(process.cwd(), 'knowledge/packs'),
      // 3. Installed as dependency
      path.resolve(process.cwd(), 'node_modules/containerization-assist-mcp/knowledge/packs'),
    ];

    const packsDir = findExistingPath(possiblePacksDirs);
    if (!packsDir) {
      logger.error(
        {
          packageRoot,
          searchedPaths: possiblePacksDirs,
        },
        'Could not find knowledge packs directory',
      );
      throw new Error('Could not find knowledge packs directory');
    }

    // Discover all .json files in packs directory
    const packFiles = readdirSync(packsDir)
      .filter((file) => file.endsWith('.json'))
      .sort();
    stats.packsAttempted = packFiles.length;

    logger.info({ packsDir, totalPacks: packFiles.length }, 'Discovered knowledge packs');

    // Load each pack
    for (const packFile of packFiles) {
      try {
        const packPath = path.join(packsDir, packFile);
        const content = readFileSync(packPath, 'utf-8');

        // Parse JSON
        let data: unknown;
        try {
          data = JSON.parse(content);
        } catch (parseError) {
          throw new Error(`Invalid JSON: ${parseError}`);
        }

        // Validate and normalize pack structure
        const result = validateAndNormalizePack(packFile, data);
        if (!result.valid || !result.entries) {
          stats.packsFailed++;
          stats.failures.push({
            file: packFile,
            error: 'Pack validation failed (see previous log)',
          });
          continue;
        }

        const entries = result.entries;
        logger.debug({ pack: packFile, count: entries.length }, 'Loading knowledge pack');

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
        stats.failures.push({
          file: packFile,
          error: String(packError),
        });
        logger.warn({ pack: packFile, error: packError }, 'Failed to load knowledge pack');
      }
    }

    buildIndices();
    knowledgeState.loaded = true;

    // Log summary
    if (stats.failures.length > 0) {
      logger.warn({ failures: stats.failures }, `Failed to load ${stats.packsFailed} packs`);
    }

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
export const loadKnowledgeData = async (): Promise<{ entries: LoadedEntry[] }> => {
  if (!isKnowledgeLoaded()) {
    await loadKnowledgeBase();
  }
  return {
    entries: getAllEntries(),
  };
};
