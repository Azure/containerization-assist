#!/usr/bin/env tsx
/**
 * Dev-only preview harness for the gradient HTML report. Generates a
 * deterministic synthetic GradientResult (no Azure, no model calls) so we can
 * iterate on the visual design fast.
 *
 *   npx tsx test/agent-eval/preview.ts [outPath]
 *
 * Default outPath: /tmp/gradient-preview.html
 */

import { promises as fs } from 'node:fs';
import { formatGradientHtml, LEVELS, type GradientResult, type GradientRunRecord } from './gradient.js';
import type { CheckResult } from './checks.js';

const MODELS = ['azure:gpt-4.1', 'azure:gpt-4o', 'azure:gpt-4.1-mini'];
const FIXTURES = [
  '/repo/test/fixtures/legacy-java/spring-boot-rest-api',
  '/repo/test/fixtures/legacy-java/coolstore',
  '/repo/test/fixtures/legacy-java/spring-mvc-war',
];
const CHECKS = ['docker-builds', 'requires-azure-base', 'has-required-labels'];
const REPS = 3;

// Deterministic PRNG so previews are stable across runs.
function mulberry32(seed: number): () => number {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rand = mulberry32(42);

// Base pass probability per (level, check). Encodes the headline story:
// bare struggles, mcp helps, skills nearly always passes.
const PASS_PROB: Record<string, Record<string, number>> = {
  bare: { 'docker-builds': 0.55, 'requires-azure-base': 0.1, 'has-required-labels': 0.05 },
  mcp: { 'docker-builds': 0.85, 'requires-azure-base': 0.8, 'has-required-labels': 0.75 },
  skills: { 'docker-builds': 0.95, 'requires-azure-base': 0.97, 'has-required-labels': 0.9 },
};

// Token cost grows with the richer context paths.
const TOKENS_IN: Record<string, number> = { bare: 3500, mcp: 22000, skills: 38000 };
const TOKENS_OUT: Record<string, number> = { bare: 2200, mcp: 5200, skills: 6100 };

// Model quality modifier (mini is weaker, 4.1 strongest).
const MODEL_MOD: Record<string, number> = {
  'azure:gpt-4.1': 0.08,
  'azure:gpt-4o': 0.0,
  'azure:gpt-4.1-mini': -0.18,
};

const runs: GradientRunRecord[] = [];
for (const model of MODELS) {
  for (const fixture of FIXTURES) {
    for (const level of LEVELS) {
      for (let rep = 0; rep < REPS; rep++) {
        const checks: CheckResult[] = CHECKS.map((name) => {
          const base = PASS_PROB[level.id]?.[name] ?? 0.5;
          const p = Math.max(0, Math.min(1, base + (MODEL_MOD[model] ?? 0)));
          const passed = rand() < p;
          return {
            name,
            passed,
            message: passed ? 'ok' : 'failed',
          };
        });
        const jitter = 0.85 + rand() * 0.3;
        runs.push({
          fixture,
          model,
          level: level.id,
          label: level.label,
          rep,
          tokensIn: Math.round((TOKENS_IN[level.id] ?? 5000) * jitter),
          tokensOut: Math.round((TOKENS_OUT[level.id] ?? 3000) * jitter),
          toolCallCount: Math.round(rand() * 12),
          durationMs: Math.round((20000 + rand() * 60000) * (level.id === 'bare' ? 0.6 : 1)),
          checks,
          finalText: 'synthetic run',
        });
      }
    }
  }
}

// Inject one errored cell so the error styling is exercised.
runs.push({
  fixture: FIXTURES[1],
  model: MODELS[2],
  level: 'mcp',
  label: 'CA MCP',
  rep: 0,
  checks: [],
  error: 'kubectl rollout timed out after 300s (synthetic)',
});

const result: GradientResult = {
  models: MODELS,
  levels: LEVELS,
  timestamp: new Date('2026-06-21T12:00:00Z').toISOString(),
  runs,
};

const out = process.argv[2] ?? '/tmp/gradient-preview.html';
await fs.writeFile(out, formatGradientHtml(result), 'utf8');
console.log(`wrote ${out}`);
