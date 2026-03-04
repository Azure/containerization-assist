#!/usr/bin/env tsx
/**
 * Validate Version Sync Script
 *
 * Checks version synchronization across all four version carriers:
 * - package.json (.version, .mcpName)
 * - package-lock.json (.version, .packages[""].version)
 * - server.json (.version, .packages[0].version, .name)
 * - CHANGELOG.md (latest ## [x.y.z] heading)
 *
 * Exit codes:
 * - 0: All versions in sync
 * - 1: Mismatch or missing data detected
 */

import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const root = process.cwd();

interface VersionSource {
  label: string;
  version: string;
}

function readJson(filename: string): Record<string, unknown> {
  const filepath = join(root, filename);
  try {
    return JSON.parse(readFileSync(filepath, 'utf-8'));
  } catch {
    console.error(`✗ Error: Could not read ${filename}`);
    process.exit(1);
  }
}

function extractChangelogVersion(filename: string): string {
  const filepath = join(root, filename);
  let content: string;
  try {
    content = readFileSync(filepath, 'utf-8');
  } catch {
    console.error(`✗ Error: Could not read ${filename}`);
    process.exit(1);
  }

  const match = content.match(/^## \[([0-9]+\.[0-9]+\.[0-9]+[^\]]*)\]/m);
  if (!match) {
    console.error(`✗ Error: No ## [x.y.z] heading found in ${filename}`);
    process.exit(1);
  }
  return match[1];
}

// --- Read all sources ---

const pkg = readJson('package.json');
const lock = readJson('package-lock.json');
const server = readJson('server.json');

const packageVersion = pkg.version as string;
const packageMcpName = pkg.mcpName as string;

const lockVersion = lock.version as string;
const lockPackages = lock.packages as Record<string, { version?: string }>;
const lockPackagesVersion = lockPackages?.['']?.version;

const serverVersion = server.version as string;
if (!serverVersion) {
  console.error('✗ Error: .version is required in server.json');
  process.exit(1);
}

const serverPackages = server.packages as Array<{ version?: string }>;
const serverPackagesVersion = serverPackages?.[0]?.version;
if (!serverPackagesVersion) {
  console.error('✗ Error: .packages[0].version is required in server.json');
  process.exit(1);
}

const serverName = server.name as string;
const changelogVersion = extractChangelogVersion('CHANGELOG.md');

// --- Validate ---

const mismatches: string[] = [];

const sources: VersionSource[] = [
  { label: 'server.json .version', version: serverVersion ?? '' },
  { label: 'server.json .packages[0].version', version: serverPackagesVersion },
  { label: 'package-lock.json .version', version: lockVersion },
  { label: 'package-lock.json .packages[""].version', version: lockPackagesVersion ?? '' },
  { label: 'CHANGELOG.md', version: changelogVersion },
];

for (const source of sources) {
  if (packageVersion !== source.version) {
    mismatches.push(
      `${source.label} (package.json=${packageVersion}, ${source.label}=${source.version})`,
    );
  }
}

if (packageMcpName !== serverName) {
  mismatches.push(`name (package.json=${packageMcpName}, server.json=${serverName})`);
}

// --- Result ---

if (mismatches.length === 0) {
  console.log(`✓ Versions in sync: ${packageVersion}`);
  process.exit(0);
} else {
  console.error(`✗ Version mismatch:\n${mismatches.map((m) => `  - ${m}`).join('\n')}`);
  process.exit(1);
}
