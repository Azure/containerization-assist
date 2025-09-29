import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, KnowledgeStats, LoadedEntry, CompilationStats } from './types';
import { KnowledgeEntrySchema } from './schemas';
import { z } from 'zod';
import { readFileSync, existsSync } from 'fs';
import path from 'path';

const logger = createLogger().child({ module: 'knowledge-loader' });

interface KnowledgeState {
  entries: Map<string, LoadedEntry>;
  byCategory: Map<string, LoadedEntry[]>;
  byTag: Map<string, LoadedEntry[]>;
  loaded: boolean;
  compilationStats: CompilationStats;
}

let knowledgeState: KnowledgeState = {
  entries: new Map(),
  byCategory: new Map(),
  byTag: new Map(),
  loaded: false,
  compilationStats: {
    totalEntries: 0,
    compiledSuccessfully: 0,
    compilationErrors: 0,
    avgCompilationTime: 0,
  },
};

const findExistingPath = (paths: readonly string[]): string | null => {
  for (const path of paths) {
    if (existsSync(path)) {
      return path;
    }
  }
  return null;
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
          errors: error.issues.map((e: any) => ({
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

const updateCompilationStats = (time: number): void => {
  const { totalEntries, avgCompilationTime } = knowledgeState.compilationStats;
  knowledgeState.compilationStats.totalEntries++;
  knowledgeState.compilationStats.avgCompilationTime =
    (avgCompilationTime * totalEntries + time) / (totalEntries + 1);
};

const compilePattern = (entry: KnowledgeEntry): LoadedEntry => {
  const startTime = performance.now();
  const loaded: LoadedEntry = { ...entry };

  if (entry.pattern) {
    try {
      // Compile with case-insensitive and multiline flags
      const regex = new RegExp(entry.pattern, 'gmi');
      loaded.compiledCache = {
        pattern: regex,
        lastCompiled: Date.now(),
      };
      knowledgeState.compilationStats.compiledSuccessfully++;
    } catch (error) {
      logger.warn(
        {
          entryId: entry.id,
          pattern: entry.pattern,
          error: error instanceof Error ? error.message : String(error),
        },
        'Failed to compile pattern',
      );
      loaded.compiledCache = {
        pattern: null,
        lastCompiled: Date.now(),
        compilationError: error instanceof Error ? error.message : 'Unknown error',
      };
      knowledgeState.compilationStats.compilationErrors++;
    }
  }

  const compilationTime = performance.now() - startTime;
  updateCompilationStats(compilationTime);

  return loaded;
};

const addEntry = (entry: KnowledgeEntry): void => {
  const loadedEntry = compilePattern(entry);
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

  try {
    const knowledgePacks = [
      'starter-pack.json',
      'base-images-pack.json',
      'nodejs-pack.json',
      'python-pack.json',
      'java-pack.json',
      'go-pack.json',
      'dotnet-pack.json',
      'dotnet-framework-pack.json',
      'ruby-pack.json',
      'rust-pack.json',
      'php-pack.json',
      'database-pack.json',
      'kubernetes-pack.json',
      'security-pack.json',
      'azure-container-apps-pack.json',
    ] as const;

    for (const packFile of knowledgePacks) {
      try {
        const possiblePaths = [
          path.resolve(process.cwd(), 'knowledge/packs', packFile),
          path.resolve(process.cwd(), 'dist/knowledge/packs', packFile),
          path.resolve(
            process.cwd(),
            'node_modules/@thgamble/containerization-assist-mcp/knowledge/packs',
            packFile,
          ),
        ] as const;

        const dataPath = findExistingPath(possiblePaths);
        if (!dataPath) {
          throw new Error(`Could not find knowledge pack: ${packFile}`);
        }

        const content = readFileSync(dataPath, 'utf-8');
        const entries = JSON.parse(content) as KnowledgeEntry[];

        logger.debug({ pack: packFile, count: entries.length }, 'Loading knowledge pack');

        for (const entry of entries) {
          if (validateEntry(entry)) {
            addEntry(entry);
          } else {
            logger.warn(
              {
                pack: packFile,
                entryId: (entry as { id?: string })?.id || 'unknown',
              },
              'Invalid knowledge entry, skipping',
            );
          }
        }
      } catch (packError) {
        logger.warn(
          {
            pack: packFile,
            error: packError,
          },
          'Failed to load knowledge pack, continuing',
        );
      }
    }

    buildIndices();
    knowledgeState.loaded = true;

    logger.info(
      {
        totalEntries: knowledgeState.entries.size,
        packsLoaded: knowledgePacks.length,
        categories: Array.from(knowledgeState.byCategory.keys()),
        topTags: getTopTags(5),
      },
      'Knowledge base loaded successfully',
    );
  } catch (error) {
    logger.error({ error }, 'Failed to load knowledge base');
  }
};

/**
 * Get entry by ID
 */
export const getEntryById = (id: string): LoadedEntry | undefined => {
  return knowledgeState.entries.get(id);
};

/**
 * Get entries by category
 */
export const getEntriesByCategory = (category: string): LoadedEntry[] => {
  return knowledgeState.byCategory.get(category) || [];
};

/**
 * Get entries by tag
 */
export const getEntriesByTag = (tag: string): LoadedEntry[] => {
  return knowledgeState.byTag.get(tag) || [];
};

/**
 * Get all entries
 */
export const getAllEntries = (): LoadedEntry[] => {
  return Array.from(knowledgeState.entries.values());
};

/**
 * Get pattern compilation statistics
 */
export const getCompilationStats = (): CompilationStats => {
  return { ...knowledgeState.compilationStats };
};

/**
 * Get knowledge base statistics
 */
export const getKnowledgeStats = (): KnowledgeStats => {
  const byCategory: Record<string, number> = {};
  const bySeverity: Record<string, number> = {};
  const tagCounts: Record<string, number> = {};

  for (const entry of knowledgeState.entries.values()) {
    // Count by category
    byCategory[entry.category] = (byCategory[entry.category] || 0) + 1;

    // Count by severity
    const severity = entry.severity || 'medium';
    bySeverity[severity] = (bySeverity[severity] || 0) + 1;

    // Count tags
    if (entry.tags) {
      for (const tag of entry.tags) {
        tagCounts[tag] = (tagCounts[tag] || 0) + 1;
      }
    }
  }

  // Get top tags
  const topTags = Object.entries(tagCounts)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 10)
    .map(([tag, count]) => ({ tag, count }));

  return {
    totalEntries: knowledgeState.entries.size,
    byCategory,
    bySeverity,
    topTags,
  };
};

/**
 * Check if knowledge base is loaded
 */
export const isKnowledgeLoaded = (): boolean => {
  return knowledgeState.loaded;
};

/**
 * Force reload the knowledge base
 */
export const reloadKnowledgeBase = async (): Promise<void> => {
  knowledgeState = {
    entries: new Map(),
    byCategory: new Map(),
    byTag: new Map(),
    loaded: false,
    compilationStats: {
      totalEntries: 0,
      compiledSuccessfully: 0,
      compilationErrors: 0,
      avgCompilationTime: 0,
    },
  };
  await loadKnowledgeBase();
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
