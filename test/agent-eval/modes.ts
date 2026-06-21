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
    resourceGroup: process.env.AGENT_EVAL_RESOURCE_GROUP ?? 'ca-eval-rg',
    clusterName: process.env.AGENT_EVAL_CLUSTER ?? 'ca-eval-aks',
    namespace: process.env.AGENT_EVAL_NAMESPACE ?? 'eval-ns',
    imageName: process.env.AGENT_EVAL_IMAGE ?? 'eval-image',
  };
}

/**
 * Best-effort `kubectl delete deployment,service <imageName> -n <ns>` so runs
 * don't inherit a stuck pod from the previous attempt.
 */
export async function cleanupAzureResources(ctx: AzureContext = loadAzureContext()): Promise<void> {
  try {
    await execFileP(
      'kubectl',
      [
        'delete',
        'deployment,service',
        ctx.imageName,
        '-n',
        ctx.namespace,
        '--ignore-not-found',
        '--wait=false',
      ],
      { timeout: 30_000 },
    );
  } catch {
    // best-effort; ignore (no kubectl, no cluster, etc.)
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
    `- After you have written Kubernetes manifests to disk, you MUST call \`kubectlApply\` with the manifests directory (e.g. \`{ "path": "k8s", "namespace": "${ctx.namespace}" }\`) BEFORE calling \`verify-deploy\`. The CA MCP surface has no apply tool; without \`kubectlApply\` the Deployment never reaches the cluster and verify-deploy will time out.\n` +
    `- Walk every stage of the loop end-to-end: analyze → generate-dockerfile → build → scan → generate-k8s-manifests → push → kubectlApply → verify-deploy.\n\n`
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
