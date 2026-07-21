/**
 * Layer 2 — structural schema (hand-rolled, zero-dependency).
 *
 * Walks the parsed document's plain-JS projection to assert the workflow shape a
 * deploy pipeline needs: top-level `on`/`jobs`, per-job `runs-on`/`steps`, a valid
 * `needs:` graph (no missing targets, no cycles), and unknown-key nudges.
 */

import type { Document } from 'yaml';
import { makeIssue } from './helpers';
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

const KNOWN_RUNNERS = new Set([
  'ubuntu-latest',
  'ubuntu-24.04',
  'ubuntu-22.04',
  'ubuntu-20.04',
  'windows-latest',
  'windows-2025',
  'windows-2022',
  'windows-2019',
  'macos-latest',
  'macos-15',
  'macos-14',
  'macos-13',
  'macos-12',
]);

type Dict = Record<string, unknown>;

function isDict(v: unknown): v is Dict {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

export function checkSchema(doc: Document.Parsed): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const root = doc.toJS() as unknown;

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
        }),
      );
      continue;
    }

    const isReusable = 'uses' in jobRaw;

    if (!isReusable) {
      if (!('runs-on' in jobRaw)) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/missing-runs-on',
            severity: 'required',
            message: `Job "${jobId}" is missing a \`runs-on\` runner.`,
            location: `job "${jobId}"`,
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
          }),
        );
      }

      // Runner label nudge (info only — CA emits ubuntu-latest).
      const runsOn = jobRaw['runs-on'];
      if (
        typeof runsOn === 'string' &&
        !runsOn.includes('${{') &&
        runsOn !== 'self-hosted' &&
        !KNOWN_RUNNERS.has(runsOn)
      ) {
        findings.push(
          makeIssue({
            layer: 'schema',
            ruleId: 'schema/unknown-runner',
            severity: 'low',
            message: `Job "${jobId}" uses runner "${runsOn}", which is not a known GitHub-hosted runner label.`,
            location: `job "${jobId}"`,
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
        }),
      );
    }
  }

  findings.push(...checkNeedsGraph(jobs, jobIds));

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

function checkNeedsGraph(jobs: Dict, jobIds: Set<string>): WorkflowValidationIssue[] {
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
            severity: 'high',
            message: `Job "${jobId}" declares needs: [${dep}], but no job "${dep}" exists.`,
            location: `job "${jobId}"`,
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
            severity: 'high',
            message: `Cyclic \`needs:\` dependency detected involving job "${dep}". Jobs cannot depend on each other in a cycle.`,
            location: `job "${dep}"`,
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
