#!/usr/bin/env tsx
/**
 * Validate Action Refs Script
 *
 * Scans all GitHub Actions workflow files and verifies that every
 * uses: owner/repo@ref reference points to a real commit or tag.
 *
 * Caching:
 *   - Cross-run: reads/writes .validated-action-refs file (cached by GH Actions cache)
 *   - Within-run: in-memory Set deduplicates identical refs across workflow files
 *
 * Network failsafe:
 *   When run locally (no CI env), a connectivity probe runs first.
 *   If GitHub API is unreachable, the check is skipped with exit 0
 *   so it never blocks commits on offline machines.
 *
 * Exit codes:
 *   0 — all refs valid, or network unavailable locally
 *   1 — one or more refs could not be verified
 */

import { readFileSync, writeFileSync, existsSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const root = process.cwd();
const workflowDir = join(root, '.github', 'workflows');
const cacheFile = join(root, '.validated-action-refs');
const isCI = Boolean(process.env.CI);
const token = process.env.GH_TOKEN ?? process.env.GITHUB_TOKEN ?? '';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Lightweight fetch with timeout. Returns HTTP status (0 on network error). */
async function httpStatus(url: string, timeoutMs = 10_000): Promise<number> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const headers: Record<string, string> = {
      Accept: 'application/vnd.github.v3+json',
      'User-Agent': 'validate-action-refs',
    };
    if (token) {
      headers.Authorization = `token ${token}`;
    }
    const res = await fetch(url, { signal: controller.signal, headers });
    return res.status;
  } catch {
    return 0;
  } finally {
    clearTimeout(timer);
  }
}

/** Check whether we can reach api.github.com at all. */
async function hasNetwork(): Promise<boolean> {
  const status = await httpStatus('https://api.github.com/zen', 5_000);
  return status === 200;
}

// ---------------------------------------------------------------------------
// Parsing
// ---------------------------------------------------------------------------

interface ActionRef {
  ownerRepo: string;
  ref: string;
  key: string; // "owner/repo@ref"
  file: string;
}

function extractRefs(): ActionRef[] {
  if (!existsSync(workflowDir)) {
    console.error(`⚠ No workflow directory found at ${workflowDir}`);
    return [];
  }

  const files = readdirSync(workflowDir).filter((f) => f.endsWith('.yml') || f.endsWith('.yaml'));
  const refs: ActionRef[] = [];

  for (const file of files) {
    const content = readFileSync(join(workflowDir, file), 'utf-8');
    // Match:  uses: owner/repo@ref  or  uses: owner/repo/sub@ref
    const regex = /uses:\s*([^#\s]+@[^#\s]+)/g;
    let match: RegExpExecArray | null;
    while ((match = regex.exec(content)) !== null) {
      const raw = match[1].trim();
      // Skip docker:// and local ./ references
      if (raw.startsWith('docker://') || raw.startsWith('./')) continue;

      const [actionPath, ref] = raw.split('@');
      if (!actionPath || !ref) continue;

      // owner/repo (strip sub-paths like /init, /analyze)
      const parts = actionPath.split('/');
      if (parts.length < 2) continue;
      const ownerRepo = `${parts[0]}/${parts[1]}`;

      refs.push({ ownerRepo, ref, key: `${ownerRepo}@${ref}`, file });
    }
  }

  return refs;
}

// ---------------------------------------------------------------------------
// Cache
// ---------------------------------------------------------------------------

function loadCache(): Set<string> {
  if (!existsSync(cacheFile)) return new Set();
  return new Set(
    readFileSync(cacheFile, 'utf-8')
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean),
  );
}

function saveCache(cache: Set<string>): void {
  writeFileSync(cacheFile, [...cache].sort().join('\n') + '\n');
}

// ---------------------------------------------------------------------------
// Verification
// ---------------------------------------------------------------------------

async function verifyRef(ownerRepo: string, ref: string): Promise<boolean> {
  const isSha = /^[0-9a-f]{40}$/.test(ref);

  if (isSha) {
    const status = await httpStatus(`https://api.github.com/repos/${ownerRepo}/git/commits/${ref}`);
    return status === 200;
  }

  // Try tag first
  const tagStatus = await httpStatus(
    `https://api.github.com/repos/${ownerRepo}/git/ref/tags/${ref}`,
  );
  if (tagStatus === 200) return true;

  // Fall back to branch
  const branchStatus = await httpStatus(
    `https://api.github.com/repos/${ownerRepo}/git/ref/heads/${ref}`,
  );
  return branchStatus === 200;
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  // Network failsafe for local runs
  if (!isCI) {
    const online = await hasNetwork();
    if (!online) {
      console.log('⏭ Skipping action-ref validation (no network)');
      process.exit(0);
    }
  }

  if (!token && isCI) {
    console.warn('⚠ No GH_TOKEN/GITHUB_TOKEN set — API calls may be rate-limited');
  }

  const allRefs = extractRefs();
  if (allRefs.length === 0) {
    console.log('No action references found.');
    process.exit(0);
  }

  // Deduplicate
  const seen = new Set<string>();
  const uniqueRefs: ActionRef[] = [];
  for (const r of allRefs) {
    if (!seen.has(r.key)) {
      seen.add(r.key);
      uniqueRefs.push(r);
    }
  }

  const cache = loadCache();
  const failures: Array<{ key: string; file: string }> = [];
  let cached = 0;
  let verified = 0;

  console.log(`🔍 Validating ${uniqueRefs.length} unique action refs across workflow files...\n`);

  for (const { ownerRepo, ref, key, file } of uniqueRefs) {
    if (cache.has(key)) {
      cached++;
      continue;
    }

    const ok = await verifyRef(ownerRepo, ref);
    if (ok) {
      verified++;
      cache.add(key);
    } else {
      failures.push({ key, file });
      console.log(`❌ ${key}  (${file})`);
    }
  }

  saveCache(cache);

  console.log('');
  console.log('📊 Results');
  console.log(`   Total unique refs: ${uniqueRefs.length}`);
  console.log(`   Cached (prior runs): ${cached}`);
  console.log(`   Verified (this run): ${verified}`);
  console.log(`   Failed: ${failures.length}`);

  // Write GH Actions step summary if available
  const summaryPath = process.env.GITHUB_STEP_SUMMARY;
  if (summaryPath) {
    const lines = [
      '# 🔍 Action Reference Validation',
      '',
      '| Metric | Count |',
      '|--------|-------|',
      `| Total unique refs | ${uniqueRefs.length} |`,
      `| Cached (prior runs) | ${cached} |`,
      `| Verified (this run) | ${verified} |`,
      `| Failed | ${failures.length} |`,
    ];

    if (failures.length > 0) {
      lines.push('', '## ❌ Failed References', '');
      for (const f of failures) {
        lines.push(`- \`${f.key}\` (${f.file})`);
      }
    }

    writeFileSync(summaryPath, lines.join('\n') + '\n', { flag: 'a' });
  }

  if (failures.length > 0) {
    console.log('\n❌ Some action references could not be verified.');
    process.exit(1);
  }

  console.log('\n✅ All action references are valid');
}

main().catch((err) => {
  console.error('Unexpected error while validating action references:');
  console.error(err instanceof Error ? err.stack ?? err.message : err);
  process.exit(1);
});
