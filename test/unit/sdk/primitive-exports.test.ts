import { describe, it, expect } from '@jest/globals';
import {
  queryKnowledge,
  validateDockerfile,
  validateK8sManifest,
  validateCompose,
} from '@/sdk';

describe('SDK primitive exports', () => {
  it.each([
    ['queryKnowledge', queryKnowledge],
    ['validateDockerfile', validateDockerfile],
    ['validateK8sManifest', validateK8sManifest],
    ['validateCompose', validateCompose],
  ])('%s is callable', (_, fn) => {
    expect(typeof fn).toBe('function');
  });

  it('queryKnowledge returns a Result for unknown tags', async () => {
    const r = await queryKnowledge({ tags: ['nonexistent-tag-zzz'], limit: 1 });
    expect(r).toHaveProperty('ok');
  });

  it('validateDockerfile returns a passing envelope without policy', async () => {
    const r = await validateDockerfile({ content: 'FROM node:20' });
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.value.passed).toBe(true);
  });
});
