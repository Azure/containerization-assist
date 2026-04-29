import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateK8sManifest from '@/primitives/validate-k8s-manifest';
import { createToolContext } from '@/core/context';

import { makeMockEvaluator } from './_helpers';

const silentLogger = createLogger({ level: 'silent' });

const MANIFEST = [
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
].join('\n');

describe('validate-k8s-manifest primitive', () => {
  it('returns pass+empty envelope when no policy is loaded', async () => {
    const ctx = createToolContext(silentLogger);
    expect(ctx.policy).toBeUndefined();
    const result = await validateK8sManifest.handler({ content: MANIFEST }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value).toEqual({ passed: true, violations: [], warnings: [], suggestions: [] });
  });

  it('exposes the Tool interface', () => {
    expect(validateK8sManifest.name).toBe('validate-k8s-manifest');
    expect(typeof validateK8sManifest.description).toBe('string');
    expect(validateK8sManifest.schema).toBeDefined();
    expect(validateK8sManifest.inputSchema).toBeDefined();
    expect(typeof validateK8sManifest.handler).toBe('function');
    expect(typeof validateK8sManifest.parse).toBe('function');
    expect(validateK8sManifest.metadata).toBeDefined();
  });
});

describe('validate-k8s-manifest primitive — policy branch', () => {
  it('returns violations/warnings/suggestions when policy reports findings', async () => {
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

    const result = await validateK8sManifest.handler({ content: MANIFEST }, ctx);
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

  it('passes when policy reports zero violations', async () => {
    const evaluator = makeMockEvaluator({
      allow: true,
      violations: [],
      warnings: [],
      suggestions: [],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateK8sManifest.handler({ content: MANIFEST }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(true);
  });

  it('returns Failure when the evaluator throws', async () => {
    const evaluator = makeMockEvaluator(async () => {
      throw new Error('rego compile error');
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateK8sManifest.handler({ content: MANIFEST }, ctx);
    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error).toMatch(
      /validate-k8s-manifest failed while evaluating policy: rego compile error/,
    );
  });
});
