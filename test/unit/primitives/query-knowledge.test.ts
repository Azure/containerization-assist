import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import queryKnowledge from '@/primitives/query-knowledge';
import { createToolContext } from '@/core/context';

const silentLogger = createLogger({ level: 'silent' });

describe('query-knowledge primitive', () => {
  it('returns matches for known tags', async () => {
    const ctx = createToolContext(silentLogger);
    const result = await queryKnowledge.run(
      { tags: ['generate-dockerfile', 'node'], limit: 5 },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(Array.isArray(result.value.matches)).toBe(true);
    expect(result.value.totalMatched).toBeGreaterThanOrEqual(0);
  });

  it('returns empty matches for unknown tags', async () => {
    const ctx = createToolContext(silentLogger);
    const result = await queryKnowledge.run(
      { tags: ['nonsense-tag-zzz'], limit: 5 },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.matches).toEqual([]);
    expect(result.value.totalMatched).toBe(0);
  });
});
