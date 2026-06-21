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
    description: 'BASELINE + CA `deploy-to-aks` SKILL bundle in system + CA MCP tools (real-world delivery).',
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
async function resolveLevel(level: LevelId, workingDir: string, ctx: AzureContext): Promise<ResolvedLevel> {
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
   * Number of repetitions per (model, fixture, path) cell. Reps run
   * sequentially within a model so cleanup stays correct. Default 1.
   */
  reps?: number;
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
    record.checks = await runChecks(opts.checkSpecs, {
      artifactDir: workingDir,
      fixtureDir: opts.fixture,
    });
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
 * by default (each gets its own slugged `imageName` so cleanup is isolated);
 * inside one model, fixtures and paths are sequential.
 */
export async function runGradient(opts: GradientOptions): Promise<GradientResult> {
  const checkSpecs = selectChecks(opts.checks);
  const selectedIds = new Set<LevelId>(opts.levels ?? LEVELS.map((l) => l.id));
  const levels = LEVELS.filter((l) => selectedIds.has(l.id));
  const baseCtx = loadAzureContext();
  const parallel = opts.parallelModels ?? opts.models.length > 1;
  const reps = Math.max(1, Math.floor(opts.reps ?? 1));
  const t0 = Date.now();

  // One model's worth of work — sequential over reps × fixtures × paths.
  // Rep is the outer loop so partial runs still give >=1 complete rep per cell.
  const runModel = async (model: string): Promise<GradientRunRecord[]> => {
    // Slug imageName per model so concurrent runs don't `kubectl delete` each
    // other's deployment. Single-model runs keep the default `eval-image`.
    const ctx: AzureContext = opts.models.length > 1
      ? { ...baseCtx, imageName: `${baseCtx.imageName}-${slugifyModel(model)}` }
      : baseCtx;
    console.error(`[gradient] start model=${model} imageName=${ctx.imageName} reps=${reps}`);
    const out: GradientRunRecord[] = [];
    for (let rep = 0; rep < reps; rep++) {
      for (const fixture of opts.fixtures) {
        for (const level of levels) {
          const r = await runOneLevel({ fixture, level, model, ctx, checkSpecs, rep });
          console.error(
            `[gradient] done  model=${model} rep=${rep + 1}/${reps} ` +
              `fixture=${r.fixture.split('/').pop()} ` +
              `path=${r.level} ${r.error ? 'ERROR' : 'ok'} ` +
              `(${r.durationMs ? Math.round(r.durationMs / 1000) + 's' : '?'})`,
          );
          out.push(r);
        }
      }
    }
    console.error(`[gradient] finish model=${model} (${Math.round((Date.now() - t0) / 1000)}s wall so far)`);
    return out;
  };

  const all = parallel
    ? (await Promise.all(opts.models.map(runModel))).flat()
    : (await opts.models.reduce<Promise<GradientRunRecord[]>>(
        async (accP, m) => (await accP).concat(await runModel(m)),
        Promise.resolve([]),
      ));

  return {
    models: opts.models,
    levels,
    timestamp: new Date().toISOString(),
    runs: all,
  };
}

// ---------- Markdown reporting ----------

const fmtNum = (n?: number): string => (n == null ? '—' : Math.round(n).toLocaleString());
const fmtMs = (ms?: number): string => {
  if (ms == null) return '—';
  const s = ms / 1000;
  if (s < 60) return `${s.toFixed(1)}s`;
  const m = Math.floor(s / 60);
  const rest = Math.round(s - m * 60);
  return `${m}m${rest.toString().padStart(2, '0')}s`;
};
const shortFixture = (p: string): string => {
  const ix = p.indexOf('/fixtures/');
  return ix >= 0 ? p.slice(ix + '/fixtures/'.length) : p;
};
const checkSymbol = (passed?: boolean): string => (passed == null ? '—' : passed ? '✅' : '❌');

function fmtDelta(curr?: number, prev?: number): string {
  if (curr == null || prev == null) return '—';
  const d = curr - prev;
  if (d === 0) return '0';
  const sign = d > 0 ? '+' : '−';
  return `${sign}${Math.round(Math.abs(d)).toLocaleString()}`;
}

function fmtCheckDelta(currPassed?: boolean, prevPassed?: boolean): string {
  if (currPassed == null || prevPassed == null) return '—';
  if (currPassed && !prevPassed) return '★ unlocked';
  if (!currPassed && prevPassed) return '✗ regressed';
  return '—';
}

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

function fmtKTokens(n?: number): string {
  if (n == null) return '—';
  const abs = Math.abs(n);
  if (abs < 1000) return String(Math.round(n));
  if (abs < 1_000_000) return `${(n / 1000).toFixed(abs >= 10_000 ? 0 : 1)}K`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}

function fmtDeltaTokens(curr?: number, prev?: number): string {
  if (curr == null || prev == null) return '—';
  const d = curr - prev;
  if (d === 0) return '0';
  const sign = d > 0 ? '+' : '−';
  return `${sign}${fmtKTokens(Math.abs(d))}`;
}

export function formatGradientMarkdown(result: GradientResult): string {
  const lines: string[] = [];
  const allChecks = allCheckNames(result);
  // Headline checks shown per fixture in the per-model table.
  const headlineNames = ['docker-builds', 'requires-azure-base', 'has-required-labels'].filter(
    (n) => allChecks.includes(n),
  );
  const fixtures = Array.from(new Set(result.runs.map((r) => r.fixture)));

  lines.push('# Gradient eval');
  lines.push('');
  lines.push(
    `_models: ${result.models.map((m) => `\`${m}\``).join(', ')} · ` +
      `paths: ${result.levels.map((l) => `**${l.id}**`).join(' · ')} · ` +
      `fixtures: ${fixtures.length}_`,
  );
  lines.push('');
  lines.push('## Path definitions');
  lines.push('');
  lines.push('| path | description |');
  lines.push('| --- | --- |');
  for (const l of result.levels) {
    lines.push(`| **${l.id}** | ${l.description} |`);
  }
  lines.push('');
  lines.push(
    `Three quality summary tables follow — one per path. Rows are models, ` +
      `columns are fixtures, and each fixture cell shows the **${headlineNames.join(
        '** / **',
      )}** result side by side, with a final **passing** tally counting cells where ` +
      `all ${headlineNames.length} checks pass.`,
  );
  lines.push('');

  // ---------- Per-path quality summary tables ----------
  // One table per path. Rows = models. Columns grouped by fixture, sub-cols =
  // the headline checks. Final column = pass tally ("3/3" etc).
  for (const level of result.levels) {
    lines.push(`## Quality (${level.id} path)`);
    lines.push('');

    // Header row 1: fixture group names; pad with blanks to span sub-columns.
    const headerTopParts = ['model'];
    for (const fix of fixtures) {
      headerTopParts.push(fixtureNick(fix));
      for (let i = 1; i < headlineNames.length; i++) headerTopParts.push(' ');
    }
    headerTopParts.push('passing');
    lines.push(`| ${headerTopParts.join(' | ')} |`);
    lines.push(`| ${headerTopParts.map(() => '---').join(' | ')} |`);

    // Header row 2: sub-column labels per fixture
    const headerSubParts = [' '];
    for (const _ of fixtures) {
      for (const name of headlineNames) headerSubParts.push(name);
    }
    headerSubParts.push(' ');
    lines.push(`| ${headerSubParts.join(' | ')} |`);

    // Body: one row per model
    let passCounted = 0;
    for (const model of result.models) {
      const cells: string[] = [`\`${model}\``];
      let cellsPassedAll = 0;
      let cellsTotal = 0;
      for (const fix of fixtures) {
        const run = result.runs.find(
          (r) => r.model === model && r.fixture === fix && r.level === level.id,
        );
        cellsTotal++;
        if (!run) {
          for (const _ of headlineNames) cells.push('—');
          continue;
        }
        if (run.error) {
          for (const _ of headlineNames) cells.push('⚠');
          continue;
        }
        const passed = headlineNames.map((n) => checkPassed(run, n));
        for (const p of passed) cells.push(checkSymbol(p));
        if (passed.every((p) => p === true)) cellsPassedAll++;
      }
      cells.push(`**${cellsPassedAll}/${cellsTotal}**`);
      passCounted += cellsPassedAll;
      lines.push(`| ${cells.join(' | ')} |`);
    }
    lines.push('');
    lines.push(
      `_${passCounted}/${result.models.length * fixtures.length} (model × fixture) ` +
        `cells pass all ${headlineNames.length} checks._`,
    );
    lines.push('');
  }

  // ---------- Cross-model cost summary ----------
  if (result.models.length > 1) {
    for (const [label, key] of [
      ['tokens IN (prompt + tool results)', 'tokensIn'],
      ['tokens OUT (model generations)', 'tokensOut'],
    ] as const) {
      lines.push(`## Cross-model cost: ${label}, summed across fixtures`);
      lines.push('');
      const header = ['path', ...result.models.map((m) => `\`${m}\``)];
      lines.push(`| ${header.join(' | ')} |`);
      lines.push(`| ${header.map(() => '---').join(' | ')} |`);
      for (const level of result.levels) {
        const row = [`**${level.id}**`];
        for (const m of result.models) {
          const total = result.runs
            .filter((r) => r.model === m && r.level === level.id && !r.error)
            .reduce((sum, r) => sum + (r[key] ?? 0), 0);
          row.push(fmtKTokens(total));
        }
        lines.push(`| ${row.join(' | ')} |`);
      }
      lines.push('');
    }
  }

  // ---------- Errors footer (one section, lists all) ----------
  const errored = result.runs.filter((r) => r.error);
  if (errored.length) {
    lines.push('## Errored runs');
    lines.push('');
    for (const r of errored) {
      lines.push(`- \`${r.model}\` / ${fixtureNick(r.fixture)} / **${r.level}**: ${r.error}`);
    }
    lines.push('');
  }

  return lines.join('\n');
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
 * Render a self-contained HTML report. One file, inline CSS + SVG, no deps.
 *
 * Layout, top-to-bottom:
 *   1. Banner — three giant pass/total numbers, one per path.
 *   2. Heatmap — single grid: rows = models, columns = path × fixture.
 *      Each cell is a large saturated square coloured by pass-rate. Reps
 *      are auto-aggregated, so the same view shows 0/100% today and
 *      gradient shades once reps land.
 *   3. Cost-vs-effectiveness scatter — log-scale tokens (x) vs pass rate
 *      (y). One dot per (path × model); quadrants highlight the
 *      cheap-and-effective winner zone.
 *   4. Token bars — compact reference view of absolute token spend.
 *   5. Errors footer (only if any).
 */
export function formatGradientHtml(result: GradientResult): string {
  const allChecks = allCheckNames(result);
  const headlineNames = ['docker-builds', 'requires-azure-base', 'has-required-labels'].filter(
    (n) => allChecks.includes(n),
  );
  const fixtures = Array.from(new Set(result.runs.map((r) => r.fixture)));

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

  // Per-path tally: how many (model × fixture) cells are "perfect" — i.e.
  // every rep passed every check. This is the strict headline number.
  const passingByLevel: Record<LevelId, { pass: number; total: number }> = {
    bare: { pass: 0, total: 0 },
    mcp: { pass: 0, total: 0 },
    skills: { pass: 0, total: 0 },
  };
  for (const level of result.levels) {
    for (const m of result.models) {
      for (const fix of fixtures) {
        const cell = aggregateCell(result.runs, m, level.id, fix, headlineNames);
        passingByLevel[level.id].total++;
        if (cell.fullPassRatio === 1) passingByLevel[level.id].pass++;
      }
    }
  }

  const bannerColour = (p: number, t: number): string =>
    t === 0 ? '#6e7681' : rateColour(p / t);

  // ---------- Big banner ----------
  const bannerCard = (level: LevelConfig): string => {
    const stat = passingByLevel[level.id];
    const subline = repsPerCell === 1
      ? 'cells passing all checks'
      : 'cells where every rep passes every check';
    return `<div class="banner-card">
      <div class="banner-num" style="color:${bannerColour(stat.pass, stat.total)}">${stat.pass}<span class="banner-denom">/${stat.total}</span></div>
      <div class="banner-label">${escapeHtml(level.label)}</div>
      <div class="banner-sub">${escapeHtml(level.id)} · ${subline}</div>
    </div>`;
  };

  // ---------- The Heatmap ----------
  // Rows = models. Columns = path × fixture (grouped, with a thicker
  // separator between paths so the eye reads "this whole block is bare,
  // this whole block is mcp, this block is skills").
  const colCount = result.levels.length * fixtures.length;
  const fixtureLabels = fixtures.map(fixtureNick);

  const heatmapHeaderTop = [
    `<div class="heatmap-corner"></div>`,
    ...result.levels.map(
      (l) =>
        `<div class="heatmap-path-label" style="grid-column: span ${fixtures.length};">${escapeHtml(l.label)}</div>`,
    ),
  ].join('');

  const heatmapHeaderBot = [
    `<div class="heatmap-corner"></div>`,
    ...result.levels.flatMap(() =>
      fixtureLabels.map(
        (f) => `<div class="heatmap-fixture-label">${escapeHtml(f)}</div>`,
      ),
    ),
  ].join('');

  const heatmapRows = result.models
    .map((model) => {
      const cells = result.levels
        .flatMap((level) =>
          fixtures.map((fix) => {
            const cell = aggregateCell(result.runs, model, level.id, fix, headlineNames);
            if (cell.reps === 0) {
              return `<div class="heatmap-cell empty" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: no data`)}"></div>`;
            }
            if (cell.checkRatio === null) {
              // All reps errored before checks could run.
              return `<div class="heatmap-cell errored" title="${escapeHtml(`${model} / ${level.id} / ${fixtureNick(fix)}: ${cell.errored}/${cell.reps} errored`)}"><span class="cell-pct">err</span><span class="cell-sub">${cell.errored}/${cell.reps}</span></div>`;
            }
            const validReps = cell.reps - cell.errored;
            const pct = Math.round(cell.checkRatio * 100);
            // Subtext shows the full-pass rate when reps > 1 so you can see
            // both "% of checks" and "% of perfect runs".
            const sub =
              validReps > 1 && cell.fullPassRatio !== null
                ? `${cell.fullPass}/${validReps} perfect`
                : '';
            const fullPct =
              cell.fullPassRatio === null
                ? '—'
                : `${Math.round(cell.fullPassRatio * 100)}%`;
            const tooltip =
              `${model} / ${level.id} / ${fixtureNick(fix)}\n` +
              `${cell.checksPassed}/${cell.checksTotal} checks passed (${pct}%)\n` +
              `${cell.fullPass}/${validReps} reps with all checks passing (${fullPct})` +
              (cell.errored ? `\n+${cell.errored} rep(s) errored` : '');
            return `<div class="heatmap-cell" style="background:${rateColour(cell.checkRatio)}" title="${escapeHtml(tooltip)}"><span class="cell-pct">${pct}%</span>${sub ? `<span class="cell-sub">${sub}</span>` : ''}</div>`;
          }),
        )
        .join('');
      return `<div class="heatmap-row-label">${escapeHtml(model)}</div>${cells}`;
    })
    .join('');

  const heatmapBlock = `<section class="heatmap-block">
    <p class="heatmap-explainer">
      Each cell = one (model × fixture) combo on the path shown above it.
      <strong>Color &amp; %</strong> = fraction of <em>individual checks</em>
      that passed across ${repsPerCell === 1 ? 'the run' : 'all reps'}.
      With ${headlineNames.length} checks (${headlineNames.map((n) => `<code>${escapeHtml(n)}</code>`).join(' · ')})${repsPerCell === 1 ? '' : ` × ${repsPerCell} reps = ${headlineNames.length * repsPerCell} chances per cell`},
      a partial result like <strong>67%</strong> means “two of three checks pass every time” — you see the gradient, not just pass/fail.
      ${repsPerCell > 1 ? 'Subtext <strong>M/N perfect</strong> shows how many reps were flawless (the strict bar from the banner above).' : ''}
      Hover any cell for the full breakdown.
    </p>
    <div class="heatmap-grid" style="grid-template-columns: minmax(110px, auto) repeat(${colCount}, minmax(0, 1fr));">
      ${heatmapHeaderTop}
      ${heatmapHeaderBot}
      ${heatmapRows}
    </div>
    <div class="heatmap-legend">
      <span>0% checks</span>
      <div class="heatmap-legend-bar"></div>
      <span>100% checks</span>
      <span class="legend-spacer"></span>
      <span class="legend-item"><span class="swatch errored"></span>errored</span>
      <span class="legend-item"><span class="swatch empty"></span>no data</span>
    </div>
  </section>`;

  // ---------- Cost-vs-effectiveness scatter ----------
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

  const pathColours: Record<LevelId, string> = {
    bare: '#f85149',
    mcp: '#d29922',
    skills: '#3fb950',
  };
  const fmtTokens = (n: number): string => {
    if (n < 1000) return String(Math.round(n));
    if (n < 1_000_000) return `${(n / 1000).toFixed(n >= 10000 ? 0 : 1)}K`;
    return `${(n / 1_000_000).toFixed(2)}M`;
  };

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
    const rOf = (tokensOut: number): number =>
      6 + 14 * Math.sqrt(tokensOut / maxOut);

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
        `<text x="${x}" y="${MT + innerH + 18}" class="axis-label" text-anchor="middle">${fmtTokens(Math.pow(10, exp))}</text>`,
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
      const colour = pathColours[p.level.id];
      const title = `${p.level.id} · ${p.model}  —  ${p.checksPassed}/${p.checksTotal} checks passed (${Math.round(p.rate * 100)}%)  ·  ${p.fullPass}/${p.validReps} reps perfect  ·  ${fmtTokens(p.tokensIn)} in  ·  ${fmtTokens(p.tokensOut)} out`;
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
              `<span class="legend-item"><span class="swatch" style="background:${pathColours[l.id]}"></span>${escapeHtml(l.label)}</span>`,
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
        const tokensIn = result.runs
          .filter((r) => r.model === model && r.level === level.id && !r.error)
          .reduce((s, r) => s + (r.tokensIn ?? 0), 0);
        const tokensOut = result.runs
          .filter((r) => r.model === model && r.level === level.id && !r.error)
          .reduce((s, r) => s + (r.tokensOut ?? 0), 0);
        rows.push({ level, model, tokensIn, tokensOut });
      }
    }
    if (rows.length === 0) return '';
    const maxIn = Math.max(...rows.map((r) => r.tokensIn), 1);
    const maxOut = Math.max(...rows.map((r) => r.tokensOut), 1);

    const renderRow = (r: Row): string => {
      const inPct = (r.tokensIn / maxIn) * 100;
      const outPct = (r.tokensOut / maxOut) * 100;
      const colour = pathColours[r.level.id];
      return `<div class="bar-row">
        <div class="bar-label-cell"><span class="bar-path-dot" style="background:${colour}"></span>${escapeHtml(r.level.id)} · <code>${escapeHtml(r.model)}</code></div>
        <div class="bar-track"><div class="bar-fill" style="width:${inPct}%; background:${colour}"></div></div>
        <div class="bar-value">${fmtTokens(r.tokensIn)}</div>
        <div class="bar-track"><div class="bar-fill" style="width:${outPct}%; background:${colour}; opacity:0.6"></div></div>
        <div class="bar-value">${fmtTokens(r.tokensOut)}</div>
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

  /* Banner */
  .banner { display: grid; grid-template-columns: repeat(${result.levels.length}, 1fr); gap: 16px; margin: 8px 0 24px; }
  .banner-card {
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 24px;
    text-align: center;
  }
  .banner-num { font-size: 80px; font-weight: 800; line-height: 1; letter-spacing: -2px; font-variant-numeric: tabular-nums; }
  .banner-denom { font-size: 32px; color: var(--muted); font-weight: 500; }
  .banner-label { font-size: 22px; font-weight: 600; margin-top: 4px; }
  .banner-sub { font-size: 13px; color: var(--muted); font-family: ui-monospace, SFMono-Regular, monospace; }

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
  .heatmap-path-label {
    font-size: 14px;
    font-weight: 700;
    color: var(--text);
    text-align: center;
    padding: 6px 8px 4px;
    border-bottom: 1px solid var(--border);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }
  .heatmap-fixture-label {
    font-size: 11px;
    color: var(--muted);
    text-align: center;
    padding: 4px 4px 8px;
    font-family: ui-monospace, SFMono-Regular, monospace;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
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

  /* Errors */
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

  <div class="banner">${result.levels.map(bannerCard).join('')}</div>

  <h2>Pass-rate heatmap <span class="subtle">— rows: models · columns: path × fixture</span></h2>
  ${heatmapBlock}

  <h2>Cost vs effectiveness</h2>
  ${scatterBlock}

  <h2>Token spend reference</h2>
  ${tokenBars}

  ${errorsBlock}
</body>
</html>`;
}
