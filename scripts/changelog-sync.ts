#!/usr/bin/env tsx
/**
 * Changelog Sync Script
 *
 * Parses the latest version from CHANGELOG.md and runs `npm version <ver>`
 * to cascade the update to package.json and package-lock.json.
 *
 * The postversion lifecycle hook (see package.json) then syncs server.json.
 *
 * Usage:
 *   npx tsx scripts/changelog-sync.ts          # normal mode
 *   npx tsx scripts/changelog-sync.ts --dry-run # show what would happen
 *
 * Exit codes:
 *   0 — success (or already in sync)
 *   1 — error
 */

import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { execSync } from 'node:child_process';

const root = process.cwd();
const dryRun = process.argv.includes('--dry-run');

function extractChangelogVersion(): string {
  const filepath = join(root, 'CHANGELOG.md');
  let content: string;
  try {
    content = readFileSync(filepath, 'utf-8');
  } catch {
    console.error('✗ Could not read CHANGELOG.md');
    process.exit(1);
  }

  const match = content.match(/^## \[([0-9]+\.[0-9]+\.[0-9]+[^\]]*)\]/m);
  if (!match) {
    console.error('✗ No ## [x.y.z] heading found in CHANGELOG.md');
    process.exit(1);
  }
  return match[1];
}

function getCurrentVersion(): string {
  const filepath = join(root, 'package.json');
  try {
    const pkg = JSON.parse(readFileSync(filepath, 'utf-8'));
    return pkg.version as string;
  } catch {
    console.error('✗ Could not read package.json');
    process.exit(1);
  }
}

const changelogVersion = extractChangelogVersion();
const currentVersion = getCurrentVersion();

console.log(`CHANGELOG.md version : ${changelogVersion}`);
console.log(`package.json version : ${currentVersion}`);

if (changelogVersion === currentVersion) {
  console.log('✓ Already in sync — nothing to do');
  process.exit(0);
}

if (dryRun) {
  console.log(
    `[dry-run] Would run: npm version ${changelogVersion} --no-git-tag-version --allow-same-version`,
  );
  process.exit(0);
}

console.log(`Syncing to ${changelogVersion}...`);

try {
  // --no-git-tag-version: we handle git ourselves via pre-commit guard
  // --allow-same-version: idempotent if re-run
  execSync(`npm version ${changelogVersion} --no-git-tag-version --allow-same-version`, {
    cwd: root,
    stdio: 'inherit',
  });
  console.log(`✓ package.json and package-lock.json updated to ${changelogVersion}`);
} catch {
  console.error('✗ npm version failed');
  process.exit(1);
}
