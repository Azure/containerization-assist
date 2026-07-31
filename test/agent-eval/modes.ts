import { promises as fs } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
// Side-effect import BEFORE the harness (which transitively loads CA knowledge/
// validation modules that build pino loggers at load): sets the LOG_LEVEL default.
import { oneLine, MAX_INLINE_DETAIL_CHARS } from './log-config.js';
import { assertDisposableNamespace } from './namespaces.js';
import { MCPTestHarness } from '../llm-integration/infrastructure/mcp-test-harness.js';
import { buildAksRemoteDevLoopPrompt } from '../../src/prompts/aks-loop/prompt.js';
import type { ToolSpec } from './driver.js';

const execFileP = promisify(execFile);

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const SKILLS_DIR = join(__dirname, '..', '..', 'skills');

export const BASELINE_PROMPT =
  'You are a helpful AI programming assistant. The user has asked you to ' +
  'containerize an application for deployment to Azure Kubernetes Service (AKS). ' +
  'Use the createFile tool to write any artifacts (Dockerfile, Kubernetes ' +
  'manifests, etc.) to disk.';

/**
 * User prompt for `bare` mode. Uses the SAME shared operational envelope as
 * `mcp`/`skills` (buildEvalEnvelope), plus the required attribution/k8s label
 * strings — which `bare` ALONE is given, because it has no CA layer and cannot
 * infer the arbitrary `com.azure.containerizationassist.createdby` convention.
 * `mcp`/`skills` are deliberately NOT told the labels here: their CA layer (the
 * generate-dockerfile MCP tool's `attributionLabels` / the deploy-to-aks skill)
 * supplies them, so spoon-feeding them in the prompt would mask how effectively
 * the CA layer delivers them. MCR base-image policy is withheld from every path
 * (bare included) for the same reason — it is a quality decision the CA drives.
 */
export function buildBareDeployUserPrompt(
  workingDir: string,
  ctx: AzureContext = loadAzureContext(),
): string {
  return (
    buildEvalEnvelope(workingDir, ctx) +
    `Required labels (downstream tooling depends on them): the Dockerfile MUST set ` +
    `\`LABEL com.azure.containerizationassist.createdby=<your identifier>\`, and every Kubernetes ` +
    `resource MUST carry \`app.kubernetes.io/name\` and \`app.kubernetes.io/managed-by\` under metadata.labels.\n` +
    `Containerize the repository above and deploy it to the Azure Kubernetes Service (AKS) cluster ` +
    `using the Azure values listed in the envelope. Produce a Dockerfile and Kubernetes manifests, ` +
    `then complete the build → push → apply → verify loop until the workload is Running/Ready.`
  );
}

// Skills bundled for `skills` mode: the deploy-to-aks orchestrator + every
// sub-skill it delegates to. Mirrors how vscode-aks-tools ships them.
const DEPLOY_TO_AKS_SKILL_BUNDLE: readonly string[] = [
  'deploy-to-aks',
  'analyze-repo',
  'generate-dockerfile',
  'generate-k8s-manifests',
  'fix-dockerfile',
];

export async function loadDeployToAksSkill(): Promise<string> {
  const sections: string[] = [];
  for (const name of DEPLOY_TO_AKS_SKILL_BUNDLE) {
    const text = await fs.readFile(join(SKILLS_DIR, name, 'SKILL.md'), 'utf8');
    sections.push(`### Skill: ${name}\n\n${text}`);
  }
  return (
    `${BASELINE_PROMPT}\n\n## Reference Skills (deploy-to-aks orchestrator + its dependencies)\n\n` +
    sections.join('\n\n---\n\n')
  );
}

export function dropSkillShadowedTools(tools: ToolSpec[]): ToolSpec[] {
  const shadowed = new Set<string>(DEPLOY_TO_AKS_SKILL_BUNDLE);
  const dropped = tools.filter((t) => shadowed.has(t.name)).map((t) => t.name);
  if (dropped.length) {
    console.error(`[skills] dropped MCP tools shadowed by bundled skills: ${dropped.join(', ')}`);
  } else {
    console.error(
      '[skills] WARNING: no MCP tools matched the skill bundle — nothing dropped (check tool naming).',
    );
  }
  return tools.filter((t) => !shadowed.has(t.name));
}

/** Azure context the eval injects into the dev-loop prompts. */
export interface AzureContext {
  registry: string;
  resourceGroup: string;
  clusterName: string;
  namespace: string;
  imageName: string;
}

/** Load Azure context from env vars with fallbacks to the ca-agent-eval defaults. */
export function loadAzureContext(): AzureContext {
  return {
    registry: process.env.AGENT_EVAL_REGISTRY ?? 'caagentevalacr.azurecr.io',
    resourceGroup: process.env.AGENT_EVAL_RESOURCE_GROUP ?? 'ca-agent-eval',
    clusterName: process.env.AGENT_EVAL_CLUSTER ?? 'ca-agent-eval-aks',
    namespace: process.env.AGENT_EVAL_NAMESPACE ?? 'eval-ns',
    imageName: process.env.AGENT_EVAL_IMAGE ?? 'eval-image',
  };
}

/**
 * `az acr login` the ACR derived from `ctx.registry`. ACR tokens expire (~3h),
 * so a long sweep can start with a stale credential and fail every push.
 * Best-effort: warns (doesn't throw) if it can't log in or the host isn't ACR.
 */
export async function ensureRegistryLogin(ctx: AzureContext = loadAzureContext()): Promise<void> {
  const host = ctx.registry.split('/')[0] ?? '';
  if (!host.endsWith('.azurecr.io')) return;
  const acrName = host.slice(0, -'.azurecr.io'.length);
  try {
    await execFileP('az', ['acr', 'login', '--name', acrName], { timeout: 60_000 });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(
      `[gradient] WARNING: \`az acr login --name ${acrName}\` failed — pushes may fail. ${msg.slice(0, 200)}`,
    );
  }
}

const K8S_NAME_RE = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;

function assertK8sName(kind: string, value: string): void {
  if (!K8S_NAME_RE.test(value)) {
    throw new Error(
      `Invalid ${kind} '${value}': must match RFC1123 label (^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$).`,
    );
  }
}

export async function ensureNamespace(ctx: AzureContext, namespace: string): Promise<void> {
  if (namespace === ctx.namespace) return;
  assertK8sName('namespace', namespace);
  assertK8sName('ctx.namespace', ctx.namespace);
  try {
    await execFileP('kubectl', ['create', 'namespace', namespace], { timeout: 30_000 });
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (!/AlreadyExists/i.test(msg)) {
      throw new Error(`ensureNamespace failed to create ${namespace}: ${msg}`);
    }
  }
  const host = ctx.registry.split('/')[0] ?? '';
  if (!host.endsWith('.azurecr.io')) return;
  const acrName = host.slice(0, -'.azurecr.io'.length);
  if (!K8S_NAME_RE.test(acrName)) return;
  const pullSecret = `${acrName}-pull`;
  try {
    const { stdout } = await execFileP(
      'kubectl',
      ['-n', ctx.namespace, 'get', 'secret', pullSecret, '-o', 'yaml'],
      { timeout: 30_000 },
    );
    const rewritten = stdout.replace(
      new RegExp(`^(\\s*namespace:\\s*)${ctx.namespace}\\s*$`, 'm'),
      `$1${namespace}`,
    );
    const tmpPath = join(tmpdir(), `agent-eval-pullsecret-${namespace}-${Date.now()}.yaml`);
    await fs.writeFile(tmpPath, rewritten, 'utf8');
    try {
      await execFileP('kubectl', ['apply', '-f', tmpPath], { timeout: 30_000 });
    } finally {
      await fs.unlink(tmpPath).catch(() => {});
    }
    await execFileP(
      'kubectl',
      [
        '-n',
        namespace,
        'patch',
        'serviceaccount',
        'default',
        '-p',
        JSON.stringify({ imagePullSecrets: [{ name: pullSecret }] }),
      ],
      { timeout: 30_000 },
    );
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    if (!/NotFound|not found/i.test(msg)) {
      console.error(
        `[gradient] WARNING: could not copy ACR pull secret to ${namespace} — pods may ImagePullBackOff. ${msg.slice(0, 200)}`,
      );
    }
  }
}

/**
 * Idempotent preflight that makes the eval AKS cluster disposable: reuse a
 * healthy cluster or create the next-indexed one, wire ACR pull, refresh
 * kubeconfig, and ensure the namespace — so a sweep never blocks on teardown.
 * Delegates to scripts/ensure-eval-cluster.sh (single source of truth). The
 * script's `RESOLVED_CLUSTER=` line is read back into `ctx.clusterName` /
 * `process.env.AGENT_EVAL_CLUSTER` so prompts and verify-deploy target it.
 * Throws on failure; set AGENT_EVAL_SKIP_CLUSTER_ENSURE=1 to bypass.
 */
export async function ensureEvalCluster(ctx: AzureContext = loadAzureContext()): Promise<void> {
  if (process.env.AGENT_EVAL_SKIP_CLUSTER_ENSURE) {
    console.error(
      '[gradient] AGENT_EVAL_SKIP_CLUSTER_ENSURE set — skipping AKS cluster preflight.',
    );
    return;
  }
  const script = join(__dirname, '..', '..', 'scripts', 'ensure-eval-cluster.sh');
  console.error(
    '[gradient] ensuring eval AKS cluster (reuse-or-create incremental name, attach ACR, refresh kubeconfig, ensure namespace)…',
  );
  try {
    const { stdout, stderr } = await execFileP('bash', [script], {
      timeout: 30 * 60_000,
      maxBuffer: 8 * 1024 * 1024,
      env: {
        ...process.env,
        AGENT_EVAL_RESOURCE_GROUP: ctx.resourceGroup,
        AGENT_EVAL_CLUSTER: ctx.clusterName,
        AGENT_EVAL_NAMESPACE: ctx.namespace,
        AGENT_EVAL_REGISTRY: ctx.registry,
      },
    });
    const tail = ((stderr ?? '') + (stdout ?? '')).trim().split('\n').slice(-4).join('\n');
    if (tail) console.error(tail);
    // Adopt the (possibly incremental) cluster the script selected so prompts
    // and verify-deploy target it; mutating ctx updates the shared baseCtx.
    const resolved = (stdout ?? '').match(/RESOLVED_CLUSTER=(\S+)/)?.[1];
    if (resolved && resolved !== ctx.clusterName) {
      console.error(`[gradient] using cluster ${resolved} (requested ${ctx.clusterName}).`);
      ctx.clusterName = resolved;
      process.env.AGENT_EVAL_CLUSTER = resolved;
    }
  } catch (err) {
    const e = err as { stderr?: string; stdout?: string; message?: string };
    const detail = ((e.stderr ?? '') + (e.stdout ?? '') + (e.message ?? '')).slice(
      -MAX_INLINE_DETAIL_CHARS,
    );
    throw new Error(
      `ensureEvalCluster failed — the eval AKS cluster could not be prepared.\n${detail}\n` +
        'Fix the cluster manually or set AGENT_EVAL_SKIP_CLUSTER_ENSURE=1, then re-run.',
    );
  }
}

/** Best-effort cleanup of a disposable namespace owned by the evaluation run. */
export async function cleanupAzureResources(
  ctx: AzureContext,
  ownershipToken: string,
): Promise<void> {
  assertDisposableNamespace(loadAzureContext().namespace, ctx.namespace, ownershipToken);
  try {
    await execFileP(
      'kubectl',
      [
        'delete',
        'deployment,statefulset,daemonset,replicaset,service,job,cronjob,pod,configmap,pvc',
        '--all',
        '-n',
        ctx.namespace,
        '--ignore-not-found',
        '--wait=false',
      ],
      { timeout: 60_000 },
    );
  } catch {
    // best-effort; ignore (no kubectl, no cluster, namespace absent, etc.)
  }
}

export async function deleteEvalNamespace(
  ctx: AzureContext,
  ownershipToken: string,
): Promise<void> {
  assertDisposableNamespace(loadAzureContext().namespace, ctx.namespace, ownershipToken);
  try {
    await execFileP(
      'kubectl',
      ['delete', 'namespace', ctx.namespace, '--ignore-not-found', '--wait=false'],
      { timeout: 60_000 },
    );
  } catch (err) {
    console.error(
      `[gradient] WARNING: failed to delete disposable namespace ${ctx.namespace}: ${
        err instanceof Error ? err.message : String(err)
      }`,
    );
  }
}

/**
 * Envelope prepended to dev-loop user prompts. Tells the agent the working
 * directory and the resolved Azure context so it doesn't prompt the user.
 */
export function buildEvalEnvelope(
  workingDir: string,
  ctx: AzureContext = loadAzureContext(),
): string {
  return (
    `EVALUATION HARNESS CONTEXT (read first):\n` +
    `- Repository path: ${workingDir} — use this as the working directory for every tool call.\n` +
    `- \`az\`, \`kubectl\`, and \`docker\` are configured. The current kube context targets the AKS cluster below and \`az acr login\` has already been run.\n` +
    `- Use these Azure values (do NOT prompt the user — accept as confirmed): ` +
    `registry=\`${ctx.registry}\`, resourceGroup=\`${ctx.resourceGroup}\`, clusterName=\`${ctx.clusterName}\`, namespace=\`${ctx.namespace}\`, imageName=\`${ctx.imageName}\`.\n` +
    `- Use createFile (paths relative to the working directory) to write every artefact to disk.\n` +
    `- The harness provides built-in operational tools: \`dockerBuild\` (build the image locally), \`pushImage\` (tag the built image to a registry reference and push it, verifying it landed), \`kubectlApply\` (apply manifests), and \`verifyDeploy\` (confirm the workload actually reaches Ready). Use these for the build/push/apply/verify steps — the CA MCP surface has no push or apply tool of its own.\n` +
    `- You MUST \`pushImage\` the built image to the registry BEFORE applying manifests, targeting \`${ctx.registry}/${ctx.imageName}:latest\` (or another tag). Every Kubernetes \`image:\` field MUST exactly match the reference you pushed.\n` +
    `- After writing Kubernetes manifests to disk, call \`kubectlApply\` with the manifests directory (e.g. \`{ "path": "k8s", "namespace": "${ctx.namespace}" }\`). A successful apply ONLY creates the objects — it does NOT mean the pods are healthy.\n` +
    `- You MUST then call \`verifyDeploy\` with \`{ "namespace": "${ctx.namespace}" }\` to confirm the pods reach Running/Ready. The deploy is only done when verifyDeploy returns success:true.\n` +
    `- If \`verifyDeploy\` returns success:false, treat it like a real operator would: READ the returned pod waitingReasons/waitingMessages, lastTerminated and logs, FIX the root cause (edit the manifest with createFile — e.g. add a numeric \`securityContext.runAsUser\` if the kubelet complains about a non-numeric user; or rebuild+repush if the image is missing/mismatched; or deploy a missing backing service), then call \`kubectlApply\` and \`verifyDeploy\` again. Iterate up to 3 rounds before giving up.\n` +
    `- Backing services are NOT pre-provisioned. If the app crash-loops because it needs a dependency (e.g. a database), deploy a small dev instance of that dependency (a Deployment + Service in namespace \`${ctx.namespace}\`) and wire the app's env/ConfigMap to it, then reapply.\n` +
    `- Walk every stage end-to-end: inspect the repo → write the Dockerfile → dockerBuild → write Kubernetes manifests → pushImage → kubectlApply → verifyDeploy (with the fix/reapply loop until Ready).\n\n`
  );
}

/**
 * System prompt for `mcp` mode: BASELINE + CA's real aks-loop procedural
 * workflow. aks-loop is the MCP equivalent of deploy-to-aks/SKILL.md — the
 * numbered Workflow + sharedRules block must drive the agent, so it lives in
 * the system prompt, not the user message.
 */
export function buildMcpAksLoopSystemPrompt(ctx: AzureContext = loadAzureContext()): string {
  const aksLoop = buildAksRemoteDevLoopPrompt({
    registry: ctx.registry,
    resourceGroup: ctx.resourceGroup,
    clusterName: ctx.clusterName,
    namespace: ctx.namespace,
    imageName: ctx.imageName,
  });
  return `${BASELINE_PROMPT}\n\n## Reference workflow (CA aks-loop MCP prompt)\n\n${aksLoop}`;
}

/** User prompt for `mcp` mode: short kickoff (system prompt has the workflow). */
export function buildMcpAksLoopUserPrompt(
  workingDir: string,
  ctx: AzureContext = loadAzureContext(),
): string {
  return (
    buildEvalEnvelope(workingDir, ctx) +
    `Run the **AKS remote cluster deployment iteration loop** end-to-end on the repository above using the Azure values listed in the envelope. ` +
    `Walk every numbered step of the workflow in order. Finish only after you have produced a Dockerfile, Kubernetes manifests, and attempted the deploy / verify stages.`
  );
}

/** User prompt for `skills` mode. */
export function buildSkillsAksLoopUserPrompt(
  workingDir: string,
  ctx: AzureContext = loadAzureContext(),
): string {
  return (
    buildEvalEnvelope(workingDir, ctx) +
    `Run the deploy-to-aks skill end-to-end on the repository above using the Azure values listed in the envelope. ` +
    `Walk every stage in order and finish only after you have produced a Dockerfile, Kubernetes manifests, and attempted the deploy / verify stages.`
  );
}

/** Make a kubernetes-safe slug from a model spec like 'azure:gpt-4.1' -> 'azure-gpt-4-1'. */
export function slugifyModel(model: string): string {
  return model
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/**
 * Spin up an MCP harness against `workingDir` and adapt every tool it exposes
 * into ToolSpec. Caller must await `cleanup` to stop the harness.
 */
export async function createMcpToolBundle(workingDir: string): Promise<{
  tools: ToolSpec[];
  cleanup: () => Promise<void>;
}> {
  const harness = new MCPTestHarness();
  const serverName = 'agent-eval';
  await harness.createTestServer(serverName, { workingDirectory: workingDir });
  const tools: ToolSpec[] = harness.getAvailableTools(serverName).map((t) => ({
    name: t.name,
    description: t.description,
    inputSchema: t.zodSchema,
    execute: async (args: Record<string, unknown>) => {
      const resp = await harness.executeToolCall(serverName, {
        id: `tc-${Date.now()}`,
        name: t.name,
        arguments: args,
      });
      if (resp.error) {
        console.error(`[tool] ${t.name} error: ${oneLine(String(resp.error))}`);
        return { error: resp.error };
      }
      return resp.content;
    },
  }));
  return {
    tools,
    cleanup: async () => {
      await harness.stopServer(serverName);
    },
  };
}
