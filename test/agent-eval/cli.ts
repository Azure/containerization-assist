#!/usr/bin/env tsx
/**
 * Agent Evaluation CLI
 * Compare agent success rate and token usage across modes (bare | skills | mcp).
 */

import { promises as fs } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { program } from 'commander';
import { generateText } from 'ai';
import { getModel } from './providers.js';
import { AISDKDriver } from './driver.js';
import { resolveMode, USER_PROMPT, type Mode } from './modes.js';
import { runChecks, selectChecks } from './checks.js';
import {
  runGradient,
  formatGradientHtml,
  discoverFixtures,
  LEVELS,
  type LevelId,
} from './gradient.js';

program
  .name('agent-eval')
  .description('Compare agent success rate and token usage across modes')
  .version('0.0.0');

program
  .command('run')
  .description('Run a single agent evaluation against a fixture')
  .requiredOption('--fixture <path>', 'path to fixture directory')
  .requiredOption('--mode <mode>', 'bare | skills | mcp (baseline accepted as alias for bare)')
  .requiredOption('--model <spec>', 'provider:model, e.g. azure:gpt-4o-mini or foundry:llama-3-3-70b')
  .action(async (opts: { fixture: string; mode: string; model: string }) => {
    const workingDir = await fs.mkdtemp(join(tmpdir(), 'agent-eval-'));
    await fs.cp(opts.fixture, workingDir, { recursive: true });

    const { resolved, cleanup } = await resolveMode({
      mode: opts.mode as Mode,
      workingDir,
    });
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
      console.log(JSON.stringify(result, null, 2));
      console.log('artifacts at:', workingDir);
    } finally {
      await cleanup();
    }
  });

program
  .command('ping')
  .description('Verify model connectivity with a tiny prompt')
  .requiredOption('--model <spec>', 'provider:model, e.g. azure:gpt-4o-mini or foundry:llama-3-3-70b')
  .action(async (opts: { model: string }) => {
    const { model, providerOptions } = getModel(opts.model);
    const result = await generateText({
      model,
      ...(providerOptions ? { providerOptions } : {}),
      prompt: 'Say hi',
    });
    console.log('reply:', result.text);
    console.log('usage:', result.usage);
  });

program
  .command('check')
  .description('Run validation checks against an existing artifact directory')
  .requiredOption('--dir <path>', 'artifact directory produced by `eval run`')
  .option('--checks <names>', "'all', 'none', or comma-separated check names", 'all')
  .action(async (opts: { dir: string; checks: string }) => {
    const checks = selectChecks(opts.checks);
    const results = await runChecks(checks, { artifactDir: opts.dir });
    console.log(JSON.stringify(results, null, 2));
    const failed = results.filter((r) => !r.passed).length;
    if (failed > 0) process.exitCode = 1;
  });

program
  .command('cleanup-namespaces')
  .description(
    'Delete leftover per-lane eval namespaces from a previous fleet sweep. ' +
      'Matches namespace names against a pattern (default: `^eval-` excluding the canonical `eval-ns`). ' +
      'Use --dry-run first to confirm the set.',
  )
  .option('--pattern <regex>', 'JavaScript regex applied to namespace names', '^eval-')
  .option('--keep <names>', 'comma-separated namespaces to preserve', 'eval-ns')
  .option('--dry-run', 'list matching namespaces without deleting', false)
  .action(async (opts: { pattern: string; keep: string; dryRun: boolean }) => {
    let re: RegExp;
    try {
      re = new RegExp(opts.pattern);
    } catch (err) {
      console.error(`Invalid --pattern regex: ${err instanceof Error ? err.message : String(err)}`);
      process.exit(2);
    }
    const keep = new Set(
      opts.keep
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean),
    );
    const { execFile } = await import('node:child_process');
    const { promisify } = await import('node:util');
    const execFileP = promisify(execFile);
    let stdout: string;
    try {
      const res = await execFileP('kubectl', ['get', 'namespace', '-o', 'jsonpath={.items[*].metadata.name}'], {
        timeout: 30_000,
      });
      stdout = res.stdout ?? '';
    } catch (err) {
      console.error(`kubectl failed: ${err instanceof Error ? err.message : String(err)}`);
      process.exit(1);
    }
    const all = stdout.split(/\s+/).filter(Boolean);
    const targets = all.filter((n) => re.test(n) && !keep.has(n));
    if (targets.length === 0) {
      console.log('No matching namespaces found.');
      return;
    }
    console.log(`${opts.dryRun ? 'Would delete' : 'Deleting'} ${targets.length} namespace(s):`);
    for (const n of targets) console.log(`  ${n}`);
    if (opts.dryRun) return;
    for (const n of targets) {
      try {
        await execFileP('kubectl', ['delete', 'namespace', n, '--wait=false', '--ignore-not-found'], {
          timeout: 60_000,
        });
        console.log(`  deleted ${n}`);
      } catch (err) {
        console.error(`  FAILED ${n}: ${err instanceof Error ? err.message : String(err)}`);
      }
    }
  });

program
  .command('gradient')
  .description(
    'Run the same fixture(s) under three independent CA delivery paths ' +
      '(`bare`, `mcp`, `skills`) and report per-path scores plus \u0394 vs the ' +
      '`bare` control. Use for the "value-per-token" brief.',
  )
  .option('--fixtures <paths>', 'comma-separated fixture directories (one run per fixture × model × path)')
  .option('--fixtures-dir <path>', 'directory whose subdirectories are each treated as one fixture')
  .requiredOption('--models <specs>', 'comma-separated provider:model list, e.g. azure:gpt-4.1,azure:gpt-4o,azure:gpt-4.1-mini')
  .option('--checks <names>', "'all', 'none', or comma-separated check names", 'all')
  .option(
    '--paths <ids>',
    `comma-separated subset of paths to run (default: all = ${LEVELS.map((l) => l.id).join(',')})`,
    '',
  )
  .option('--out <path>', 'write JSON results to this file (default: do not write)', '')
  .option('--parallel', 'force models to run in parallel (default: parallel when >1 model)', false)
  .option('--sequential', 'force models to run sequentially (overrides the default parallel-when-multi behavior)', false)
  .option(
    '--max-concurrent-models <n>',
    'cap on how many model lanes run concurrently. Use to stay under provider rate limits.',
    '',
  )
  .option(
    '--resume <path>',
    'resume from an existing checkpoint JSON: skip cells with a prior successful record for the same (model × fixture × path × rep). Pair with --out to keep checkpointing.',
    '',
  )
  .option('--reps <n>', 'repetitions per (model × fixture × path) cell (default: 1)', '1')
  .option('--serve', 'after the run, serve the HTML report over HTTP and print a clickable URL', false)
  .option('--port <n>', 'port for --serve (default: 7878)', '7878')
  .action(
    async (opts: {
      fixtures?: string;
      fixturesDir?: string;
      models: string;
      checks: string;
      paths: string;
      out: string;
      parallel: boolean;
      sequential: boolean;
      maxConcurrentModels: string;
      resume: string;
      reps: string;
      serve: boolean;
      port: string;
    }) => {
      const split = (s: string): string[] =>
        s.split(',').map((x) => x.trim()).filter(Boolean);
      const fail = (msg: string): never => {
        console.error(`Error: ${msg}`);
        process.exit(2);
      };
      if (!opts.fixtures === !opts.fixturesDir) {
        fail('provide exactly one of --fixtures or --fixtures-dir');
      }
      const fixtures = opts.fixturesDir
        ? await discoverFixtures(opts.fixturesDir)
        : split(opts.fixtures!);
      if (fixtures.length === 0) fail('no fixtures found');

      const models = split(opts.models);
      if (models.length === 0) fail('--models must list at least one model');

      const validIds = new Set<LevelId>(LEVELS.map((l) => l.id));
      const levels = opts.paths
        ? (split(opts.paths) as LevelId[])
        : undefined;
      if (levels) {
        for (const id of levels) {
          if (!validIds.has(id)) {
            fail(`unknown path '${id}'. Valid: ${[...validIds].join(', ')}`);
          }
        }
      }

      if (opts.parallel && opts.sequential) fail('--parallel and --sequential are mutually exclusive');
      const parallelModels: boolean | undefined = opts.sequential ? false : opts.parallel ? true : undefined;

      const reps = Number.parseInt(opts.reps, 10);
      if (!Number.isFinite(reps) || reps < 1) fail('--reps must be a positive integer');

      let maxConcurrentModels: number | undefined;
      if (opts.maxConcurrentModels) {
        const n = Number.parseInt(opts.maxConcurrentModels, 10);
        if (!Number.isFinite(n) || n < 1) fail('--max-concurrent-models must be a positive integer');
        maxConcurrentModels = n;
      }

      let resumeRuns: import('./gradient.js').GradientRunRecord[] | undefined;
      if (opts.resume) {
        try {
          const raw = await fs.readFile(opts.resume, 'utf8');
          const parsed = JSON.parse(raw) as { runs?: import('./gradient.js').GradientRunRecord[] };
          resumeRuns = Array.isArray(parsed.runs) ? parsed.runs : [];
          console.error(`[gradient] loaded ${resumeRuns.length} prior run record(s) from ${opts.resume}`);
        } catch (err) {
          fail(`could not read --resume file ${opts.resume}: ${err instanceof Error ? err.message : String(err)}`);
        }
      }

      const result = await runGradient({
        fixtures,
        models,
        checks: opts.checks,
        ...(parallelModels != null ? { parallelModels } : {}),
        reps,
        ...(maxConcurrentModels != null ? { maxConcurrentModels } : {}),
        ...(opts.out ? { checkpointPath: opts.out } : {}),
        ...(levels ? { levels } : {}),
        ...(resumeRuns ? { resumeRuns } : {}),
      });
      const html = formatGradientHtml(result);
      if (opts.out) {
        await fs.writeFile(opts.out, JSON.stringify(result, null, 2), 'utf8');
        console.log(`\nResults written to ${opts.out}`);
        // Companion file next to the JSON: a self-contained HTML heatmap view.
        const htmlPath = `${opts.out.replace(/\.json$/, '')}.html`;
        await fs.writeFile(htmlPath, html, 'utf8');
        console.log(`HTML heatmap view: ${htmlPath}`);
      }
      if (opts.serve) {
        const port = Number.parseInt(opts.port, 10) || 7878;
        // Tiny zero-dependency static server: the self-contained HTML lives in
        // memory, so the run never blocks on a browser. In remote VS Code the
        // printed localhost URL is auto-forwarded and becomes clickable.
        const server = createServer((_req, res) => {
          res.writeHead(200, { 'content-type': 'text/html; charset=utf-8' });
          res.end(html);
        });
        await new Promise<void>((resolveListen) => server.listen(port, resolveListen));
        console.log(`\n\u2728 Gradient report served at http://localhost:${port}/`);
        console.log('   (Ctrl+C to stop the server.)');
        // Keep the process alive until interrupted.
        await new Promise<void>((resolveStop) => {
          process.on('SIGINT', () => {
            server.close(() => resolveStop());
          });
        });
      }
    },
  );

void program.parseAsync(process.argv);
