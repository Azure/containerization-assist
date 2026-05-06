import { describe, it, expect } from '@jest/globals';
import { validateSchema } from '@/primitives/validate/schema';

describe('validate schema', () => {
  it.each(['dockerfile', 'k8s-manifest', 'compose'] as const)(
    'accepts minimal input for kind=%s',
    (kind) => {
      expect(validateSchema.safeParse({ kind, content: 'sample content' }).success).toBe(true);
    },
  );

  it('rejects unknown kind', () => {
    expect(
      validateSchema.safeParse({ kind: 'helm-chart', content: 'sample' }).success,
    ).toBe(false);
  });

  it('rejects missing kind', () => {
    expect(validateSchema.safeParse({ content: 'sample' }).success).toBe(false);
  });

  it('rejects empty content', () => {
    expect(validateSchema.safeParse({ kind: 'dockerfile', content: '' }).success).toBe(false);
  });

  it('rejects missing content', () => {
    expect(validateSchema.safeParse({ kind: 'dockerfile' }).success).toBe(false);
  });
});
