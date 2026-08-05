/**
 * Shared helpers for the validate-github-workflow engine:
 *   - finding construction + severity mapping (used by every layer)
 *   - workflow source resolution (inline content or file on disk)
 *
 * These are utilities, not validation layers — the four layers live in the
 * sibling *-check.ts modules.
 */

import { promises as fs } from 'node:fs';
import path from 'node:path';
import { isMap, isScalar, LineCounter, type Document } from 'yaml';
import { type Result, Success, Failure } from '@/types';
import type { ToolContext } from '@/core/context';
import { ValidationSeverity } from '@/validation/core-types';
import type {
  ValidateGithubWorkflowParams,
  WorkflowLayer,
  WorkflowValidationIssue,
} from '../schema';

// ─── Findings ─────────────────────────────────────────────────────────────────

/** Knowledge-pack severity vocabulary (a superset of the three-level enum). */
export type FindingSeverity = 'required' | 'high' | 'medium' | 'low' | 'info';

/** Map a knowledge-pack severity onto the three-level ValidationSeverity enum. */
export function mapSeverity(severity: FindingSeverity): ValidationSeverity {
  switch (severity) {
    case 'required':
      return ValidationSeverity.ERROR;
    case 'high':
    case 'medium':
      return ValidationSeverity.WARNING;
    case 'low':
    case 'info':
    default:
      return ValidationSeverity.INFO;
  }
}

export interface IssueInit {
  layer: WorkflowLayer;
  ruleId: string;
  severity: FindingSeverity;
  message: string;
  /**
   * *What* the finding is about — `job "deploy"`, `key "foo"`, `uses: actions/checkout`,
   * `indentation`. Must NOT encode a line number: consumers join it with {@link IssueInit.line}
   * and a positional value here renders as "line 12, line 12".
   */
  location?: string | undefined;
  suggestion?: string | undefined;
  actionRef?: string | undefined;
  /** *Where* — the 1-based source line. */
  line?: number | undefined;
}

/**
 * Construct a WorkflowValidationIssue. Every finding represents a failed check
 * (isValid === false); passing checks emit nothing.
 */
export function makeIssue(init: IssueInit): WorkflowValidationIssue {
  const severity = mapSeverity(init.severity);
  const isError = severity === ValidationSeverity.ERROR;
  const isWarning = severity === ValidationSeverity.WARNING;
  return {
    isValid: false,
    passed: false,
    ruleId: init.ruleId,
    layer: init.layer,
    message: init.message,
    errors: isError ? [init.message] : [],
    warnings: isWarning ? [init.message] : [],
    suggestions: init.suggestion ? [init.suggestion] : [],
    ...(init.actionRef !== undefined && { actionRef: init.actionRef }),
    ...(init.line !== undefined && { line: init.line }),
    metadata: {
      severity,
      ...(init.location !== undefined && { location: init.location }),
    },
  };
}

// ─── Positions ────────────────────────────────────────────────────────────────
// Findings are located by character offset, which has to be converted to a 1-based line.
// Scanning the source from offset 0 on every conversion would make a validation run
// O(content x findings), so the conversion always goes through a precomputed index:
//
//   - parsed sources use yaml's `LineCounter`, which the parser fills in as a side effect
//     of parsing (free) and which binary-searches its line-start table;
//   - the unparseable fallback path builds an equivalent index once via `createLineIndex`.
//
// This mirrors how `dockerfile-validator` pre-splits content once for repeated line lookups.

/**
 * 1-based line number for a character offset, via the parser's line-start index.
 *
 * `LineCounter.linePos` is 1-based for both `line` and `col` on the normal path, but has a
 * degenerate branch that returns `{ line: 0, col: <raw offset> }` when the offset precedes
 * any recorded line start (an index that was never seeded with the start of line 1). Both
 * index sources here do seed it, so that branch should be unreachable — normalize anyway
 * rather than let a 0 leak out as if it were a line number.
 */
export function lineOfOffset(lineCounter: LineCounter, offset: number): number {
  const { line } = lineCounter.linePos(offset);
  return line >= 1 ? line : 1;
}

/**
 * Build a `LineCounter` for content that was never handed to the parser (the Layer-3 line
 * scan used when YAML is unparseable). Costs one pass over the source, after which lookups
 * are the same O(log n) binary search as the parsed path.
 */
export function createLineIndex(content: string): LineCounter {
  const lineCounter = new LineCounter();
  lineCounter.addNewLine(0);
  for (let i = 0; i < content.length; i++) {
    if (content[i] === '\n') lineCounter.addNewLine(i + 1);
  }
  return lineCounter;
}

// ─── AST positions ────────────────────────────────────────────────────────
// Layers 2 and 4 reason over `doc.toJS()`, which is convenient but discards positions. These
// helpers reach back into the parsed Document to recover a line for a finding, keyed by the
// same path used to read the value. Both return undefined rather than guessing, so findings
// about absent structure simply carry no line.

/** Read the `range` start offset off a yaml node, if it has one. */
function offsetOf(node: unknown): number | undefined {
  const range = (node as { range?: readonly number[] } | null | undefined)?.range;
  return Array.isArray(range) && typeof range[0] === 'number' ? range[0] : undefined;
}

/**
 * 1-based line of the *value* at `path` — e.g. `['jobs', 'deploy', 'steps', 2]` points at the
 * third step of the deploy job.
 */
export function lineOfNode(
  doc: Document.Parsed,
  lineCounter: LineCounter,
  path: readonly unknown[],
): number | undefined {
  const offset = offsetOf(doc.getIn(path, true));
  return offset === undefined ? undefined : lineOfOffset(lineCounter, offset);
}

/**
 * 1-based line of the *key* at `path` — e.g. `['jobs', 'deploy']` points at the `deploy:` line
 * rather than at wherever the job's value happens to start. Preferred for findings about a
 * job or a mapping key, since that is what a reader looks for.
 */
export function lineOfKey(
  doc: Document.Parsed,
  lineCounter: LineCounter,
  path: readonly unknown[],
): number | undefined {
  if (path.length === 0) return undefined;
  const parentPath = path.slice(0, -1);
  const key = String(path[path.length - 1]);
  const parent = parentPath.length === 0 ? doc.contents : doc.getIn(parentPath, true);
  if (!isMap(parent)) return undefined;

  for (const item of parent.items) {
    if (isScalar(item.key) && String(item.key.value) === key) {
      const offset = offsetOf(item.key);
      return offset === undefined ? undefined : lineOfOffset(lineCounter, offset);
    }
  }
  return undefined;
}

/**
 * Line of the key at `path` when present, otherwise the key of its parent. Lets a finding
 * about a *missing* key (e.g. a job with no `permissions:`) still point at the job it
 * concerns instead of carrying no position at all.
 */
export function lineOfKeyOrParent(
  doc: Document.Parsed,
  lineCounter: LineCounter,
  path: readonly unknown[],
): number | undefined {
  return lineOfKey(doc, lineCounter, path) ?? lineOfKey(doc, lineCounter, path.slice(0, -1));
}

// ─── Source resolution ────────────────────────────────────────────────────────

export interface WorkflowSource {
  /** Raw YAML content. */
  content: string;
  /** Display path — a repo-relative path, or '<inline>' when content was supplied. */
  filePath: string;
}

/** Basename-sanitized repo-relative path for the workflow file (traversal-safe). */
export function workflowRelativePath(input: ValidateGithubWorkflowParams): string {
  const fileName =
    path.basename((input.workflowFileName ?? 'deploy.yml').replace(/\\/g, '/')) || 'deploy.yml';
  return `.github/workflows/${fileName}`;
}

/**
 * Returns `workflowContent` when supplied, otherwise reads the basename-sanitized
 * file from `<repositoryPath>/.github/workflows/`. Path traversal is prevented by
 * reducing `workflowFileName` to a bare basename.
 */
export async function resolveWorkflowSource(
  input: ValidateGithubWorkflowParams,
  ctx: ToolContext,
): Promise<Result<WorkflowSource>> {
  // A defined `workflowContent` is authoritative (documented precedence) even when blank —
  // fall back to reading from disk only when it is omitted entirely. A blank inline value
  // is a caller error, not a signal to validate a possibly-stale file on disk.
  if (input.workflowContent !== undefined) {
    if (input.workflowContent.trim().length === 0) {
      return Failure(
        'workflowContent was provided but is empty. Supply non-empty workflow YAML, or omit workflowContent to read from .github/workflows/.',
      );
    }
    return Success({ content: input.workflowContent, filePath: '<inline>' });
  }

  const relPath = workflowRelativePath(input);
  const fileName = relPath.slice('.github/workflows/'.length);
  const repoRoot = input.repositoryPath.replace(/\\/g, '/');
  const absPath = path.join(repoRoot, '.github', 'workflows', fileName);

  try {
    const content = await fs.readFile(absPath, 'utf-8');
    if (content.trim().length === 0) {
      return Failure(`Workflow file is empty: ${relPath}`);
    }
    return Success({ content, filePath: relPath });
  } catch {
    ctx.logger.warn({ absPath }, 'validate-github-workflow: workflow file not found');
    return Failure(
      `Workflow file not found: ${relPath}. Provide workflowContent or ensure the file exists under .github/workflows/.`,
    );
  }
}
