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
  let compiledPattern: RegExp | undefined;

  if (entry.pattern) {
    try {
      compiledPattern = new RegExp(entry.pattern, 'gmi');
    } catch (error) {
      logger.warn(
        { entryId: entry.id, pattern: entry.pattern, error },
        'Failed to compile pattern - will skip pattern matching for this entry',
      );
    }
  }

  const loadedEntry: LoadedEntry = compiledPattern ? { ...entry, compiledPattern } : { ...entry };

  knowledgeState.entries.set(entry.id, loadedEntry);
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
    // Find packs directory
    const possiblePacksDirs = [
      path.resolve(process.cwd(), 'knowledge/packs'),
      path.resolve(process.cwd(), 'dist/knowledge/packs'),
      path.resolve(process.cwd(), 'node_modules/containerization-assist-mcp/knowledge/packs'),
    ];

    const packsDir = findExistingPath(possiblePacksDirs);
    if (!packsDir) {
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
export const loadKnowledgeData = async (): Promise<void> => {
  if (!isKnowledgeLoaded()) {
    await loadKnowledgeBase();
  }
};

/**
 * Get entries by category using index.
 */
export const getEntriesByCategory = (category: string): LoadedEntry[] => {
  if (!isKnowledgeLoaded()) {
    logger.warn('Knowledge base not loaded, cannot filter by category');
    return [];
  }
  return knowledgeState.byCategory.get(category) || [];
};

/**
 * Get entries by tag using index.
 */
export const getEntriesByTag = (tag: string): LoadedEntry[] => {
  if (!isKnowledgeLoaded()) {
    logger.warn('Knowledge base not loaded, cannot filter by tag');
    return [];
  }
  return knowledgeState.byTag.get(tag) || [];
};
