import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateDockerfile from '@/primitives/validate-dockerfile';
import { createToolContext } from '@/core/context';

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
