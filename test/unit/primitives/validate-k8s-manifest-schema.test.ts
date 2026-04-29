import { describe, it, expect } from '@jest/globals';
import { validateK8sManifestSchema } from '@/primitives/validate-k8s-manifest/schema';

describe('validate-k8s-manifest schema', () => {
  it('accepts minimal input', () => {
    expect(validateK8sManifestSchema.safeParse({ content: 'kind: Pod' }).success).toBe(true);
  });

  it('rejects empty content', () => {
    expect(validateK8sManifestSchema.safeParse({ content: '' }).success).toBe(false);
  });

  it('rejects missing content', () => {
    expect(validateK8sManifestSchema.safeParse({}).success).toBe(false);
  });
});
