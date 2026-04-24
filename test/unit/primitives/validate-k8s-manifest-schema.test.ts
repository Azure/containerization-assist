import { describe, it, expect } from '@jest/globals';
import { validateK8sManifestSchema } from '@/primitives/validate-k8s-manifest/schema';

describe('validate-k8s-manifest schema', () => {
  it('accepts minimal input', () => {
    expect(validateK8sManifestSchema.safeParse({ content: 'kind: Pod' }).success).toBe(true);
  });
  it('accepts full context', () => {
    expect(
      validateK8sManifestSchema.safeParse({
        content: 'kind: Pod',
        context: { environment: 'production' },
      }).success,
    ).toBe(true);
  });
  it('rejects empty content', () => {
    expect(validateK8sManifestSchema.safeParse({ content: '' }).success).toBe(false);
  });
  it('rejects invalid environment', () => {
    expect(
      validateK8sManifestSchema.safeParse({
        content: 'kind: Pod',
        context: { environment: 'qa' as unknown as 'dev' },
      }).success,
    ).toBe(false);
  });
  it('rejects unknown keys in context', () => {
    expect(
      validateK8sManifestSchema.safeParse({
        content: 'kind: Pod',
        // @ts-expect-error - testing that strict() rejects unknown keys
        context: { environment: 'dev', typo: true },
      }).success,
    ).toBe(false);
  });
});
