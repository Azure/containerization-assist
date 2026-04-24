import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import validateK8sManifest from '@/primitives/validate-k8s-manifest';
import { createToolContext } from '@/core/context';

const silentLogger = createLogger({ level: 'silent' });

const MANIFEST = [
  'apiVersion: apps/v1',
  'kind: Deployment',
  'metadata:',
  '  name: demo',
  'spec:',
  '  replicas: 1',
  '  selector: { matchLabels: { app: demo } }',
  '  template:',
  '    metadata: { labels: { app: demo } }',
  '    spec:',
  '      containers: [{ name: app, image: myapp:1.0 }]',
].join('\n');

describe('validate-k8s-manifest primitive', () => {
  it('returns pass+empty envelope when no policy is loaded', async () => {
    const ctx = createToolContext(silentLogger);
    expect(ctx.policy).toBeUndefined();
    const result = await validateK8sManifest.handler({ content: MANIFEST }, ctx);
    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value).toEqual({ passed: true, violations: [], warnings: [], suggestions: [] });
  });

  it('exposes the Tool interface', () => {
    expect(validateK8sManifest.name).toBe('validate-k8s-manifest');
    expect(typeof validateK8sManifest.description).toBe('string');
    expect(validateK8sManifest.schema).toBeDefined();
    expect(validateK8sManifest.inputSchema).toBeDefined();
    expect(typeof validateK8sManifest.handler).toBe('function');
    expect(typeof validateK8sManifest.parse).toBe('function');
    expect(validateK8sManifest.metadata).toBeDefined();
  });
});
