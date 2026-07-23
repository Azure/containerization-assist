import { mcrManifestExists, mcrRefExists, __clearMcrCache } from '@/validation/mcr-registry';

// Determine once whether MCR is reachable so live assertions can be skipped
// offline without failing the suite.
async function mcrReachable(): Promise<boolean> {
  const r = await mcrManifestExists('openjdk/jdk', '17-azurelinux');
  return r !== null;
}

describe('mcr-registry ref parsing', () => {
  beforeEach(() => __clearMcrCache());

  it('returns null for non-MCR refs (nothing to verify)', async () => {
    expect(await mcrRefExists('eclipse-temurin:17-jdk')).toBeNull();
    expect(await mcrRefExists('node:20-alpine')).toBeNull();
  });

  it('returns null for an MCR ref with no tag', async () => {
    expect(await mcrRefExists('mcr.microsoft.com/openjdk/jdk')).toBeNull();
  });

  it('returns null for empty repo/tag inputs', async () => {
    expect(await mcrManifestExists('', '17')).toBeNull();
    expect(await mcrManifestExists('openjdk/jdk', '')).toBeNull();
  });
});

describe('mcr-registry live manifest checks', () => {
  beforeEach(() => __clearMcrCache());

  it('confirms a real tag (200) and rejects a hallucinated tag (404)', async () => {
    if (!(await mcrReachable())) {
      console.warn('MCR unreachable — skipping live manifest checks');
      return;
    }
    __clearMcrCache();
    // Real, verified in the catalog.
    expect(await mcrManifestExists('openjdk/jdk', '17-azurelinux')).toBe(true);
    // Plausible but nonexistent — model hallucination pattern.
    expect(await mcrManifestExists('java/jre', '8u372-zulu-ubuntu')).toBe(false);
    // Real via full-ref wrapper.
    expect(await mcrRefExists('mcr.microsoft.com/dotnet/sdk:8.0')).toBe(true);
  }, 30000);

  it('caches definitive results', async () => {
    if (!(await mcrReachable())) return;
    __clearMcrCache();
    const first = await mcrManifestExists('openjdk/jdk', '17-azurelinux');
    const second = await mcrManifestExists('openjdk/jdk', '17-azurelinux');
    expect(first).toBe(true);
    expect(second).toBe(true);
  }, 30000);
});
