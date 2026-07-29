/**
 * Layer 2 — structural schema (hand-rolled, zero-dependency).
 *
 * Walks the parsed document's plain-JS projection to assert the workflow shape a
 * deploy pipeline needs: top-level `on`/`jobs`, per-job `runs-on`/`steps`, a valid
 * `needs:` graph (no missing targets, no cycles), and unknown-key nudges.
 */

import type { Document, LineCounter } from 'yaml';
import { makeIssue, lineOfKey, lineOfKeyOrParent } from './helpers';
import type { WorkflowValidationIssue } from '../schema';

const WORKFLOW_KEYS = new Set([
  'name',
  'run-name',
  'on',
  'permissions',
  'env',
  'defaults',
  'concurrency',
  'jobs',
]);

const JOB_KEYS = new Set([
  'name',
  'permissions',
  'needs',
  'if',
  'runs-on',
  'environment',
  'concurrency',
  'outputs',
  'env',
  'defaults',
  'steps',
  'timeout-minutes',
  'strategy',
  'continue-on-error',
  'container',
  'services',
  'uses',
]);

// Keys that are only valid on reusable-workflow-call jobs (those with `uses:`).
const REUSABLE_ONLY_JOB_KEYS = new Set(['with', 'secrets']);

// No runner-label check: it needs an allow-list that changes every time GitHub ships a
// runner, and a stale list reports valid labels as unknown. Ours had already drifted — it
// rejected every larger runner (`ubuntu-latest-8-cores`), every ARM variant
// (`ubuntu-24.04-arm`, `windows-11-arm`) and `macos-26*`, while still listing retired
// images. actionlint maintains that list properly; CA emits `ubuntu-latest` and delegates.

type Dict = Record<string, unknown>;

function isDict(v: unknown): v is Dict {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

export function checkSchema(
  doc: Document.Parsed,
  lineCounter: LineCounter,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const root = doc.toJS() as unknown;

  // Positions are recovered from the parsed Document by the same path used to read the
  // value, since `toJS()` above has already thrown them away.
  const keyLine = (path: readonly unknown[]): number | undefined =>
    lineOfKey(doc, lineCounter, path);
  const jobKeyLine = (path: readonly unknown[]): number | undefined =>
    lineOfKeyOrParent(doc, lineCounter, path);

  if (!isDict(root)) {
    findings.push(
      makeIssue({
        layer: 'schema',
        ruleId: 'schema/root',
        severity: 'required',
        message: 'Workflow must be a YAML mapping at the top level.',
      }),
    );
    return findings;
  }

  // `on:` — under YAML 1.2 core schema (yaml v2 default) this stays the string "on",
  // but guard the legacy boolean coercion ("true") just in case.
  if (!('on' in root) && !('true' in root)) {
    findings.push(
      makeIssue({
        layer: 'schema',
        ruleId: 'schema/missing-on',
        severity: 'required',
        message: 'Workflow is missing a top-level `on:` trigger block.',
      }),
    );
  }

  // Unknown top-level keys.
  for (const key of Object.keys(root)) {
    if (!WORKFLOW_KEYS.has(key) && key !== 'true') {
      findings.push(
        makeIssue({
          layer: 'schema',
          ruleId: 'schema/unknown-workflow-key',
          severity: 'medium',
          message: `Unknown top-level workflow key "${key}".`,
          location: `key "${key}"`,
          line: keyLine([key]),
        }),
      );
    }
  }

  // `jobs:` must exist and contain at least one job.
  const jobs = root.jobs;
  if (!isDict(jobs) || Object.keys(jobs).length === 0) {
    findings.push(
      makeIssue({
        layer: 'schema',
        ruleId: 'schema/missing-jobs',
        severity: 'required',
        message: 'Workflow is missing a non-empty top-level `jobs:` mapping.',
      }),
    );
    return findings;
  }

  const jobIds = new Set(Object.keys(jobs));

  for (const [jobId, jobRaw] of Object.entries(jobs)) {
    if (!isDict(jobRaw)) {
      findings.push(
        makeIssue({
          layer: 'schema',
          ruleId: 'schema/invalid-job',
          severity: 'required',
          message: `Job "${jobId}" must be a mapping.`,
          location: `job "${jobId}"`,
          line: keyLine(['jobs', jobId]),
        }),
      );
      continue;
    }

    // A job is a reusable-workflow call only when `uses` is a non-empty string. A `uses`
    // key holding any other value is malformed: flag it and fall through to the normal-job
    // checks (runs-on/steps) rather than silently treating it as a reusable call.
    const usesValue = jobRaw.uses;
    const isReusable = typeof usesValue === 'string' && usesValue.trim().length > 0;
    if ('uses' in jobRaw && !isReusable) {
      findings.push(
        makeIssue({
          layer: 'schema',
          ruleId: 'schema/invalid-uses',
          severity: 'required',
          message: `Job "${jobId}" has an invalid \`uses\` value; it must be a non-empty string referencing a reusable workflow, e.g. \`octo/repo/.github/workflows/deploy.yml@v1\` or \`./.github/workflows/deploy.yml\`.`,
          location: `job "${jobId}"`,
          line: jobKeyLine(['jobs', jobId, 'uses']),
        }),
      );
    }

    if (!isReusable) {
      if (!('runs-on' in jobRaw)) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/missing-runs-on',
            severity: 'required',
            message: `Job "${jobId}" is missing a \`runs-on\` runner.`,
            location: `job "${jobId}"`,
            line: keyLine(['jobs', jobId]),
          }),
        );
      }
      if (!('steps' in jobRaw)) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/missing-steps',
            severity: 'required',
            message: `Job "${jobId}" is missing a \`steps\` list.`,
            location: `job "${jobId}"`,
            line: keyLine(['jobs', jobId]),
          }),
        );
      } else if (!Array.isArray(jobRaw.steps)) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/invalid-steps',
            severity: 'required',
            message: `Job "${jobId}" \`steps\` must be a list.`,
            location: `job "${jobId}"`,
            line: jobKeyLine(['jobs', jobId, 'steps']),
          }),
        );
      }
    }

    // Unknown job keys. `with`/`secrets` are valid only on reusable-workflow-call
    // jobs (those with `uses:`); on a normal job they are misplaced, not merely unknown.
    for (const key of Object.keys(jobRaw)) {
      if (JOB_KEYS.has(key)) continue;
      if (REUSABLE_ONLY_JOB_KEYS.has(key)) {
        if (!isReusable) {
          findings.push(
            makeIssue({
              layer: 'schema',
              ruleId: 'schema/invalid-job-key',
              severity: 'medium',
              message: `Key "${key}" in job "${jobId}" is only valid on reusable-workflow-call jobs (those with \`uses:\`).`,
              location: `job "${jobId}"`,
              line: jobKeyLine(['jobs', jobId, key]),
            }),
          );
        }
        continue;
      }
      findings.push(
        makeIssue({
          layer: 'schema',
          ruleId: 'schema/unknown-job-key',
          severity: 'medium',
          message: `Unknown key "${key}" in job "${jobId}".`,
          location: `job "${jobId}"`,
          line: jobKeyLine(['jobs', jobId, key]),
        }),
      );
    }
  }

  findings.push(...checkNeedsGraph(jobs, jobIds, doc, lineCounter));

  return findings;
}

/** Normalize a job's `needs` value to a string array. */
function needsOf(job: unknown): string[] {
  if (!isDict(job)) return [];
  const n = job.needs;
  if (typeof n === 'string') return [n];
  if (Array.isArray(n)) return n.filter((x): x is string => typeof x === 'string');
  return [];
}

function checkNeedsGraph(
  jobs: Dict,
  jobIds: Set<string>,
  doc: Document.Parsed,
  lineCounter: LineCounter,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const graph = new Map<string, string[]>();

  for (const [jobId, job] of Object.entries(jobs)) {
    const deps = needsOf(job);
    graph.set(jobId, deps);
    for (const dep of deps) {
      if (!jobIds.has(dep)) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/unknown-needs',
            severity: 'required',
            message: `Job "${jobId}" declares needs: [${dep}], but no job "${dep}" exists.`,
            location: `job "${jobId}"`,
            line: lineOfKeyOrParent(doc, lineCounter, ['jobs', jobId, 'needs']),
          }),
        );
      }
    }
  }

  // Cycle detection via DFS with a recursion stack.
  const WHITE = 0;
  const GRAY = 1;
  const BLACK = 2;
  const color = new Map<string, number>();
  for (const id of graph.keys()) color.set(id, WHITE);
  let cycleReported = false;

  const visit = (id: string): void => {
    if (cycleReported) return;
    color.set(id, GRAY);
    for (const dep of graph.get(id) ?? []) {
      if (!graph.has(dep)) continue;
      const c = color.get(dep);
      if (c === GRAY) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/needs-cycle',
            severity: 'required',
            message: `Cyclic \`needs:\` dependency detected involving job "${dep}". Jobs cannot depend on each other in a cycle.`,
            location: `job "${dep}"`,
            line: lineOfKey(doc, lineCounter, ['jobs', dep]),
          }),
        );
        cycleReported = true;
        return;
      }
      if (c === WHITE) visit(dep);
    }
    color.set(id, BLACK);
  };

  for (const id of graph.keys()) {
    if (color.get(id) === WHITE) visit(id);
  }

  return findings;
}
