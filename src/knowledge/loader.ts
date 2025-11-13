/**
 * Knowledge Pack Loader
 * Loads and manages static knowledge packs for AI enhancement
 *
 * @see {@link ../../docs/adr/003-knowledge-enhancement.md ADR-003: Knowledge Enhancement System}
 */

import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { createLogger } from '@/lib/logger';
import { getModuleUrl } from '@/lib/module-url';
import { resolveModulePaths } from '@/lib/module-path-resolver';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema, KnowledgePackSchema } from './schemas';
import { z } from 'zod';

// ===== Constants =====

const JSON_FILE_EXTENSION = '.json';

const logger = createLogger({ name: 'knowledge-loader' });

// Capture import.meta.url at module scope (ESM builds only)
// This is used for module path resolution
const MODULE_URL = getModuleUrl();

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

// ===== Path Resolution =====
// Uses shared module path resolver utility

/**
 * Find JSON files in the given directory
 */
function findJsonFiles(directory: string): string[] {
  if (!existsSync(directory)) {
    return [];
  }

  return readdirSync(directory)
    .filter((file) => file.endsWith(JSON_FILE_EXTENSION))
    .map((file) => resolve(join(directory, file)));
}

/**
 * Discover built-in knowledge pack JSON files using a chain of resolution strategies.
 *
 * Search priority:
 *  1. Relative to the installed module location (CJS __dirname)
 *  2. Relative to the installed module location (ESM import.meta.url)
 *  3. Heuristic based on process.argv[1] (CLI entrypoint with symlink resolution)
 *  4. Walk upward from process.cwd() (dev / repo root)
 */
export function discoverBuiltInKnowledgePacks(): string[] {
  try {
    // Use shared path resolver utility
    const searchPaths = resolveModulePaths({
      relativePath: 'knowledge/packs',
      logger,
      ...(MODULE_URL && { moduleUrl: MODULE_URL }),
    });

    // Try each search path until we find one with JSON files
    for (const packsDir of searchPaths) {
      logger.debug({ path: packsDir, exists: existsSync(packsDir) }, 'Checking knowledge pack path');

      const files = findJsonFiles(packsDir);
      if (files.length > 0) {
        logger.debug(
          { count: files.length, dir: packsDir, files: files.slice(0, 3) },
          'Found files in knowledge pack directory',
        );
        logger.info({ count: files.length, dir: packsDir }, 'Discovered built-in knowledge packs');
        return files;
      }
    }

    logger.error(
      { searchPaths, cwd: process.cwd() },
      'FATAL: No knowledge packs found in any search path',
    );
    return [];
  } catch (error) {
    logger.warn({ error }, 'Failed to discover built-in knowledge packs');
    return [];
  }
}

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
function validateAndNormalizePack(packFile: string, data: unknown): ValidationResult {
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
          pack: packFile,
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
  failures: Array<{ file: string; error: string }>;
}

/**
 * Load a single knowledge pack file
 */
function loadPackFile(packPath: string, stats: LoadStats): void {
  try {
    // Read and parse JSON file
    const fileContent = readFileSync(packPath, 'utf-8');
    const data = JSON.parse(fileContent);

    // Validate and normalize pack structure
    const result = validateAndNormalizePack(packPath, data);

    if (!result.success) {
      const error = 'Pack validation failed (see previous log)';
      stats.packsFailed++;
      stats.failures.push({ file: packPath, error });
      throw new Error(`Failed to load built-in knowledge pack ${packPath}: ${error}`);
    }

    logger.debug({ pack: packPath, count: result.entries.length }, 'Loading knowledge pack');

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
    stats.failures.push({ file: packPath, error: errorMessage });
    logger.error({ pack: packPath, error }, 'Failed to load knowledge pack');
    throw new Error(`Failed to load built-in knowledge pack ${packPath}: ${errorMessage}`);
  }
}

/**
 * Load knowledge entries from built-in knowledge packs
 * Throws an error if any built-in pack fails to load
 */
export function loadKnowledgeBase(): void {
  if (knowledgeState.loaded) {
    return;
  }

  const stats: LoadStats = {
    packsAttempted: 0,
    packsLoaded: 0,
    packsFailed: 0,
    entriesValid: 0,
    entriesInvalid: 0,
    failures: [],
  };

  const packPaths = discoverBuiltInKnowledgePacks();
  stats.packsAttempted = packPaths.length;

  if (packPaths.length === 0) {
    // Get search paths for better error message
    const searchPaths = resolveModulePaths({
      relativePath: 'knowledge/packs',
      logger,
      ...(MODULE_URL && { moduleUrl: MODULE_URL }),
    });

    const error = new Error(
      `No knowledge packs discovered - server cannot start without knowledge base.\n` +
        `\n` +
        `Searched locations:\n${searchPaths.map((p) => `  - ${p}`).join('\n')}\n` +
        `\n` +
        `Expected: JSON files matching pattern knowledge/packs/*.json\n` +
        `\n` +
        `Resolution:\n` +
        `  • If running from source: Ensure knowledge/packs/ directory exists\n` +
        `  • If installed via npm: Report this as a packaging issue\n` +
        `  • Current directory: ${process.cwd()}\n` +
        `  • Platform: ${process.platform}`,
    );

    logger.error(
      {
        error,
        searchPaths,
        cwd: process.cwd(),
        platform: process.platform,
      },
      'No knowledge packs discovered',
    );

    throw error;
  }

  logger.info({ totalPacks: packPaths.length }, 'Loading built-in knowledge packs');

  // Load each discovered pack (loadPackFile already throws on failure)
  for (const packPath of packPaths) {
    loadPackFile(packPath, stats);
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
