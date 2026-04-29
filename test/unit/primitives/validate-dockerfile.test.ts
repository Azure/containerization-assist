import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateDockerfile from '@/primitives/validate-dockerfile';
import { createToolContext } from '@/core/context';

import { makeMockEvaluator } from './_helpers';

const silentLogger = createLogger({ level: 'silent' });

describe('validate-dockerfile primitive', () => {
  const ROOT_DF = ['FROM node:20', 'COPY . /app', 'CMD ["node", "index.js"]'].join('\n');

  it('returns pass+empty envelope when no policy is loaded', async () => {
    const ctx = createToolContext(silentLogger);
    expect(ctx.policy).toBeUndefined();
    const result = await validateDockerfile.handler({ content: ROOT_DF }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value).toEqual({ passed: true, violations: [], warnings: [], suggestions: [] });
  });

  it('exposes the Tool interface', () => {
    expect(validateDockerfile.name).toBe('validate-dockerfile');
    expect(typeof validateDockerfile.description).toBe('string');
    expect(validateDockerfile.schema).toBeDefined();
    expect(validateDockerfile.inputSchema).toBeDefined();
    expect(typeof validateDockerfile.handler).toBe('function');
    expect(typeof validateDockerfile.parse).toBe('function');
    expect(validateDockerfile.metadata).toBeDefined();
  });
});

describe('validate-dockerfile primitive — policy branch', () => {
  it('returns violations/warnings/suggestions when policy reports findings', async () => {
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

    const result = await validateDockerfile.handler({ content: 'FROM node:20' }, ctx);
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

  it('passes when policy reports zero violations (warnings/suggestions OK)', async () => {
    const evaluator = makeMockEvaluator({
      allow: true,
      violations: [],
      warnings: [
        {
          rule: 'pin-base',
          message: 'Pin base image digest',
          severity: 'warn',
          category: 'best-practices',
        },
      ],
      suggestions: [],
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateDockerfile.handler({ content: 'FROM node:20' }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.passed).toBe(true);
    expect(result.value.warnings).toHaveLength(1);
  });

  it('returns Failure when the evaluator throws', async () => {
    const evaluator = makeMockEvaluator(async () => {
      throw new Error('opa wasm panic');
    });
    const ctx = createToolContext(silentLogger, { policy: evaluator });

    const result = await validateDockerfile.handler({ content: 'FROM node:20' }, ctx);
    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.error).toMatch(/validate-dockerfile failed while evaluating policy: opa wasm panic/);
  });
});
