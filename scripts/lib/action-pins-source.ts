/**
 * Pure source-rewriting helper for scripts/refresh-action-pins.ts.
 *
 * Extracted into its own module so it can be unit-tested directly, without importing the
 * CLI script (which — like every other script in scripts/ — runs `main()` on load). This
 * keeps the script a plain entrypoint with no test-runner-specific guards.
 */

import type { ActionPin } from '../../src/tools/generate-github-workflow/action-pins';

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * Replace the sha/version for a single pin (anchored by its unique `ref`). Returns the
 * rewritten source, or `null` when the pin block was not matched exactly once (so callers
 * never record an "update" that did not actually change the source, e.g. on formatting drift).
 */
export function updatePinInSource(
  src: string,
  pin: ActionPin,
  sha: string,
  version: string,
): string | null {
  const re = new RegExp(
    `(ref: '${escapeRegex(pin.ref)}',\\s*\\n\\s*sha: ')[0-9a-fA-F]+(',\\s*\\n\\s*version: ')[^']*(')`,
    'g',
  );
  let count = 0;
  const out = src.replace(re, (_m, p1: string, p2: string, p3: string) => {
    count++;
    return `${p1}${sha}${p2}${version}${p3}`;
  });
  return count === 1 ? out : null;
}
