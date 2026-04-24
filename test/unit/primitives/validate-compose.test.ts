import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateCompose from '@/primitives/validate-compose';
import { createToolContext } from '@/core/context';

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
