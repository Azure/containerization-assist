/**
 * Gradient runner — same (fixture, model) under 3 independent CA paths:
 *   bare    baseline prompt only
 *   mcp     baseline prompt + CA aks-loop MCP prompt + CA MCP tools
 *   skills  CA deploy-to-aks SKILL bundle + skill user prompt
 *
 * All paths share fs tools and dockerBuild. Output: per-fixture raw scores
 * plus Δ vs `bare`.
 */

import { promises as fs } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { getModel } from './providers.js';
import { AISDKDriver, type ToolSpec } from './driver.js';
import {
  BASELINE_PROMPT,
  USER_PROMPT,
  loadDeployToAksSkill,
  buildSkillsAksLoopUserPrompt,
  buildMcpAksLoopSystemPrompt,
  buildMcpAksLoopUserPrompt,
  createMcpToolBundle,
  cleanupAzureResources,
  ensureRegistryLogin,
  ensureEvalCluster,
  ensureNamespace,
  loadAzureContext,
  slugifyModel,
  type AzureContext,
} from './modes.js';
import { runChecks, selectChecks, type CheckResult } from './checks.js';

/** Discover fixtures: each immediate subdirectory of `dir` is one fixture. */
export async function discoverFixtures(dir: string): Promise<string[]> {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  return entries
    .filter((e) => e.isDirectory())
    .map((e) => join(dir, e.name))
    .sort();
}

// ---------- Path definitions ----------

export type LevelId = 'bare' | 'mcp' | 'skills';

export interface LevelConfig {
  id: LevelId;
  label: string;
  description: string;
}

export const LEVELS: readonly LevelConfig[] = [
  {
    id: 'bare',
    label: 'bare',
    description: 'Baseline prompt only. No CA skill, no CA MCP. The control.',
  },
  {
    id: 'mcp',
    label: 'CA MCP',
    description: 'BASELINE + CA `aks-loop` procedural prompt in system + CA MCP tools.',
  },
  {
    id: 'skills',
    label: 'CA skills',
    description:
      'BASELINE + CA `deploy-to-aks` SKILL bundle in system + CA MCP tools (real-world delivery).',
  },
];

/** Resolved prompts + tools for one path. */
interface ResolvedLevel {
  systemPrompt: string;
  userPrompt: string;
  tools: ToolSpec[];
  cleanup: () => Promise<void>;
}

/** Build runtime config. `mcp` spins up MCP harness — caller must await cleanup. */
async function resolveLevel(
  level: LevelId,
  workingDir: string,
  ctx: AzureContext,
): Promise<ResolvedLevel> {
  switch (level) {
    case 'bare':
      return {
        systemPrompt: BASELINE_PROMPT,
        userPrompt: USER_PROMPT(workingDir),
        tools: [],
        cleanup: async () => {},
      };
    case 'mcp': {
      const { tools, cleanup } = await createMcpToolBundle(workingDir);
      return {
        systemPrompt: buildMcpAksLoopSystemPrompt(ctx),
        userPrompt: buildMcpAksLoopUserPrompt(workingDir, ctx),
        tools,
        cleanup,
      };
    }
    case 'skills': {
      const systemPrompt = await loadDeployToAksSkill();
      const { tools, cleanup } = await createMcpToolBundle(workingDir);
      return {
        systemPrompt,
        userPrompt: buildSkillsAksLoopUserPrompt(workingDir, ctx),
        tools,
        cleanup,
      };
    }
    default:
      throw new Error(`Unknown path: ${level as string}`);
  }
}

// ---------- Runner ----------

export interface GradientRunRecord {
  fixture: string;
  model: string;
  level: LevelId;
  label: string;
  /** 0-indexed rep for this (model, fixture, level) cell. */
  rep?: number;
  tokensIn?: number;
  tokensOut?: number;
  toolCallCount?: number;
  toolCallsByName?: Record<string, number>;
  durationMs?: number;
  /**
   * True once the harness's verifyDeploy confirmed the workload reached
   * Running/Ready on the cluster. Informational only — NOT part of the scored
   * checks. Absent for the `bare` control (no deploy is attempted there).
   */
  deployVerified?: boolean;
  checks: CheckResult[];
  /** Final assistant text. Useful for debugging "agent bailed early" runs. */
  finalText?: string;
  error?: string;
}

export interface GradientResult {
  models: string[];
  levels: readonly LevelConfig[];
  timestamp: string;
  runs: GradientRunRecord[];
}

export interface GradientOptions {
  fixtures: string[];
  models: string[];
  checks: string;
  /** Subset of paths to run. Defaults to all. */
  levels?: LevelId[];
  /** Run models concurrently. Defaults to true when >1 model. */
  parallelModels?: boolean;
  /**
   * Cap on how many model lanes run concurrently when `parallelModels` is on.
   * Undefined = unlimited (one lane per model, classic behavior). Useful to
   * stay under provider rate limits during a wide sweep.
   */
  maxConcurrentModels?: number;
  /**
   * Number of repetitions per (model, fixture, path) cell. Reps run
   * sequentially within a model so cleanup stays correct. Default 1.
   */
  reps?: number;
  /**
   * If set, the full {@link GradientResult} is rewritten to this path after
   * every completed cell. Lets a long sweep be interrupted (or sharded) while
   * still leaving a usable, populated report on disk.
   */
  checkpointPath?: string;
  /**
   * Pre-populated runs from a previous checkpoint. Cells that already have a
   * successful (no `error`) record for the same `(model, fixture, level, rep)`
   * key are skipped, so an interrupted sweep can be resumed without re-running
   * already-completed work.
   */
  resumeRuns?: GradientRunRecord[];
}

/**
 * Bounded-concurrency worker pool: process `items` with at most `limit`
 * `worker` calls in flight at once. Used to cap how many model lanes hit the
 * provider in parallel so a wide sweep doesn't trip rate limits. Caller is
 * responsible for flattening if `R` is itself an array.
 */
async function runWithConcurrency<T, R>(
  items: T[],
  limit: number,
  worker: (item: T) => Promise<R>,
): Promise<R[]> {
  const results: R[] = new Array(items.length);
  let next = 0;
  const launch = async (): Promise<void> => {
    while (true) {
      const i = next++;
      if (i >= items.length) return;
      results[i] = await worker(items[i] as T);
    }
  };
  const lanes = Math.max(1, Math.min(limit, items.length));
  await Promise.all(Array.from({ length: lanes }, () => launch()));
  return results;
}

async function runOneLevel(opts: {
  fixture: string;
  level: LevelConfig;
  model: string;
  ctx: AzureContext;
  checkSpecs: ReturnType<typeof selectChecks>;
  rep: number;
}): Promise<GradientRunRecord> {
  const record: GradientRunRecord = {
    fixture: opts.fixture,
    model: opts.model,
    level: opts.level.id,
    label: opts.level.label,
    rep: opts.rep,
    checks: [],
  };
  // Wipe any leftover deployment so verify-deploy doesn't read stale state.
  await cleanupAzureResources(opts.ctx);
  // Refresh ACR credentials per cell — tokens expire (~3h) mid-sweep otherwise.
  await ensureRegistryLogin(opts.ctx);
  const workingDir = await fs.mkdtemp(join(tmpdir(), 'agent-eval-grad-'));
  let resolved: ResolvedLevel | undefined;
  try {
    await fs.cp(opts.fixture, workingDir, { recursive: true });
    resolved = await resolveLevel(opts.level.id, workingDir, opts.ctx);
    const { model, providerOptions } = getModel(opts.model);
    const result = await new AISDKDriver().run({
      model,
      providerOptions,
      systemPrompt: resolved.systemPrompt,
      userPrompt: resolved.userPrompt,
      workingDir,
      tools: resolved.tools,
      // bare is the no-deploy control; mcp/skills are expected to deploy, so let
      // the harness nudge them through push→apply→verify if they stall early.
      requireDeploy: opts.level.id !== 'bare',
    });
    record.tokensIn = result.tokensIn;
    record.tokensOut = result.tokensOut;
    record.toolCallCount = result.toolCalls.length;
    record.toolCallsByName = result.toolCalls.reduce<Record<string, number>>((acc, c) => {
      acc[c.name] = (acc[c.name] ?? 0) + 1;
      return acc;
    }, {});
    record.durationMs = result.durationMs;
    record.finalText = result.text;
    if (opts.level.id !== 'bare') record.deployVerified = result.deployVerified;
    record.checks = await runChecks(opts.checkSpecs, { artifactDir: workingDir });
  } catch (err) {
    record.error = err instanceof Error ? err.message : String(err);
  } finally {
    if (resolved) {
      try {
        await resolved.cleanup();
      } catch {
        // best-effort
      }
    }
    // Tear down the deployment we just created so the next run starts clean.
    await cleanupAzureResources(opts.ctx);
  }
  return record;
}

/**
 * Run every selected path for every (fixture, model). Models run concurrently
 * when there is more than one (each lane gets its own slugged `imageName` so
 * cleanup is isolated); pass `parallelModels: false` to force sequential.
 * Inside one model, fixtures and paths are always sequential.
 */
export async function runGradient(opts: GradientOptions): Promise<GradientResult> {
  const checkSpecs = selectChecks(opts.checks);
  const selectedIds = new Set<LevelId>(opts.levels ?? LEVELS.map((l) => l.id));
  const levels = LEVELS.filter((l) => selectedIds.has(l.id));
  const baseCtx = loadAzureContext();
  const parallel = opts.parallelModels ?? opts.models.length > 1;
  const reps = Math.max(1, Math.floor(opts.reps ?? 1));
  const t0 = Date.now();

  // Make the cluster disposable: reuse a healthy cluster or create the next-
  // indexed one if none is, wire ACR pull, refresh kubeconfig, and ensure the
  // namespace — so a sweep never blocks on a cluster that's mid-deletion.
  await ensureEvalCluster(baseCtx);

  // Refresh the ACR credential once up front. Without this a stale token makes
  // every pushImage fail (and the kubectlApply gate then blocks every deploy).
  await ensureRegistryLogin(baseCtx);

  // Shared collector + serialized checkpoint writer. Model lanes run in
  // parallel, so every completed cell pushes here and rewrites the full result
  // to disk. Writes are chained so concurrent lanes can't corrupt the file.
  // Resume: seed with any prior successful records so they're preserved in the
  // checkpoint and the cells aren't re-run below.
  const collected: GradientRunRecord[] = [];
  const cellKey = (r: { fixture: string; model: string; level: string; rep?: number }): string =>
    JSON.stringify([r.model, r.fixture, r.level, r.rep ?? 0]);
  const completed = new Set<string>();
  if (opts.resumeRuns?.length) {
    let resumed = 0;
    for (const r of opts.resumeRuns) {
      if (!r.error && r.fixture && r.model && r.level != null) {
        collected.push(r);
        completed.add(cellKey(r));
        resumed += 1;
      }
    }
    if (resumed) console.error(`[gradient] resuming with ${resumed} prior cell(s) skipped`);
  }
  let writeChain: Promise<void> = Promise.resolve();
  let checkpointWriteFailures = 0;
  const writeCheckpoint = (): Promise<void> => {
    if (!opts.checkpointPath) return Promise.resolve();
    const path = opts.checkpointPath;
    writeChain = writeChain.then(async () => {
      const snapshot: GradientResult = {
        models: opts.models,
        levels,
        timestamp: new Date().toISOString(),
        runs: collected,
      };
      try {
        await fs.writeFile(path, JSON.stringify(snapshot, null, 2));
      } catch (err) {
        checkpointWriteFailures += 1;
        console.error(
          `[gradient] checkpoint write failed: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    });
    return writeChain;
  };

  // One model's worth of work — sequential over reps × fixtures × paths.
  // Rep is the outer loop so partial runs still give >=1 complete rep per cell.
  const runModel = async (model: string): Promise<GradientRunRecord[]> => {
    const slug = slugifyModel(model);
    const nsBudget = 63 - baseCtx.namespace.length - 1;
    if (opts.models.length > 1 && nsBudget < 1) {
      throw new Error(
        `baseCtx.namespace '${baseCtx.namespace}' is too long (${baseCtx.namespace.length} chars) — needs room for '-<model-slug>' within 63 chars (RFC1123).`,
      );
    }
    const nsSlug = slug.slice(0, nsBudget).replace(/-+$/, '');
    const ctx: AzureContext =
      opts.models.length > 1
        ? {
            ...baseCtx,
            imageName: `${baseCtx.imageName}-${slug}`,
            namespace: `${baseCtx.namespace}-${nsSlug}`,
          }
        : baseCtx;
    await ensureNamespace(baseCtx, ctx.namespace);
    console.error(
      `[gradient] start model=${model} imageName=${ctx.imageName} namespace=${ctx.namespace} reps=${reps}`,
    );
    const out: GradientRunRecord[] = [];
    for (let rep = 0; rep < reps; rep++) {
      for (const fixture of opts.fixtures) {
        for (const level of levels) {
          const key = cellKey({ fixture, model, level: level.id, rep });
          if (completed.has(key)) {
            console.error(
              `[gradient] skip  model=${model} rep=${rep + 1}/${reps} ` +
                `fixture=${fixture.split('/').pop()} path=${level.id} (resumed)`,
            );
            continue;
          }
          const r = await runOneLevel({ fixture, level, model, ctx, checkSpecs, rep });
          console.error(
            `[gradient] done  model=${model} rep=${rep + 1}/${reps} ` +
              `fixture=${r.fixture.split('/').pop()} ` +
              `path=${r.level} ${r.error ? 'ERROR' : 'ok'} ` +
              `(${r.durationMs ? Math.round(r.durationMs / 1000) + 's' : '?'})`,
          );
          out.push(r);
          collected.push(r);
          if (!r.error) completed.add(key);
          void writeCheckpoint();
        }
      }
    }
    console.error(
      `[gradient] finish model=${model} (${Math.round((Date.now() - t0) / 1000)}s wall so far)`,
    );
    return out;
  };

  const concurrency = parallel
    ? Math.max(1, Math.min(opts.maxConcurrentModels ?? opts.models.length, opts.models.length))
    : 1;
  const all =
    concurrency === 1
      ? await opts.models.reduce<Promise<GradientRunRecord[]>>(
          async (accP, m) => (await accP).concat(await runModel(m)),
          Promise.resolve([]),
        )
      : (await runWithConcurrency(opts.models, concurrency, runModel)).flat();

  // Flush the final checkpoint before returning.
  await writeCheckpoint();
  if (checkpointWriteFailures > 0) {
    console.error(
      `[gradient] WARNING: ${checkpointWriteFailures} checkpoint write(s) failed during this run — resume data on disk may be incomplete.`,
    );
  }

  // `collected` includes any resumed (seeded) records plus everything run this
  // session; `all` holds only cells run this session. Returning `collected`
  // keeps resumed cells in the final result/report instead of dropping them.
  void all;
  return {
    models: opts.models,
    levels,
    timestamp: new Date().toISOString(),
    runs: collected,
  };
}

// ---------- Report helpers (shared by the HTML report) ----------

/** All check names across the result, in first-seen order. */
function allCheckNames(result: GradientResult): string[] {
  const seen = new Set<string>();
  const order: string[] = [];
  for (const r of result.runs) {
    for (const c of r.checks) {
      if (!seen.has(c.name)) {
        seen.add(c.name);
        order.push(c.name);
      }
    }
  }
  return order;
}

const HEADLINE_CHECK_NAMES = [
  'docker-builds',
  'requires-azure-base',
  'has-required-labels',
] as const;

/** Headline check names present in this result, in canonical order. */
function headlineChecks(result: GradientResult): string[] {
  const present = new Set(allCheckNames(result));
  return HEADLINE_CHECK_NAMES.filter((n) => present.has(n));
}

/** Distinct fixtures across the result, first-seen order. */
function uniqueFixtures(result: GradientResult): string[] {
  return Array.from(new Set(result.runs.map((r) => r.fixture)));
}

/** Sum a token field across non-errored runs. */
function sumTokens(runs: GradientRunRecord[], key: 'tokensIn' | 'tokensOut'): number {
  return runs.reduce((s, r) => s + (r.error ? 0 : (r[key] ?? 0)), 0);
}

function checkPassed(r: GradientRunRecord, name: string): boolean | undefined {
  const c = r.checks.find((x) => x.name === name);
  return c?.passed;
}

/** Short fixture nickname used as a column-group header. */
function fixtureNick(p: string): string {
  const base = (p.split('/').pop() ?? p).toLowerCase();
  if (base.startsWith('spring-boot')) return 'spring-boot';
  if (base.startsWith('spring-mvc')) return 'spring-mvc';
  if (base.startsWith('coolstore')) return 'coolstore';
  return base.slice(0, 12);
}

/** Compact token count: 1234 → "1.2K", 1_200_000 → "1.2M". Shared by both reports. */
function fmtKTokens(n?: number): string {
  if (n == null) return '—';
  const abs = Math.abs(n);
  if (abs < 1000) return String(Math.round(n));
  if (abs < 1_000_000) return `${(n / 1000).toFixed(abs >= 10_000 ? 0 : 1)}K`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

// ---------- HTML reporting ----------

const escapeHtml = (s: string): string =>
  s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');

/**
 * Linear interpolation between two integers.
 */
const lerp = (a: number, b: number, t: number): number => Math.round(a + (b - a) * t);

/**
 * Map a 0..1 rate to a continuous red→amber→green colour. Used by the
 * heatmap cells and scatter dots so the eye reads "more green = better"
 * regardless of which view it lands on.
 */
const rateColour = (rate: number): string => {
  if (!Number.isFinite(rate)) return '#6e7681';
  const r = Math.max(0, Math.min(1, rate));
  // Two-stop gradient: red (#f85149) → amber (#d29922) at 0.5 → green (#3fb950)
  let R: number, G: number, B: number;
  if (r < 0.5) {
    const t = r / 0.5;
    R = lerp(0xf8, 0xd2, t);
    G = lerp(0x51, 0x99, t);
    B = lerp(0x49, 0x22, t);
  } else {
    const t = (r - 0.5) / 0.5;
    R = lerp(0xd2, 0x3f, t);
    G = lerp(0x99, 0xb9, t);
    B = lerp(0x22, 0x50, t);
  }
  return `#${R.toString(16).padStart(2, '0')}${G.toString(16).padStart(2, '0')}${B.toString(16).padStart(2, '0')}`;
};

/**
 * Brand/identity colour per delivery path. Deliberately distinct from the
 * red→green performance ramp: `bare` is a neutral grey (it's the control,
 * not a "failure"), so method identity never reads as a pass/fail signal.
 */
const METHOD_COLOURS: Record<LevelId, string> = {
  bare: '#8b949e',
  mcp: '#a371f7',
  skills: '#3fb950',
};

/**
 * Diverging colour for a lift value `d` (Δ check-pass fraction vs `bare`,
 * in [-1, 1]). Neutral slate at ~0, saturating to green for gains and red
 * for regressions. Uses sqrt magnitude so small but real lifts stay visible.
 */
const liftColour = (d: number): string => {
  if (!Number.isFinite(d)) return '#21262d';
  if (Math.abs(d) < 0.005) return '#2d333b';
  const mag = Math.min(1, Math.sqrt(Math.abs(d)));
  const neutral = [0x2d, 0x33, 0x3b] as const;
  const target = d > 0 ? ([0x3f, 0xb9, 0x50] as const) : ([0xf8, 0x51, 0x49] as const);
  const R = lerp(neutral[0], target[0], mag);
  const G = lerp(neutral[1], target[1], mag);
  const B = lerp(neutral[2], target[2], mag);
  return `#${R.toString(16).padStart(2, '0')}${G.toString(16).padStart(2, '0')}${B.toString(16).padStart(2, '0')}`;
};

/**
 * One aggregated heatmap cell. Aggregates across reps with two views:
 *
 *   - checkRatio:    individual checks passed / total checks evaluated.
 *                    The gradient signal. e.g. 5 reps × 2-of-3 checks
 *                    = 10/15 = 67%.
 *   - fullPassRatio: reps where every check passed / valid reps.
 *                    The strict "perfect run" rate. e.g. same data
 *                    above = 0/5 = 0%.
 *
 * checkRatio drives the cell colour so partial progress is visible;
 * fullPassRatio is shown as subtext so we don't lose the binary signal.
 */
interface CellAgg {
  /** Number of valid (non-errored) reps where every headline check passed. */
  fullPass: number;
  /** Sum of individual headline checks that passed across all valid reps. */
  checksPassed: number;
  /** Sum of individual headline checks evaluated across all valid reps. */
  checksTotal: number;
  /** Total reps recorded for this cell (valid + errored). */
  reps: number;
  /** Reps that errored before checks could run. */
  errored: number;
  /** checksPassed / checksTotal; null when no valid reps. */
  checkRatio: number | null;
  /** fullPass / validReps; null when no valid reps. */
  fullPassRatio: number | null;
}

const aggregateCell = (
  runs: GradientRunRecord[],
  model: string,
  levelId: LevelId,
  fixture: string,
  headlineNames: string[],
): CellAgg => {
  const matching = runs.filter(
    (r) => r.model === model && r.level === levelId && r.fixture === fixture,
  );
  const empty: CellAgg = {
    fullPass: 0,
    checksPassed: 0,
    checksTotal: 0,
    reps: 0,
    errored: 0,
    checkRatio: null,
    fullPassRatio: null,
  };
  if (matching.length === 0) return empty;
  let fullPass = 0;
  let errored = 0;
  let valid = 0;
  let checksPassed = 0;
  let checksTotal = 0;
  for (const r of matching) {
    if (r.error) {
      errored++;
      continue;
    }
    valid++;
    let allOk = true;
    for (const name of headlineNames) {
      checksTotal++;
      if (checkPassed(r, name) === true) checksPassed++;
      else allOk = false;
    }
    if (allOk) fullPass++;
  }
  return {
    fullPass,
    checksPassed,
    checksTotal,
    reps: matching.length,
    errored,
    checkRatio: checksTotal === 0 ? null : checksPassed / checksTotal,
    fullPassRatio: valid === 0 ? null : fullPass / valid,
  };
};

/**
 * Render a self-contained HTML report (inline CSS + SVG + a little vanilla JS,
 * no external deps). Sections: scoreboard + verdict, method-grouped quality
 * heatmap (absolute / lift-vs-bare toggle), cost-vs-effectiveness scatter,
 * per-validation breakdown, token-spend bars, and an errors footer.
 */
export function formatGradientHtml(result: GradientResult): string {
  const headlineNames = headlineChecks(result);
  const fixtures = uniqueFixtures(result);

  // Reps per cell — used in the heatmap explainer and the banner subline.
  // If every cell has the same N, we surface that; -1 if mixed; 0 if empty.
  const repsPerCell = (() => {
    const counts = new Set<number>();
    for (const level of result.levels) {
      for (const m of result.models) {
        for (const fix of fixtures) {
          const c = result.runs.filter(
            (r) => r.model === m && r.level === level.id && r.fixture === fix,
          ).length;
          if (c > 0) counts.add(c);
        }
      }
    }
    if (counts.size === 0) return 0;
    if (counts.size === 1) return [...counts][0];
    return -1;
  })();

  // ---------- Per-level rollups (drive the scoreboard + verdict) ----------
  interface Rollup {
    level: LevelConfig;
    perfectCells: number;
    totalCells: number;
    checksPassed: number;
    checksTotal: number;
    tokensIn: number;
    tokensOut: number;
    validReps: number;
    erroredReps: number;
  }
  const rollupFor = (level: LevelConfig): Rollup => {
    let perfectCells = 0;
    let totalCells = 0;
    let checksPassed = 0;
    let checksTotal = 0;
    let validReps = 0;
    let erroredReps = 0;
    for (const model of result.models) {
      for (const fix of fixtures) {
        const cell = aggregateCell(result.runs, model, level.id, fix, headlineNames);
        if (cell.reps === 0) continue;
        totalCells++;
        if (cell.fullPassRatio === 1) perfectCells++;
        checksPassed += cell.checksPassed;
        checksTotal += cell.checksTotal;
        validReps += cell.reps - cell.errored;
        erroredReps += cell.errored;
      }
    }
    const levelRuns = result.runs.filter((r) => r.level === level.id);
    const tokensIn = sumTokens(levelRuns, 'tokensIn');
    const tokensOut = sumTokens(levelRuns, 'tokensOut');
    return {
      level,
      perfectCells,
      totalCells,
      checksPassed,
      checksTotal,
      tokensIn,
      tokensOut,
      validReps,
      erroredReps,
    };
  };
  const rollups = result.levels.map(rollupFor);
  const rollupById = new Map<LevelId, Rollup>(rollups.map((r) => [r.level.id, r]));
  const ratioOf = (r: Rollup): number => (r.checksTotal === 0 ? 0 : r.checksPassed / r.checksTotal);
  const bareRoll = rollupById.get('bare');
  const bestByRatio = [...rollups].sort((a, b) => ratioOf(b) - ratioOf(a))[0];

  // ---------- Scoreboard (exec comparison of the 3 paths) ----------
  const scoreCard = (r: Rollup): string => {
    const ratio = ratioOf(r);
    const isWinner = bestByRatio != null && r.level.id === bestByRatio.level.id && ratio > 0;
    const lift = bareRoll && r.level.id !== 'bare' ? ratio - ratioOf(bareRoll) : null;
    const liftHtml =
      lift == null
        ? '<span class="score-delta muted">baseline / control</span>'
        : `<span class="score-delta" style="color:${lift >= 0 ? '#3fb950' : '#f85149'}">${lift >= 0 ? '▲' : '▼'} ${Math.abs(Math.round(lift * 100))} pts vs bare</span>`;
    const costMult = bareRoll && bareRoll.tokensIn > 0 ? r.tokensIn / bareRoll.tokensIn : null;
    return `<div class="score-card${isWinner ? ' winner' : ''}" style="--accent:${METHOD_COLOURS[r.level.id]}">
      ${isWinner ? '<div class="score-crown">★ best quality</div>' : ''}
      <div class="score-method"><span class="score-dot" style="background:${METHOD_COLOURS[r.level.id]}"></span>${escapeHtml(r.level.label)} <span class="score-id">${escapeHtml(r.level.id)}</span></div>
      <div class="score-ratio" style="color:${rateColour(ratio)}">${Math.round(ratio * 100)}<span class="score-pct">%</span></div>
      <div class="score-ratio-sub">checks passed (${r.checksPassed}/${r.checksTotal})</div>
      ${liftHtml}
      <div class="score-stats">
        <div><span class="score-stat-label">perfect cells</span><span class="score-stat-val">${r.perfectCells}/${r.totalCells}</span></div>
        <div><span class="score-stat-label">tokens in</span><span class="score-stat-val">${fmtKTokens(r.tokensIn)}</span></div>
        <div><span class="score-stat-label">cost vs bare</span><span class="score-stat-val">${costMult == null ? '—' : `${costMult.toFixed(1)}×`}</span></div>
        <div><span class="score-stat-label">tok / passing check</span><span class="score-stat-val">${r.checksPassed > 0 ? fmtKTokens(r.tokensIn / r.checksPassed) : '—'}</span></div>
      </div>
    </div>`;
  };
  const scoreboardSection = `<div class="scoreboard">${rollups.map(scoreCard).join('')}</div>`;

  // ---------- Verdict line ----------
  const verdictSection = ((): string => {
    if (!bareRoll || rollups.length === 0 || bestByRatio == null) return '';
    const winner = bestByRatio;
    const lift = ratioOf(winner) - ratioOf(bareRoll);
    const costMult = bareRoll.tokensIn > 0 ? winner.tokensIn / bareRoll.tokensIn : null;
    if (winner.level.id === 'bare') {
      return `<p class="verdict">No CA path beat the <strong>bare</strong> control on check pass-rate in this run.</p>`;
    }
    return `<p class="verdict">
      <strong style="color:${METHOD_COLOURS[winner.level.id]}">${escapeHtml(winner.level.label)}</strong>
      leads on quality — <strong>${Math.round(ratioOf(winner) * 100)}%</strong> of checks pass,
      <strong style="color:#3fb950">+${Math.round(lift * 100)} pts</strong> over bare${
        costMult ? ` for <strong>${costMult.toFixed(1)}×</strong> the input tokens` : ''
      }. Use the cost view to judge whether that lift is worth the spend.
    </p>`;
  })();

  // ---------- Heatmaps (absolute + lift), METHOD-GROUPED ----------
  // Columns are grouped by method in bare → mcp → skills order, each method
  // spanning all fixtures, so a method's cells are read together as a block.
  const colCount = result.levels.length * fixtures.length;

  const heatmapHeaderTop = [
    `<div class="heatmap-corner"></div>`,
    ...result.levels.map(
      (l) =>
        `<div class="heatmap-method-group" style="grid-column: span ${fixtures.length};"><span class="hm-dot" style="background:${METHOD_COLOURS[l.id]}"></span>${escapeHtml(l.label)}</div>`,
    ),
  ].join('');

  const heatmapHeaderBot = [
    `<div class="heatmap-corner"></div>`,
    ...result.levels.flatMap(() =>
      fixtures.map((f) => `<div class="heatmap-fixture-label">${escapeHtml(fixtureNick(f))}</div>`),
    ),
  ].join('');

  const renderAbsCell = (model: string, level: LevelConfig, fix: string): string => {
    const cell = aggregateCell(result.runs, model, level.id, fix, headlineNames);
    if (cell.reps === 0) {
      return `<div class="heatmap-cell empty" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: no data`)}"></div>`;
    }
    if (cell.checkRatio === null) {
      return `<div class="heatmap-cell errored" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: ${cell.errored}/${cell.reps} errored`)}"><span class="cell-pct">err</span><span class="cell-sub">${cell.errored}/${cell.reps}</span></div>`;
    }
    const validReps = cell.reps - cell.errored;
    const pct = Math.round(cell.checkRatio * 100);
    const sub =
      validReps > 1 && cell.fullPassRatio !== null ? `${cell.fullPass}/${validReps} perfect` : '';
    const fullPct = cell.fullPassRatio === null ? '—' : `${Math.round(cell.fullPassRatio * 100)}%`;
    const tooltip =
      `${model} / ${level.id} / ${fixtureNick(fix)}\n` +
      `${cell.checksPassed}/${cell.checksTotal} checks passed (${pct}%)\n` +
      `${cell.fullPass}/${validReps} reps with all checks passing (${fullPct})` +
      (cell.errored ? `\n+${cell.errored} rep(s) errored` : '');
    return `<div class="heatmap-cell" style="background:${rateColour(cell.checkRatio)}" title="${escapeHtml(tooltip)}"><span class="cell-pct">${pct}%</span>${sub ? `<span class="cell-sub">${sub}</span>` : ''}</div>`;
  };

  const renderLiftCell = (model: string, level: LevelConfig, fix: string): string => {
    const cell = aggregateCell(result.runs, model, level.id, fix, headlineNames);
    if (level.id === 'bare') {
      const base = cell.checkRatio;
      const baseTxt = base == null ? '—' : `${Math.round(base * 100)}%`;
      return `<div class="heatmap-cell base" title="${escapeHtml(`${model} / bare / ${fixtureNick(fix)}: baseline ${baseTxt}`)}"><span class="cell-pct">${baseTxt}</span><span class="cell-sub">base</span></div>`;
    }
    const bareCell = aggregateCell(result.runs, model, 'bare', fix, headlineNames);
    if (cell.checkRatio === null || bareCell.checkRatio === null) {
      return `<div class="heatmap-cell empty" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: no comparison`)}">—</div>`;
    }
    const d = cell.checkRatio - bareCell.checkRatio;
    const pts = Math.round(d * 100);
    const sign = pts > 0 ? '+' : pts < 0 ? '−' : '±';
    const tooltip =
      `${model} / ${level.id} vs bare / ${fixtureNick(fix)}\n` +
      `${Math.round(cell.checkRatio * 100)}% vs ${Math.round(bareCell.checkRatio * 100)}% bare\n` +
      `Δ ${sign}${Math.abs(pts)} pts`;
    return `<div class="heatmap-cell lift" style="background:${liftColour(d)}" title="${escapeHtml(tooltip)}"><span class="cell-pct">${sign}${Math.abs(pts)}</span><span class="cell-sub">pts</span></div>`;
  };

  const buildRows = (render: (m: string, l: LevelConfig, f: string) => string): string =>
    result.models
      .map((model) => {
        const cells = result.levels
          .flatMap((level) => fixtures.map((fix) => render(model, level, fix)))
          .join('');
        return `<div class="heatmap-row-label">${escapeHtml(model)}</div>${cells}`;
      })
      .join('');

  const heatmapRowsAbs = buildRows(renderAbsCell);
  const heatmapRowsLift = buildRows(renderLiftCell);
  const hasBare = result.levels.some((l) => l.id === 'bare');
  const gridStyle = `grid-template-columns: minmax(110px, auto) repeat(${colCount}, minmax(0, 1fr));`;

  const heatmapSection = `<section class="heatmap-block">
    <div class="heatmap-toolbar">
      <div class="heatmap-tabs">
        <button id="btn-abs" class="hm-tab active" onclick="hmView('abs')">Absolute</button>
        <button id="btn-lift" class="hm-tab" onclick="hmView('lift')"${hasBare ? '' : ' disabled title="needs the bare control"'}>Lift vs bare</button>
      </div>
      <p class="heatmap-explainer" id="hm-explainer-abs">
        Rows = models. Columns are grouped by method in <strong>bare → mcp → skills</strong> order.
        <strong>Colour &amp; %</strong> = fraction of
        individual checks that passed${repsPerCell > 1 ? ` across ${repsPerCell} reps` : repsPerCell === -1 ? ' across all reps' : ''},
        on a <span style="color:#f85149">red</span> (low) to <span style="color:#3fb950">green</span> (high) scale.
        ${repsPerCell !== 1 ? 'Subtext <strong>M/N perfect</strong> = flawless reps. ' : ''}Hover any cell for detail.
      </p>
      <p class="heatmap-explainer" id="hm-explainer-lift" style="display:none">
        Each non-bare cell shows its <strong>lift in percentage points vs the bare control</strong> for the
        same model × fixture. <span style="color:#3fb950">Green</span> = CA improved it,
        <span style="color:#f85149">red</span> = regressed, slate = no change.
      </p>
    </div>
    <div id="heatmap-abs" class="heatmap-grid" style="${gridStyle}">
      ${heatmapHeaderTop}
      ${heatmapHeaderBot}
      ${heatmapRowsAbs}
    </div>
    <div id="heatmap-lift" class="heatmap-grid" style="${gridStyle} display:none;">
      ${heatmapHeaderTop}
      ${heatmapHeaderBot}
      ${heatmapRowsLift}
    </div>
    <div class="heatmap-legend" id="legend-abs">
      <span>0% checks</span>
      <div class="heatmap-legend-bar"></div>
      <span>100% checks</span>
      <span class="legend-spacer"></span>
      <span class="legend-item"><span class="swatch errored"></span>errored</span>
      <span class="legend-item"><span class="swatch empty"></span>no data</span>
    </div>
    <div class="heatmap-legend" id="legend-lift" style="display:none">
      <span>−100 pts</span>
      <div class="heatmap-legend-bar lift"></div>
      <span>+100 pts</span>
      <span class="legend-spacer"></span>
      <span class="legend-item"><span class="swatch base"></span>bare baseline</span>
    </div>
  </section>`;

  // ---------- Deploy-readiness grid (informational, harness-measured) --------
  // Distinct from the quality heatmap and NOT part of the scored checks: did the
  // run's workload actually reach Running/Ready on the cluster (verifyDeploy
  // success)? Only the deploy-expected paths (mcp/skills) carry the signal; the
  // bare control attempts no deploy. Rendered only if any run has the signal.
  const anyDeploySignal = result.runs.some((r) => typeof r.deployVerified === 'boolean');
  const renderReadyCell = (model: string, level: LevelConfig, fix: string): string => {
    const matching = result.runs.filter(
      (r) => r.model === model && r.level === level.id && r.fixture === fix,
    );
    if (matching.length === 0) {
      return `<div class="heatmap-cell empty" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: no data`)}"></div>`;
    }
    const signal = matching.filter((r) => typeof r.deployVerified === 'boolean');
    if (signal.length === 0) {
      return `<div class="heatmap-cell empty" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: no deploy attempted`)}"><span class="cell-pct">—</span></div>`;
    }
    const ready = signal.filter((r) => r.deployVerified === true).length;
    const total = signal.length;
    const bg = ready === total ? '#1f6f3f' : ready > 0 ? '#7a5a1f' : '#6f1f2a';
    const icon = ready === total ? '✅' : ready > 0 ? '◐' : '❌';
    const sub = total > 1 ? `${ready}/${total}` : '';
    const tooltip =
      `${model} / ${level.id} / ${fixtureNick(fix)}\n` +
      `${ready}/${total} rep(s) reached Running/Ready on the cluster (verifyDeploy success)`;
    return `<div class="heatmap-cell" style="background:${bg}" title="${escapeHtml(tooltip)}"><span class="cell-pct">${icon}</span>${sub ? `<span class="cell-sub">${sub}</span>` : ''}</div>`;
  };

  const readinessSection = !anyDeploySignal
    ? ''
    : `<section class="heatmap-block">
    <p class="heatmap-explainer">
      Ground truth measured by the harness (not the agent's own claims): did the run's workload
      actually reach <strong>Running/Ready</strong> on the cluster (<code>verifyDeploy</code> success)?
      <span style="color:#3fb950">✅ all reps ready</span> ·
      <span style="color:#d8a13a">◐ some ready</span> ·
      <span style="color:#f85149">❌ not ready</span> ·
      <span style="color:#8b949e">— no deploy attempted</span>.
      Informational only — <strong>not part of the scored checks</strong>.
    </p>
    <div class="heatmap-grid" style="${gridStyle}">
      ${heatmapHeaderTop}
      ${heatmapHeaderBot}
      ${buildRows(renderReadyCell)}
    </div>
  </section>`;

  // One dot per (path × model). Position = (total tokens IN, overall
  // check pass rate). Dot size scales with tokens OUT. Using check-level
  // pass rate (not full-pass) keeps it consistent with the heatmap colour.
  interface ScatterPoint {
    level: LevelConfig;
    model: string;
    tokensIn: number;
    tokensOut: number;
    rate: number;
    checksPassed: number;
    checksTotal: number;
    fullPass: number;
    validReps: number;
  }
  const scatterPoints: ScatterPoint[] = [];
  for (const level of result.levels) {
    for (const model of result.models) {
      const matching = result.runs.filter((r) => r.model === model && r.level === level.id);
      if (matching.length === 0) continue;
      let tokensIn = 0;
      let tokensOut = 0;
      let checksPassed = 0;
      let checksTotal = 0;
      let fullPass = 0;
      let validReps = 0;
      for (const r of matching) {
        tokensIn += r.tokensIn ?? 0;
        tokensOut += r.tokensOut ?? 0;
        if (r.error) continue;
        validReps++;
        let allOk = true;
        for (const name of headlineNames) {
          checksTotal++;
          if (checkPassed(r, name) === true) checksPassed++;
          else allOk = false;
        }
        if (allOk) fullPass++;
      }
      scatterPoints.push({
        level,
        model,
        tokensIn,
        tokensOut,
        rate: checksTotal === 0 ? 0 : checksPassed / checksTotal,
        checksPassed,
        checksTotal,
        fullPass,
        validReps,
      });
    }
  }

  const scatterBlock = ((): string => {
    if (scatterPoints.length === 0) return '';
    const W = 760;
    const H = 420;
    const ML = 70;
    const MR = 30;
    const MT = 30;
    const MB = 55;
    const innerW = W - ML - MR;
    const innerH = H - MT - MB;

    const maxIn = Math.max(...scatterPoints.map((p) => p.tokensIn), 1);
    const minIn = Math.max(
      1,
      Math.min(...scatterPoints.filter((p) => p.tokensIn > 0).map((p) => p.tokensIn)),
    );
    const logMin = Math.floor(Math.log10(minIn));
    const logMax = Math.ceil(Math.log10(maxIn));
    const xOf = (tokens: number): number => {
      const t = tokens <= 0 ? 0 : (Math.log10(tokens) - logMin) / (logMax - logMin);
      return ML + Math.max(0, Math.min(1, t)) * innerW;
    };
    const yOf = (rate: number): number => MT + (1 - rate) * innerH;

    const maxOut = Math.max(...scatterPoints.map((p) => p.tokensOut), 1);
    const rOf = (tokensOut: number): number => 6 + 14 * Math.sqrt(tokensOut / maxOut);

    // Axes + gridlines
    const xTicks: number[] = [];
    for (let i = logMin; i <= logMax; i++) xTicks.push(i);
    const yTicks = [0, 0.25, 0.5, 0.75, 1.0];

    const parts: string[] = [
      `<svg viewBox="0 0 ${W} ${H}" width="100%" role="img" aria-label="Cost vs effectiveness scatter">`,
      // Winner zone (top-left)
      `<rect x="${ML}" y="${MT}" width="${innerW / 2}" height="${innerH / 2}" fill="#3fb95015"/>`,
      `<text x="${ML + 10}" y="${MT + 22}" class="zone-label zone-win">WINNER ZONE — cheap & effective</text>`,
      // Loser zone (bottom-right)
      `<rect x="${ML + innerW / 2}" y="${MT + innerH / 2}" width="${innerW / 2}" height="${innerH / 2}" fill="#f8514915"/>`,
      `<text x="${ML + innerW - 10}" y="${MT + innerH - 10}" class="zone-label zone-lose" text-anchor="end">LOSER ZONE — expensive & ineffective</text>`,
    ];

    // Y gridlines + labels
    for (const t of yTicks) {
      const y = yOf(t);
      parts.push(
        `<line x1="${ML}" y1="${y}" x2="${ML + innerW}" y2="${y}" class="grid"/>`,
        `<text x="${ML - 8}" y="${y + 4}" class="axis-label" text-anchor="end">${Math.round(t * 100)}%</text>`,
      );
    }
    // X gridlines + labels
    for (const exp of xTicks) {
      const x = xOf(Math.pow(10, exp));
      parts.push(
        `<line x1="${x}" y1="${MT}" x2="${x}" y2="${MT + innerH}" class="grid"/>`,
        `<text x="${x}" y="${MT + innerH + 18}" class="axis-label" text-anchor="middle">${fmtKTokens(Math.pow(10, exp))}</text>`,
      );
    }
    // Axis lines
    parts.push(
      `<line x1="${ML}" y1="${MT}" x2="${ML}" y2="${MT + innerH}" class="axis"/>`,
      `<line x1="${ML}" y1="${MT + innerH}" x2="${ML + innerW}" y2="${MT + innerH}" class="axis"/>`,
      // Axis titles
      `<text x="${ML + innerW / 2}" y="${H - 12}" class="axis-title" text-anchor="middle">total tokens IN (log scale) — lower is cheaper →</text>`,
      `<text x="18" y="${MT + innerH / 2}" class="axis-title" text-anchor="middle" transform="rotate(-90 18 ${MT + innerH / 2})">checks passed (%) — higher is better ↑</text>`,
    );

    // Journey lines: connect bare→mcp→skills for each model so you can see
    // how adding CA moves that model up-and-(usually)-right through the chart.
    const levelOrder = result.levels.map((l) => l.id);
    for (const model of result.models) {
      const pts = scatterPoints
        .filter((p) => p.model === model)
        .sort((a, b) => levelOrder.indexOf(a.level.id) - levelOrder.indexOf(b.level.id));
      if (pts.length < 2) continue;
      const poly = pts
        .map((p) => `${xOf(p.tokensIn).toFixed(1)},${yOf(p.rate).toFixed(1)}`)
        .join(' ');
      parts.push(`<polyline points="${poly}" class="journey"/>`);
    }

    // Dots (drawn after gridlines so they sit on top). Sort so smaller
    // dots draw last to stay visible.
    const sorted = [...scatterPoints].sort((a, b) => b.tokensOut - a.tokensOut);
    // Track which dot positions we've already used so we can stagger labels
    // when multiple dots land near the same spot.
    const placed: Array<{ x: number; y: number; count: number }> = [];
    for (const p of sorted) {
      const x = xOf(p.tokensIn);
      const y = yOf(p.rate);
      const r = rOf(p.tokensOut);
      const colour = METHOD_COLOURS[p.level.id];
      const title = `${p.level.id} · ${p.model}  —  ${p.checksPassed}/${p.checksTotal} checks passed (${Math.round(p.rate * 100)}%)  ·  ${p.fullPass}/${p.validReps} reps perfect  ·  ${fmtKTokens(p.tokensIn)} in  ·  ${fmtKTokens(p.tokensOut)} out`;
      // Find any existing dot within 28px; stack the label below it.
      const slot = placed.find((s) => Math.hypot(s.x - x, s.y - y) < 28);
      let labelDy = 4;
      if (slot) {
        slot.count++;
        labelDy = 4 + slot.count * 14;
      } else {
        placed.push({ x, y, count: 0 });
      }
      const labelText = `${p.level.id}/${p.model.replace(/^azure:/, '')}`;
      parts.push(
        `<g><title>${escapeHtml(title)}</title>`,
        `<circle cx="${x}" cy="${y}" r="${r}" fill="${colour}" fill-opacity="0.7" stroke="${colour}" stroke-width="2"/>`,
        `<text x="${x + r + 4}" y="${y + labelDy}" class="dot-label" style="fill:${colour}">${escapeHtml(labelText)}</text>`,
        `</g>`,
      );
    }

    parts.push(`</svg>`);

    return `<section class="chart-block">
      <h3>Cost vs effectiveness <span class="subtle">— X: total tokens IN (log) · Y: % of individual checks passed · dot size: tokens OUT</span></h3>
      <div class="legend">
        ${result.levels
          .map(
            (l) =>
              `<span class="legend-item"><span class="swatch" style="background:${METHOD_COLOURS[l.id]}"></span>${escapeHtml(l.label)}</span>`,
          )
          .join('')}
        <span class="legend-spacer"></span>
        <span class="legend-note">dot size ∝ tokens OUT</span>
      </div>
      ${parts.join('')}
    </section>`;
  })();

  // ---------- Token bar charts (compact reference) ----------
  const tokenBars = ((): string => {
    interface Row {
      level: LevelConfig;
      model: string;
      tokensIn: number;
      tokensOut: number;
    }
    const rows: Row[] = [];
    for (const level of result.levels) {
      for (const model of result.models) {
        const cellRuns = result.runs.filter((r) => r.model === model && r.level === level.id);
        rows.push({
          level,
          model,
          tokensIn: sumTokens(cellRuns, 'tokensIn'),
          tokensOut: sumTokens(cellRuns, 'tokensOut'),
        });
      }
    }
    if (rows.length === 0) return '';
    const maxIn = Math.max(...rows.map((r) => r.tokensIn), 1);
    const maxOut = Math.max(...rows.map((r) => r.tokensOut), 1);

    const renderRow = (r: Row): string => {
      const inPct = (r.tokensIn / maxIn) * 100;
      const outPct = (r.tokensOut / maxOut) * 100;
      const colour = METHOD_COLOURS[r.level.id];
      return `<div class="bar-row">
        <div class="bar-label-cell"><span class="bar-path-dot" style="background:${colour}"></span>${escapeHtml(r.level.id)} · <code>${escapeHtml(r.model)}</code></div>
        <div class="bar-track"><div class="bar-fill" style="width:${inPct}%; background:${colour}"></div></div>
        <div class="bar-value">${fmtKTokens(r.tokensIn)}</div>
        <div class="bar-track"><div class="bar-fill" style="width:${outPct}%; background:${colour}; opacity:0.6"></div></div>
        <div class="bar-value">${fmtKTokens(r.tokensOut)}</div>
      </div>`;
    };

    return `<section class="chart-block">
      <h3>Token spend by path × model <span class="subtle">— absolute totals</span></h3>
      <div class="bar-grid">
        <div class="bar-header"></div>
        <div class="bar-header">tokens IN</div>
        <div class="bar-header value">total</div>
        <div class="bar-header">tokens OUT</div>
        <div class="bar-header value">total</div>
        ${rows.map(renderRow).join('')}
      </div>
    </section>`;
  })();

  // ---------- Per-validation breakdown ----------
  // For each headline check, how often does each path pass it? Answers the
  // concrete question "which specific guarantees does CA actually buy me?"
  const validationsSection = ((): string => {
    if (headlineNames.length === 0) return '';
    const passRate = (checkName: string, levelId: LevelId): { passed: number; total: number } => {
      let passed = 0;
      let total = 0;
      for (const r of result.runs) {
        if (r.level !== levelId || r.error) continue;
        const c = r.checks.find((x) => x.name === checkName);
        if (!c) continue;
        total++;
        if (c.passed) passed++;
      }
      return { passed, total };
    };
    const panels = headlineNames
      .map((name) => {
        const bars = result.levels
          .map((level) => {
            const { passed, total } = passRate(name, level.id);
            const rate = total === 0 ? 0 : passed / total;
            const pct = Math.round(rate * 100);
            const colour = METHOD_COLOURS[level.id];
            return `<div class="vbar-row" title="${escapeHtml(`${level.id} · ${name}: ${passed}/${total} passed`)}">
              <div class="vbar-label"><span class="hm-dot" style="background:${colour}"></span>${escapeHtml(level.id)}</div>
              <div class="vbar-track"><div class="vbar-fill" style="width:${pct}%; background:${colour}"></div><span class="vbar-pct">${total === 0 ? '—' : pct + '%'}</span></div>
              <div class="vbar-count">${passed}/${total}</div>
            </div>`;
          })
          .join('');
        return `<div class="vpanel">
          <div class="vpanel-title"><code>${escapeHtml(name)}</code></div>
          ${bars}
        </div>`;
      })
      .join('');
    return `<section class="chart-block">
      <h3>Per-validation pass rate <span class="subtle">— which checks each path actually satisfies, across all models × fixtures × reps</span></h3>
      <div class="vpanels">${panels}</div>
    </section>`;
  })();

  const errored = result.runs.filter((r) => r.error);
  const errorsBlock = errored.length
    ? `<section class="errors-block">
        <h3>Errored runs (${errored.length})</h3>
        <ul>${errored
          .map(
            (r) =>
              `<li><code>${escapeHtml(r.model)}</code> · ${escapeHtml(fixtureNick(r.fixture))} · <strong>${escapeHtml(r.level)}</strong> — ${escapeHtml(r.error ?? 'unknown')}</li>`,
          )
          .join('')}</ul>
      </section>`
    : '';

  const repsLabel =
    repsPerCell === 0
      ? ''
      : repsPerCell === 1
        ? '1 rep per cell'
        : repsPerCell > 1
          ? `${repsPerCell} reps per cell`
          : 'mixed reps per cell';

  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Gradient eval — ${escapeHtml(result.timestamp)}</title>
<style>
  :root {
    --bg: #0d1117;
    --panel: #161b22;
    --border: #30363d;
    --text: #e6edf3;
    --muted: #8b949e;
  }
  * { box-sizing: border-box; }
  body {
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    margin: 0;
    padding: 32px;
    line-height: 1.45;
    max-width: 1400px;
    margin-left: auto;
    margin-right: auto;
  }
  h1 { font-size: 32px; margin: 0 0 4px; }
  h2 { font-size: 22px; margin: 36px 0 14px; border-bottom: 1px solid var(--border); padding-bottom: 6px; }
  h2 .subtle, h3 .subtle { color: var(--muted); font-weight: 400; font-size: 14px; margin-left: 6px; }
  h3 { font-size: 16px; margin: 0 0 12px; font-weight: 600; }
  .meta { color: var(--muted); margin-bottom: 24px; }
  .meta code { background: var(--panel); padding: 1px 6px; border-radius: 4px; }

  /* Sticky section nav */
  .nav {
    position: sticky; top: 0; z-index: 10;
    display: flex; gap: 8px; flex-wrap: wrap;
    background: rgba(13,17,23,0.92); backdrop-filter: blur(6px);
    border-bottom: 1px solid var(--border);
    margin: 0 -32px 24px; padding: 12px 32px;
  }
  .nav a {
    color: var(--muted); text-decoration: none; font-size: 13px; font-weight: 600;
    padding: 5px 12px; border-radius: 6px; border: 1px solid transparent;
  }
  .nav a:hover { color: var(--text); background: var(--panel); border-color: var(--border); }

  /* Verdict */
  .verdict {
    background: var(--panel); border: 1px solid var(--border); border-left: 3px solid #3fb950;
    border-radius: 8px; padding: 14px 18px; margin: 0 0 20px; font-size: 15px; line-height: 1.5;
  }
  .verdict strong { color: var(--text); }

  /* Scoreboard */
  .scoreboard { display: grid; grid-template-columns: repeat(${result.levels.length}, 1fr); gap: 16px; margin: 8px 0 8px; }
  .score-card {
    position: relative; background: var(--panel); border: 1px solid var(--border);
    border-top: 3px solid var(--accent); border-radius: 8px; padding: 20px; text-align: center;
  }
  .score-card.winner { box-shadow: 0 0 0 1px var(--accent), 0 8px 24px rgba(63,185,80,0.12); }
  .score-crown {
    position: absolute; top: -11px; left: 50%; transform: translateX(-50%);
    background: #3fb950; color: #0d1117; font-size: 11px; font-weight: 800; letter-spacing: 0.5px;
    padding: 2px 10px; border-radius: 10px; text-transform: uppercase; white-space: nowrap;
  }
  .score-method { font-size: 17px; font-weight: 700; display: flex; align-items: center; justify-content: center; gap: 7px; }
  .score-id { color: var(--muted); font-weight: 500; font-size: 12px; font-family: ui-monospace, SFMono-Regular, monospace; }
  .score-dot { width: 11px; height: 11px; border-radius: 50%; display: inline-block; }
  .score-ratio { font-size: 56px; font-weight: 800; line-height: 1.05; letter-spacing: -2px; font-variant-numeric: tabular-nums; margin-top: 8px; }
  .score-pct { font-size: 26px; font-weight: 600; opacity: 0.7; }
  .score-ratio-sub { font-size: 12px; color: var(--muted); }
  .score-delta { display: inline-block; margin-top: 8px; font-size: 13px; font-weight: 700; }
  .score-delta.muted { color: var(--muted); font-weight: 500; }
  .score-stats { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; margin-top: 16px; text-align: left; border-top: 1px solid var(--border); padding-top: 14px; }
  .score-stats > div { display: flex; flex-direction: column; }
  .score-stat-label { font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.4px; }
  .score-stat-val { font-size: 16px; font-weight: 700; font-variant-numeric: tabular-nums; }

  /* Heatmap */
  .heatmap-block {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 20px;
    margin-bottom: 16px;
  }
  .heatmap-explainer {
    color: var(--muted);
    font-size: 12.5px;
    margin: 0 0 14px;
    line-height: 1.5;
  }
  .heatmap-explainer code { background: var(--bg); padding: 1px 6px; border-radius: 3px; font-size: 11px; color: var(--text); }
  .heatmap-explainer strong { color: var(--text); }
  .heatmap-grid {
    display: grid;
    gap: 3px;
    align-items: stretch;
  }
  .heatmap-corner { background: transparent; }
  .heatmap-toolbar { display: flex; flex-direction: column; gap: 10px; margin-bottom: 14px; }
  .heatmap-tabs { display: inline-flex; gap: 4px; background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 3px; align-self: flex-start; }
  .hm-tab { background: transparent; color: var(--muted); border: 0; border-radius: 6px; padding: 6px 14px; font-size: 13px; font-weight: 600; cursor: pointer; }
  .hm-tab.active { background: var(--panel); color: var(--text); box-shadow: 0 1px 2px rgba(0,0,0,0.3); }
  .hm-tab:disabled { opacity: 0.4; cursor: not-allowed; }
  .heatmap-method-group {
    font-size: 14px; font-weight: 700; color: var(--text); text-align: center;
    padding: 6px 8px 5px; border-bottom: 1px solid var(--border);
    text-transform: uppercase; letter-spacing: 0.5px;
    display: flex; align-items: center; justify-content: center; gap: 7px;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .heatmap-fixture-label {
    font-size: 11px; color: var(--muted); text-align: center; padding: 5px 2px 8px;
    font-family: ui-monospace, SFMono-Regular, monospace;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  .hm-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; flex: none; }
  .heatmap-cell.base { background: #21262d; color: var(--muted); }
  .heatmap-cell.lift { color: var(--text); }
  .heatmap-row-label {
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
    font-family: ui-monospace, SFMono-Regular, monospace;
    padding: 0 12px 0 4px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
  }
  .heatmap-cell {
    aspect-ratio: 1;
    min-height: 64px;
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: rgba(0, 0, 0, 0.85);
    font-variant-numeric: tabular-nums;
    transition: transform 0.1s ease;
  }
  .heatmap-cell:hover { transform: scale(1.05); z-index: 2; position: relative; }
  .heatmap-cell.empty { background: #21262d; }
  .heatmap-cell.errored { background: #6e7681; color: var(--text); }
  .cell-pct { font-size: 22px; font-weight: 800; line-height: 1; }
  .cell-sub { font-size: 10px; font-weight: 600; opacity: 0.7; margin-top: 2px; font-family: ui-monospace, SFMono-Regular, monospace; }

  .heatmap-legend {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 14px;
    font-size: 12px;
    color: var(--muted);
  }
  .heatmap-legend-bar {
    width: 200px;
    height: 12px;
    border-radius: 3px;
    background: linear-gradient(to right, #f85149, #d29922, #3fb950);
  }
  .legend-spacer { flex: 1; }
  .legend-item { display: inline-flex; align-items: center; gap: 6px; }
  .legend-item .swatch { width: 12px; height: 12px; border-radius: 2px; display: inline-block; }
  .legend-item .swatch.errored { background: #6e7681; }
  .legend-item .swatch.empty { background: #21262d; }
  .legend-note { color: var(--muted); font-size: 12px; }

  /* Charts */
  .chart-block {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 20px;
    margin-bottom: 16px;
  }
  .legend { display: flex; gap: 16px; flex-wrap: wrap; align-items: center; font-size: 12px; color: var(--muted); margin: 4px 0 12px; }
  .swatch { display: inline-block; width: 12px; height: 12px; border-radius: 2px; margin-right: 6px; vertical-align: middle; }

  svg .grid { stroke: #30363d; stroke-width: 1; stroke-dasharray: 2 3; }
  svg .axis { stroke: var(--muted); stroke-width: 1; }
  svg .axis-label { fill: var(--muted); font-family: ui-monospace, SFMono-Regular, monospace; font-size: 11px; }
  svg .axis-title { fill: var(--muted); font-size: 12px; }
  svg .dot-label { fill: var(--text); font-family: ui-monospace, SFMono-Regular, monospace; font-size: 11px; font-weight: 500; }
  svg .zone-label { font-size: 10px; font-weight: 700; letter-spacing: 1px; font-family: ui-monospace, SFMono-Regular, monospace; }
  svg .zone-win { fill: #3fb950; opacity: 0.55; }
  svg .zone-lose { fill: #f85149; opacity: 0.55; }

  /* Compact token bars */
  .bar-grid {
    display: grid;
    grid-template-columns: 280px 1fr 70px 1fr 70px;
    gap: 6px 12px;
    align-items: center;
  }
  .bar-header { color: var(--muted); font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.5px; padding-bottom: 4px; border-bottom: 1px solid var(--border); }
  .bar-header.value { text-align: right; }
  .bar-row { display: contents; }
  .bar-label-cell { font-size: 12px; color: var(--text); display: flex; align-items: center; gap: 6px; }
  .bar-label-cell code { background: var(--bg); padding: 1px 5px; border-radius: 3px; font-size: 11px; }
  .bar-path-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
  .bar-track { background: #21262d; border-radius: 3px; height: 18px; overflow: hidden; }
  .bar-fill { height: 100%; border-radius: 3px; transition: width 0.2s; }
  .bar-value { font-family: ui-monospace, SFMono-Regular, monospace; font-size: 11px; color: var(--text); text-align: right; }

  .heatmap-legend-bar.lift { background: linear-gradient(to right, #f85149, #2d333b, #3fb950); }
  .legend-item .swatch.base { background: #21262d; }
  svg .journey { fill: none; stroke: #8b949e; stroke-width: 1.5; stroke-dasharray: 3 4; opacity: 0.45; }

  /* Per-validation panels */
  .vpanels { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px; }
  .vpanel { background: var(--bg); border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; }
  .vpanel-title { margin-bottom: 12px; }
  .vpanel-title code { background: var(--panel); padding: 2px 8px; border-radius: 4px; font-size: 12px; color: var(--text); }
  .vbar-row { display: grid; grid-template-columns: 64px 1fr 44px; gap: 8px; align-items: center; margin: 7px 0; }
  .vbar-label { font-size: 12px; color: var(--text); display: flex; align-items: center; gap: 6px; }
  .vbar-track { position: relative; background: #21262d; border-radius: 4px; height: 20px; overflow: hidden; }
  .vbar-fill { height: 100%; border-radius: 4px; transition: width 0.2s; }
  .vbar-pct { position: absolute; right: 7px; top: 50%; transform: translateY(-50%); font-size: 11px; font-weight: 700; color: var(--text); text-shadow: 0 1px 2px rgba(0,0,0,0.6); }
  .vbar-count { font-size: 11px; color: var(--muted); font-family: ui-monospace, SFMono-Regular, monospace; text-align: right; }

  /* Errors */
  html { scroll-behavior: smooth; }
  h2[id] { scroll-margin-top: 72px; }
  .errors-block { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 16px 20px; margin-top: 16px; }
  .errors-block ul { margin: 8px 0 0; padding-left: 20px; color: var(--muted); }
  .errors-block code { background: var(--bg); padding: 1px 6px; border-radius: 3px; }
</style>
</head>
<body>
  <h1>Gradient eval</h1>
  <p class="meta">
    ${result.models.map((m) => `<code>${escapeHtml(m)}</code>`).join(' · ')}
    · ${fixtures.length} fixtures
    · ${result.levels.length} paths
    ${repsLabel ? `· ${escapeHtml(repsLabel)}` : ''}
    · ${escapeHtml(result.timestamp)}
  </p>

  <nav class="nav">
    <a href="#overview">Overview</a>
    <a href="#quality">Quality heatmap</a>
    ${readinessSection ? '<a href="#deploy">Deploy readiness</a>' : ''}
    <a href="#cost">Cost vs effectiveness</a>
    <a href="#validations">Validations</a>
    <a href="#spend">Token spend</a>
    ${errored.length ? '<a href="#errors">Errors</a>' : ''}
  </nav>

  <h2 id="overview">Scoreboard <span class="subtle">— who wins on quality, and what it costs</span></h2>
  ${verdictSection}
  ${scoreboardSection}

  <h2 id="quality">Quality heatmap <span class="subtle">— rows: models · columns: bare → mcp → skills (red → green)</span></h2>
  ${heatmapSection}

  ${
    readinessSection
      ? `<h2 id="deploy">Deploy readiness <span class="subtle">— did it actually run on the cluster (informational, not scored)</span></h2>\n  ${readinessSection}`
      : ''
  }

  <h2 id="cost">Cost vs effectiveness</h2>
  ${scatterBlock}

  <h2 id="validations">Per-validation breakdown</h2>
  ${validationsSection}

  <h2 id="spend">Token spend reference</h2>
  ${tokenBars}

  ${errored.length ? '<h2 id="errors">Errors</h2>' : ''}
  ${errorsBlock}

  <script>
    function hmView(v){
      var abs = v === 'abs';
      document.getElementById('heatmap-abs').style.display = abs ? '' : 'none';
      document.getElementById('heatmap-lift').style.display = abs ? 'none' : '';
      document.getElementById('legend-abs').style.display = abs ? '' : 'none';
      document.getElementById('legend-lift').style.display = abs ? 'none' : '';
      document.getElementById('hm-explainer-abs').style.display = abs ? '' : 'none';
      document.getElementById('hm-explainer-lift').style.display = abs ? 'none' : '';
      document.getElementById('btn-abs').classList.toggle('active', abs);
      document.getElementById('btn-lift').classList.toggle('active', !abs);
    }
  </script>
</body>
</html>`;
}
