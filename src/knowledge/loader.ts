/**
 * Knowledge Pack Loader
 * Loads and manages static knowledge packs for AI enhancement
 *
 * Hybrid loading strategy:
 * 1. Built-in packs: Pre-loaded at build time (instant, zero I/O)
 * 2. Custom packs: Optionally loaded from CUSTOM_KNOWLEDGE_PACKS_DIR env var
 *
 * @see {@link ../../docs/adr/003-knowledge-enhancement.md ADR-003: Knowledge Enhancement System}
 */

import { createLogger } from '@/lib/logger';
import { config } from '@/config';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from './schemas';
import { z } from 'zod';
import { readFileSync, existsSync, readdirSync } from 'fs';
import path from 'path';
import { BUILT_IN_PACKS } from './generated-packs';

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
 * Load custom knowledge packs from a directory
 */
const loadCustomPacks = (customDir: string): void => {
  const stats = {
    packsAttempted: 0,
    packsLoaded: 0,
    packsFailed: 0,
    entriesValid: 0,
    entriesInvalid: 0,
    failures: [] as Array<{ file: string; error: string }>,
  };

  try {
    if (!existsSync(customDir)) {
      logger.warn({ customDir }, 'Custom knowledge packs directory does not exist');
      return;
    }

    // Discover all .json files in custom packs directory
    const packFiles = readdirSync(customDir)
      .filter((file) => file.endsWith('.json'))
      .sort();
    stats.packsAttempted = packFiles.length;

    logger.info({ customDir, totalPacks: packFiles.length }, 'Loading custom knowledge packs');

    // Load each pack
    for (const packFile of packFiles) {
      try {
        const packPath = path.join(customDir, packFile);
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
        logger.debug({ pack: packFile, count: entries.length }, 'Loading custom pack');

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
        logger.warn({ pack: packFile, error: packError }, 'Failed to load custom pack');
      }
    }

    // Log summary
    if (stats.failures.length > 0) {
      logger.warn({ failures: stats.failures }, `Failed to load ${stats.packsFailed} custom packs`);
    }

    logger.info(
      {
        packsAttempted: stats.packsAttempted,
        packsLoaded: stats.packsLoaded,
        packsFailed: stats.packsFailed,
        entriesValid: stats.entriesValid,
        entriesInvalid: stats.entriesInvalid,
      },
      'Custom knowledge packs loaded',
    );
  } catch (error) {
    logger.warn({ error, customDir }, 'Failed to load custom knowledge packs (non-fatal)');
  }
};

/**
 * Load knowledge entries from built-in and custom packs
 *
 * Hybrid loading strategy:
 * 1. Load pre-validated built-in packs (instant, zero I/O)
 * 2. Optionally load custom packs from CUSTOM_KNOWLEDGE_PACKS_DIR
 */
export const loadKnowledgeBase = (): void => {
  if (knowledgeState.loaded) {
    return;
  }

  logger.info('Loading knowledge base');

  try {
    // 1. Load built-in packs (pre-validated at build time)
    logger.debug(
      { builtInEntries: BUILT_IN_PACKS.length },
      'Loading pre-validated built-in knowledge packs',
    );

    for (const entry of BUILT_IN_PACKS) {
      addEntry(entry);
    }

    const builtInCount = knowledgeState.entries.size;
    logger.info({ entries: builtInCount }, 'Built-in knowledge packs loaded');

    // 2. Load custom packs if configured
    const customDir = config.knowledge.customPacksDir;
    if (customDir) {
      logger.info({ customDir }, 'Loading custom knowledge packs');
      loadCustomPacks(customDir);
    }

    // 3. Build indices for fast lookups
    buildIndices();
    knowledgeState.loaded = true;

    // Log final summary
    logger.info(
      {
        builtInEntries: builtInCount,
        customEntries: knowledgeState.entries.size - builtInCount,
        totalEntries: knowledgeState.entries.size,
        categories: Array.from(knowledgeState.byCategory.keys()),
        topTags: getTopTags(5),
      },
      'Knowledge base loaded successfully',
    );
  } catch (error) {
    logger.error({ error }, 'Failed to load knowledge base');
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
