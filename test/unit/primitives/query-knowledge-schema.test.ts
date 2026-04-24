import { describe, it, expect } from '@jest/globals';
import { queryKnowledgeSchema } from '@/primitives/query-knowledge/schema';

describe('query-knowledge schema', () => {
  it('accepts a valid input', () => {
    const result = queryKnowledgeSchema.safeParse({ tags: ['node'], limit: 5 });
    expect(result.success).toBe(true);
  });

  it('applies the default limit of 10', () => {
    const result = queryKnowledgeSchema.safeParse({ tags: ['node'] });
    expect(result.success).toBe(true);
    if (!result.success) return;
    expect(result.data.limit).toBe(10);
  });

  it('rejects an empty tags array', () => {
    const result = queryKnowledgeSchema.safeParse({ tags: [] });
    expect(result.success).toBe(false);
  });

  it('rejects a limit above 100', () => {
    const result = queryKnowledgeSchema.safeParse({ tags: ['x'], limit: 101 });
    expect(result.success).toBe(false);
  });

  it('rejects a non-positive limit', () => {
    const result = queryKnowledgeSchema.safeParse({ tags: ['x'], limit: 0 });
    expect(result.success).toBe(false);
  });
});
