import { describe, it, expect } from '@jest/globals';
import { validateComposeSchema } from '@/primitives/validate-compose/schema';

describe('validate-compose schema', () => {
  it('accepts minimal input', () => {
    expect(validateComposeSchema.safeParse({ content: 'version: "3"' }).success).toBe(true);
  });
  it('accepts full context', () => {
    expect(
      validateComposeSchema.safeParse({
        content: 'version: "3"',
        context: { environment: 'production' },
      }).success,
    ).toBe(true);
  });
  it('rejects empty content', () => {
    expect(validateComposeSchema.safeParse({ content: '' }).success).toBe(false);
  });
  it('rejects invalid environment', () => {
    expect(
      validateComposeSchema.safeParse({
        content: 'version: "3"',
        context: { environment: 'qa' as unknown as 'dev' },
      }).success,
    ).toBe(false);
  });
  it('rejects unknown keys in context', () => {
    expect(
      validateComposeSchema.safeParse({
        content: 'version: "3"',
        // @ts-expect-error - testing that strict() rejects unknown keys
        context: { environment: 'dev', typo: true },
      }).success,
    ).toBe(false);
  });
});
