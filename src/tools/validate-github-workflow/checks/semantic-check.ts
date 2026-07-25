/**
 * Layer 4 — CA semantic invariants.
 *
 * The unique value-add: encodes the exact contract generate-github-workflow promises.
 * Knowledge snippets (from github-actions-pack.json) supply the recommendation text so
 * generator and validator stay in lockstep.
 */

import type { Document, LineCounter } from 'yaml';
import type { CategorizedKnowledge } from '../../shared/knowledge-tool-pattern';
import {
  JOB_KEYS,
  REQUIRED_SECRETS,
  BUILD_COMMAND,
  BUILD_COMMAND_RE,
  FORBIDDEN_BUILD_ACTIONS,
  FORBIDDEN_BUILD_COMMANDS,
  FORBIDDEN_BUILD_LABELS,
  FORBIDDEN_DEPLOY_ACTIONS,
  FORBIDDEN_DEPLOY_COMMANDS,
  FORBIDDEN_DEPLOY_LABELS,
  REQUIRED_DEPLOY_ACTIONS,
  LOGIN_ACTION,
  USE_KUBELOGIN_ACTION,
  AKS_SET_CONTEXT_ACTION,
  BAKE_ACTION,
  AKS_CONTEXT_FLAGS,
  actionRefPattern,
  joinWithConjunction,
} from '../../shared/workflow-contract';
import {
  makeIssue,
  lineOfKey,
  lineOfKeyOrParent,
  lineOfNode,
  type FindingSeverity,
} from './helpers';
import type { ValidateGithubWorkflowParams, WorkflowValidationIssue } from '../schema';

/**
 * Severity for the CA deployment contract.
 *
 * The generator states these rules as "⛔ CRITICAL RULES — these MUST be followed exactly",
 * so a violation has to *fail* validation and reach the `fix-files` loop, not be reported as
 * a warning the client can ignore. Reporting a workflow with renamed jobs, missing OIDC
 * secrets or no `azure/login` as "✅ passed all required checks" was the opposite of the
 * tool's purpose.
 *
 * Deliberately excluded: `semantic/bake-step`, which is conditional on the caller-supplied
 * `manifestFormat` hint. If that hint is wrong, gating on it would push the agent to add a
 * bake step the workflow does not need — a false positive is worse than an advisory there.
 */
const CONTRACT: FindingSeverity = 'required';

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
 * A step together with where it lives, so a finding about it can be given a line.
 * `doc.toJS()` discards positions, so the job id + index are kept to look the node back
 * up via the path `['jobs', jobId, 'steps', index]`.
 */
interface StepRef {
  jobId: string;
  index: number;
  step: Dict;
}

/**
 * Every step across all jobs, tagged with its job and index.
 *
 * Exposed both as a flat list (document order, for whole-workflow scans) and grouped by job
 * id, because most rules are job-scoped. Grouping once here keeps those rules O(steps)
 * overall — re-filtering the flat list inside a per-job loop would make the section
 * O(jobs × steps).
 *
 * Evaluating parsed step values (rather than the raw YAML text) keeps semantic checks from
 * matching comments, docs, or echoed strings, while still covering multiline `run:` blocks
 * (which parse to a single string).
 */
interface StepIndex {
  all: StepRef[];
  byJob: Map<string, StepRef[]>;
}

function collectStepRefs(jobs: Dict): StepIndex {
  const all: StepRef[] = [];
  const byJob = new Map<string, StepRef[]>();
  for (const [jobId, job] of Object.entries(jobs)) {
    if (!isDict(job) || !Array.isArray(job.steps)) continue;
    const jobSteps: StepRef[] = [];
    job.steps.forEach((step, index) => {
      if (!isDict(step)) return;
      const ref: StepRef = { jobId, index, step };
      jobSteps.push(ref);
      all.push(ref);
    });
    byJob.set(jobId, jobSteps);
  }
  return { all, byJob };
}

/** The `run` script of a step, when it has one. */
function runOf(s: StepRef): string | undefined {
  return typeof s.step.run === 'string' ? s.step.run : undefined;
}

/** The `uses` ref of a step, when it has one. */
function usesOf(s: StepRef): string | undefined {
  return typeof s.step.uses === 'string' ? s.step.uses : undefined;
}

/** First step whose `run` matches, or undefined. */
function findByRun(steps: readonly StepRef[], re: RegExp): StepRef | undefined {
  return steps.find((s) => {
    const run = runOf(s);
    return run !== undefined && re.test(run);
  });
}

/** First step whose `uses` matches, or undefined. */
function findByUses(steps: readonly StepRef[], re: RegExp): StepRef | undefined {
  return steps.find((s) => {
    const uses = usesOf(s);
    return uses !== undefined && re.test(uses);
  });
}

export function checkSemantic(
  doc: Document.Parsed,
  lineCounter: LineCounter,
  knowledge: CategorizedKnowledge,
  input: ValidateGithubWorkflowParams,
): WorkflowValidationIssue[] {
  const findings: WorkflowValidationIssue[] = [];
  const root = doc.toJS() as unknown;
  if (!isDict(root)) return findings;

  const jobs = isDict(root.jobs) ? root.jobs : {};
  const buildImage = jobs[JOB_KEYS.BUILD];
  const deploy = jobs[JOB_KEYS.DEPLOY];

  // Positions are recovered from the parsed Document by path, since `toJS()` drops them.
  const jobLine = (jobId: string): number | undefined =>
    lineOfKey(doc, lineCounter, ['jobs', jobId]);
  const jobKeyLine = (jobId: string, key: string): number | undefined =>
    lineOfKeyOrParent(doc, lineCounter, ['jobs', jobId, key]);
  const stepLine = (s: StepRef): number | undefined =>
    lineOfNode(doc, lineCounter, ['jobs', s.jobId, 'steps', s.index]);

  // ── 1. Job keys buildImage + deploy; deploy needs [buildImage] ───────────────
  const jobKeysRec = rec(
    knowledge,
    'workflow-two-job-structure',
    `Split the workflow into two jobs — '${JOB_KEYS.BUILD}' and '${JOB_KEYS.DEPLOY}' — with ${JOB_KEYS.DEPLOY} depending on ${JOB_KEYS.BUILD} via needs: [${JOB_KEYS.BUILD}].`,
  );
  if (!buildImage) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/job-keys',
        severity: CONTRACT,
        message: `Missing the literal \`${JOB_KEYS.BUILD}\` job. Job keys must be exactly \`${JOB_KEYS.BUILD}\` and \`${JOB_KEYS.DEPLOY}\`.`,
        suggestion: jobKeysRec,
      }),
    );
  }
  if (!deploy) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/job-keys',
        severity: CONTRACT,
        message: `Missing the literal \`${JOB_KEYS.DEPLOY}\` job. Job keys must be exactly \`${JOB_KEYS.BUILD}\` and \`${JOB_KEYS.DEPLOY}\`.`,
        suggestion: jobKeysRec,
      }),
    );
  } else if (!needsOf(deploy).includes(JOB_KEYS.BUILD)) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/deploy-needs',
        severity: CONTRACT,
        message: `The \`${JOB_KEYS.DEPLOY}\` job must declare \`needs: [${JOB_KEYS.BUILD}]\` so it runs after the image is in ACR.`,
        location: `job "${JOB_KEYS.DEPLOY}"`,
        line: jobKeyLine(JOB_KEYS.DEPLOY, 'needs'),
        suggestion: jobKeysRec,
      }),
    );
  }

  // ── 2. Image built with az acr build only ────────────────────────────────────
  // Evaluate parsed step `run`/`uses` values directly (not the raw YAML text) so
  // matches in comments/docs/echoed strings don't raise false failures, and multiline
  // `run:` blocks are still covered (they parse to a single string).
  const { all: allSteps, byJob: stepsByJob } = collectStepRefs(jobs);
  const buildImageSteps = stepsByJob.get(JOB_KEYS.BUILD) ?? [];
  const deploySteps = stepsByJob.get(JOB_KEYS.DEPLOY) ?? [];
  const acrRec = rec(
    knowledge,
    'docker-build-push-acr',
    `Build and push the image with '${BUILD_COMMAND}' ONLY — never ${joinWithConjunction(FORBIDDEN_BUILD_LABELS)}.`,
  );
  const forbiddenBuild: Array<{ label: string; hit: StepRef }> = [];
  for (const label of FORBIDDEN_BUILD_ACTIONS) {
    const hit = findByUses(allSteps, actionRefPattern(label));
    if (hit) forbiddenBuild.push({ label, hit });
  }
  for (const { label, pattern } of FORBIDDEN_BUILD_COMMANDS) {
    const hit = findByRun(allSteps, pattern);
    if (hit) forbiddenBuild.push({ label, hit });
  }
  for (const { label, hit } of forbiddenBuild) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/az-acr-build',
        severity: 'required',
        message: `Forbidden build method \`${label}\` detected. The image must be built with \`${BUILD_COMMAND}\` (runs the build in Azure, not on the runner).`,
        location: `job "${hit.jobId}"`,
        line: stepLine(hit),
        suggestion: acrRec,
      }),
    );
  }
  // The image build must live in the buildImage job (scoped to that job, not a global
  // text search that a differently-named job could satisfy).
  if (buildImage && !findByRun(buildImageSteps, BUILD_COMMAND_RE)) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/az-acr-build',
        severity: CONTRACT,
        message: `The \`${JOB_KEYS.BUILD}\` job must build and push the image with \`${BUILD_COMMAND}\` (no \`${BUILD_COMMAND}\` run step found in it).`,
        location: `job "${JOB_KEYS.BUILD}"`,
        line: jobLine(JOB_KEYS.BUILD),
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
          line: jobKeyLine(jobId, 'environment'),
          suggestion: envRec,
        }),
      );
    }
  }

  // ── 4. Correct Azure actions ─────────────────────────────────────────────────
  const azureRec = rec(
    knowledge,
    'aks-get-credentials',
    `Use ${AKS_SET_CONTEXT_ACTION}@<sha> with admin: ${AKS_CONTEXT_FLAGS.admin} and use-kubelogin: ${AKS_CONTEXT_FLAGS['use-kubelogin']} (with ${USE_KUBELOGIN_ACTION}) instead of ${joinWithConjunction(FORBIDDEN_DEPLOY_LABELS)}.`,
  );
  for (const { label, pattern } of FORBIDDEN_DEPLOY_COMMANDS) {
    const hit = findByRun(allSteps, pattern);
    if (hit) {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/azure-actions',
          severity: CONTRACT,
          message: `Forbidden \`${label}\` detected. Use \`${AKS_SET_CONTEXT_ACTION}\` (admin: ${AKS_CONTEXT_FLAGS.admin}, use-kubelogin: ${AKS_CONTEXT_FLAGS['use-kubelogin']}) instead.`,
          location: `job "${hit.jobId}"`,
          line: stepLine(hit),
          suggestion: azureRec,
        }),
      );
    }
  }
  for (const label of FORBIDDEN_DEPLOY_ACTIONS) {
    const hit = findByUses(allSteps, actionRefPattern(label));
    if (hit) {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/azure-actions',
          severity: CONTRACT,
          message: `Forbidden \`${label}\` detected. kubectl is configured via \`${AKS_SET_CONTEXT_ACTION}\` with kubelogin.`,
          location: `job "${hit.jobId}"`,
          line: stepLine(hit),
          suggestion: azureRec,
        }),
      );
    }
  }
  // Inspect the actual azure/aks-set-context step's `with` mapping (not the whole YAML)
  // so admin/use-kubelogin keys from other steps, jobs, or comments can't mask a
  // misconfigured step. `String(...)` normalizes YAML string ('false') and boolean (false)
  // scalars alike. Report once per misconfigured step.
  const aksSetContextRe = actionRefPattern(AKS_SET_CONTEXT_ACTION);
  const aksSetContextSteps = allSteps.filter((s) => {
    const uses = usesOf(s);
    return uses !== undefined && aksSetContextRe.test(uses);
  });
  for (const s of aksSetContextSteps) {
    const w = isDict(s.step.with) ? s.step.with : {};
    const misconfigured = Object.entries(AKS_CONTEXT_FLAGS).some(
      ([flag, expected]) => String(w[flag]) !== expected,
    );
    if (misconfigured) {
      findings.push(
        makeIssue({
          layer: 'semantic',
          ruleId: 'semantic/aks-context-flags',
          severity: CONTRACT,
          message: `${AKS_SET_CONTEXT_ACTION} must set \`admin: "${AKS_CONTEXT_FLAGS.admin}"\` and \`use-kubelogin: "${AKS_CONTEXT_FLAGS['use-kubelogin']}"\` for least-privilege OIDC access.`,
          location: `job "${s.jobId}"`,
          line: stepLine(s),
          suggestion: azureRec,
        }),
      );
    }
  }

  // ── 5. Per-job permissions ───────────────────────────────────────────────────
  const permRec = rec(
    knowledge,
    'github-oidc-permissions',
    `${JOB_KEYS.BUILD} needs contents: read + id-token: write; ${JOB_KEYS.DEPLOY} additionally needs actions: read.`,
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
          severity: CONTRACT,
          message: `The \`${JOB_KEYS.BUILD}\` job must set \`permissions: id-token: write\` (and contents: read) for the OIDC token request.`,
          location: `job "${JOB_KEYS.BUILD}"`,
          line: jobKeyLine(JOB_KEYS.BUILD, 'permissions'),
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
          severity: CONTRACT,
          message: `The \`${JOB_KEYS.DEPLOY}\` job must set \`permissions: actions: read, contents: read, id-token: write\`.`,
          location: `job "${JOB_KEYS.DEPLOY}"`,
          line: jobKeyLine(JOB_KEYS.DEPLOY, 'permissions'),
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
    `Use ${LOGIN_ACTION} with OIDC federated credentials (client-id/tenant-id/subscription-id from secrets).`,
  );
  const loginRe = actionRefPattern(LOGIN_ACTION);
  if (buildImage && !findByUses(buildImageSteps, loginRe)) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/azure-actions',
        severity: CONTRACT,
        message: `The \`${JOB_KEYS.BUILD}\` job must include an \`${LOGIN_ACTION}\` step (OIDC authentication to Azure).`,
        location: `job "${JOB_KEYS.BUILD}"`,
        line: jobLine(JOB_KEYS.BUILD),
        suggestion: loginRec,
      }),
    );
  }
  if (deploy) {
    for (const label of REQUIRED_DEPLOY_ACTIONS) {
      if (!findByUses(deploySteps, actionRefPattern(label))) {
        findings.push(
          makeIssue({
            layer: 'semantic',
            ruleId: 'semantic/azure-actions',
            severity: CONTRACT,
            message: `The \`${JOB_KEYS.DEPLOY}\` job must include a \`${label}\` step to authenticate and deploy to AKS.`,
            location: `job "${JOB_KEYS.DEPLOY}"`,
            line: jobLine(JOB_KEYS.DEPLOY),
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
    `Store ${REQUIRED_SECRETS.join(', ')} as GitHub repository secrets.`,
  );
  let sawLoginStep = false;
  // Jobs without a `steps` list are absent from the group map; they can hold no login step,
  // so they contribute nothing to this rule. Iteration order still follows document order.
  for (const [jobId, jobSteps] of stepsByJob) {
    const jobMissing = new Set<string>();
    let firstLogin: StepRef | undefined;
    for (const s of jobSteps) {
      const uses = usesOf(s);
      if (uses === undefined || !loginRe.test(uses)) continue;
      sawLoginStep = true;
      firstLogin ??= s;
      const withText = (isDict(s.step.with) ? Object.values(s.step.with) : [])
        .map((v) => String(v))
        .join('\n');
      for (const secret of REQUIRED_SECRETS) {
        if (!new RegExp(`\\$\\{\\{\\s*secrets\\.${secret}\\s*\\}\\}`, 'i').test(withText)) {
          jobMissing.add(secret);
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
          severity: CONTRACT,
          message: `The \`${LOGIN_ACTION}\` step(s) in job "${jobId}" are missing reference(s) to required OIDC secret(s): ${missing.join(', ')}. Reference them via \${{ secrets.<NAME> }} in the ${LOGIN_ACTION} step.`,
          location: `job "${jobId}"`,
          ...(firstLogin && { line: stepLine(firstLogin) }),
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
        severity: CONTRACT,
        message: `No \`${LOGIN_ACTION}\` step found, so the required OIDC secret(s) are unreferenced: ${REQUIRED_SECRETS.join(', ')}. Add an \`${LOGIN_ACTION}\` step that passes them via \${{ secrets.<NAME> }}.`,
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
        severity: CONTRACT,
        message:
          'Missing a `concurrency` block with `cancel-in-progress: true` to prevent stale deploys from racing.',
        // Points at an existing-but-misconfigured `concurrency:`; absent entirely, no line.
        line: lineOfKey(doc, lineCounter, ['concurrency']),
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
  const hasBakeStep = findByUses(deploySteps, actionRefPattern(BAKE_ACTION)) !== undefined;
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
