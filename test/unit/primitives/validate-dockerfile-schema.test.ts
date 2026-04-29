import { describe, it, expect } from '@jest/globals';
import { validateDockerfileSchema } from '@/primitives/validate-dockerfile/schema';

describe('validate-dockerfile schema', () => {
  it('accepts minimal input', () => {
    expect(validateDockerfileSchema.safeParse({ content: 'FROM node:20' }).success).toBe(true);
  });

  it('rejects empty content', () => {
    expect(validateDockerfileSchema.safeParse({ content: '' }).success).toBe(false);
  });

  it('rejects missing content', () => {
    expect(validateDockerfileSchema.safeParse({}).success).toBe(false);
  });
});
