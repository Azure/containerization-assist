#!/usr/bin/env tsx
/**
 * Script to remove backward compatibility code from tools
 */

import { promises as fs } from 'node:fs';
import * as path from 'node:path';

async function removeBackwardCompatCode(filePath: string): Promise<number> {
  let content = await fs.readFile(filePath, 'utf-8');
  let changes = 0;

  // Pattern to match backward compatibility session update blocks
  const backwardCompatPattern = /\s*\/\/ Update session metadata for backward compatibility[\s\S]*?catch \(error\) \{[\s\S]*?\}\s*\}/g;

  const matches = content.match(backwardCompatPattern);
  if (matches) {
    console.log(`Found ${matches.length} backward compat blocks in ${path.basename(path.dirname(filePath))}/tool.ts`);

    // Remove each block
    content = content.replace(backwardCompatPattern, '');
    changes = matches.length;
  }

  // Also remove any empty try-catch blocks that might remain
  content = content.replace(/\s*try \{\s*\} catch \(error\) \{\s*\}/g, '');

  // Clean up extra blank lines (more than 2 consecutive)
  content = content.replace(/\n{3,}/g, '\n\n');

  if (changes > 0) {
    await fs.writeFile(filePath, content, 'utf-8');
  }

  return changes;
}

async function main() {
  console.log('🧹 Removing backward compatibility code...\n');

  const toolsDir = path.join(process.cwd(), 'src', 'tools');
  const entries = await fs.readdir(toolsDir, { withFileTypes: true });

  let totalChanges = 0;
  let filesChanged = 0;

  for (const entry of entries) {
    if (entry.isDirectory()) {
      const toolFile = path.join(toolsDir, entry.name, 'tool.ts');
      try {
        await fs.access(toolFile);
        const changes = await removeBackwardCompatCode(toolFile);
        if (changes > 0) {
          filesChanged++;
          totalChanges += changes;
        }
      } catch {
        // File doesn't exist, skip
      }
    }
  }

  console.log('\n📊 Summary:');
  console.log(`   Files processed: ${entries.length}`);
  console.log(`   Files changed: ${filesChanged}`);
  console.log(`   Blocks removed: ${totalChanges}`);

  // Also remove the migration script
  try {
    await fs.unlink(path.join(process.cwd(), 'scripts', 'migrate-tools.ts'));
    console.log('\n✅ Removed migration script');
  } catch {
    // Already removed
  }
}

main().catch(console.error);