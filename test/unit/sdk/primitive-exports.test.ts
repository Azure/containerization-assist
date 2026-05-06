import { describe, it, expect } from '@jest/globals';
import { queryKnowledge, validate } from '@/sdk';

describe('SDK primitive exports', () => {
  it.each([
    ['queryKnowledge', queryKnowledge],
    ['validate', validate],
  ])('%s is callable', (_, fn) => {
    expect(typeof fn).toBe('function');
  });

  it('queryKnowledge returns an empty Success for unknown tags', async () => {
    const r = await queryKnowledge({ tags: ['nonexistent-tag-zzz'], limit: 1 });
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.value.matches).toEqual([]);
    expect(r.value.totalMatched).toBe(0);
  });

  it.each(['dockerfile', 'k8s-manifest', 'compose'] as const)(
    'validate returns a passing envelope without policy (kind=%s)',
    async (kind) => {
      const r = await validate({ kind, content: 'sample content' });
      expect(r.ok).toBe(true);
      if (!r.ok) return;
      expect(r.value.passed).toBe(true);
    },
  );
});

describe('SDK tools registry', () => {
  it('exposes both primitives in the tools registry', async () => {
    const { tools } = await import('@/sdk');
    expect(tools.queryKnowledge).toBeDefined();
    expect(tools.validate).toBeDefined();
    expect(tools.queryKnowledge.name).toBe('query-knowledge');
    expect(tools.validate.name).toBe('validate');
  });
});
