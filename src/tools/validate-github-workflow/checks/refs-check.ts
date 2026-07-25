/**
 * Layer 3 — action reference (`uses:`) SHA-pinning.
 *
 * The `uses:` grammar itself lives in `shared/workflow-contract.ts`, shared with
 * `scripts/validate-action-refs.ts` so the two cannot disagree about what counts as an
 * action reference.
 *
 * This layer is entirely offline and deterministic. Whether a pinned SHA still resolves
 * upstream is verified out-of-band instead: `scripts/refresh-action-pins.ts` refreshes the
 * `ACTION_PINS` registry via reviewed PRs, and `scripts/validate-action-refs.ts` re-checks
 * this repo's own workflows in CI. Repeating that lookup per tool run would add network
 * latency and rate-limit flakiness to a check that can only ever degrade to "unknown".
 */

import { type Document, type LineCounter } from 'yaml';
import {
  USES_RE,
  parseActionRef,
  extractActionRefsFromDoc,
  type ParsedActionRef,
} from '../../shared/workflow-contract';
import { makeIssue, lineOfOffset, createLineIndex } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

export interface ActionRef extends ParsedActionRef {
  /** 1-based line number of the `uses:` line. */
  line: number;
}

/** Build an ActionRef from a raw `owner/repo[/sub]@ref` string; returns null to skip
 * (local `./`, `docker://`, or non-pinnable shapes). */
function toActionRef(raw: string, line: number, comment?: string): ActionRef | null {
  const parsed = parseActionRef(raw, comment);
  return parsed ? { ...parsed, line } : null;
}

/**
 * Extract every pinnable `uses:` reference from the parsed document. Style-agnostic (block
 * steps, inline maps, flow sequences) and never matches `uses:` inside comments or `run:`
 * scripts. Node offsets give accurate 1-based line numbers.
 */
export function extractUsesRefsFromDoc(
  doc: Document.Parsed,
  lineCounter: LineCounter,
): ActionRef[] {
  return extractActionRefsFromDoc(doc).map(({ offset, ...parsed }) => ({
    ...parsed,
    line: lineOfOffset(lineCounter, offset),
  }));
}

/** Extract every pinnable `uses:` reference via a line scan; skips local (`./`) and
 * `docker://` refs. Used only as a fallback when the document could not be parsed. */
export function extractUsesRefs(content: string): ActionRef[] {
  const refs: ActionRef[] = [];
  // Built once for the whole scan rather than rescanning the source per match.
  const lineIndex = createLineIndex(content);
  const re = new RegExp(USES_RE);
  let match: RegExpExecArray | null;
  while ((match = re.exec(content)) !== null) {
    const actionRef = toActionRef(
      match[1] ?? '',
      lineOfOffset(lineIndex, match.index),
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

/**
 * Every pinnable `uses:` reference in the workflow.
 *
 * Node extraction is authoritative: it sees inline maps and flow sequences, and never treats
 * `uses:` inside a comment or a `run:` script as an action. So it is always used when a
 * document is available — including a document that parsed *with errors*, since `yaml` returns
 * a usable partial tree alongside `doc.errors`.
 *
 * When the parse is degraded the two extractors are unioned, because neither dominates: a
 * parse error truncates the subtree it occurs in, so the AST can miss refs *below* the fault
 * that the line scan still matches, while the line scan can never see an inline map. Gating
 * node extraction on a clean parse meant one stray tab downgraded the whole file to the
 * blind-spotted scan — and an unreported ref is an unpinned action that ships unnoticed,
 * which is precisely what this layer exists to prevent.
 */
export function collectRefs(
  content: string,
  doc: Document.Parsed | null | undefined,
  lineCounter: LineCounter,
  parseDegraded: boolean,
): ActionRef[] {
  if (!doc) return extractUsesRefs(content);

  const fromDoc = extractUsesRefsFromDoc(doc, lineCounter);
  if (!parseDegraded) return fromDoc;

  // Same occurrence found by both extractors resolves to the same 1-based line, so
  // `line:raw` dedupes without collapsing two steps that legitimately use the same action.
  const byKey = new Map<string, ActionRef>();
  for (const r of [...fromDoc, ...extractUsesRefs(content)]) {
    const key = `${r.line}:${r.raw}`;
    if (!byKey.has(key)) byKey.set(key, r);
  }
  return [...byKey.values()].sort((a, b) => a.line - b.line);
}

export function checkRefs(
  content: string,
  doc: Document.Parsed | null | undefined,
  lineCounter: LineCounter,
  parseDegraded = false,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const refs = collectRefs(content, doc, lineCounter, parseDegraded);

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
          location: `uses: ${actionPath}`,
          line: r.line,
          actionRef,
          suggestion: `Replace @${r.ref} with the commit SHA that tag resolves to, e.g. ${actionPath}@<40-char-sha> # ${r.ref}. Resolve it via: curl -s https://api.github.com/repos/${r.ownerRepo}/commits/${r.ref} | grep -m1 '"sha"'.`,
        }),
      );
      continue; // no point checking the version comment of an unpinned ref
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
          location: `uses: ${actionPath}`,
          line: r.line,
          actionRef,
        }),
      );
    }
  }

  return findings;
}
