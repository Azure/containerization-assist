// Matrix runner: cross-product of (model × fixture × mode) with N reps per cell.
// Per-run errors (rate limits, network blips) are captured as failures so a
// transient blip never invalidates the whole run.

import { promises as fs } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { getModel } from './providers.js';
import { AISDKDriver } from './driver.js';
import { resolveMode, USER_PROMPT, type Mode } from './modes.js';
import { runChecks, selectChecks, type CheckResult } from './checks.js';

/** Discover fixtures: each immediate subdirectory of `dir` is one fixture. */
export async function discoverFixtures(dir: string): Promise<string[]> {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  return entries
    .filter((e) => e.isDirectory())
    .map((e) => join(dir, e.name))
    .sort();
}

/** One execution of (fixture, mode). */
export interface RunResult {
  artifactDir?: string;
  tokensIn?: number;
  tokensOut?: number;
  toolCallCount?: number;
  /** Per-tool call counts, e.g. { dockerBuild: 3, createFile: 5 }. */
  toolCallsByName?: Record<string, number>;
  durationMs?: number;
  ok?: boolean;
  error?: string;
  checks: CheckResult[];
}

export interface MatrixCellResult {
  fixture: string;
  mode: Mode;
  runs: RunResult[];
}

export interface MatrixResult {
  model: string;
  timestamp: string;
  reps: number;
  cells: MatrixCellResult[];
}

interface MatrixOptions {
  fixtures: string[];
  modes: Mode[];
  model: string;
  checks: string;
  reps?: number;
}

async function runOne(opts: {
  fixture: string;
  mode: Mode;
  model: string;
  checkSpecs: ReturnType<typeof selectChecks>;
}): Promise<RunResult> {
  const run: RunResult = { checks: [] };
  const workingDir = await fs.mkdtemp(join(tmpdir(), 'agent-eval-'));

  try {
    await fs.cp(opts.fixture, workingDir, { recursive: true });
    const { resolved, cleanup } = await resolveMode({ mode: opts.mode, workingDir });
    try {
      const { model, providerOptions } = getModel(opts.model);
      const result = await new AISDKDriver().run({
        model,
        providerOptions,
        systemPrompt: resolved.systemPrompt,
        userPrompt: resolved.userPrompt ?? USER_PROMPT(workingDir),
        workingDir,
        tools: resolved.tools,
      });
      run.artifactDir = workingDir;
      run.tokensIn = result.tokensIn;
      run.tokensOut = result.tokensOut;
      run.toolCallCount = result.toolCalls.length;
      run.toolCallsByName = result.toolCalls.reduce<Record<string, number>>((acc, c) => {
        acc[c.name] = (acc[c.name] ?? 0) + 1;
        return acc;
      }, {});
      run.durationMs = result.durationMs;
      run.ok = result.ok;
      run.checks = await runChecks(opts.checkSpecs, {
        artifactDir: workingDir,
        fixtureDir: opts.fixture,
      });
    } finally {
      await cleanup();
    }
  } catch (err) {
    run.error = err instanceof Error ? err.message : String(err);
  }

  return run;
}

async function runMatrix(opts: MatrixOptions): Promise<MatrixResult> {
  const reps = Math.max(1, opts.reps ?? 1);
  const cells: MatrixCellResult[] = [];
  const checkSpecs = selectChecks(opts.checks);

  for (const fixture of opts.fixtures) {
    for (const mode of opts.modes) {
      const runs: RunResult[] = [];
      for (let i = 0; i < reps; i++) {
        runs.push(await runOne({ fixture, mode, model: opts.model, checkSpecs }));
      }
      cells.push({ fixture, mode, runs });
    }
  }

  return {
    model: opts.model,
    timestamp: new Date().toISOString(),
    reps,
    cells,
  };
}

// ---------- Multi-model matrix (3D: model × fixture × mode) ----------

export interface MultiModelMatrixOptions {
  models: string[];
  fixtures: string[];
  modes: Mode[];
  checks: string;
  reps?: number;
}

export interface MultiModelMatrixResult {
  models: string[];
  reps: number;
  timestamp: string;
  perModel: MatrixResult[];
}

/**
 * Run the full 3D matrix sequentially across all models. Models are the OUTER
 * loop so a single model's quota / rate limit is fully spent before moving on.
 */
export async function runMultiModelMatrix(
  opts: MultiModelMatrixOptions,
): Promise<MultiModelMatrixResult> {
  const reps = Math.max(1, opts.reps ?? 1);
  const perModel: MatrixResult[] = [];
  for (const model of opts.models) {
    perModel.push(
      await runMatrix({
        model,
        fixtures: opts.fixtures,
        modes: opts.modes,
        checks: opts.checks,
        reps,
      }),
    );
  }
  return {
    models: opts.models,
    reps,
    timestamp: new Date().toISOString(),
    perModel,
  };
}

// ---------- Display helpers ----------

const fmtNum = (n?: number): string => (n == null ? '—' : Math.round(n).toLocaleString());
const shortFixture = (p: string): string => {
  const ix = p.indexOf('/fixtures/');
  return ix >= 0 ? p.slice(ix + '/fixtures/'.length) : p;
};
const shortModel = (spec: string): string => {
  const ix = spec.indexOf(':');
  return ix < 0 ? spec : spec.slice(ix + 1);
};

/** Average a numeric field across runs, ignoring undefined and errored runs. */
function avg(runs: RunResult[], pick: (r: RunResult) => number | undefined): number | undefined {
  const values = runs
    .filter((r) => !r.error)
    .map(pick)
    .filter((v): v is number => v != null);
  if (values.length === 0) return undefined;
  return values.reduce((a, b) => a + b, 0) / values.length;
}

/** Per-check pass count across runs of one cell. */
function checkPassesInCell(cell: MatrixCellResult, checkName: string): { passes: number; total: number } {
  let passes = 0;
  let total = 0;
  for (const run of cell.runs) {
    if (run.error) {
      total += 1;
      continue;
    }
    const r = run.checks.find((c) => c.name === checkName);
    if (r) {
      total += 1;
      if (r.passed) passes += 1;
    }
  }
  return { passes, total };
}

/**
 * Render the multi-model report: one section per fixture with two compact
 * tables (per-check pass rate, avg input tokens), columns = model × mode.
 * Errored runs are listed in a footer.
 */
export function formatMultiModelMarkdown(result: MultiModelMatrixResult): string {
  const lines: string[] = [];
  const allModes = Array.from(
    new Set(result.perModel.flatMap((r) => r.cells.map((c) => c.mode))),
  );
  const allFixtures = Array.from(
    new Set(result.perModel.flatMap((r) => r.cells.map((c) => c.fixture))),
  );
  const allChecks = Array.from(
    new Set(
      result.perModel.flatMap((r) =>
        r.cells.flatMap((c) => c.runs.flatMap((run) => run.checks.map((chk) => chk.name))),
      ),
    ),
  );

  lines.push(
    `_models: ${result.models.map((m) => `\`${m}\``).join(', ')} · ` +
      `fixtures: ${allFixtures.length} · modes: ${allModes.length} · ` +
      `reps: ${result.reps}_`,
  );
  lines.push('');

  // Each column is one (model, mode) cell.
  const columns: Array<{ model: string; mode: Mode; result: MatrixResult }> = [];
  for (const r of result.perModel) {
    for (const mode of allModes) {
      columns.push({ model: r.model, mode, result: r });
    }
  }
  const colHeader = columns.map((c) => `${shortModel(c.model)} · ${c.mode}`).join(' | ');
  const colDivider = columns.map(() => '---:').join(' | ');

  const passRateForCell = (
    r: MatrixResult,
    fixture: string,
    mode: Mode,
    checkName: string,
  ): string => {
    const cell = r.cells.find((c) => c.fixture === fixture && c.mode === mode);
    if (!cell) return '—';
    const { passes, total } = checkPassesInCell(cell, checkName);
    if (total === 0) return '—';
    if (total === 1) return passes === 1 ? '✅' : '❌';
    return `${passes}/${total}`;
  };

  const avgTokensInForCell = (r: MatrixResult, fixture: string, mode: Mode): string => {
    const cell = r.cells.find((c) => c.fixture === fixture && c.mode === mode);
    if (!cell) return '—';
    return fmtNum(avg(cell.runs, (run) => run.tokensIn));
  };

  for (const fixture of allFixtures) {
    lines.push(`## ${shortFixture(fixture)}`);
    lines.push('');

    lines.push('### Per-check pass rate');
    lines.push('');
    lines.push(`| check | ${colHeader} |`);
    lines.push(`| --- | ${colDivider} |`);
    for (const checkName of allChecks) {
      const cells = columns.map((col) => passRateForCell(col.result, fixture, col.mode, checkName));
      lines.push(`| \`${checkName}\` | ${cells.join(' | ')} |`);
    }
    lines.push('');

    lines.push('### Avg input tokens');
    lines.push('');
    lines.push(`| | ${colHeader} |`);
    lines.push(`| --- | ${colDivider} |`);
    const tokenCells = columns.map((col) => avgTokensInForCell(col.result, fixture, col.mode));
    lines.push(`| input tokens | ${tokenCells.join(' | ')} |`);
    lines.push('');
  }

  // Footer: any runs that errored, with their messages.
  const errored: Array<{ model: string; fixture: string; mode: Mode; rep: number; error: string }> = [];
  for (const r of result.perModel) {
    for (const cell of r.cells) {
      cell.runs.forEach((run, i) => {
        if (run.error) {
          errored.push({ model: r.model, fixture: cell.fixture, mode: cell.mode, rep: i + 1, error: run.error });
        }
      });
    }
  }
  if (errored.length > 0) {
    lines.push('### Errors');
    lines.push('');
    for (const e of errored) {
      lines.push(`- \`${shortModel(e.model)}\` / \`${shortFixture(e.fixture)}\` / \`${e.mode}\` rep ${e.rep}: ${e.error}`);
    }
    lines.push('');
  }

  return lines.join('\n').replace(/\n{3,}/g, '\n\n').trimEnd();
}
