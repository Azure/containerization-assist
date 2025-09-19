/**
 * Robust Prompt Locator
 *
 * Provides deterministic path resolution for prompts with fallback to embedded content.
 * Part of Sprint 2: Implement Prompt Manifest System
 */

import { readFile, stat } from 'node:fs/promises';
import * as path from 'node:path';
import { createHash } from 'node:crypto';

export interface PromptMeta {
  id: string;
  path: string;
  sha256?: string;
  category?: string;
  version?: string;
}

export interface PromptManifest {
  prompts: PromptMeta[];
  embedded: string[]; // IDs of prompts that are embedded
  generatedAt: string;
}

// Cache for loaded prompts to avoid filesystem hits
const promptCache = new Map<string, string>();

// Import embedded prompts
import EMBEDDED_PROMPTS_DATA from './embedded-prompts.generated';

// Manifest (will be loaded at runtime)
let manifest: PromptManifest | null = null;

// Add manifest version tracking for cache invalidation
let manifestVersion: string | null = null;

/**
 * Get current directory with environment-specific fallbacks
 */
function getCurrentDirectory(): string {
  // Try different methods based on environment
  if (typeof __dirname !== 'undefined') {
    // CommonJS environment
    return __dirname;
  }

  // Fallback to process.cwd() for ESM environment
  return process.cwd();
}

/**
 * Find application root by looking for package.json
 */
async function findAppRoot(startDir: string): Promise<string> {
  let dir = startDir;

  while (dir !== path.dirname(dir)) {
    // Stop at filesystem root
    try {
      await stat(path.join(dir, 'package.json'));
      return dir;
    } catch {
      dir = path.dirname(dir);
    }
  }

  // Fallback to current working directory
  return process.cwd();
}

/**
 * Generate candidate paths for a prompt in order of preference
 */
async function* candidatePathsFor(meta: PromptMeta): AsyncGenerator<string> {
  // Get current directory with fallbacks
  const here = getCurrentDirectory();
  const appRoot = await findAppRoot(here);

  // 1) Relative to this file (stable in dev/prod)
  yield path.resolve(here, meta.path);

  // 2) Relative to app root
  yield path.resolve(appRoot, meta.path);

  // 3) PROMPTS_DIR override
  if (process.env.PROMPTS_DIR) {
    yield path.resolve(process.env.PROMPTS_DIR, path.basename(meta.path));
  }

  // 4) Last resort: CWD
  yield path.resolve(process.cwd(), meta.path);
}

/**
 * Load prompt manifest from file system
 */
async function loadManifest(): Promise<PromptManifest | null> {
  if (manifest) return manifest;

  try {
    const here = getCurrentDirectory();
    const manifestPath = path.resolve(here, 'manifest.json');

    const manifestText = await readFile(manifestPath, 'utf-8');
    manifest = JSON.parse(manifestText) as PromptManifest;
    return manifest;
  } catch {
    // Manifest not found - running in embedded mode
    return null;
  }
}

/**
 * Get embedded prompts
 */
function getEmbeddedPrompts(): Record<string, string> {
  return EMBEDDED_PROMPTS_DATA;
}

/**
 * Verify file integrity using SHA256 hash
 */
async function verifyFileIntegrity(filePath: string, expectedSha256?: string): Promise<boolean> {
  if (!expectedSha256) return true;

  try {
    const content = await readFile(filePath);
    const hash = createHash('sha256').update(content).digest('hex');
    return hash === expectedSha256;
  } catch {
    return false;
  }
}

/**
 * Load prompt text by ID with fallback chain
 */
export async function loadPromptText(id: string): Promise<string> {
  // Try to load manifest first to check for version changes
  const manifestData = await loadManifest();

  // Clear cache if manifest version changed
  if (manifestData && manifestData.generatedAt !== manifestVersion) {
    promptCache.clear();
    manifestVersion = manifestData.generatedAt;
  }

  // Check cache after potential invalidation
  if (promptCache.has(id)) {
    return promptCache.get(id)!;
  }

  if (manifestData) {
    // Manifest mode - try to load from filesystem
    const promptMeta = manifestData.prompts.find((p) => p.id === id);
    if (promptMeta) {
      // Try each candidate path
      for await (const candidatePath of candidatePathsFor(promptMeta)) {
        try {
          await stat(candidatePath);

          // Verify integrity if hash is provided
          if (await verifyFileIntegrity(candidatePath, promptMeta.sha256)) {
            const content = await readFile(candidatePath, 'utf-8');
            promptCache.set(id, content);
            return content;
          }
        } catch {
          // Try next candidate
          continue;
        }
      }
    }
  }

  // Fallback to embedded prompts
  const embeddedPrompts = getEmbeddedPrompts();
  if (embeddedPrompts?.[id]) {
    const content = embeddedPrompts[id];
    if (content) {
      promptCache.set(id, content);
      return content;
    }
  }

  throw new Error(`Prompt not found: ${id}`);
}

/**
 * Get prompt metadata from manifest
 */
export async function getPromptMetadata(id: string): Promise<PromptMeta | null> {
  const manifestData = await loadManifest();
  if (!manifestData) return null;

  return manifestData.prompts.find((p) => p.id === id) || null;
}

/**
 * List all available prompt IDs
 */
export async function listPromptIds(): Promise<string[]> {
  const manifestData = await loadManifest();

  if (manifestData) {
    return manifestData.prompts.map((p) => p.id);
  }

  // Fallback to embedded prompts
  return Object.keys(getEmbeddedPrompts());
}

/**
 * Check if prompt is embedded (critical prompt that's always available)
 */
export async function isPromptEmbedded(id: string): Promise<boolean> {
  const manifestData = await loadManifest();

  if (manifestData) {
    return manifestData.embedded.includes(id);
  }

  // In embedded-only mode, all prompts are "embedded"
  return id in getEmbeddedPrompts();
}

/**
 * Clear cache (useful for testing)
 */
export function clearCache(): void {
  promptCache.clear();
  manifest = null;
  manifestVersion = null;
}

/**
 * Get current manifest version (for testing)
 */
export function getManifestVersion(): string | null {
  return manifestVersion;
}
