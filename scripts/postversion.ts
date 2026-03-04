#!/usr/bin/env tsx
/**
 * Postversion Script
 *
 * Called after `npm version` (either manually or via changelog-sync).
 * Syncs the new version into server.json at both:
 *   .version
 *   .packages[0].version
 *
 * Then stages server.json so it's included in the version commit.
 *
 * Exit codes:
 *   0 — success
 *   1 — error
 */

import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { execSync } from 'node:child_process';

const root = process.cwd();

function readJson(filename: string): Record<string, unknown> {
  const filepath = join(root, filename);
  try {
    return JSON.parse(readFileSync(filepath, 'utf-8'));
  } catch {
    console.error(`✗ Could not read ${filename}`);
    process.exit(1);
  }
}

// Read the version that npm just set
const pkg = readJson('package.json');
const newVersion = pkg.version as string;

if (!newVersion) {
  console.error('✗ No version found in package.json');
  process.exit(1);
}

// Update server.json
const serverPath = join(root, 'server.json');
const server = readJson('server.json');

const packages = server.packages as Array<{ version?: string }> | undefined;

if (!packages?.[0]) {
  console.error('✗ server.json missing expected structure (.packages[0])');
  process.exit(1);
}

server.version = newVersion;
packages[0].version = newVersion;

try {
  writeFileSync(serverPath, JSON.stringify(server, null, 2) + '\n', 'utf-8');
  console.log(`✓ server.json updated to ${newVersion}`);
} catch {
  console.error('✗ Failed to write server.json');
  process.exit(1);
}

// Stage server.json so it's included in the commit
try {
  execSync('git add server.json', { cwd: root, stdio: 'pipe' });
  console.log('✓ server.json staged');
} catch {
  // Not fatal — might not be in a git context (e.g., CI)
  console.warn('⚠ Could not stage server.json (not in a git repo?)');
}
