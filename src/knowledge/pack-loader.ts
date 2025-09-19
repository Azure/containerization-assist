/**
 * Enhanced knowledge loader with simple caching
 */
import { createLogger } from '@/lib/logger';
import type { KnowledgeEntry, LoadedEntry } from './types';
import { KnowledgeEntrySchema } from './pack-schema';
import { readFileSync, existsSync } from 'fs';
import { resolve } from 'node:path';

const logger = createLogger().child({ module: 'enhanced-loader' });

// Simple cache for compiled patterns
const patternCache = new Map<string, RegExp>();
const loadedPacks = new Map<string, LoadedEntry[]>();

const KNOWLEDGE_PACKS = [
  'starter-pack.json',
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

/**
 * Load and cache a knowledge pack
 */
export async function loadKnowledgePack(packName: string): Promise<LoadedEntry[]> {
  // Return from cache if already loaded
  const cached = loadedPacks.get(packName);
  if (cached) {
    return cached;
  }

  try {
    const possiblePaths = [
      resolve(process.cwd(), 'src/knowledge/data', packName),
      resolve(process.cwd(), 'dist/src/knowledge/data', packName),
      resolve(__dirname, 'data', packName),
    ];

    const dataPath = possiblePaths.find(existsSync);
    if (!dataPath) {
      logger.warn({ pack: packName }, 'Knowledge pack not found');
      return [];
    }

    const content = readFileSync(dataPath, 'utf-8');
    const rawEntries = JSON.parse(content) as unknown[];

    const entries: LoadedEntry[] = [];
    for (const entry of rawEntries) {
      try {
        const validated = KnowledgeEntrySchema.parse(entry) as KnowledgeEntry;
        entries.push(compileEntry(validated));
      } catch {
        logger.debug({ pack: packName }, 'Invalid entry in pack');
      }
    }

    // Cache the loaded pack
    loadedPacks.set(packName, entries);
    logger.info({ pack: packName, count: entries.length }, 'Loaded knowledge pack');

    return entries;
  } catch (error) {
    logger.error({ pack: packName, error }, 'Failed to load knowledge pack');
    return [];
  }
}

/**
 * Compile pattern with simple caching
 */
function compileEntry(entry: KnowledgeEntry): LoadedEntry {
  const loaded: LoadedEntry = { ...entry };

  if (entry.pattern) {
    // Check cache first
    let regex = patternCache.get(entry.pattern);

    if (!regex) {
      try {
        regex = new RegExp(entry.pattern, 'gmi');
        patternCache.set(entry.pattern, regex);
      } catch (error) {
        logger.debug({ pattern: entry.pattern }, 'Failed to compile pattern');
        loaded._compiled = {
          pattern: null,
          lastCompiled: Date.now(),
          compilationError: error instanceof Error ? error.message : 'Unknown error',
        };
        return loaded;
      }
    }

    loaded._compiled = {
      pattern: regex,
      lastCompiled: Date.now(),
    };
  }

  return loaded;
}

/**
 * Get entries by category with optional filtering
 */
export async function getEntriesByCategoryEnhanced(
  category: string,
  options?: {
    language?: string;
    framework?: string;
  },
): Promise<LoadedEntry[]> {
  const allEntries: LoadedEntry[] = [];

  // Load relevant packs based on category/language
  const packsToLoad = KNOWLEDGE_PACKS.filter((pack) => {
    if (category === 'kubernetes' && !pack.includes('kubernetes')) return false;
    if (category === 'security' && !pack.includes('security')) return false;
    if (options?.language && !pack.includes(options.language) && !pack.includes('starter'))
      return false;
    return true;
  });

  for (const packName of packsToLoad) {
    const entries = await loadKnowledgePack(packName);

    // Filter by category and optional criteria
    const filtered = entries.filter((entry) => {
      if (entry.category !== category) return false;

      // Simple tag matching
      if (options?.language && entry.tags?.includes(options.language)) {
        return true;
      }
      if (options?.framework && entry.tags?.includes(options.framework)) {
        return true;
      }

      // Include if no specific filters
      return !options?.language && !options?.framework;
    });

    allEntries.push(...filtered);
  }

  return allEntries;
}

/**
 * Clear all caches
 */
export function clearCaches(): void {
  loadedPacks.clear();
  patternCache.clear();
  logger.info('Cleared knowledge caches');
}
