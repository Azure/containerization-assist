import { describe, it, expect } from '@jest/globals';
import { validateComposeSchema } from '@/primitives/validate-compose/schema';

describe('validate-compose schema', () => {
  it('accepts minimal input', () => {
    expect(validateComposeSchema.safeParse({ content: 'version: "3"' }).success).toBe(true);
  });

  it('rejects empty content', () => {
    expect(validateComposeSchema.safeParse({ content: '' }).success).toBe(false);
  });

  it('rejects missing content', () => {
    expect(validateComposeSchema.safeParse({}).success).toBe(false);
  });
});
