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
import { BUILTIN_PACKS } from './built-in-packs';

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
    stats.packsAttempted = BUILTIN_PACKS.length;

    logger.info({ totalPacks: BUILTIN_PACKS.length }, 'Loading built-in knowledge packs');

    // Load each built-in pack
    for (const pack of BUILTIN_PACKS) {
      try {
        const data = pack.data;

        // Validate and normalize pack structure
        const result = validateAndNormalizePack(pack.name, data);
        if (!result.valid || !result.entries) {
          const error = 'Pack validation failed (see previous log)';
          stats.packsFailed++;
          stats.failures.push({
            file: pack.name,
            error,
          });
          // Throw error for built-in packs - they must all load successfully
          throw new Error(`Failed to load built-in knowledge pack ${pack.name}: ${error}`);
        }

        const entries = result.entries;
        logger.debug({ pack: pack.name, count: entries.length }, 'Loading knowledge pack');

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
          file: pack.name,
          error: errorMessage,
        });
        logger.error({ pack: pack.name, error: packError }, 'Failed to load knowledge pack');
        // Re-throw the error to ensure server startup fails
        throw new Error(`Failed to load built-in knowledge pack ${pack.name}: ${errorMessage}`);
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
