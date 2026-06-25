import { promises as fs } from 'node:fs';
import { join, dirname, isAbsolute, relative, resolve } from 'node:path';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { tool, Experimental_Agent as Agent } from 'ai';
import type { LanguageModel } from 'ai';
import { z } from 'zod';

const execFileP = promisify(execFile);

export interface ToolSpec {
  name: string;
  description: string;
  inputSchema: z.ZodTypeAny;
  execute: (args: Record<string, unknown>) => Promise<unknown>;
}

export interface AgentRunInput {
  model: LanguageModel;
  systemPrompt: string;
  userPrompt: string;
  workingDir: string;
  tools: ToolSpec[];
  maxSteps?: number;
  providerOptions?: Record<string, Record<string, unknown>>;
  /** Install built-in `dockerBuild` tool. Default true. */
  includeDockerBuild?: boolean;
  /** Hard cap on `dockerBuild` calls. Undefined = no cap. */
  dockerBuildMaxRetries?: number;
  /**
   * Max retries for upstream HTTP 429 / rate-limit errors raised by the model
   * provider. Honors `Retry-After` when present, otherwise exponential backoff.
   * Default 3.
   */
  maxRateLimitRetries?: number;
}

export interface AgentRunResult {
  ok: boolean;
  artifactDir: string;
  tokensIn: number;
  tokensOut: number;
  toolCalls: Array<{ name: string; argsSummary: string }>;
  durationMs: number;
  text: string;
}

export interface AgentDriver {
  run(input: AgentRunInput): Promise<AgentRunResult>;
}

function summarize(args: unknown): string {
  const s = JSON.stringify(args ?? {});
  return s.length > 80 ? s.slice(0, 77) + '...' : s;
}

/**
 * Registry-side existence check for a fully-qualified image reference.
 * Returns `exists: true|false` when we can determine it, or `null` when the
 * check itself could not run (so callers can warn rather than hard-fail).
 * ACR (`*.azurecr.io`) refs are checked with `az acr repository show-tags`;
 * anything else (or an inconclusive az call) falls back to `docker manifest inspect`.
 */
async function imageExistsInRegistry(ref: string): Promise<{ exists: boolean | null; detail: string }> {
  const lastColon = ref.lastIndexOf(':');
  const lastSlash = ref.lastIndexOf('/');
  const hasTag = lastColon > lastSlash;
  const tag = hasTag ? ref.slice(lastColon + 1) : 'latest';
  const repoWithHost = hasTag ? ref.slice(0, lastColon) : ref;
  const firstSlash = repoWithHost.indexOf('/');
  const host = firstSlash > 0 ? repoWithHost.slice(0, firstSlash) : '';
  const repository = firstSlash > 0 ? repoWithHost.slice(firstSlash + 1) : repoWithHost;

  if (host.endsWith('.azurecr.io')) {
    const acrName = host.slice(0, -'.azurecr.io'.length);
    try {
      const { stdout } = await execFileP(
        'az',
        ['acr', 'repository', 'show-tags', '--name', acrName, '--repository', repository, '--output', 'json'],
        { timeout: 30_000 },
      );
      const tags = JSON.parse(stdout) as string[];
      return tags.includes(tag)
        ? { exists: true, detail: `az acr: ${acrName}/${repository}:${tag} present` }
        : {
            exists: false,
            detail: `az acr: tag '${tag}' not in ${acrName}/${repository} (have: ${tags.slice(0, 8).join(', ') || 'none'})`,
          };
    } catch (err) {
      const msg = (err as { stderr?: string; message?: string }).stderr ?? (err as Error).message ?? '';
      if (/not found|does not exist|was not found|ResourceNotFound/i.test(msg)) {
        return { exists: false, detail: `az acr: repository '${repository}' not found in ${acrName}` };
      }
      // Inconclusive (az missing, not logged in, network) — fall through to docker.
    }
  }

  try {
    await execFileP('docker', ['manifest', 'inspect', ref], { timeout: 30_000 });
    return { exists: true, detail: `docker manifest inspect: ${ref} present` };
  } catch (err) {
    const msg = (err as { stderr?: string; message?: string }).stderr ?? (err as Error).message ?? '';
    if (/no such manifest|not found|manifest unknown/i.test(msg)) {
      return { exists: false, detail: `docker manifest inspect: ${ref} not found` };
    }
    return { exists: null, detail: `could not verify ${ref}: ${msg.slice(0, 160)}` };
  }
}

export class AISDKDriver implements AgentDriver {
  async run(input: AgentRunInput): Promise<AgentRunResult> {
    // 64 covers ~18-step aks-loop + tool calls + retries.
    const maxSteps = input.maxSteps ?? 64;
    const toolCalls: Array<{ name: string; argsSummary: string }> = [];
    let tokensIn = 0;
    let tokensOut = 0;
    let createFileCalled = false;

    const resolveInWorkingDir = (p: string): string => {
      const full = isAbsolute(p) ? p : join(input.workingDir, p);
      const normalized = resolve(full);
      const root = resolve(input.workingDir);
      const rel = relative(root, normalized);
      if (rel.startsWith('..') || isAbsolute(rel)) {
        throw new Error(`Path '${p}' escapes working directory`);
      }
      return normalized;
    };

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const sdkTools: Record<string, ReturnType<typeof tool<any, any>>> = {
      createFile: tool({
        description:
          'Create a file with the specified content. Use this to actually create Dockerfiles, configuration files, etc. based on information from other tools.',
        inputSchema: z.object({
          filePath: z.string().describe('Path where the file should be created (relative to working directory)'),
          content: z.string().describe('Content to write to the file'),
          reason: z.string().describe('Why this file is being created and what it accomplishes'),
        }),
        execute: async ({ filePath, content, reason }) => {
          const fullPath = resolveInWorkingDir(filePath);
          await fs.mkdir(dirname(fullPath), { recursive: true });
          await fs.writeFile(fullPath, content, 'utf8');
          return { success: true, filePath: fullPath, message: `Created ${filePath}. ${reason}` };
        },
      }),
      readFile: tool({
        description:
          'Read the contents of a file in the working directory. Use this to inspect source files, manifests (pom.xml, package.json, etc.), and other project files before generating containerization artifacts.',
        inputSchema: z.object({
          filePath: z.string().describe('Path to the file to read (relative to working directory)'),
        }),
        execute: async ({ filePath }) => {
          const fullPath = resolveInWorkingDir(filePath);
          const content = await fs.readFile(fullPath, 'utf8');
          return { filePath, content };
        },
      }),
      listDir: tool({
        description:
          'List the contents of a directory in the working directory (non-recursive). Use this to discover the project structure before reading files.',
        inputSchema: z.object({
          dirPath: z.string().describe('Path to the directory to list (relative to working directory; use "." for the working directory itself)'),
        }),
        execute: async ({ dirPath }) => {
          const fullPath = resolveInWorkingDir(dirPath);
          const entries = await fs.readdir(fullPath, { withFileTypes: true });
          return {
            dirPath,
            entries: entries.map((e) => ({ name: e.name, type: e.isDirectory() ? 'dir' : 'file' })),
          };
        },
      }),
    };

    if (input.includeDockerBuild !== false) {
      const maxRetries = input.dockerBuildMaxRetries;
      let dockerBuildCalls = 0;
      sdkTools.dockerBuild = tool({
        description:
          'Build the Docker image from the current Dockerfile in the working directory using `docker buildx build --platform linux/amd64`. ' +
          'The image is built for linux/amd64 because the target AKS nodes are amd64 — building for the host arch (e.g. arm64) causes `no match for platform` ImagePullBackOff. ' +
          'Returns the build exit status, the last ~6000 chars of combined stdout/stderr, and the tagged image name on success. ' +
          'Call this AFTER you have written the Dockerfile to disk to verify it actually builds. ' +
          'If the build fails, READ the error output, FIX the Dockerfile (use createFile to overwrite, or the fix-dockerfile MCP tool if available), then call dockerBuild again. ' +
          (maxRetries != null
            ? `You may retry up to ${maxRetries} times total. After ${maxRetries} attempts the tool refuses further calls. `
            : 'You may retry up to 3 times. ') +
          'Do not call any deploy/push/scan steps until dockerBuild succeeds.',
        inputSchema: z.object({
          tag: z
            .string()
            .optional()
            .describe('Optional image tag (default: agent-eval-build:check)'),
        }),
        execute: async ({ tag }) => {
          dockerBuildCalls += 1;
          if (maxRetries != null && dockerBuildCalls > maxRetries) {
            return {
              success: false,
              budgetExhausted: true,
              attempts: dockerBuildCalls - 1,
              maxRetries,
              message:
                `dockerBuild retry budget exhausted (${maxRetries} attempts used). ` +
                'Do not call dockerBuild again. Finish the workflow with whatever Dockerfile is currently on disk.',
            };
          }
          const imageTag = (tag as string | undefined) ?? `agent-eval-build-${Date.now()}:check`;
          try {
            // Build for linux/amd64 (the AKS node arch). On an arm64 host a plain
            // `docker build` produces an arm64 image that the amd64 nodes can't
            // run, surfacing as `no match for platform` ImagePullBackOff. buildx
            // with --load puts the single-arch image in the local store so
            // pushImage can tag+push it.
            const { stdout, stderr } = await execFileP(
              'docker',
              ['buildx', 'build', '--platform', 'linux/amd64', '--load', '-t', imageTag, input.workingDir],
              {
                maxBuffer: 16 * 1024 * 1024,
                // Hard wall-clock cap. buildkit can wedge indefinitely when a
                // referenced base image errors mid-stream (e.g. registry 404 +
                // missing cgroup) — without this the agent loop blocks forever
                // and stalls the whole gradient. SIGTERM lets the agent see a
                // timeout error and iterate on the Dockerfile instead.
                timeout: 10 * 60 * 1000,
                killSignal: 'SIGTERM',
              },
            );
            return {
              success: true,
              imageTag,
              attempt: dockerBuildCalls,
              output: ((stdout ?? '') + (stderr ?? '')).slice(-6000),
            };
          } catch (err) {
            const e = err as { code?: string | number; stderr?: string; stdout?: string; message?: string };
            const combined = ((e.stdout ?? '') + (e.stderr ?? '') + (e.message ?? '')).slice(-6000);
            const remaining =
              maxRetries != null ? Math.max(0, maxRetries - dockerBuildCalls) : null;
            return {
              success: false,
              imageTag,
              exitCode: typeof e.code === 'number' ? e.code : null,
              attempt: dockerBuildCalls,
              ...(remaining != null ? { remainingRetries: remaining } : {}),
              output: combined,
              hint:
                remaining != null && remaining === 0
                  ? 'Last attempt failed and no retries remain. Do not call dockerBuild again.'
                  : 'Read the error above, fix the Dockerfile, then call dockerBuild again.',
            };
          }
        },
      });
    }

    sdkTools.pushImage = tool({
      description:
        'Tag a locally-built image to a registry reference and push it, then verify the tag exists in the registry. ' +
        'Call this AFTER dockerBuild succeeds and BEFORE writing/applying Kubernetes manifests that reference the image. ' +
        '`source` is the local image produced by dockerBuild; `target` is the full registry reference the manifests will use, e.g. `myregistry.azurecr.io/app:latest`. ' +
        'Returns success only once the image is confirmed present in the registry; on failure, read the error and retry.',
      inputSchema: z.object({
        source: z
          .string()
          .describe('Local image reference to push (e.g. the imageTag returned by dockerBuild).'),
        target: z
          .string()
          .describe('Destination registry reference `<registry>/<repo>:<tag>` (e.g. `myregistry.azurecr.io/app:latest`).'),
      }),
      execute: async ({ source, target }) => {
        const src = String(source);
        const dst = String(target);
        if (src !== dst) {
          try {
            await execFileP('docker', ['tag', src, dst], { timeout: 60_000 });
          } catch (err) {
            const e = err as { stderr?: string; stdout?: string; message?: string };
            return {
              success: false,
              step: 'tag',
              command: `docker tag ${src} ${dst}`,
              output: ((e.stdout ?? '') + (e.stderr ?? '') + (e.message ?? '')).slice(-4000),
              hint: `Could not tag '${src}'. Use the exact imageTag that dockerBuild returned as 'source'.`,
            };
          }
        }
        try {
          const { stdout, stderr } = await execFileP('docker', ['push', dst], {
            maxBuffer: 16 * 1024 * 1024,
            timeout: 300_000,
          });
          const pushOut = ((stdout ?? '') + (stderr ?? '')).slice(-4000);
          const check = await imageExistsInRegistry(dst);
          if (check.exists === false) {
            return {
              success: false,
              step: 'verify',
              pushedRef: dst,
              output: pushOut,
              verification: check.detail,
              hint:
                `docker push reported success but the registry does not show ${dst}. ` +
                'Check the registry/repo/tag and that you are authenticated (az acr login).',
            };
          }
          return {
            success: true,
            pushedRef: dst,
            verified: check.exists === true,
            verification: check.detail,
            output: pushOut,
          };
        } catch (err) {
          const e = err as { code?: string | number; stderr?: string; stdout?: string; message?: string };
          return {
            success: false,
            step: 'push',
            command: `docker push ${dst}`,
            exitCode: typeof e.code === 'number' ? e.code : null,
            output: ((e.stdout ?? '') + (e.stderr ?? '') + (e.message ?? '')).slice(-4000),
            hint:
              'Push failed. If this is an auth error, registry login may have expired (az acr login). ' +
              'Otherwise check the target reference.',
          };
        }
      },
    });

    sdkTools.kubectlApply = tool({
      description:
        'Apply Kubernetes manifests to the current kube-context cluster via `kubectl apply -f <path>`. ' +
        '`path` is resolved relative to the working directory. Call this AFTER you have written manifest files (typically under `./k8s/` or `./manifests/`). ' +
        'IMPORTANT: a successful apply only CREATES the Kubernetes objects — it does NOT mean the pods are healthy. ' +
        'Always call `verifyDeploy` next to confirm the workload actually reaches Ready, and iterate on whatever failure it reports. ' +
        'Returns the kubectl exit code plus the last ~4000 chars of combined stdout/stderr.',
      inputSchema: z.object({
        path: z
          .string()
          .describe('File or directory of manifests, relative to the working directory (e.g. "k8s" or "k8s/deployment.yaml").'),
        namespace: z
          .string()
          .optional()
          .describe('Kubernetes namespace to apply into. Defaults to the namespace already set on the kube-context.'),
      }),
      execute: async ({ path: relPath, namespace }) => {
        const fullPath = join(input.workingDir, String(relPath));
        // No pre-apply image check: a real `kubectl apply` never pre-verifies
        // the registry. If an image isn't there yet, the pod will surface
        // ImagePullBackOff via verifyDeploy and the agent must recover via the
        // honest loop (rebuild / repush / fix image: ref / reapply).
        const args = ['apply', '-f', fullPath];
        if (namespace) args.push('-n', String(namespace));
        try {
          const { stdout, stderr } = await execFileP('kubectl', args, {
            maxBuffer: 8 * 1024 * 1024,
          });
          return {
            success: true,
            command: `kubectl ${args.join(' ')}`,
            output: ((stdout ?? '') + (stderr ?? '')).slice(-4000),
            note:
              'Objects applied. This does NOT confirm the pods are healthy. Call verifyDeploy now to check the ' +
              'rollout actually reaches Ready, and fix+reapply if it reports a failure.',
          };
        } catch (err) {
          const e = err as { code?: string | number; stderr?: string; stdout?: string; message?: string };
          return {
            success: false,
            command: `kubectl ${args.join(' ')}`,
            exitCode: typeof e.code === 'number' ? e.code : null,
            output: ((e.stdout ?? '') + (e.stderr ?? '') + (e.message ?? '')).slice(-4000),
            hint: 'Inspect the error, fix the manifest with createFile, then call kubectlApply again.',
          };
        }
      },
    });

    sdkTools.verifyDeploy = tool({
      description:
        'Verify that an applied workload actually became healthy on the cluster — the deploy is only a success once the pods are Running and Ready. ' +
        'Polls the Deployment(s) in the namespace and, when anything is unhealthy, returns the real pod-level failure signal: each container\'s waiting reason + message ' +
        '(e.g. CreateContainerConfigError, ImagePullBackOff, CrashLoopBackOff), the last terminated state, recent cluster events, and the last container log lines — ' +
        'exactly what `kubectl describe pod` and `kubectl logs` would show you. ' +
        'Call this AFTER kubectlApply. If it returns success:false, READ the reason, FIX the underlying cause (edit the manifest with createFile, or rebuild/repush the image, or deploy a missing dependency), reapply with kubectlApply, then call verifyDeploy again. Iterate until success:true (try up to 3 rounds).',
      inputSchema: z.object({
        namespace: z
          .string()
          .optional()
          .describe('Namespace to check. Defaults to the kube-context namespace. Pass the same namespace you applied into.'),
        deploymentName: z
          .string()
          .optional()
          .describe('Specific Deployment to verify. If omitted, every Deployment in the namespace is checked.'),
        timeoutSeconds: z
          .number()
          .optional()
          .describe('Max seconds to wait for readiness before reporting failure (default 90).'),
      }),
      execute: async ({ namespace, deploymentName, timeoutSeconds }) => {
        const ns = namespace ? ['-n', String(namespace)] : [];
        const timeout = typeof timeoutSeconds === 'number' && timeoutSeconds > 0 ? timeoutSeconds : 90;
        const FATAL =
          /CreateContainerConfigError|CrashLoopBackOff|ImagePullBackOff|ErrImagePull|InvalidImageName|RunContainerError|CreateContainerError/;
        const kget = async (a: string[]): Promise<string> => {
          try {
            const { stdout } = await execFileP('kubectl', [...a, ...ns], { maxBuffer: 8 * 1024 * 1024 });
            return stdout ?? '';
          } catch (err) {
            const e = err as { stdout?: string; stderr?: string; message?: string };
            return (e.stdout ?? '') + (e.stderr ?? '') + (e.message ?? '');
          }
        };

        let deployNames: string[] = [];
        if (deploymentName) {
          deployNames = [String(deploymentName)];
        } else {
          const out = await kget(['get', 'deploy', '-o', 'jsonpath={.items[*].metadata.name}']);
          deployNames = out.split(/\s+/).filter(Boolean);
        }
        if (deployNames.length === 0) {
          return {
            success: false,
            reason: 'no-deployment',
            hint:
              'No Deployment found in this namespace. Make sure you ran kubectlApply with the manifests directory and the correct namespace before verifyDeploy.',
          };
        }

        interface PodView {
          name?: string;
          phase?: string;
          ready: boolean;
          restarts: number;
          waitingReasons: string[];
          waitingMessages: string[];
          lastTerminated: Array<{ reason?: string; exitCode?: number }>;
        }
        const deadline = Date.now() + timeout * 1000;
        let lastPods: PodView[] = [];
        while (Date.now() < deadline) {
          let items: Array<Record<string, unknown>> = [];
          try {
            const raw = await kget(['get', 'pods', '-o', 'json']);
            items = (JSON.parse(raw).items ?? []) as Array<Record<string, unknown>>;
          } catch {
            items = [];
          }
          const pods: PodView[] = items.map((p) => {
            const status = (p.status ?? {}) as Record<string, unknown>;
            const cs = (status.containerStatuses ?? []) as Array<Record<string, unknown>>;
            const waitingReasons: string[] = [];
            const waitingMessages: string[] = [];
            const lastTerminated: Array<{ reason?: string; exitCode?: number }> = [];
            let restarts = 0;
            let allReady = cs.length > 0;
            for (const c of cs) {
              const state = (c.state ?? {}) as Record<string, Record<string, unknown>>;
              const waiting = state.waiting;
              if (waiting?.reason) waitingReasons.push(String(waiting.reason));
              if (waiting?.message) waitingMessages.push(String(waiting.message));
              const last = (c.lastState ?? {}) as Record<string, Record<string, unknown>>;
              if (last.terminated) {
                lastTerminated.push({
                  reason: last.terminated.reason ? String(last.terminated.reason) : undefined,
                  exitCode:
                    typeof last.terminated.exitCode === 'number' ? (last.terminated.exitCode as number) : undefined,
                });
              }
              restarts += typeof c.restartCount === 'number' ? (c.restartCount as number) : 0;
              if (!c.ready) allReady = false;
            }
            return {
              name: (p.metadata as Record<string, unknown> | undefined)?.name as string | undefined,
              phase: status.phase as string | undefined,
              ready: allReady,
              restarts,
              waitingReasons,
              waitingMessages,
              lastTerminated,
            };
          });
          lastPods = pods;
          const allReady = pods.length > 0 && pods.every((p) => p.ready && p.phase === 'Running');
          if (allReady) {
            return {
              success: true,
              deployments: deployNames,
              pods: pods.map((p) => ({ name: p.name, phase: p.phase, restarts: p.restarts })),
              message: `All ${pods.length} pod(s) are Running and Ready.`,
            };
          }
          const fatal = pods.some(
            (p) =>
              p.waitingReasons.some((r) => FATAL.test(r)) ||
              p.lastTerminated.some((t) => FATAL.test(t.reason ?? '')) ||
              p.restarts >= 3,
          );
          if (fatal) break;
          await new Promise((r) => setTimeout(r, 4000));
        }

        const events = await kget(['get', 'events', '--sort-by=.lastTimestamp']);
        const eventTail = events.split('\n').slice(-25).join('\n');
        const logs: Record<string, string> = {};
        for (const p of lastPods.filter((x) => !x.ready && x.name).slice(0, 3)) {
          let log = await kget(['logs', String(p.name), '--previous', '--tail=40']);
          if (!log || /error from server|not found|previous terminated container/i.test(log)) {
            log = await kget(['logs', String(p.name), '--tail=40']);
          }
          logs[String(p.name)] = (log ?? '').slice(-3000);
        }
        return {
          success: false,
          deployments: deployNames,
          pods: lastPods,
          recentEvents: eventTail,
          logs,
          hint:
            "The workload did not become Ready. Read each pod's waitingReasons / waitingMessages / lastTerminated and the logs above, then fix and reapply:\n" +
            '• CreateContainerConfigError "runAsNonRoot ... non-numeric user" → the manifest sets runAsNonRoot:true but the image USER is a name; add a numeric securityContext.runAsUser (e.g. 10001) at pod AND container level, then reapply.\n' +
            '• ImagePullBackOff / ErrImagePull → the image is missing from the registry or the manifest image: tag does not match; rebuild with dockerBuild, repush with pushImage, fix the image: field, reapply.\n' +
            '• CrashLoopBackOff → read the logs: the app usually needs a dependency (e.g. a database) or config. Deploy the dependency (a dev Deployment+Service in this namespace) and/or set the right env vars, then reapply.\n' +
            'After fixing, call kubectlApply again, then verifyDeploy again. Iterate up to 3 rounds.',
        };
      },
    });

    for (const t of input.tools) {
      sdkTools[t.name] = tool({
        description: t.description,
        inputSchema: t.inputSchema,
        execute: async (args: Record<string, unknown>) => t.execute(args),
      });
    }

    const agent = new Agent({
      model: input.model,
      system: input.systemPrompt,
      tools: sdkTools,
      stopWhen: ({ steps }) => steps.length >= maxSteps,
      onStepFinish: ({ toolCalls: stepCalls, usage }) => {
        if (usage) {
          tokensIn += usage.inputTokens ?? 0;
          tokensOut += usage.outputTokens ?? 0;
        }
        for (const tc of stepCalls ?? []) {
          toolCalls.push({ name: tc.toolName, argsSummary: summarize((tc as { input?: unknown }).input) });
          if (tc.toolName === 'createFile') createFileCalled = true;
        }
      },
    });

    const start = Date.now();
    // Wrap the agent run with a transparent 429 / rate-limit backoff so a brief
    // Azure Foundry throttle doesn't lose a whole eval cell. We honor the
    // upstream `Retry-After` header when present (the SDK surfaces it on the
    // error's `responseHeaders` / `data` payload); otherwise we fall back to a
    // bounded exponential backoff (30s, 60s, 120s). Total of `maxRateLimitRetries`
    // attempts. Non-429 errors propagate immediately.
    const maxRetries = input.maxRateLimitRetries ?? 3;
    let result: Awaited<ReturnType<typeof agent.generate>>;
    let attempt = 0;
    while (true) {
      try {
        result = await agent.generate({
          prompt: input.userPrompt,
          ...(input.providerOptions ? { providerOptions: input.providerOptions } : {}),
        });
        break;
      } catch (err) {
        const info = classifyRateLimit(err);
        if (!info.isRateLimit || attempt >= maxRetries) throw err;
        const waitMs = info.retryAfterMs ?? Math.min(120_000, 30_000 * 2 ** attempt);
        attempt += 1;
        console.error(
          `[driver] rate-limited (attempt ${attempt}/${maxRetries}); ` +
            `sleeping ${Math.round(waitMs / 1000)}s before retry. ${info.detail}`,
        );
        await new Promise((r) => setTimeout(r, waitMs));
      }
    }
    const durationMs = Date.now() - start;

    return {
      ok: createFileCalled,
      artifactDir: input.workingDir,
      tokensIn,
      tokensOut,
      toolCalls,
      durationMs,
      text: result.text ?? '',
    };
  }
}

/**
 * Detect HTTP 429 / rate-limit responses surfaced by the AI SDK and extract
 * a `Retry-After` hint when the provider sent one. The SDK doesn't normalize
 * provider errors, so we look for the status code in any of the conventional
 * shapes (`statusCode`, `status`, error-message text) and check both
 * `responseHeaders` and any JSON `error.retryAfter` field.
 */
function classifyRateLimit(err: unknown): {
  isRateLimit: boolean;
  retryAfterMs?: number;
  detail: string;
} {
  const e = err as {
    statusCode?: number;
    status?: number;
    message?: string;
    responseHeaders?: Record<string, string> | undefined;
    headers?: Record<string, string> | undefined;
    data?: { error?: { retryAfter?: number; retry_after?: number } } | undefined;
  };
  const msg = e?.message ?? '';
  const status = e?.statusCode ?? e?.status;
  const isRateLimit =
    status === 429 ||
    /\b429\b|too many requests|rate.?limit/i.test(msg);
  if (!isRateLimit) return { isRateLimit: false, detail: '' };

  const headers = e?.responseHeaders ?? e?.headers ?? undefined;
  let retryAfterMs: number | undefined;
  const headerVal = headers?.['retry-after'] ?? headers?.['Retry-After'];
  if (headerVal) {
    const asNum = Number(headerVal);
    if (Number.isFinite(asNum)) {
      retryAfterMs = Math.max(1_000, asNum * 1000);
    } else {
      const dateMs = Date.parse(String(headerVal));
      if (Number.isFinite(dateMs)) {
        retryAfterMs = Math.max(1_000, dateMs - Date.now());
      }
    }
  }
  const fromBody = e?.data?.error?.retryAfter ?? e?.data?.error?.retry_after;
  if (retryAfterMs == null && typeof fromBody === 'number') {
    retryAfterMs = Math.max(1_000, fromBody * 1000);
  }
  return {
    isRateLimit: true,
    ...(retryAfterMs != null ? { retryAfterMs } : {}),
    detail: msg.slice(0, 200),
  };
}
