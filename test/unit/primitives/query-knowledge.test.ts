import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import queryKnowledge from '@/primitives/query-knowledge';
import { createToolContext } from '@/core/context';

const silentLogger = createLogger({ level: 'silent' });

describe('query-knowledge primitive', () => {
  it('returns matches for known tags', async () => {
    const ctx = createToolContext(silentLogger);
    const result = await queryKnowledge.handler(
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
    const result = await queryKnowledge.handler(
      { tags: ['nonsense-tag-zzz'], limit: 5 },
      ctx,
    );
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.matches).toEqual([]);
    expect(result.value.totalMatched).toBe(0);
  });

  it('exposes the required Tool interface fields', () => {
    expect(queryKnowledge.name).toBe('query-knowledge');
    expect(typeof queryKnowledge.description).toBe('string');
    expect(queryKnowledge.schema).toBeDefined();
    expect(queryKnowledge.inputSchema).toBeDefined();
    expect(typeof queryKnowledge.handler).toBe('function');
    expect(typeof queryKnowledge.parse).toBe('function');
    expect(queryKnowledge.metadata).toBeDefined();
  });

  // Regression: phantom severity-only hits ranked above real matches must not
  // consume the caller's limit budget. Filtering happens before slicing, so
  // asking for limit:N returns up to N *real* matches.
  it('returns up to `limit` real matches even when phantom hits would have crowded them out', async () => {
    const ctx = createToolContext(silentLogger);
    const tightResult = await queryKnowledge.handler(
      { tags: ['generate-dockerfile', 'node'], limit: 1 },
      ctx,
    );
    expect(tightResult.ok).toBe(true);
    if (!tightResult.ok) return;

    // The handler must respect the caller's limit.
    expect(tightResult.value.matches.length).toBeLessThanOrEqual(1);
    expect(tightResult.value.totalMatched).toBe(tightResult.value.matches.length);

    // If wider request returns at least one real match, the tight request
    // (limit:1) must also return one — otherwise the filter is dropping
    // matches that should fit within the caller's budget.
    const wideResult = await queryKnowledge.handler(
      { tags: ['generate-dockerfile', 'node'], limit: 50 },
      ctx,
    );
    if (wideResult.ok && wideResult.value.matches.length > 0) {
      expect(tightResult.value.matches.length).toBe(1);
    }
  });
});
