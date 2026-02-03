/**
 * Knowledge Pack Loader
 * Loads and manages static knowledge packs for AI enhancement
 *
 * Knowledge packs are embedded at build time via static JSON imports,
 * eliminating the need for filesystem access at runtime. This allows
 * the SDK to work correctly when bundled by consumer applications.
 *
 * @see {@link ../../docs/adr/003-knowledge-enhancement.md ADR-003: Knowledge Enhancement System}
 */

import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from './schemas';
import { EMBEDDED_PACKS } from './embedded-packs';
import { z } from 'zod';

// ===== Constants =====

const logger = createLogger({ name: 'knowledge-loader' });

// ===== Types =====

interface KnowledgeState {
  entries: Map<string, LoadedEntry>;
  byCategory: Map<string, LoadedEntry[]>;
  byTag: Map<string, LoadedEntry[]>;
  loaded: boolean;
}

interface PackValidationResult {
  success: true;
  entries: KnowledgeEntry[];
}

interface PackValidationError {
  success: false;
  error: string;
}

type ValidationResult = PackValidationResult | PackValidationError;

// ===== State =====

const knowledgeState: KnowledgeState = {
  entries: new Map(),
  byCategory: new Map(),
  byTag: new Map(),
  loaded: false,
};

// ===== Validation =====

/**
 * Format Zod errors for logging
 */
function formatZodErrors(errors: z.ZodIssue[]): Array<{ path: string; message: string }> {
  return errors.map((e) => ({
    path: e.path.join('.'),
    message: e.message,
  }));
}

/**
 * Validate and normalize pack structure
 * Handles both array and object-wrapped pack formats
 */
function validateAndNormalizePack(packName: string, data: unknown): ValidationResult {
  try {
    const validated = KnowledgePackSchema.parse(data);

    // Extract entries based on format
    const entries: KnowledgeEntry[] = Array.isArray(validated)
      ? (validated as KnowledgeEntry[]) // Format 1: Flat array
      : (validated.rules as KnowledgeEntry[]); // Format 2: Object with rules

    return { success: true, entries };
  } catch (error) {
    if (error instanceof z.ZodError) {
      logger.warn(
        {
          pack: packName,
          errors: formatZodErrors(error.issues.slice(0, 5)),
          totalErrors: error.issues.length,
        },
        'Pack validation failed',
      );
    }
    return { success: false, error: String(error) };
  }
}

/**
 * Validate a single knowledge entry
 */
function validateEntry(entry: unknown): entry is KnowledgeEntry {
  try {
    KnowledgeEntrySchema.parse(entry);
    return true;
  } catch (error) {
    if (error instanceof z.ZodError) {
      logger.warn(
        {
          entryId: (entry as { id?: string })?.id || 'unknown',
          errors: formatZodErrors(error.issues),
        },
        'Entry validation failed',
      );
    }
    return false;
  }
}

// ===== State Management =====

/**
 * Add an entry to the knowledge state
 */
function addEntry(entry: KnowledgeEntry): void {
  knowledgeState.entries.set(entry.id, entry);
}

/**
 * Helper to add entry to a map with array values
 */
function addToMapArray<K, V>(map: Map<K, V[]>, key: K, value: V): void {
  const existing = map.get(key);
  if (existing) {
    existing.push(value);
  } else {
    map.set(key, [value]);
  }
}

/**
 * Build category and tag indices for fast lookup
 */
function buildIndices(): void {
  knowledgeState.byCategory.clear();
  knowledgeState.byTag.clear();

  for (const entry of knowledgeState.entries.values()) {
    // Index by category
    addToMapArray(knowledgeState.byCategory, entry.category, entry);

    // Index by tags
    if (entry.tags) {
      for (const tag of entry.tags) {
        addToMapArray(knowledgeState.byTag, tag, entry);
      }
    }
  }
}

/**
 * Get top N most common tags with their counts
 */
function getTopTags(limit: number): Array<{ tag: string; count: number }> {
  const tagCounts = new Map<string, number>();

  for (const entry of knowledgeState.entries.values()) {
    if (entry.tags) {
      for (const tag of entry.tags) {
        tagCounts.set(tag, (tagCounts.get(tag) ?? 0) + 1);
      }
    }
  }

  return Array.from(tagCounts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([tag, count]) => ({ tag, count }));
}

// ===== Pack Loading =====

interface LoadStats {
  packsAttempted: number;
  packsLoaded: number;
  packsFailed: number;
  entriesValid: number;
  entriesInvalid: number;
  failures: Array<{ name: string; error: string }>;
}

/**
 * Load a single embedded knowledge pack
 */
function loadEmbeddedPack(packName: string, packData: unknown, stats: LoadStats): void {
  try {
    // Validate and normalize pack structure
    const result = validateAndNormalizePack(packName, packData);

    if (!result.success) {
      const error = 'Pack validation failed (see previous log)';
      stats.packsFailed++;
      stats.failures.push({ name: packName, error });
      throw new Error(`Failed to load embedded knowledge pack ${packName}: ${error}`);
    }

    logger.debug({ pack: packName, count: result.entries.length }, 'Loading knowledge pack');

    // Validate and add individual entries
    for (const entry of result.entries) {
      if (validateEntry(entry)) {
        addEntry(entry);
        stats.entriesValid++;
      } else {
        stats.entriesInvalid++;
      }
    }

    stats.packsLoaded++;
  } catch (error) {
    stats.packsFailed++;
    const errorMessage = String(error);
    stats.failures.push({ name: packName, error: errorMessage });
    logger.error({ pack: packName, error }, 'Failed to load knowledge pack');
    throw new Error(`Failed to load embedded knowledge pack ${packName}: ${errorMessage}`);
  }
}

/**
 * Load knowledge entries from embedded knowledge packs
 * Throws an error if any embedded pack fails to load
 */
export function loadKnowledgeBase(): void {
  if (knowledgeState.loaded) {
    return;
  }

  const stats: LoadStats = {
    packsAttempted: EMBEDDED_PACKS.length,
    packsLoaded: 0,
    packsFailed: 0,
    entriesValid: 0,
    entriesInvalid: 0,
    failures: [],
  };

  if (EMBEDDED_PACKS.length === 0) {
    const error = new Error(
      `No knowledge packs embedded - server cannot start without knowledge base.\n` +
        `\n` +
        `This is likely a build issue. The embedded-packs.ts file should contain\n` +
        `static imports of all knowledge pack JSON files.\n` +
        `\n` +
        `Resolution:\n` +
        `  - Ensure knowledge/packs/*.json files exist\n` +
        `  - Rebuild the package with: npm run build`,
    );

    logger.error({ error }, 'No knowledge packs embedded');
    throw error;
  }

  logger.info({ totalPacks: EMBEDDED_PACKS.length }, 'Loading embedded knowledge packs');

  // Load each embedded pack
  for (const pack of EMBEDDED_PACKS) {
    loadEmbeddedPack(pack.name, pack.data, stats);
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
}

// ===== Public API =====

/**
 * Get all loaded knowledge entries
 */
export function getAllEntries(): LoadedEntry[] {
  return Array.from(knowledgeState.entries.values());
}

/**
 * Check if knowledge base is loaded
 */
export function isKnowledgeLoaded(): boolean {
  return knowledgeState.loaded;
}

/**
 * Load knowledge data and return entries.
 * Used by prompt engine for knowledge selection.
 */
export function loadKnowledgeData(): { entries: LoadedEntry[] } {
  if (!isKnowledgeLoaded()) {
    loadKnowledgeBase();
  }
  return {
    entries: getAllEntries(),
  };
}

/**
 * Reset knowledge state (for testing purposes)
 */
export function resetKnowledgeState(): void {
  knowledgeState.entries.clear();
  knowledgeState.byCategory.clear();
  knowledgeState.byTag.clear();
  knowledgeState.loaded = false;
}
