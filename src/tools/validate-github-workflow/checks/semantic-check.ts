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

export function checkSemantic(
  doc: Document.Parsed,
  content: string,
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
  const acrRec = rec(
    knowledge,
    'docker-build-push-acr',
    "Build and push the image with 'az acr build' ONLY — never docker/build-push-action, docker build, docker buildx, docker/setup-buildx-action, or docker/login-action.",
  );
  const forbiddenBuild: Array<{ label: string; re: RegExp }> = [
    { label: 'docker/build-push-action', re: /docker\/build-push-action/ },
    { label: 'docker/setup-buildx-action', re: /docker\/setup-buildx-action/ },
    { label: 'docker/login-action', re: /docker\/login-action/ },
    { label: 'docker buildx', re: /\bdocker\s+buildx\b/ },
    { label: 'docker build', re: /\bdocker\s+build(?!x)/ },
  ];
  for (const { label, re } of forbiddenBuild) {
    if (re.test(content)) {
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
  }
  if (!/\baz\s+acr\s+build\b/.test(content)) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/az-acr-build',
        severity: 'high',
        message:
          'No `az acr build` command found. The image should be built and pushed with `az acr build`.',
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
  if (/az\s+aks\s+get-credentials/.test(content)) {
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
  if (/azure\/setup-kubectl/.test(content)) {
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
  if (/azure\/aks-set-context/.test(content)) {
    const adminFalse = /admin:\s*['"]?false['"]?/.test(content);
    const useKubelogin = /use-kubelogin:\s*['"]?true['"]?/.test(content);
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
  if (!/azure\/login/.test(content)) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/azure-actions',
        severity: 'high',
        message:
          'No `azure/login` step found. Each job must authenticate to Azure via azure/login (OIDC).',
        suggestion: rec(
          knowledge,
          'azure-login-oidc',
          'Use azure/login with OIDC federated credentials (client-id/tenant-id/subscription-id from secrets).',
        ),
      }),
    );
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
    if (p?.['id-token'] !== 'write') {
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
    if (p?.['actions'] !== 'read' || p?.['id-token'] !== 'write') {
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

  // ── 6. Required secrets referenced ───────────────────────────────────────────
  const missingSecrets = REQUIRED_SECRETS.filter((s) => !content.includes(s));
  if (missingSecrets.length > 0) {
    findings.push(
      makeIssue({
        layer: 'semantic',
        ruleId: 'semantic/required-secrets',
        severity: 'high',
        message: `Missing reference(s) to required OIDC secret(s): ${missingSecrets.join(', ')}. Reference them via \${{ secrets.<NAME> }} in the azure/login step.`,
        suggestion: rec(
          knowledge,
          'required-secrets-guidance',
          'Store AZURE_CLIENT_ID, AZURE_TENANT_ID and AZURE_SUBSCRIPTION_ID as GitHub repository secrets.',
        ),
      }),
    );
  }

  // ── 7. Concurrency with cancel-in-progress ───────────────────────────────────
  if (!('concurrency' in root) || !/cancel-in-progress:\s*true/.test(content)) {
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
  if (
    (input.manifestFormat === 'helm' || input.manifestFormat === 'kustomize') &&
    !/azure\/k8s-bake/.test(content)
  ) {
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
