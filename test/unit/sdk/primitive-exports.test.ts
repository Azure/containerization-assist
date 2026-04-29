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

describe('SDK tools registry', () => {
  it('exposes all 4 primitives in the tools registry', async () => {
    const { tools } = await import('@/sdk');
    expect(tools.queryKnowledge).toBeDefined();
    expect(tools.validateDockerfile).toBeDefined();
    expect(tools.validateK8sManifest).toBeDefined();
    expect(tools.validateCompose).toBeDefined();
    expect(tools.queryKnowledge.name).toBe('query-knowledge');
    expect(tools.validateDockerfile.name).toBe('validate-dockerfile');
    expect(tools.validateK8sManifest.name).toBe('validate-k8s-manifest');
    expect(tools.validateCompose.name).toBe('validate-compose');
  });
});
