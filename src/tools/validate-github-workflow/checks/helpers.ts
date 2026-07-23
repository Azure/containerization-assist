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
  location?: string | undefined;
  suggestion?: string | undefined;
  actionRef?: string | undefined;
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
    metadata: {
      severity,
      ...(init.location !== undefined && { location: init.location }),
    },
  };
}

/** 1-based line number for a character offset within `content`. */
export function lineOfOffset(content: string, offset: number): number {
  let line = 1;
  const end = Math.min(offset, content.length);
  for (let i = 0; i < end; i++) {
    if (content[i] === '\n') line++;
  }
  return line;
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
