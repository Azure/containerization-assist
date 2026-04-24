import { describe, it, expect } from '@jest/globals';
import { validateDockerfileSchema } from '@/primitives/validate-dockerfile/schema';

describe('validate-dockerfile schema', () => {
  it('accepts minimal input', () => {
    expect(validateDockerfileSchema.safeParse({ content: 'FROM node:20' }).success).toBe(true);
  });
  it('accepts full context', () => {
    expect(
      validateDockerfileSchema.safeParse({
        content: 'FROM node:20',
        context: { environment: 'production', language: 'javascript' },
      }).success,
    ).toBe(true);
  });
  it('rejects empty content', () => {
    expect(validateDockerfileSchema.safeParse({ content: '' }).success).toBe(false);
  });
  it('rejects unknown environment', () => {
    expect(
      validateDockerfileSchema.safeParse({
        content: 'FROM node:20',
        context: { environment: 'qa' as unknown as 'dev' },
      }).success,
    ).toBe(false);
  });
});
