import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateCompose from '@/primitives/validate-compose';
import { createToolContext } from '@/core/context';

import { makeMockEvaluator } from './_helpers';

const silentLogger = createLogger({ level: 'silent' });

const COMPOSE = [
  'version: "3.9"',
  'services:',
  '  web:',
  '    image: myapp:1.0',
  '    ports:',
  '      - "8080:8080"',
].join('\n');

describe('validate-compose primitive', () => {
  it('returns pass+empty envelope when no policy is loaded', async () => {
    const ctx = createToolContext(silentLogger);
    expect(ctx.policy).toBeUndefined();
    const result = await validateCompose.handler({ content: COMPOSE }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value).toEqual({ passed: true, violations: [], warnings: [], suggestions: [] });
  });

  it('exposes the Tool interface', () => {
    expect(validateCompose.name).toBe('validate-compose');
    expect(typeof validateCompose.description).toBe('string');
    expect(validateCompose.schema).toBeDefined();
    expect(validateCompose.inputSchema).toBeDefined();
    expect(typeof validateCompose.handler).toBe('function');
    expect(typeof validateCompose.parse).toBe('function');
    expect(validateCompose.metadata).toBeDefined();
  });
});

describe('validate-compose primitive — policy branch', () => {
  it('returns violations/warnings/suggestions when policy reports findings', async () => {
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

    const result = await validateCompose.handler({ content: COMPOSE }, ctx);
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

  it('passes when policy reports zero violations', async () => {
    const evaluator = makeMockEvaluator({
      allow: true,
      violations: [],
      warnings: [],
      suggestions: [],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateCompose.handler({ content: COMPOSE }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(true);
  });

  it('returns Failure when the evaluator throws', async () => {
    const evaluator = makeMockEvaluator(async () => {
      throw new Error('evaluator boom');
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateCompose.handler({ content: COMPOSE }, ctx);
    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error).toMatch(
      /validate-compose failed while evaluating policy: evaluator boom/,
    );
  });
});
