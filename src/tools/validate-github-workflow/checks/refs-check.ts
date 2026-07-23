/**
 * Layer 3 — action reference (`uses:`) SHA-pinning.
 *
 * The extraction regex derives from scripts/validate-action-refs.ts, extended to capture
 * the optional trailing `# vX.Y.Z` comment and anchored to the start of a YAML line (so
 * commented-out lines and `uses:` text inside `run:` scripts are ignored). The SHA
 * *format* check is always offline; the optional *existence* probe is offline-safe.
 */

import { visit, isScalar, type Document } from 'yaml';
import type { ToolContext } from '@/core/context';
import { makeIssue, lineOfOffset } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

// Anchored to the start of a YAML line (optional indentation + optional list `-`) so only
// real `uses:` keys match — not commented-out lines (`# uses: ...`) or `uses:` inside a
// `run:` script. Group 1 = the `owner/repo[/sub]@ref`; group 2 = the trailing `# comment`.
const USES_RE = /^[ \t]*-?[ \t]*uses:\s*([^#\s]+@[^#\s]+)\s*(?:#\s*(.+))?/gm;

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

/** Build an ActionRef from a raw `owner/repo[/sub]@ref` string; returns null to skip
 * (local `./`, `docker://`, or non-pinnable shapes). */
function toActionRef(raw: string, line: number, comment?: string): ActionRef | null {
  const trimmed = raw.trim();
  if (!trimmed || trimmed.startsWith('./') || trimmed.startsWith('docker://')) return null;

  const at = trimmed.lastIndexOf('@');
  if (at <= 0) return null;
  const actionPath = trimmed.slice(0, at);
  const ref = trimmed.slice(at + 1);

  const parts = actionPath.split('/');
  if (parts.length < 2) return null;
  const ownerRepo = `${parts[0]}/${parts[1]}`;

  return { raw: trimmed, ownerRepo, ref, ...(comment ? { comment } : {}), line };
}

/**
 * Extract every pinnable `uses:` reference from the parsed document. Because it walks real
 * mapping pairs, `uses:` text inside `run:` scripts or comments is never matched (unlike a
 * raw line scan). Scalar node ranges give accurate 1-based line numbers, and the trailing
 * `# vX.Y.Z` comment is read from the node's own comment.
 */
export function extractUsesRefsFromDoc(doc: Document.Parsed, content: string): ActionRef[] {
  const refs: ActionRef[] = [];
  visit(doc, {
    Pair(_, pair) {
      const key = pair.key;
      const value = pair.value;
      if (!isScalar(key) || key.value !== 'uses') return;
      if (!isScalar(value) || typeof value.value !== 'string') return;
      const offset = Array.isArray(value.range) ? value.range[0] : 0;
      const comment = typeof value.comment === 'string' ? value.comment.trim() : undefined;
      const actionRef = toActionRef(value.value, lineOfOffset(content, offset), comment);
      if (actionRef) refs.push(actionRef);
    },
  });
  return refs;
}

/** Extract every pinnable `uses:` reference via a line scan; skips local (`./`) and
 * `docker://` refs. Used only as a fallback when the document could not be parsed. */
export function extractUsesRefs(content: string): ActionRef[] {
  const refs: ActionRef[] = [];
  const re = new RegExp(USES_RE);
  let match: RegExpExecArray | null;
  while ((match = re.exec(content)) !== null) {
    const actionRef = toActionRef(
      match[1] ?? '',
      lineOfOffset(content, match.index),
      match[2]?.trim(),
    );
    if (actionRef) refs.push(actionRef);
  }
  return refs;
}

/** True when `ref` is a full commit SHA (SHA-1 40 hex, or SHA-256 64 hex). */
export function isPinnedSha(ref: string): boolean {
  return /^[0-9a-f]{40}$/i.test(ref) || /^[0-9a-f]{64}$/i.test(ref);
}

/**
 * True when a trailing `uses:` comment looks like a version tag (`v6`, `v6.0`, `v6.0.3`).
 * Normalizes (trims) internally so the exported API is robust regardless of caller behavior.
 * Requires the leading `v` — matching the rule's own guidance (`# vX.Y.Z`) — and bounds it
 * to semver-shaped 1–3 numeric segments, so notes like `# pinned` or `# v6 (LTS)` do not count.
 */
export function isVersionComment(comment?: string): boolean {
  const c = comment?.trim();
  return Boolean(c && /^v\d+(?:\.\d+){0,2}$/i.test(c));
}

/**
 * Make an untrusted trailing comment safe to embed in a diagnostic message: collapse any
 * whitespace/newlines to single spaces, neutralize backticks (which would break the inline
 * code span), and cap the length so a crafted comment can't produce a huge/noisy finding.
 */
const COMMENT_MAX_LEN = 50;
const COMMENT_ELLIPSIS = '\u2026';
function sanitizeComment(comment: string): string {
  const oneLine = comment.replace(/\s+/g, ' ').replace(/`/g, "'").trim();
  if (oneLine.length <= COMMENT_MAX_LEN) return oneLine;
  return `${oneLine.slice(0, COMMENT_MAX_LEN - COMMENT_ELLIPSIS.length)}${COMMENT_ELLIPSIS}`;
}

export interface RefsCheckOptions {
  checkActionExistence: boolean;
}

export async function checkRefs(
  content: string,
  opts: RefsCheckOptions,
  ctx: ToolContext,
  doc?: Document.Parsed | null,
): Promise<WorkflowValidationIssue[]> {
  const findings: WorkflowValidationIssue[] = [];
  // Prefer parsed-node extraction so `uses:` in comments or `run:` scripts is never treated
  // as a real action; fall back to the line scan only when the YAML could not be parsed
  // (Layer 3 still runs on structurally-broken YAML).
  const refs = doc ? extractUsesRefsFromDoc(doc, content) : extractUsesRefs(content);
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

    // Normalize once so the version check and the `detail` message agree: extraction already
    // trims, but treat any whitespace-only comment as "no comment" for both branches below.
    const comment = r.comment?.trim();
    if (!isVersionComment(comment)) {
      const detail = comment
        ? `has a non-version trailing comment (\`# ${sanitizeComment(comment)}\`)`
        : 'has no version comment';
      findings.push(
        makeIssue({
          layer: 'refs',
          ruleId: 'refs/version-comment',
          severity: 'low',
          message: `Pinned action \`${r.ownerRepo}\` ${detail}. Add a trailing version comment (\`# vX\`, \`# vX.Y\`, or \`# vX.Y.Z\` — e.g. \`# v6.0.3\`) so the pinned version is human-readable.`,
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
