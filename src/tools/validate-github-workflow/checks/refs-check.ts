/**
 * Layer 3 — action reference (`uses:`) SHA-pinning.
 *
 * The extraction regex is shared with scripts/validate-action-refs.ts (single source
 * of truth), extended to also capture the optional trailing `# vX.Y.Z` comment. The
 * SHA *format* check is always offline; the optional *existence* probe is offline-safe.
 */

import type { ToolContext } from '@/core/context';
import { makeIssue, lineOfOffset } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

// from scripts/validate-action-refs.ts — reused, then extended for the `# vX.Y.Z` comment
const USES_RE = /uses:\s*([^#\s]+@[^#\s]+)\s*(?:#\s*(.+))?/g;

export interface ActionRef {
  /** The raw `owner/repo[/sub]@ref` string. */
  raw: string;
  /** `owner/repo` (first two path segments). */
  ownerRepo: string;
  /** The ref portion (after the last `@`). */
  ref: string;
  /** Trailing `# ...` comment, if any. */
  comment?: string;
  /** 1-based line number of the `uses:` line. */
  line: number;
}

/** Extract every pinnable `uses:` reference; skips local (`./`) and `docker://` refs. */
export function extractUsesRefs(content: string): ActionRef[] {
  const refs: ActionRef[] = [];
  const re = new RegExp(USES_RE);
  let match: RegExpExecArray | null;
  while ((match = re.exec(content)) !== null) {
    const raw = match[1]?.trim();
    if (!raw) continue;
    if (raw.startsWith('./') || raw.startsWith('docker://')) continue;

    const at = raw.lastIndexOf('@');
    if (at <= 0) continue;
    const actionPath = raw.slice(0, at);
    const ref = raw.slice(at + 1);

    const parts = actionPath.split('/');
    if (parts.length < 2) continue;
    const ownerRepo = `${parts[0]}/${parts[1]}`;

    refs.push({
      raw,
      ownerRepo,
      ref,
      ...(match[2] ? { comment: match[2].trim() } : {}),
      line: lineOfOffset(content, match.index),
    });
  }
  return refs;
}

/** True when `ref` is a full commit SHA (SHA-1 40 hex, or SHA-256 64 hex). */
export function isPinnedSha(ref: string): boolean {
  return /^[0-9a-f]{40}$/i.test(ref) || /^[0-9a-f]{64}$/i.test(ref);
}

export interface RefsCheckOptions {
  checkActionExistence: boolean;
}

export async function checkRefs(
  content: string,
  opts: RefsCheckOptions,
  ctx: ToolContext,
): Promise<WorkflowValidationIssue[]> {
  const findings: WorkflowValidationIssue[] = [];
  const refs = extractUsesRefs(content);
  let existenceSkipped = false;

  for (const r of refs) {
    // Use the raw `uses:` reference so action subpaths (owner/repo/path@ref) are
    // preserved — reconstructing from ownerRepo alone drops the subpath and can point
    // sha-pin findings (and remediation) at the wrong action.
    const actionRef = r.raw;
    const actionPath = r.raw.slice(0, r.raw.lastIndexOf('@')); // owner/repo[/sub], preserves subpath

    if (!isPinnedSha(r.ref)) {
      findings.push(
        makeIssue({
          layer: 'refs',
          ruleId: 'refs/sha-pin',
          severity: 'required',
          message: `Action \`${actionRef}\` is pinned to a mutable ref. Pin to a full 40-character commit SHA to prevent supply-chain tampering.`,
          location: `line ${r.line}`,
          actionRef,
          suggestion: `Replace @${r.ref} with the commit SHA that tag resolves to, e.g. ${actionPath}@<40-char-sha> # ${r.ref}. Resolve it via: curl -s https://api.github.com/repos/${r.ownerRepo}/commits/${r.ref} | grep -m1 '"sha"'.`,
        }),
      );
      continue; // no point checking the comment or existence of an unpinned ref
    }

    if (!r.comment) {
      findings.push(
        makeIssue({
          layer: 'refs',
          ruleId: 'refs/version-comment',
          severity: 'low',
          message: `Pinned action \`${r.ownerRepo}\` has no version comment. Add a trailing \`# vX.Y.Z\` so the pinned version is human-readable.`,
          location: `line ${r.line}`,
          actionRef,
        }),
      );
    }

    if (opts.checkActionExistence) {
      const result = await verifyRefExists(r.ownerRepo, r.ref, ctx);
      if (result === 'missing') {
        findings.push(
          makeIssue({
            layer: 'refs',
            ruleId: 'refs/sha-exists',
            severity: 'high',
            message: `The pinned SHA for \`${r.ownerRepo}\` was not found upstream. It may be invalid, from a fork, or force-removed.`,
            location: `line ${r.line}`,
            actionRef,
          }),
        );
      } else if (result === 'skipped') {
        existenceSkipped = true;
      }
    }
  }

  if (existenceSkipped) {
    findings.push(
      makeIssue({
        layer: 'refs',
        ruleId: 'refs/sha-exists-skipped',
        severity: 'info',
        message:
          'SHA existence check was skipped — api.github.com is unreachable (offline) or rate-limited. Format (SHA-pinning) checks still applied.',
      }),
    );
  }

  return findings;
}

/** Fetch with timeout; returns HTTP status, or 0 on network error. */
async function httpStatus(url: string, timeoutMs = 8_000): Promise<number> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const headers: Record<string, string> = {
      Accept: 'application/vnd.github+json',
      'User-Agent': 'validate-github-workflow',
    };
    const token = process.env.GH_TOKEN ?? process.env.GITHUB_TOKEN ?? '';
    if (token) headers.Authorization = `token ${token}`;
    const res = await fetch(url, { signal: controller.signal, headers });
    return res.status;
  } catch {
    return 0;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * Offline-safe existence probe: check connectivity first, then the commit endpoint.
 * Never throws; returns 'skipped' when offline/rate-limited so the tool never fails
 * a workflow just because the network is unavailable.
 */
export async function verifyRefExists(
  ownerRepo: string,
  sha: string,
  ctx: ToolContext,
): Promise<'ok' | 'missing' | 'skipped'> {
  const online = await httpStatus('https://api.github.com/zen', 5_000);
  if (online === 0) {
    ctx.logger.debug(
      'validate-github-workflow: GitHub API unreachable, skipping SHA existence check',
    );
    return 'skipped';
  }

  const status = await httpStatus(`https://api.github.com/repos/${ownerRepo}/commits/${sha}`);
  if (status === 200) return 'ok';
  if (status === 404 || status === 422) return 'missing';
  return 'skipped'; // 403 (rate limit) or anything else — do not fail on it
}
