#!/usr/bin/env tsx
/**
 * Refresh Action Pins Script
 *
 * Resolves the LATEST commit SHA within each action's current major from GitHub and
 * rewrites src/tools/generate-github-workflow/action-pins.ts. Intended to run
 * periodically (scheduled CI) and land via a reviewed PR — mirroring how the codebase
 * keeps other pinned versions (e.g. prepare-cluster's KIND_VERSION) current, but
 * automated + human-reviewed.
 *
 * Modes:
 *   (default)  Rewrite action-pins.ts in place with any newer SHAs. Exit 0.
 *   --check    Report drift without writing. Exit 1 if any pin is stale/missing
 *              (useful as a CI staleness signal). Exit 0 if all current or skipped.
 *
 * Version selection: stays WITHIN each pin's current major (v6 → newest v6.x commit).
 * Major bumps are intentionally left to a human (a new major can rename inputs).
 *
 * Network failsafe: if GitHub is unreachable (or rate-limited) the affected pin is
 * skipped, never treated as a failure. Set GH_TOKEN/GITHUB_TOKEN to avoid rate limits.
 *
 * Exit codes:
 *   0 — pins refreshed / already current / skipped offline
 *   1 — (--check only) drift or a missing major tag detected
 */

import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import {
  ACTION_PINS,
  actionMajor,
  type ActionPin,
} from '../src/tools/generate-github-workflow/action-pins';
import { updatePinInSource } from './lib/action-pins-source';

const CHECK_ONLY = process.argv.includes('--check');
const token = process.env.GH_TOKEN ?? process.env.GITHUB_TOKEN ?? '';
const PINS_FILE = resolve(process.cwd(), 'src/tools/generate-github-workflow/action-pins.ts');

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

function headers(): Record<string, string> {
  const h: Record<string, string> = {
    Accept: 'application/vnd.github+json',
    'User-Agent': 'refresh-action-pins',
  };
  if (token) h.Authorization = `token ${token}`;
  return h;
}

async function ghJson(url: string, timeoutMs = 10_000): Promise<{ status: number; body: unknown }> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { signal: controller.signal, headers: headers() });
    const body = res.status === 200 ? await res.json() : null;
    return { status: res.status, body };
  } catch {
    return { status: 0, body: null };
  } finally {
    clearTimeout(timer);
  }
}

async function hasNetwork(): Promise<boolean> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 5_000);
  try {
    const res = await fetch('https://api.github.com/zen', {
      signal: controller.signal,
      headers: headers(),
    });
    return res.status === 200;
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}

// ---------------------------------------------------------------------------
// Resolution
// ---------------------------------------------------------------------------

type Resolved =
  | { kind: 'ok'; sha: string; version: string }
  | { kind: 'missing' }
  | { kind: 'skip' };

/** Resolve the newest commit SHA (and precise version tag) within `major`. */
async function resolveLatest(ref: string, major: string): Promise<Resolved> {
  const commit = await ghJson(`https://api.github.com/repos/${ref}/commits/${major}`);
  if (commit.status === 404) return { kind: 'missing' };
  const commitBody = commit.body as { sha?: string } | null;
  if (commit.status !== 200 || typeof commitBody?.sha !== 'string') return { kind: 'skip' };

  const sha: string = commitBody.sha;

  // Find the most specific semver tag (within the major) pointing at that SHA.
  let version = major;
  const tags = await ghJson(`https://api.github.com/repos/${ref}/tags?per_page=100`);
  if (tags.status === 200 && Array.isArray(tags.body)) {
    const withinMajor = new RegExp(`^${major}(\\.|$)`);
    const matches = (tags.body as Array<{ name: string; commit?: { sha?: string } }>)
      .filter(
        (t) =>
          typeof t.commit?.sha === 'string' &&
          t.commit.sha.toLowerCase() === sha.toLowerCase() &&
          withinMajor.test(t.name),
      )
      .map((t) => t.name)
      .sort((a, b) => b.length - a.length); // prefer the most specific (e.g. v6.0.4 over v6)
    if (matches[0]) version = matches[0];
  }

  return { kind: 'ok', sha, version };
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  if (!(await hasNetwork())) {
    console.log('⏭ Skipping action-pin refresh (no network)');
    process.exit(0);
  }
  if (!token) {
    console.warn('⚠ No GH_TOKEN/GITHUB_TOKEN set — API calls may be rate-limited');
  }

  let src = readFileSync(PINS_FILE, 'utf-8');
  const pins = Object.values(ACTION_PINS) as ActionPin[];

  const changes: string[] = [];
  const missing: string[] = [];
  let skipped = 0;

  for (const pin of pins) {
    const major = actionMajor(pin.version);
    const result = await resolveLatest(pin.ref, major);

    if (result.kind === 'missing') {
      missing.push(`${pin.ref}: major tag ${major} not found on GitHub`);
      console.log(`❌ ${pin.ref}@${major} — major tag not found`);
      continue;
    }
    if (result.kind === 'skip') {
      skipped++;
      console.log(`⏭ ${pin.ref}@${major} — skipped (network/rate limit)`);
      continue;
    }

    if (result.sha.toLowerCase() === pin.sha.toLowerCase()) {
      console.log(`✅ ${pin.ref}@${pin.version} — up to date`);
      continue;
    }

    const rewritten = updatePinInSource(src, pin, result.sha, result.version);
    if (rewritten === null) {
      missing.push(`${pin.ref}: stale pin could not be rewritten (source pattern not matched)`);
      console.log(`❌ ${pin.ref} — could not rewrite pin in source (no change written)`);
      continue;
    }
    src = rewritten;

    changes.push(
      `${pin.ref}: ${pin.version} ${pin.sha.slice(0, 12)}… → ${result.version} ${result.sha.slice(0, 12)}…`,
    );
    console.log(`🔄 ${pin.ref}: ${pin.version} → ${result.version} (${result.sha.slice(0, 12)}…)`);
  }

  console.log('\n📊 Results');
  console.log(`   Total pins: ${pins.length}`);
  console.log(`   Updated:    ${changes.length}`);
  console.log(`   Skipped:    ${skipped}`);
  console.log(`   Missing:    ${missing.length}`);

  const summaryPath = process.env.GITHUB_STEP_SUMMARY;
  if (summaryPath) {
    const lines = ['# 🔄 Action Pin Refresh', ''];
    if (changes.length) {
      lines.push('## Updated', '', ...changes.map((c) => `- ${c}`), '');
    }
    if (missing.length) {
      lines.push('## ❌ Missing major tags', '', ...missing.map((m) => `- ${m}`), '');
    }
    if (!changes.length && !missing.length) lines.push('All pins are up to date. ✅');
    writeFileSync(summaryPath, lines.join('\n') + '\n', { flag: 'a' });
  }

  if (CHECK_ONLY) {
    if (changes.length || missing.length) {
      console.log('\n❌ Pins are stale or missing (run `npm run refresh:action-pins` to update).');
      process.exit(1);
    }
    console.log('\n✅ All action pins are current.');
    return;
  }

  if (missing.length) {
    console.log('\n❌ A tracked major tag was not found — review before committing.');
    process.exit(1);
  }
  if (changes.length) {
    writeFileSync(PINS_FILE, src);
    console.log(`\n✅ Updated ${changes.length} pin(s) in ${PINS_FILE}`);
  } else {
    console.log('\n✅ All action pins already up to date — no changes written.');
  }
}

main().catch((err) => {
  console.error('Unexpected error while refreshing action pins:');
  console.error(err instanceof Error ? (err.stack ?? err.message) : err);
  process.exit(1);
});
