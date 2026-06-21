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
          'Build the Docker image from the current Dockerfile in the working directory using `docker build`. ' +
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
            const { stdout, stderr } = await execFileP(
              'docker',
              ['build', '-t', imageTag, input.workingDir],
              { maxBuffer: 16 * 1024 * 1024 },
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

    sdkTools.kubectlApply = tool({
      description:
        'Apply Kubernetes manifests to the current kube-context cluster via `kubectl apply -f <path>`. ' +
        '`path` is resolved relative to the working directory. Call this AFTER you have written manifest files (typically under `./k8s/` or `./manifests/`) and BEFORE calling verify-deploy. ' +
        'Without this call, verify-deploy will time out because no Deployment exists in the cluster yet. ' +
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
    const result = await agent.generate({
      prompt: input.userPrompt,
      ...(input.providerOptions ? { providerOptions: input.providerOptions } : {}),
    });
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
