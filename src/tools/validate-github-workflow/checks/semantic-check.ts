/**
 * Layer 4 — CA semantic invariants.
 *
 * The unique value-add: encodes the exact contract generate-github-workflow promises.
 * Knowledge snippets (from github-actions-pack.json) supply the recommendation text so
 * generator and validator stay in lockstep.
 */

import type { Document } from 'yaml';
import type { CategorizedKnowledge } from '../../shared/knowledge-tool-pattern';
import { makeIssue } from './helpers';
import type { ValidateGithubWorkflowParams, WorkflowValidationIssue } from '../schema';

const REQUIRED_SECRETS = ['AZURE_CLIENT_ID', 'AZURE_TENANT_ID', 'AZURE_SUBSCRIPTION_ID'] as const;

type Dict = Record<string, unknown>;

function isDict(v: unknown): v is Dict {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/** Recommendation text from the shared knowledge pack, with a hard-coded fallback. */
function rec(knowledge: CategorizedKnowledge, id: string, fallback: string): string {
  return knowledge.all.find((s) => s.id === id)?.text ?? fallback;
}

function needsOf(job: unknown): string[] {
  if (!isDict(job)) return [];
  const n = job.needs;
  if (typeof n === 'string') return [n];
  if (Array.isArray(n)) return n.filter((x): x is string => typeof x === 'string');
  return [];
}

/**
 * Every step (as a mapping) across all jobs, in document order.
 */
function collectStepObjects(jobs: Dict): Dict[] {
  const steps: Dict[] = [];
  for (const job of Object.values(jobs)) {
    if (!isDict(job) || !Array.isArray(job.steps)) continue;
    for (const step of job.steps) {
      if (isDict(step)) steps.push(step);
    }
  }
  return steps;
}

/**
 * Collect every step's `run` script and `uses` ref across all jobs. Evaluating these
 * parsed values (rather than the raw YAML text) keeps semantic checks from matching
 * comments, docs, or echoed strings, while still covering multiline `run:` blocks
 * (which parse to a single string).
 */
function collectSteps(jobs: Dict): { runs: string[]; uses: string[] } {
  const runs: string[] = [];
  const uses: string[] = [];
  for (const step of collectStepObjects(jobs)) {
    if (typeof step.run === 'string') runs.push(step.run);
    if (typeof step.uses === 'string') uses.push(step.uses);
  }
  return { runs, uses };
}

/** A single job's `run` scripts and `uses` refs (parsed step values only). */
function jobStepStreams(job: unknown): { runs: string[]; uses: string[] } {
  const runs: string[] = [];
  const uses: string[] = [];
  if (isDict(job) && Array.isArray(job.steps)) {
    for (const step of job.steps) {
      if (!isDict(step)) continue;
      if (typeof step.run === 'string') runs.push(step.run);
      if (typeof step.uses === 'string') uses.push(step.uses);
    }
  }
  return { runs, uses };
}

export function checkSemantic(
  doc: Document.Parsed,
  knowledge: CategorizedKnowledge,
  input: ValidateGithubWorkflowParams,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const root = doc.toJS() as unknown;
  if (!isDict(root)) return findings;

  const jobs = isDict(root.jobs) ? root.jobs : {};
  const buildImage = jobs.buildImage;
  const deploy = jobs.deploy;

  // ── 1. Job keys buildImage + deploy; deploy needs [buildImage] ───────────────
  const jobKeysRec = rec(
    knowledge,
    'workflow-two-job-structure',
    "Split the workflow into two jobs — 'buildImage' and 'deploy' — with deploy depending on buildImage via needs: [buildImage].",
  );
  if (!buildImage) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/job-keys',
        severity: 'high',
        message:
          'Missing the literal `buildImage` job. Job keys must be exactly `buildImage` and `deploy`.',
        suggestion: jobKeysRec,
      }),
    );
  }
  if (!deploy) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/job-keys',
        severity: 'high',
        message:
          'Missing the literal `deploy` job. Job keys must be exactly `buildImage` and `deploy`.',
        suggestion: jobKeysRec,
      }),
    );
  } else if (!needsOf(deploy).includes('buildImage')) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/deploy-needs',
        severity: 'high',
        message:
          'The `deploy` job must declare `needs: [buildImage]` so it runs after the image is in ACR.',
        location: 'job "deploy"',
        suggestion: jobKeysRec,
      }),
    );
  }

  // ── 2. Image built with az acr build only ────────────────────────────────────
  // Evaluate parsed step `run`/`uses` values directly (not the raw YAML text) so
  // matches in comments/docs/echoed strings don't raise false failures, and multiline
  // `run:` blocks are still covered (they parse to a single string).
  const { runs: stepRuns, uses: stepUses } = collectSteps(jobs);
  const buildImageStreams = jobStepStreams(buildImage);
  const deployStreams = jobStepStreams(deploy);
  const acrRec = rec(
    knowledge,
    'docker-build-push-acr',
    "Build and push the image with 'az acr build' ONLY — never docker/build-push-action, docker build, docker buildx, docker/setup-buildx-action, or docker/login-action.",
  );
  const forbiddenUses: Array<{ label: string; re: RegExp }> = [
    { label: 'docker/build-push-action', re: /docker\/build-push-action/ },
    { label: 'docker/setup-buildx-action', re: /docker\/setup-buildx-action/ },
    { label: 'docker/login-action', re: /docker\/login-action/ },
  ];
  const forbiddenRun: Array<{ label: string; re: RegExp }> = [
    { label: 'docker buildx', re: /\bdocker\s+buildx\b/ },
    { label: 'docker build', re: /\bdocker\s+build(?!x)/ },
  ];
  const forbiddenBuild = [
    ...forbiddenUses.filter(({ re }) => stepUses.some((u) => re.test(u))),
    ...forbiddenRun.filter(({ re }) => stepRuns.some((r) => re.test(r))),
  ];
  for (const { label } of forbiddenBuild) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/az-acr-build',
        severity: 'required',
        message: `Forbidden build method \`${label}\` detected. The image must be built with \`az acr build\` (runs the build in Azure, not on the runner).`,
        suggestion: acrRec,
      }),
    );
  }
  // The image build must live in the buildImage job (scoped to that job, not a global
  // text search that a differently-named job could satisfy).
  if (buildImage && !buildImageStreams.runs.some((r) => /\baz\s+acr\s+build\b/.test(r))) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/az-acr-build',
        severity: 'high',
        message:
          'The `buildImage` job must build and push the image with `az acr build` (no `az acr build` run step found in it).',
        location: 'job "buildImage"',
        suggestion: acrRec,
      }),
    );
  }

  // ── 3. No job-level environment: ─────────────────────────────────────────────
  const envRec = rec(
    knowledge,
    'no-job-environment-oidc',
    "Do NOT add an 'environment:' key to any job — it changes the OIDC subject claim and breaks Azure federated-credential authentication.",
  );
  for (const [jobId, job] of Object.entries(jobs)) {
    if (isDict(job) && 'environment' in job) {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/no-job-environment',
          severity: 'required',
          message: `Job "${jobId}" has an \`environment:\` key. A job-level environment breaks Azure OIDC authentication (changes the token subject claim).`,
          location: `job "${jobId}"`,
          suggestion: envRec,
        }),
      );
    }
  }

  // ── 4. Correct Azure actions ─────────────────────────────────────────────────
  const azureRec = rec(
    knowledge,
    'aks-get-credentials',
    'Use azure/aks-set-context@<sha> with admin: false and use-kubelogin: true (with azure/use-kubelogin) instead of az aks get-credentials or azure/setup-kubectl.',
  );
  if (stepRuns.some((r) => /az\s+aks\s+get-credentials/.test(r))) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/azure-actions',
        severity: 'high',
        message:
          'Forbidden `az aks get-credentials` detected. Use `azure/aks-set-context` (admin: false, use-kubelogin: true) instead.',
        suggestion: azureRec,
      }),
    );
  }
  if (stepUses.some((u) => /azure\/setup-kubectl/.test(u))) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/azure-actions',
        severity: 'high',
        message:
          'Forbidden `azure/setup-kubectl` detected. kubectl is configured via `azure/aks-set-context` with kubelogin.',
        suggestion: azureRec,
      }),
    );
  }
  // Inspect the actual azure/aks-set-context step's `with` mapping (not the whole YAML)
  // so admin/use-kubelogin keys from other steps, jobs, or comments can't mask a
  // misconfigured step. `String(...)` normalizes YAML string ('false') and boolean (false)
  // scalars alike. Report once per misconfigured step.
  const aksSetContextSteps = collectStepObjects(jobs).filter(
    (s) => typeof s.uses === 'string' && /azure\/aks-set-context/i.test(s.uses),
  );
  for (const step of aksSetContextSteps) {
    const w = isDict(step.with) ? step.with : {};
    const adminFalse = String(w['admin']) === 'false';
    const useKubelogin = String(w['use-kubelogin']) === 'true';
    if (!adminFalse || !useKubelogin) {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/aks-context-flags',
          severity: 'high',
          message:
            'azure/aks-set-context must set `admin: "false"` and `use-kubelogin: "true"` for least-privilege OIDC access.',
          suggestion: azureRec,
        }),
      );
    }
  }

  // ── 5. Per-job permissions ───────────────────────────────────────────────────
  const permRec = rec(
    knowledge,
    'github-oidc-permissions',
    'buildImage needs contents: read + id-token: write; deploy additionally needs actions: read.',
  );
  const perm = (job: unknown): Dict | undefined =>
    isDict(job) && isDict(job.permissions) ? job.permissions : undefined;
  if (buildImage) {
    const p = perm(buildImage);
    if (p?.['id-token'] !== 'write' || p?.['contents'] !== 'read') {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/permissions',
          severity: 'high',
          message:
            'The `buildImage` job must set `permissions: id-token: write` (and contents: read) for the OIDC token request.',
          location: 'job "buildImage"',
          suggestion: permRec,
        }),
      );
    }
  }
  if (deploy) {
    const p = perm(deploy);
    if (p?.['actions'] !== 'read' || p?.['contents'] !== 'read' || p?.['id-token'] !== 'write') {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/permissions',
          severity: 'high',
          message:
            'The `deploy` job must set `permissions: actions: read, contents: read, id-token: write`.',
          location: 'job "deploy"',
          suggestion: permRec,
        }),
      );
    }
  }

  // Per-job deployment contract — the generator promises specific actions in each job.
  // Enforcing per job (not via a global text search) prevents an otherwise well-formed
  // two-job workflow that omits the deployment actions from silently passing.
  const loginRec = rec(
    knowledge,
    'azure-login-oidc',
    'Use azure/login with OIDC federated credentials (client-id/tenant-id/subscription-id from secrets).',
  );
  if (buildImage && !buildImageStreams.uses.some((u) => /azure\/login/i.test(u))) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/azure-actions',
        severity: 'high',
        message:
          'The `buildImage` job must include an `azure/login` step (OIDC authentication to Azure).',
        location: 'job "buildImage"',
        suggestion: loginRec,
      }),
    );
  }
  if (deploy) {
    const deployRequired: Array<{ re: RegExp; label: string }> = [
      { re: /azure\/login/i, label: 'azure/login' },
      { re: /azure\/use-kubelogin/i, label: 'azure/use-kubelogin' },
      { re: /azure\/aks-set-context/i, label: 'azure/aks-set-context' },
      { re: /azure\/k8s-deploy/i, label: 'Azure/k8s-deploy' },
    ];
    for (const { re, label } of deployRequired) {
      if (!deployStreams.uses.some((u) => re.test(u))) {
        findings.push(
          makeIssue({
            layer: 'semantic',
            ruleId: 'semantic/azure-actions',
            severity: 'high',
            message: `The \`deploy\` job must include a \`${label}\` step to authenticate and deploy to AKS.`,
            location: 'job "deploy"',
            suggestion: azureRec,
          }),
        );
      }
    }
  }

  // ── 6. Required secrets referenced ───────────────────────────────────────────
  // Each azure/login step authenticates on its own, so it must reference ALL three OIDC
  // secrets itself — one job's complete login must not paper over another job's incomplete
  // one. Inspect each login step's parsed `with` mapping (where the secrets are actually
  // consumed) rather than the raw text, so a secret named only in a comment doesn't count.
  // Aggregate the gaps per job (across all its login steps) into a single finding so a job
  // with multiple logins isn't reported repeatedly. Case-insensitive: GitHub contexts/secret
  // names are case-insensitive.
  const secretsRec = rec(
    knowledge,
    'required-secrets-guidance',
    'Store AZURE_CLIENT_ID, AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID as GitHub repository secrets.',
  );
  let sawLoginStep = false;
  for (const [jobId, job] of Object.entries(jobs)) {
    if (!isDict(job) || !Array.isArray(job.steps)) continue;
    const jobMissing = new Set<string>();
    for (const step of job.steps) {
      if (!isDict(step) || typeof step.uses !== 'string' || !/azure\/login/i.test(step.uses)) {
        continue;
      }
      sawLoginStep = true;
      const withText = (isDict(step.with) ? Object.values(step.with) : [])
        .map((v) => String(v))
        .join('\n');
      for (const s of REQUIRED_SECRETS) {
        if (!new RegExp(`\\$\\{\\{\\s*secrets\\.${s}\\s*\\}\\}`, 'i').test(withText)) {
          jobMissing.add(s);
        }
      }
    }
    if (jobMissing.size > 0) {
      // Preserve REQUIRED_SECRETS order in the message regardless of discovery order.
      const missing = REQUIRED_SECRETS.filter((s) => jobMissing.has(s));
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/required-secrets',
          severity: 'high',
          message: `The \`azure/login\` step(s) in job "${jobId}" are missing reference(s) to required OIDC secret(s): ${missing.join(', ')}. Reference them via \${{ secrets.<NAME> }} in the azure/login step.`,
          location: `job "${jobId}"`,
          suggestion: secretsRec,
        }),
      );
    }
  }
  // No azure/login step at all means none of the OIDC secrets are referenced. The per-job
  // login-presence checks only fire for the literal buildImage/deploy jobs, so surface the
  // missing secrets here too rather than letting a login-less workflow pass this rule.
  if (!sawLoginStep) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/required-secrets',
        severity: 'high',
        message: `No \`azure/login\` step found, so the required OIDC secret(s) are unreferenced: ${REQUIRED_SECRETS.join(', ')}. Add an \`azure/login\` step that passes them via \${{ secrets.<NAME> }}.`,
        suggestion: secretsRec,
      }),
    );
  }

  // ── 7. Concurrency with cancel-in-progress ───────────────────────────────────
  // Validate the parsed top-level `concurrency` mapping (not the raw text) so a
  // commented-out `cancel-in-progress: true` can't satisfy the check. Accept both the
  // boolean `true` and the string `'true'` scalar forms.
  const concurrency = root.concurrency;
  const cancelInProgress =
    isDict(concurrency) &&
    (concurrency['cancel-in-progress'] === true || concurrency['cancel-in-progress'] === 'true');
  if (!cancelInProgress) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/concurrency',
        severity: 'medium',
        message:
          'Missing a `concurrency` block with `cancel-in-progress: true` to prevent stale deploys from racing.',
        suggestion: rec(
          knowledge,
          'workflow-concurrency',
          'Add concurrency: { group: ${{ github.workflow }}-${{ github.ref }}, cancel-in-progress: true }.',
        ),
      }),
    );
  }

  // ── 8. Bake step required for helm/kustomize ─────────────────────────────────
  // Scope to the deploy job's parsed step `uses:` refs: azure/k8s-bake renders the
  // manifests that Azure/k8s-deploy consumes via step outputs (which are job-scoped), so a
  // bake step in buildImage wouldn't feed the deploy and must not satisfy this check. Using
  // parsed `uses:` (not raw YAML) also ignores the ref in a comment or echoed run-script.
  const hasBakeStep = deployStreams.uses.some((u) => /azure\/k8s-bake/i.test(u));
  if ((input.manifestFormat === 'helm' || input.manifestFormat === 'kustomize') && !hasBakeStep) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/bake-step',
        severity: 'low',
        message: `manifestFormat is "${input.manifestFormat}" but no \`azure/k8s-bake\` step was found. Helm/Kustomize manifests should be baked before deploy.`,
        suggestion: rec(
          knowledge,
          'k8s-bake-manifests',
          'Use azure/k8s-bake to render Helm charts or Kustomize overlays before Azure/k8s-deploy.',
        ),
      }),
    );
  }

  return findings;
}
