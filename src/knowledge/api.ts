/**
 * Knowledge API - Unified interface for accessing knowledge snippets and documents
 */

import { readFile, readdir } from 'node:fs/promises';
import { join, extname } from 'node:path';
import { parse as parseYaml } from 'yaml';
import type { Logger } from 'pino';
import { Result, Success, Failure } from '@types';
import { safeJsonParse } from '@lib/parsing-utils';
import {
  KnowledgeSnippet,
  KnowledgeDocument,
  KnowledgePack,
  KnowledgeSnippetSchema,
  KnowledgeDocumentSchema,
  KnowledgePackSchema,
  isKnowledgeFresh,
  filterByCategory,
  filterByTags,
  getApplicableKnowledge,
  sortByPriority,
} from './pack-schema';

interface KnowledgeState {
  snippets: Map<string, KnowledgeSnippet>;
  documents: Map<string, KnowledgeDocument>;
  packs: Map<string, KnowledgePack>;
  logger?: Logger;
  initialized: boolean;
  lastRefresh: number;
}

const state: KnowledgeState = {
  snippets: new Map(),
  documents: new Map(),
  packs: new Map(),
  initialized: false,
  lastRefresh: 0,
};

const CACHE_TTL = 60000; // 1 minute cache

/**
 * Initialize knowledge base by loading from directories
 */
export async function initializeKnowledge(baseDir: string, logger?: Logger): Promise<Result<void>> {
  try {
    if (logger) {
      state.logger = logger.child({ component: 'KnowledgeAPI' });
    }
    state.logger?.info({ baseDir }, 'Initializing knowledge base');

    // Load snippets
    const snippetsDir = join(baseDir, 'snippets');
    const snippets = await loadSnippets(snippetsDir);
    state.snippets = snippets;

    // Load documents
    const docsDir = join(baseDir, 'product-handbook');
    const apiGuidesDir = join(baseDir, 'api-guides');
    const documents = await loadDocuments([docsDir, apiGuidesDir]);
    state.documents = documents;

    // Load packs
    const packsDir = join(baseDir, 'data');
    const packs = await loadPacks(packsDir);
    state.packs = packs;

    state.initialized = true;
    state.lastRefresh = Date.now();

    state.logger?.info(
      {
        snippetsCount: snippets.size,
        documentsCount: documents.size,
        packsCount: packs.size,
      },
      'Knowledge base initialized',
    );

    return Success(undefined);
  } catch (error) {
    const message = `Failed to initialize knowledge: ${error}`;
    state.logger?.error({ error }, message);
    return Failure(message);
  }
}

/**
 * Get a knowledge snippet by ID
 */
export function getSnippet<T = unknown>(id: string): Result<KnowledgeSnippet & { data: T }> {
  if (!state.initialized) {
    return Failure('Knowledge base not initialized');
  }

  const snippet = state.snippets.get(id);
  if (!snippet) {
    return Failure(`Snippet not found: ${id}`);
  }

  // Check TTL
  if (!isKnowledgeFresh(snippet)) {
    state.logger?.warn({ id }, 'Snippet is stale based on TTL');
  }

  return Success(snippet as KnowledgeSnippet & { data: T });
}

/**
 * Search knowledge snippets
 */
export function searchSnippets(options: {
  category?: KnowledgeSnippet['category'];
  tags?: string[];
  tool?: string;
  query?: string;
}): KnowledgeSnippet[] {
  if (!state.initialized) return [];

  let results = Array.from(state.snippets.values());

  // Filter by category
  if (options.category) {
    results = filterByCategory(results, options.category);
  }

  // Filter by tags
  if (options.tags && options.tags.length > 0) {
    results = filterByTags(results, options.tags);
  }

  // Filter by tool applicability
  if (options.tool) {
    results = getApplicableKnowledge(results, options.tool);
  }

  // Text search in title and description
  if (options.query) {
    const query = options.query.toLowerCase();
    results = results.filter(
      (s) => s.title.toLowerCase().includes(query) || s.description?.toLowerCase().includes(query),
    );
  }

  // Sort by priority
  return sortByPriority(results);
}

/**
 * Get a knowledge document by ID
 */
export function getDocument(id: string): Result<KnowledgeDocument> {
  if (!state.initialized) {
    return Failure('Knowledge base not initialized');
  }

  const document = state.documents.get(id);
  if (!document) {
    return Failure(`Document not found: ${id}`);
  }

  return Success(document);
}

/**
 * Search knowledge documents
 */
export function searchDocuments(options: {
  category?: string;
  tags?: string[];
  query?: string;
}): KnowledgeDocument[] {
  if (!state.initialized) return [];

  let results = Array.from(state.documents.values());

  // Filter by category
  if (options.category) {
    results = results.filter((d) => d.category === options.category);
  }

  // Filter by tags
  if (options.tags && options.tags.length > 0) {
    const tags = options.tags;
    results = results.filter((d) => d.tags?.some((tag) => tags.includes(tag)));
  }

  // Text search in title and content
  if (options.query) {
    const query = options.query.toLowerCase();
    results = results.filter(
      (d) => d.title.toLowerCase().includes(query) || d.content.toLowerCase().includes(query),
    );
  }

  return results;
}

/**
 * Get aggregated knowledge for a tool
 */
export function getToolKnowledge(toolName: string): {
  snippets: KnowledgeSnippet[];
  documents: KnowledgeDocument[];
} {
  if (!state.initialized) {
    return { snippets: [], documents: [] };
  }

  // Get applicable snippets
  const snippets = searchSnippets({ tool: toolName });

  // Get relevant documents based on tool category
  const toolCategory = getToolCategory(toolName);
  const documents = searchDocuments({ category: toolCategory });

  return { snippets, documents };
}

/**
 * Refresh knowledge if cache is stale
 */
export async function refreshKnowledge(baseDir: string): Promise<Result<void>> {
  const now = Date.now();
  if (now - state.lastRefresh > CACHE_TTL) {
    return initializeKnowledge(baseDir, state.logger);
  }
  return Success(undefined);
}

/**
 * Knowledge API interface
 */
export interface KnowledgeAPI {
  get<T = unknown>(id: string): Result<KnowledgeSnippet & { data: T }>;
  search(options: {
    category?: KnowledgeSnippet['category'];
    tags?: string[];
    tool?: string;
    query?: string;
  }): KnowledgeSnippet[];
  getDoc(id: string): Result<KnowledgeDocument>;
  searchDocs(options: { category?: string; tags?: string[]; query?: string }): KnowledgeDocument[];
  forTool(toolName: string): {
    snippets: KnowledgeSnippet[];
    documents: KnowledgeDocument[];
  };
  refresh(baseDir: string): Promise<Result<void>>;
}

/**
 * Public Knowledge API
 */
export const knowledge: KnowledgeAPI = {
  get: getSnippet,
  search: searchSnippets,
  getDoc: getDocument,
  searchDocs: searchDocuments,
  forTool: getToolKnowledge,
  refresh: refreshKnowledge,
};

// Private helpers

async function loadSnippets(directory: string): Promise<Map<string, KnowledgeSnippet>> {
  const snippets = new Map<string, KnowledgeSnippet>();

  try {
    const files = await readdir(directory);
    for (const file of files) {
      if (!['.yaml', '.yml', '.json'].includes(extname(file))) continue;

      const filePath = join(directory, file);
      const content = await readFile(filePath, 'utf8');
      const data =
        extname(file) === '.json'
          ? (() => {
              const parseResult = safeJsonParse(content);
              if (!parseResult.ok) {
                throw new Error(`Invalid JSON in ${file}: ${parseResult.error}`);
              }
              return parseResult.value;
            })()
          : parseYaml(content);

      if (Array.isArray(data)) {
        // Multiple snippets in one file
        for (const item of data) {
          const parsed = KnowledgeSnippetSchema.safeParse(item);
          if (parsed.success) {
            snippets.set(parsed.data.id, parsed.data);
          } else {
            state.logger?.warn({ file, error: parsed.error }, 'Invalid snippet');
          }
        }
      } else {
        // Single snippet
        const parsed = KnowledgeSnippetSchema.safeParse(data);
        if (parsed.success) {
          snippets.set(parsed.data.id, parsed.data);
        } else {
          state.logger?.warn({ file, error: parsed.error }, 'Invalid snippet');
        }
      }
    }
  } catch (error) {
    state.logger?.warn({ directory, error }, 'Failed to load snippets from directory');
  }

  return snippets;
}

async function loadDocuments(directories: string[]): Promise<Map<string, KnowledgeDocument>> {
  const documents = new Map<string, KnowledgeDocument>();

  for (const directory of directories) {
    try {
      const files = await readdir(directory);
      for (const file of files) {
        if (!['.md', '.yaml', '.yml'].includes(extname(file))) continue;

        const filePath = join(directory, file);
        const content = await readFile(filePath, 'utf8');

        if (extname(file) === '.md') {
          // Markdown document
          const id = file.replace(extname(file), '');
          const doc: KnowledgeDocument = {
            id,
            category: directory.split('/').pop() ?? 'general',
            title: id.replace(/-/g, ' '),
            content,
          };
          documents.set(id, doc);
        } else {
          // YAML document metadata
          const data = parseYaml(content);
          const parsed = KnowledgeDocumentSchema.safeParse(data);
          if (parsed.success) {
            documents.set(parsed.data.id, parsed.data);
          } else {
            state.logger?.warn({ file, error: parsed.error }, 'Invalid document');
          }
        }
      }
    } catch (error) {
      state.logger?.warn({ directory, error }, 'Failed to load documents from directory');
    }
  }

  return documents;
}

async function loadPacks(directory: string): Promise<Map<string, KnowledgePack>> {
  const packs = new Map<string, KnowledgePack>();

  try {
    const files = await readdir(directory);
    for (const file of files) {
      if (!['.json'].includes(extname(file))) continue;

      const filePath = join(directory, file);
      const content = await readFile(filePath, 'utf8');
      const parseResult = safeJsonParse(content);
      if (!parseResult.ok) {
        state.logger?.warn(`Invalid JSON in pack file ${file}: ${parseResult.error}`);
        continue;
      }
      const data = parseResult.value;

      const parsed = KnowledgePackSchema.safeParse(data);
      if (parsed.success) {
        packs.set(parsed.data.id, parsed.data);
      } else {
        state.logger?.warn({ file, error: parsed.error }, 'Invalid pack');
      }
    }
  } catch (error) {
    state.logger?.warn({ directory, error }, 'Failed to load packs from directory');
  }

  return packs;
}

function getToolCategory(toolName: string): string {
  const toolCategories: Record<string, string> = {
    'generate-dockerfile': 'containerization',
    'fix-dockerfile': 'containerization',
    'build-image': 'containerization',
    scan: 'security',
    'tag-image': 'containerization',
    'push-image': 'containerization',
    'generate-k8s-manifests': 'orchestration',
    'prepare-cluster': 'orchestration',
    deploy: 'orchestration',
    'verify-deploy': 'orchestration',
    'resolve-base-images': 'containerization',
    'generate-aca-manifests': 'cloud',
    'convert-aca-to-k8s': 'orchestration',
    'generate-helm-charts': 'orchestration',
    'analyze-repo': 'languages',
    'inspect-session': 'debugging',
    ops: 'operations',
  };

  return toolCategories[toolName] ?? 'general';
}
