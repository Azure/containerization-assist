import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, LoadedEntry } from './types';
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
            'node_modules/containerization-assist-mcp/knowledge/packs',
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
