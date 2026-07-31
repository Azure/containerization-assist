import { describe, expect, it } from '@jest/globals';
import {
  assertDisposableNamespace,
  createDisposableNamespace,
} from '../../agent-eval/namespaces.js';

describe('agent-eval disposable namespaces', () => {
  const ownershipToken = 'a1b2c3d4e5f6';

  it('creates an RFC1123 child namespace without exceeding the length limit', () => {
    const namespace = createDisposableNamespace(
      'evaluation-namespace-with-a-long-configured-name',
      'Azure:GPT-5.4-run-abc123',
      ownershipToken,
    );

    expect(namespace).toMatch(/^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/);
    expect(namespace.length).toBeLessThanOrEqual(63);
    expect(() =>
      assertDisposableNamespace(
        'evaluation-namespace-with-a-long-configured-name',
        namespace,
        ownershipToken,
      ),
    ).not.toThrow();
  });

  it('refuses namespace-wide cleanup of the configured base namespace', () => {
    expect(() => assertDisposableNamespace('eval-ns', 'eval-ns', ownershipToken)).toThrow(
      'Refusing namespace-wide cleanup',
    );
  });

  it('refuses cleanup of an unrelated namespace', () => {
    expect(() => assertDisposableNamespace('eval-ns', 'production', ownershipToken)).toThrow(
      'Refusing namespace-wide cleanup',
    );
    expect(() =>
      assertDisposableNamespace('eval-ns', 'eval-ns-caeval-deadbeef-production', ownershipToken),
    ).toThrow('Refusing namespace-wide cleanup');
  });

  it('refuses a correctly shaped namespace owned by another run', () => {
    const namespace = createDisposableNamespace('eval-ns', 'production', 'deadbeef1234');

    expect(() => assertDisposableNamespace('eval-ns', namespace, ownershipToken)).toThrow(
      'Refusing namespace-wide cleanup',
    );
  });
});
