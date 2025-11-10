#!/usr/bin/env tsx
/**
 * Build-time Knowledge Pack Generator
 *
 * Reads all JSON knowledge packs and generates a TypeScript module with
 * pre-validated, inlined entries for fast runtime loading.
 */

import { readFileSync, readdirSync, writeFileSync, existsSync } from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { KnowledgePackSchema, KnowledgeEntrySchema } from '../src/knowledge/schemas.js';
import type { KnowledgeEntry } from '../src/knowledge/types.js';
import { z } from 'zod';

// Get script directory
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '..');

interface GenerationStats {
  packsProcessed: number;
  packsFailed: number;
  entriesValid: number;
  entriesInvalid: number;
  failures: Array<{ file: string; error: string }>;
}

/**
 * Validate and normalize pack structure
 */
function validateAndNormalizePack(
  packFile: string,
  data: unknown,
): { valid: boolean; entries?: KnowledgeEntry[] } {
  try {
    const validated = KnowledgePackSchema.parse(data);

    let entries: KnowledgeEntry[];
    if (Array.isArray(validated)) {
      entries = validated as KnowledgeEntry[];
    } else {
      entries = validated.rules as KnowledgeEntry[];
    }

    return { valid: true, entries };
  } catch (error) {
    if (error instanceof z.ZodError) {
      console.error(`❌ Pack validation failed: ${packFile}`);
      error.issues.slice(0, 5).forEach((issue) => {
        console.error(`   - ${issue.path.join('.')}: ${issue.message}`);
      });
      if (error.issues.length > 5) {
        console.error(`   ... and ${error.issues.length - 5} more errors`);
      }
    }
    return { valid: false };
  }
}

/**
 * Validate individual entry
 */
function validateEntry(entry: unknown, packFile: string): entry is KnowledgeEntry {
  try {
    KnowledgeEntrySchema.parse(entry);
    return true;
  } catch (error) {
    if (error instanceof z.ZodError) {
      const entryId = (entry as { id?: string })?.id || 'unknown';
      console.error(`❌ Entry validation failed: ${packFile}#${entryId}`);
      error.issues.forEach((issue) => {
        console.error(`   - ${issue.path.join('.')}: ${issue.message}`);
      });
    }
    return false;
  }
}

/**
 * Load all knowledge packs from directory
 */
function loadAllPacks(packsDir: string): { entries: KnowledgeEntry[]; stats: GenerationStats } {
  const stats: GenerationStats = {
    packsProcessed: 0,
    packsFailed: 0,
    entriesValid: 0,
    entriesInvalid: 0,
    failures: [],
  };

  const allEntries: KnowledgeEntry[] = [];

  if (!existsSync(packsDir)) {
    throw new Error(`Packs directory not found: ${packsDir}`);
  }

  const packFiles = readdirSync(packsDir)
    .filter((file) => file.endsWith('.json'))
    .sort();

  console.log(`📦 Found ${packFiles.length} knowledge packs in ${path.relative(projectRoot, packsDir)}`);

  for (const packFile of packFiles) {
    stats.packsProcessed++;

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

      // Validate and normalize pack
      const result = validateAndNormalizePack(packFile, data);
      if (!result.valid || !result.entries) {
        stats.packsFailed++;
        stats.failures.push({
          file: packFile,
          error: 'Pack validation failed',
        });
        continue;
      }

      // Validate individual entries
      for (const entry of result.entries) {
        if (validateEntry(entry, packFile)) {
          allEntries.push(entry);
          stats.entriesValid++;
        } else {
          stats.entriesInvalid++;
        }
      }

      console.log(`  ✓ ${packFile}: ${result.entries.length} entries`);
    } catch (error) {
      stats.packsFailed++;
      stats.failures.push({
        file: packFile,
        error: String(error),
      });
      console.error(`  ✗ ${packFile}: ${error}`);
    }
  }

  return { entries: allEntries, stats };
}

/**
 * Generate TypeScript module content
 */
function generateTypeScriptModule(entries: KnowledgeEntry[]): string {
  const header = `/**
 * Generated Knowledge Packs
 *
 * This file is auto-generated at build time from JSON knowledge packs.
 * DO NOT EDIT MANUALLY - changes will be overwritten.
 *
 * To modify knowledge entries, edit the JSON files in knowledge/packs/
 * and run: npm run prebuild
 *
 * Generated: ${new Date().toISOString()}
 * Total entries: ${entries.length}
 */

import type { KnowledgeEntry } from './types';

export const BUILT_IN_PACKS: KnowledgeEntry[] = `;

  // Generate the entries array with proper formatting
  const entriesJson = JSON.stringify(entries, null, 2);

  return header + entriesJson + ';\n';
}

/**
 * Main generation function
 */
function main() {
  console.log('🚀 Generating knowledge pack module...\n');

  const packsDir = path.join(projectRoot, 'knowledge/packs');
  const outputPath = path.join(projectRoot, 'src/knowledge/generated-packs.ts');

  // Load and validate all packs
  const { entries, stats } = loadAllPacks(packsDir);

  // Report stats
  console.log('\n📊 Generation Statistics:');
  console.log(`   Packs processed: ${stats.packsProcessed}`);
  console.log(`   Packs loaded: ${stats.packsProcessed - stats.packsFailed}`);
  console.log(`   Packs failed: ${stats.packsFailed}`);
  console.log(`   Valid entries: ${stats.entriesValid}`);
  console.log(`   Invalid entries: ${stats.entriesInvalid}`);

  if (stats.failures.length > 0) {
    console.log('\n⚠️  Failures:');
    stats.failures.forEach(({ file, error }) => {
      console.log(`   - ${file}: ${error}`);
    });
  }

  // Fail build if no valid entries
  if (entries.length === 0) {
    console.error('\n❌ ERROR: No valid knowledge entries found!');
    process.exit(1);
  }

  // Fail build if critical packs failed
  if (stats.packsFailed > 0) {
    console.error('\n❌ ERROR: Some knowledge packs failed validation!');
    process.exit(1);
  }

  // Generate TypeScript module
  const moduleContent = generateTypeScriptModule(entries);
  writeFileSync(outputPath, moduleContent, 'utf-8');

  console.log(`\n✅ Generated: ${path.relative(projectRoot, outputPath)}`);
  console.log(`   Total entries: ${entries.length}\n`);
}

// Run generation
main();
