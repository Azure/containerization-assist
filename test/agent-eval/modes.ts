import { promises as fs } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { MCPTestHarness } from '../llm-integration/infrastructure/mcp-test-harness.js';
import { buildAksRemoteDevLoopPrompt } from '../../src/prompts/aks-loop/prompt.js';
import type { ToolSpec } from './driver.js';

const execFileP = promisify(execFile);

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const SKILLS_DIR = join(__dirname, '..', '..', 'skills');

export const BASELINE_PROMPT =
  'You are a helpful AI programming assistant. The user has asked you to ' +
  'containerize an application. Use the createFile tool to write any artifacts ' +
  '(Dockerfile, Kubernetes manifests, etc.) to disk.';

export const USER_PROMPT = (workingDir: string): string =>
  `The application source is at: ${workingDir}\n\n` +
  'Containerize this application. Generate a Dockerfile and any Kubernetes ' +
  'manifests appropriate for the application. When tools require a repository ' +
  `or working directory path, pass the absolute path above (${workingDir}). ` +
  'Write all artifacts using the createFile tool with paths relative to the ' +
  'working directory. After the Dockerfile is written, call the dockerBuild ' +
  'tool to verify it builds; if it fails, fix the Dockerfile and retry (up to ' +
  '3 attempts).';

// Skills bundled for `skills` mode: the deploy-to-aks orchestrator + every
// sub-skill it delegates to. Mirrors how vscode-aks-tools ships them.
const DEPLOY_TO_AKS_SKILL_BUNDLE: readonly string[] = [
  'deploy-to-aks',
  'analyze-repo',
  'generate-dockerfile',
  'generate-k8s-manifests',
  'fix-dockerfile',
];

export type Mode = 'baseline' | 'skills' | 'mcp';

export interface ResolvedMode {
  systemPrompt: string;
  /** Optional override for the default USER_PROMPT (set by modes that ship their own). */
  userPrompt?: string;
  tools: ToolSpec[];
}

export interface ResolvedModeBundle {
  resolved: ResolvedMode;
  cleanup: () => Promise<void>;
}

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

/** Azure context the eval injects into the dev-loop prompts. */
export interface AzureContext {
  registry: string;
  resourceGroup: string;
  clusterName: string;
  namespace: string;
  imageName: string;
}

/** Load Azure context from env vars with fallbacks to the ca-eval-* defaults. */
export function loadAzureContext(): AzureContext {
  return {
    registry: process.env.AGENT_EVAL_REGISTRY ?? 'caevalacr.azurecr.io',
    resourceGroup: process.env.AGENT_EVAL_RESOURCE_GROUP ?? 'ca-test-suite',
    clusterName: process.env.AGENT_EVAL_CLUSTER ?? 'ca-eval-aks',
    namespace: process.env.AGENT_EVAL_NAMESPACE ?? 'eval-ns',
    imageName: process.env.AGENT_EVAL_IMAGE ?? 'eval-image',
  };
}

/**
 * Ensure the local Docker client is authenticated to the ACR derived from
 * `ctx.registry` via `az acr login`. ACR tokens expire (~3h), so a long sweep
 * can begin with a stale credential and then every `pushImage` fails, so the
 * image never lands and the deploy comes up ImagePullBackOff. Best-effort:
 * warns (doesn't throw) if it can't log in (no `az`, the registry isn't ACR,
 * not signed in, etc.).
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

/**
 * Idempotent preflight that guarantees the eval AKS cluster is usable: creates
 * it if missing, waits out any in-progress deletion, attaches the ACR, refreshes
 * kubeconfig, ensures the eval namespace, and pushes the GC `deletion_due_time`
 * tag forward. The ca-eval sandbox subscription stamps that TTL tag and garbage-
 * collects the cluster when it expires, so without this a run can find the
 * cluster mid-deletion — `kubectl` then just fails to resolve the API server.
 * This makes the cluster disposable instead of a pet someone must remember to
 * recreate.
 *
 * Delegates to scripts/ensure-eval-cluster.sh (single source of truth, also
 * runnable by hand). The script uses INCREMENTAL cluster names: it reuses any
 * healthy `<base>` / `<base>-N` cluster, and if none is healthy it creates the
 * next-indexed one INSTEAD of waiting for a dying cluster to finish deleting —
 * so a sweep never blocks on teardown. The resolved name is read back from the
 * script's `RESOLVED_CLUSTER=` line and written into `ctx.clusterName` (and
 * `process.env.AGENT_EVAL_CLUSTER`) so prompts/verify-deploy target it.
 *
 * Throws on failure, since the whole sweep depends on a reachable cluster. Set
 * AGENT_EVAL_SKIP_CLUSTER_ENSURE=1 to bypass (e.g. when you manage the cluster
 * yourself or target a non-Azure / kind context).
 */
export async function ensureEvalCluster(ctx: AzureContext = loadAzureContext()): Promise<void> {
  if (process.env.AGENT_EVAL_SKIP_CLUSTER_ENSURE) {
    console.error('[gradient] AGENT_EVAL_SKIP_CLUSTER_ENSURE set — skipping AKS cluster preflight.');
    return;
  }
  const script = join(__dirname, '..', '..', 'scripts', 'ensure-eval-cluster.sh');
  console.error(
    '[gradient] ensuring eval AKS cluster (reuse-or-create incremental name, attach ACR, refresh kubeconfig, bump GC TTL)…',
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
    const tail = ((stderr ?? '') + (stdout ?? ''))
      .trim()
      .split('\n')
      .slice(-4)
      .join('\n');
    if (tail) console.error(tail);
    // The script may have selected/created a different (incremental) cluster.
    // Adopt that name so prompts and verify-deploy target the right cluster;
    // mutating ctx in place updates the shared baseCtx the runner spreads from.
    const resolved = (stdout ?? '').match(/RESOLVED_CLUSTER=(\S+)/)?.[1];
    if (resolved && resolved !== ctx.clusterName) {
      console.error(`[gradient] using cluster ${resolved} (requested ${ctx.clusterName}).`);
      ctx.clusterName = resolved;
      process.env.AGENT_EVAL_CLUSTER = resolved;
    }
  } catch (err) {
    const e = err as { stderr?: string; stdout?: string; message?: string };
    const detail = ((e.stderr ?? '') + (e.stdout ?? '') + (e.message ?? '')).slice(-1000);
    throw new Error(
      `ensureEvalCluster failed — the eval AKS cluster could not be prepared.\n${detail}\n` +
        'Fix the cluster manually or set AGENT_EVAL_SKIP_CLUSTER_ENSURE=1, then re-run.',
    );
  }
}

/**
 * Best-effort cleanup so runs don't inherit a stuck pod from a previous attempt.
 * The agent names its Deployment after the working directory (e.g.
 * `agent-eval-grad-xxxx`), NOT after `ctx.imageName`, so we can't delete by a
 * fixed name. Instead we find Deployments in the namespace whose container image
 * is the one this run pushed (`<registry>/<imageName>:`) and delete those plus
 * their backing Service. The trailing `:` keeps the match exact per repo so
 * `eval-image-azure-gpt-4-1` doesn't also match `eval-image-azure-gpt-4-1-mini`.
 */
export async function cleanupAzureResources(ctx: AzureContext = loadAzureContext()): Promise<void> {
  const needle = `${ctx.registry}/${ctx.imageName}:`;
  try {
    const { stdout } = await execFileP(
      'kubectl',
      [
        'get',
        'deployment',
        '-n',
        ctx.namespace,
        '-o',
        'jsonpath={range .items[*]}{.metadata.name}{"\\t"}{.spec.template.spec.containers[*].image}{"\\n"}{end}',
      ],
      { timeout: 30_000 },
    );
    const names = stdout
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l.includes(needle))
      .map((l) => l.split('\t')[0])
      .filter((n): n is string => Boolean(n));
    for (const name of names) {
      await execFileP(
        'kubectl',
        ['delete', 'deployment,service', name, '-n', ctx.namespace, '--ignore-not-found', '--wait=false'],
        { timeout: 30_000 },
      ).catch(() => {});
    }
  } catch {
    // best-effort; ignore (no kubectl, no cluster, namespace absent, etc.)
  }
}

/**
 * Idempotently spin up the shared backing databases the fleet's fixtures may
 * need (Postgres + MySQL), each as a single-pod Deployment + ClusterIP Service
 * in the eval namespace. Mirrors the Azure-customer reality where a managed DB
 * endpoint already exists in the namespace's DNS — the agent's job is to wire
 * env/ConfigMap to it, not provision it. Uses `emptyDir` storage so no PVC is
 * required; data is wiped when the pod restarts (fine for eval).
 *
 * The Service names are stable so the eval envelope can advertise them:
 *   - eval-postgres:5432  (user=postgres, password=eval, db=eval)
 *   - eval-mysql:3306     (user=root,     password=eval, db=eval; also `eval`/`eval`)
 *
 * Best-effort: warns (doesn't throw) if kubectl/cluster is unreachable.
 * Set `AGENT_EVAL_SKIP_DB_PROVISION=1` to skip.
 */
export async function provisionEvalDatabases(ctx: AzureContext = loadAzureContext()): Promise<void> {
  if (process.env.AGENT_EVAL_SKIP_DB_PROVISION) {
    console.error('[gradient] AGENT_EVAL_SKIP_DB_PROVISION set — skipping DB provisioning.');
    return;
  }
  const manifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: eval-postgres
  namespace: ${ctx.namespace}
  labels: { app: eval-postgres, managed-by: containerization-assist-eval }
spec:
  replicas: 1
  selector: { matchLabels: { app: eval-postgres } }
  template:
    metadata:
      labels: { app: eval-postgres }
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports: [{ containerPort: 5432 }]
          env:
            - { name: POSTGRES_USER,     value: postgres }
            - { name: POSTGRES_PASSWORD, value: eval }
            - { name: POSTGRES_DB,       value: eval }
            - { name: PGDATA,            value: /var/lib/postgresql/data/pgdata }
          volumeMounts:
            - { name: data, mountPath: /var/lib/postgresql/data }
          readinessProbe:
            exec: { command: [pg_isready, -U, postgres] }
            periodSeconds: 5
      volumes:
        - { name: data, emptyDir: {} }
---
apiVersion: v1
kind: Service
metadata:
  name: eval-postgres
  namespace: ${ctx.namespace}
spec:
  selector: { app: eval-postgres }
  ports: [{ port: 5432, targetPort: 5432 }]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: eval-mysql
  namespace: ${ctx.namespace}
  labels: { app: eval-mysql, managed-by: containerization-assist-eval }
spec:
  replicas: 1
  selector: { matchLabels: { app: eval-mysql } }
  template:
    metadata:
      labels: { app: eval-mysql }
    spec:
      containers:
        - name: mysql
          image: mysql:8
          ports: [{ containerPort: 3306 }]
          env:
            - { name: MYSQL_ROOT_PASSWORD, value: eval }
            - { name: MYSQL_DATABASE,      value: eval }
            - { name: MYSQL_USER,          value: eval }
            - { name: MYSQL_PASSWORD,      value: eval }
          volumeMounts:
            - { name: data, mountPath: /var/lib/mysql }
          readinessProbe:
            exec: { command: [bash, -c, 'mysqladmin ping -h 127.0.0.1 -u root -p"$MYSQL_ROOT_PASSWORD" --silent'] }
            periodSeconds: 5
      volumes:
        - { name: data, emptyDir: {} }
---
apiVersion: v1
kind: Service
metadata:
  name: eval-mysql
  namespace: ${ctx.namespace}
spec:
  selector: { app: eval-mysql }
  ports: [{ port: 3306, targetPort: 3306 }]
`;
  try {
    await new Promise<void>((resolveApply, rejectApply) => {
      const child = execFile(
        'kubectl',
        ['apply', '-n', ctx.namespace, '-f', '-'],
        { timeout: 60_000 },
        (err) => (err ? rejectApply(err) : resolveApply()),
      );
      child.stdin?.end(manifest);
    });
    console.error(`[gradient] DB sidecars applied (eval-postgres, eval-mysql) in namespace ${ctx.namespace}`);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    console.error(
      `[gradient] WARNING: could not provision DB sidecars — apps that need a database may CrashLoopBackOff. ${msg.slice(0, 300)}`,
    );
  }
}

/**
 * Independent, harness-measured readiness signal: did the workload this run
 * pushed actually reach Running/Ready on the cluster? This is the ground-truth
 * "did it deploy" metric — measured by the harness, NOT trusting the agent's
 * own claims or its verifyDeploy calls. We look at Deployments whose container
 * image matches this run's pushed reference (`<registry>/<imageName>:`) and
 * report ready only when every matched Deployment has
 * `readyReplicas >= replicas` (and replicas > 0). Best-effort: returns
 * `ready:false` with a detail string if kubectl/cluster/namespace is absent.
 * Call this BEFORE cleanupAzureResources tears the deployment down.
 */
export async function probeDeployReady(
  ctx: AzureContext = loadAzureContext(),
): Promise<{ ready: boolean; detail: string }> {
  const needle = `${ctx.registry}/${ctx.imageName}:`;
  try {
    const { stdout } = await execFileP(
      'kubectl',
      [
        'get',
        'deployment',
        '-n',
        ctx.namespace,
        '-o',
        'jsonpath={range .items[*]}{.metadata.name}{"\\t"}{.spec.template.spec.containers[*].image}{"\\t"}{.spec.replicas}{"\\t"}{.status.readyReplicas}{"\\n"}{end}',
      ],
      { timeout: 30_000 },
    );
    const rows = stdout
      .split('\n')
      .map((l) => l.trim())
      .filter((l) => l.includes(needle))
      .map((l) => l.split('\t'));
    if (rows.length === 0) {
      return { ready: false, detail: "no Deployment matching this run's pushed image was found in the namespace" };
    }
    const allReady = rows.every((p) => {
      const want = Number(p[2] ?? '0');
      const got = Number(p[3] ?? '0');
      return want > 0 && got >= want;
    });
    const detail = rows.map((p) => `${p[0]} ${Number(p[3] ?? 0)}/${Number(p[2] ?? 0)} ready`).join('; ');
    return { ready: allReady, detail };
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    return { ready: false, detail: `probe failed: ${msg.slice(0, 200)}` };
  }
}

/**
 * Envelope prepended to dev-loop user prompts. Tells the agent the working
 * directory and the resolved Azure context so it doesn't prompt the user.
 */
export function buildEvalEnvelope(workingDir: string, ctx: AzureContext = loadAzureContext()): string {
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
    `- Backing services are NOT pre-provisioned beyond what the harness sets up. If the app crash-loops because it needs a dependency (e.g. a database), deploy a small dev instance of that dependency (a Deployment + Service in namespace \`${ctx.namespace}\`) and wire the app's env/ConfigMap to it, then reapply.\n` +
    `- Two in-cluster databases ARE pre-provisioned in namespace \`${ctx.namespace}\` for any app that needs one (the same way an Azure customer's namespace would already have managed DB endpoints in DNS) — do NOT redeploy them, just wire your app's env/ConfigMap to whichever one matches:\n` +
    `    • Postgres 16 at \`eval-postgres:5432\` — user \`postgres\`, password \`eval\`, db \`eval\` (also exposed via standard env: \`PGHOST=eval-postgres PGPORT=5432 PGUSER=postgres PGPASSWORD=eval PGDATABASE=eval\`; JDBC: \`jdbc:postgresql://eval-postgres:5432/eval\`).\n` +
    `    • MySQL 8 at \`eval-mysql:3306\` — root password \`eval\` and unprivileged user \`eval\`/\`eval\`, db \`eval\` (JDBC: \`jdbc:mysql://eval-mysql:3306/eval\`).\n` +
    `  If your app doesn't need a database, ignore these endpoints — don't add unused env vars.\n` +
    `- Walk every stage end-to-end: analyze → generate-dockerfile → dockerBuild → scan → generate-k8s-manifests → pushImage → kubectlApply → verifyDeploy (with the fix/reapply loop until Ready).\n\n`
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
  return (
    `${BASELINE_PROMPT}\n\n## Reference workflow (CA aks-loop MCP prompt)\n\n${aksLoop}`
  );
}

/** User prompt for `mcp` mode: short kickoff (system prompt has the workflow). */
export function buildMcpAksLoopUserPrompt(workingDir: string, ctx: AzureContext = loadAzureContext()): string {
  return (
    buildEvalEnvelope(workingDir, ctx) +
    `Run the **AKS remote cluster deployment iteration loop** end-to-end on the repository above using the Azure values listed in the envelope. ` +
    `Walk every numbered step of the workflow in order. Finish only after you have produced a Dockerfile, Kubernetes manifests, and attempted the deploy / verify stages.`
  );
}

/** User prompt for `skills` mode. */
export function buildSkillsAksLoopUserPrompt(workingDir: string, ctx: AzureContext = loadAzureContext()): string {
  return (
    buildEvalEnvelope(workingDir, ctx) +
    `Run the deploy-to-aks skill end-to-end on the repository above using the Azure values listed in the envelope. ` +
    `Walk every stage in order and finish only after you have produced a Dockerfile, Kubernetes manifests, and attempted the deploy / verify stages.`
  );
}

/** Make a kubernetes-safe slug from a model spec like 'azure:gpt-4.1' -> 'azure-gpt-4-1'. */
export function slugifyModel(model: string): string {
  return model.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
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
      return resp.error ? { error: resp.error } : resp.content;
    },
  }));
  return {
    tools,
    cleanup: async () => {
      await harness.stopServer(serverName);
    },
  };
}

export async function resolveMode(opts: {
  mode: Mode;
  workingDir: string;
}): Promise<ResolvedModeBundle> {
  switch (opts.mode) {
    case 'baseline':
      return {
        resolved: { systemPrompt: BASELINE_PROMPT, tools: [] },
        cleanup: async () => {},
      };
    case 'skills':
      return {
        resolved: {
          systemPrompt: await loadDeployToAksSkill(),
          userPrompt: buildSkillsAksLoopUserPrompt(opts.workingDir),
          tools: [],
        },
        cleanup: async () => {},
      };
    case 'mcp': {
      const { tools, cleanup } = await createMcpToolBundle(opts.workingDir);
      return {
        resolved: {
          systemPrompt: buildMcpAksLoopSystemPrompt(),
          userPrompt: buildMcpAksLoopUserPrompt(opts.workingDir),
          tools,
        },
        cleanup,
      };
    }
    default:
      throw new Error(
        `Unknown mode '${opts.mode as string}'. Supported: baseline, skills, mcp`,
      );
  }
}
