const K8S_NAME_RE = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;
const OWNERSHIP_TOKEN_RE = /^[a-z0-9]{8,32}$/;

function assertK8sName(kind: string, value: string): void {
  if (!K8S_NAME_RE.test(value)) {
    throw new Error(`Invalid ${kind} '${value}': must be an RFC1123 label.`);
  }
}

function ownershipMarker(ownershipToken: string): string {
  if (!OWNERSHIP_TOKEN_RE.test(ownershipToken)) {
    throw new Error('Invalid evaluation namespace ownership token.');
  }
  return `-caeval-${ownershipToken}-`;
}

export function createDisposableNamespace(
  baseNamespace: string,
  suffix: string,
  ownershipToken: string,
): string {
  assertK8sName('base namespace', baseNamespace);
  const marker = ownershipMarker(ownershipToken);
  const normalizedSuffix = suffix
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  assertK8sName('namespace suffix', normalizedSuffix);

  const maxBaseLength = 63 - normalizedSuffix.length - marker.length;
  if (maxBaseLength < 1) {
    throw new Error(`Namespace suffix '${normalizedSuffix}' leaves no room for a base namespace.`);
  }
  const trimmedBase = baseNamespace.slice(0, maxBaseLength).replace(/-+$/g, '');
  if (!trimmedBase) {
    throw new Error(
      `Base namespace '${baseNamespace}' cannot be combined with '${normalizedSuffix}'.`,
    );
  }
  return `${trimmedBase}${marker}${normalizedSuffix}`;
}

export function assertDisposableNamespace(
  baseNamespace: string,
  namespace: string,
  ownershipToken: string,
): void {
  assertK8sName('base namespace', baseNamespace);
  assertK8sName('cleanup namespace', namespace);
  const marker = ownershipMarker(ownershipToken);
  const markerIndex = namespace.indexOf(marker);
  const suffix = markerIndex > 0 ? namespace.slice(markerIndex + marker.length) : '';
  if (!suffix || createDisposableNamespace(baseNamespace, suffix, ownershipToken) !== namespace) {
    throw new Error(
      `Refusing namespace-wide cleanup of '${namespace}': expected a disposable child of '${baseNamespace}'.`,
    );
  }
}
