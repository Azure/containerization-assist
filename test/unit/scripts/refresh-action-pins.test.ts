/**
 * Transformation-focused tests for the action-pin refresh source rewriter.
 * These exercise updatePinInSource in isolation (no network) — the CLI entrypoint
 * is guarded by JEST_WORKER_ID so importing the script has no side effects.
 */

import { describe, it, expect } from '@jest/globals';
import { updatePinInSource } from '../../../scripts/refresh-action-pins';
import type { ActionPin } from '@/tools/generate-github-workflow/action-pins';

const SOURCE = `export const ACTION_PINS = {
  checkout: {
    ref: 'actions/checkout',
    sha: 'df4cb1c069e1874edd31b4311f1884172cec0e10',
    version: 'v6.0.3',
  },
  azureLogin: {
    ref: 'azure/login',
    sha: '532459ea530d8321f2fb9bb10d1e0bcf23869a43',
    version: 'v3.0.0',
  },
} as const;
`;

const checkout: ActionPin = {
  ref: 'actions/checkout',
  sha: 'df4cb1c069e1874edd31b4311f1884172cec0e10',
  version: 'v6.0.3',
};

describe('updatePinInSource', () => {
  it('rewrites the sha and version for a matched pin, leaving other pins untouched', () => {
    const newSha = 'a'.repeat(40);
    const out = updatePinInSource(SOURCE, checkout, newSha, 'v6.1.0');
    expect(out).not.toBeNull();
    expect(out).toContain(`sha: '${newSha}'`);
    expect(out).toContain("version: 'v6.1.0'");
    // azure/login pin is unchanged.
    expect(out).toContain("sha: '532459ea530d8321f2fb9bb10d1e0bcf23869a43'");
    expect(out).toContain("version: 'v3.0.0'");
  });

  it('returns null when the pin ref is absent (never reports a phantom update)', () => {
    const missing: ActionPin = {
      ref: 'actions/does-not-exist',
      sha: 'x',
      version: 'v1',
    };
    expect(updatePinInSource(SOURCE, missing, 'b'.repeat(40), 'v2')).toBeNull();
  });

  it('returns null on formatting drift (single-line pin block the pattern cannot match)', () => {
    const drifted = `export const ACTION_PINS = {
  checkout: { ref: 'actions/checkout', sha: 'df4cb1c069e1874edd31b4311f1884172cec0e10', version: 'v6.0.3' },
} as const;
`;
    expect(updatePinInSource(drifted, checkout, 'c'.repeat(40), 'v6.1.0')).toBeNull();
  });
});
