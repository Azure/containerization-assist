import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validate from '@/primitives/validate';
import { createToolContext } from '@/core/context';

import { makeMockEvaluator } from './_helpers';

const silentLogger = createLogger({ level: 'silent' });

const SAMPLES = {
  dockerfile: ['FROM node:20', 'COPY . /app', 'CMD ["node", "index.js"]'].join('\n'),
  'k8s-manifest': [
    'apiVersion: apps/v1',
    'kind: Deployment',
    'metadata:',
    '  name: demo',
    'spec:',
    '  replicas: 1',
    '  selector: { matchLabels: { app: demo } }',
    '  template:',
    '    metadata: { labels: { app: demo } }',
    '    spec:',
    '      containers: [{ name: app, image: myapp:1.0 }]',
  ].join('\n'),
  compose: [
    'version: "3.9"',
    'services:',
    '  web:',
    '    image: myapp:1.0',
    '    ports:',
    '      - "8080:8080"',
  ].join('\n'),
} as const;

const KINDS = ['dockerfile', 'k8s-manifest', 'compose'] as const;

describe('validate primitive', () => {
  it('exposes the Tool interface', () => {
    expect(validate.name).toBe('validate');
    expect(typeof validate.description).toBe('string');
    expect(validate.schema).toBeDefined();
    expect(validate.inputSchema).toBeDefined();
    expect(typeof validate.handler).toBe('function');
    expect(typeof validate.parse).toBe('function');
    expect(validate.metadata).toBeDefined();
  });

  it.each(KINDS)('returns pass+empty envelope when no policy is loaded (kind=%s)', async (kind) => {
    const ctx = createToolContext(silentLogger);
    expect(ctx.policy).toBeUndefined();
    const result = await validate.handler({ kind, content: SAMPLES[kind] }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value).toEqual({ passed: true, violations: [], warnings: [], suggestions: [] });
  });
});

describe('validate primitive — policy branch', () => {
  it('returns violations/warnings/suggestions when policy reports findings (dockerfile)', async () => {
    const evaluator = makeMockEvaluator({
      allow: false,
      violations: [
        {
          rule: 'no-root',
          message: 'Must use non-root USER',
          severity: 'block',
          category: 'security',
          priority: 100,
          description: 'Add USER directive',
        },
      ],
      warnings: [
        {
          rule: 'pin-base',
          message: 'Pin base image digest',
          severity: 'warn',
          category: 'best-practices',
        },
      ],
      suggestions: [
        {
          rule: 'multi-stage',
          message: 'Consider multi-stage build',
          severity: 'suggest',
          category: 'optimization',
        },
      ],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validate.handler(
      { kind: 'dockerfile', content: SAMPLES.dockerfile },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;

    expect(result.value.passed).toBe(false);
    expect(result.value.violations).toEqual([
      {
        rule: 'no-root',
        severity: 'block',
        message: 'Must use non-root USER',
        category: 'security',
        priority: 100,
        hint: 'Add USER directive',
      },
    ]);
    expect(result.value.warnings).toEqual([
      {
        rule: 'pin-base',
        severity: 'warn',
        message: 'Pin base image digest',
        category: 'best-practices',
      },
    ]);
    expect(result.value.suggestions).toEqual([
      {
        rule: 'multi-stage',
        severity: 'suggest',
        message: 'Consider multi-stage build',
        category: 'optimization',
      },
    ]);
  });

  it('returns violations/warnings/suggestions when policy reports findings (k8s-manifest)', async () => {
    const evaluator = makeMockEvaluator({
      allow: false,
      violations: [
        {
          rule: 'require-resource-limits',
          message: 'Container must declare resource limits',
          severity: 'block',
          category: 'reliability',
          priority: 90,
        },
      ],
      warnings: [
        {
          rule: 'recommend-readiness-probe',
          message: 'Add a readinessProbe to surface readiness',
          severity: 'warn',
          category: 'best-practices',
        },
      ],
      suggestions: [],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validate.handler(
      { kind: 'k8s-manifest', content: SAMPLES['k8s-manifest'] },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(false);
    expect(result.value.violations).toHaveLength(1);
    expect(result.value.violations[0]).toMatchObject({
      rule: 'require-resource-limits',
      severity: 'block',
      category: 'reliability',
      priority: 90,
    });
    expect(result.value.warnings).toHaveLength(1);
    expect(result.value.suggestions).toEqual([]);
  });

  it('returns violations/warnings/suggestions when policy reports findings (compose)', async () => {
    const evaluator = makeMockEvaluator({
      allow: false,
      violations: [
        {
          rule: 'no-host-network',
          message: 'host network mode is not allowed',
          severity: 'block',
          category: 'security',
        },
      ],
      warnings: [],
      suggestions: [
        {
          rule: 'consider-named-network',
          message: 'Use a named network instead of the default bridge',
          severity: 'suggest',
          category: 'best-practices',
        },
      ],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validate.handler(
      { kind: 'compose', content: SAMPLES.compose },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(false);
    expect(result.value.violations).toHaveLength(1);
    expect(result.value.violations[0]).toMatchObject({
      rule: 'no-host-network',
      severity: 'block',
      category: 'security',
    });
    expect(result.value.suggestions).toHaveLength(1);
  });

  it.each(KINDS)('passes when policy reports zero violations (kind=%s)', async (kind) => {
    const evaluator = makeMockEvaluator({
      allow: true,
      violations: [],
      warnings: [],
      suggestions: [],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validate.handler({ kind, content: SAMPLES[kind] }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(true);
  });

  it.each(KINDS)('returns Failure when the evaluator throws (kind=%s)', async (kind) => {
    const evaluator = makeMockEvaluator(async () => {
      throw new Error(`opa boom for ${kind}`);
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validate.handler({ kind, content: SAMPLES[kind] }, ctx);
    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error).toMatch(
      new RegExp(`validate failed while evaluating policy: opa boom for ${kind}`),
    );
  });
});
