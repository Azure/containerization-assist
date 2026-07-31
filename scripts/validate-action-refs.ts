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
import { parseDocument } from 'yaml';
// Relative import (no `@/` alias) so this stays runnable under tsx, matching how
// refresh-action-pins.ts consumes action-pins.ts.
import {
  USES_RE,
  parseActionRef,
  extractActionRefsFromDoc,
} from '../src/tools/shared/workflow-contract';

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
    const doc = parseDocument(content);

    // Deduplicate per file so a ref found by both extractors is only listed once.
    const perFile = new Map<string, ActionRef>();
    const add = (ownerRepo: string, ref: string): void => {
      const key = `${ownerRepo}@${ref}`;
      if (!perFile.has(key)) perFile.set(key, { ownerRepo, ref, key, file });
    };

    // Always harvest the AST, errors or not: `yaml` returns a usable partial tree alongside
    // `doc.errors`, and it is the only extractor that sees inline-map and flow-sequence steps
    // (`- { uses: x@sha }`). Gating this on a clean parse meant one non-fatal error — a
    // duplicate key, a stray tab — silently downgraded the whole file to the blind-spotted
    // line scan.
    for (const { ownerRepo, ref } of extractActionRefsFromDoc(doc)) {
      add(ownerRepo, ref);
    }

    if (doc.errors.length > 0) {
      // Damaged file: neither extractor is trustworthy alone. A parse error truncates the
      // subtree it occurs in, so the AST can miss refs *below* the fault that the line scan
      // still matches; the line scan in turn cannot see inline maps. Union both and report
      // loudly — a missed ref here is a silently unverified action, the exact hole this
      // script exists to close.
      console.error(
        `\u26a0 ${file}: YAML parse error (${doc.errors[0]?.message ?? 'unknown'}) — ` +
          `refs recovered from a partial parse plus a line scan; fix the file to restore ` +
          `full coverage.`,
      );
      const regex = new RegExp(USES_RE);
      let match: RegExpExecArray | null;
      while ((match = regex.exec(content)) !== null) {
        const parsed = parseActionRef(match[1] ?? '');
        if (!parsed) continue;
        add(parsed.ownerRepo, parsed.ref);
      }
    }

    refs.push(...perFile.values());
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
  console.error(err instanceof Error ? (err.stack ?? err.message) : err);
  process.exit(1);
});
