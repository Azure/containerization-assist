#!/usr/bin/env tsx
/**
 * Agent Evaluation CLI
 * Compare agent success rate and token usage across modes (baseline | skills | mcp).
 */

import { promises as fs } from 'node:fs';
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
  formatGradientMarkdown,
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
  .requiredOption('--mode <mode>', 'baseline | skills | mcp')
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
  .option('--fixture <path>', 'original fixture directory (for checks that compare against source)', '')
  .option('--checks <names>', "'all', 'none', or comma-separated check names", 'all')
  .action(async (opts: { dir: string; fixture: string; checks: string }) => {
    const checks = selectChecks(opts.checks);
    const results = await runChecks(checks, {
      artifactDir: opts.dir,
      fixtureDir: opts.fixture,
    });
    console.log(JSON.stringify(results, null, 2));
    const failed = results.filter((r) => !r.passed).length;
    if (failed > 0) process.exitCode = 1;
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
  .option('--parallel', 'run models in parallel (default: sequential, safer on small clusters)', false)
  .option('--reps <n>', 'repetitions per (model × fixture × path) cell (default: 1)', '1')
  .action(
    async (opts: {
      fixtures?: string;
      fixturesDir?: string;
      models: string;
      checks: string;
      paths: string;
      out: string;
      parallel: boolean;
      reps: string;
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

      const reps = Number.parseInt(opts.reps, 10);
      if (!Number.isFinite(reps) || reps < 1) fail('--reps must be a positive integer');

      const result = await runGradient({
        fixtures,
        models,
        checks: opts.checks,
        parallelModels: opts.parallel,
        reps,
        ...(levels ? { levels } : {}),
      });
      console.log(formatGradientMarkdown(result));
      if (opts.out) {
        await fs.writeFile(opts.out, JSON.stringify(result, null, 2), 'utf8');
        console.log(`\nResults written to ${opts.out}`);
        // Companion files next to the JSON: a markdown report and a
        // self-contained HTML heatmap-style view.
        const base = opts.out.replace(/\.json$/, '');
        const mdPath = `${base}.md`;
        const htmlPath = `${base}.html`;
        await fs.writeFile(mdPath, formatGradientMarkdown(result), 'utf8');
        await fs.writeFile(htmlPath, formatGradientHtml(result), 'utf8');
        console.log(`Markdown report:   ${mdPath}`);
        console.log(`HTML heatmap view: ${htmlPath}`);
      }
    },
  );

void program.parseAsync(process.argv);
