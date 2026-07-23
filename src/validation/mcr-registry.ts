/**
 * Live Microsoft Container Registry (MCR) manifest verification.
 *
 * The containerization agent frequently proposes `mcr.microsoft.com/...` base
 * images. A string that merely *looks* like an MCR reference is not proof the
 * tag exists — models routinely hallucinate plausible-but-nonexistent tags
 * (e.g. `mcr.microsoft.com/java/jre:8u372-zulu-ubuntu`). This module asks the
 * registry directly.
 *
 * MCR implements the read-only Docker Registry v2 API and, unlike Docker Hub,
 * serves manifests unauthenticated. A HEAD on
 * `/v2/<repo>/manifests/<tag>` returns 200 when the tag is real and 404 when it
 * is not. That makes it a reliable, cheap oracle that is reachable in-pipeline
 * (Docker Hub is not).
 */

const MCR_HOST = 'mcr.microsoft.com';
const MANIFEST_TIMEOUT_MS = 5000;

/** Accept header covering multi-arch indexes and single-arch manifests. */
const MANIFEST_ACCEPT = [
  'application/vnd.docker.distribution.manifest.list.v2+json',
  'application/vnd.oci.image.index.v1+json',
  'application/vnd.docker.distribution.manifest.v2+json',
  'application/vnd.oci.image.manifest.v1+json',
].join(',');

/**
 * Cache of resolved lookups. `true`/`false` are definitive; we deliberately do
 * NOT cache `null` (undetermined) so a transient network failure can be retried
 * later in the same process.
 */
const cache = new Map<string, boolean>();

/** Strip the mcr host (and optional docker.io/library prefixes) from a repo. */
function normalizeMcrRepo(repo: string): string {
  return repo
    .replace(/^https?:\/\//i, '')
    .replace(new RegExp(`^${MCR_HOST}/`, 'i'), '')
    .replace(/^\/+/, '');
}

/**
 * Does `mcr.microsoft.com/<repo>:<tag>` resolve to a real manifest?
 *
 * @returns `true` if the manifest exists (HTTP 200), `false` if it definitively
 *   does not (HTTP 404), or `null` when the result is undetermined — the
 *   registry was unreachable, timed out, or returned an unexpected status. A
 *   `null` result MUST NOT be treated as "bad": offline callers cannot prove a
 *   tag is invalid.
 */
export async function mcrManifestExists(repo: string, tag: string): Promise<boolean | null> {
  const cleanRepo = normalizeMcrRepo(repo);
  if (!cleanRepo || !tag) return null;

  const key = `${cleanRepo}:${tag}`;
  const cached = cache.get(key);
  if (cached !== undefined) return cached;

  const url = `https://${MCR_HOST}/v2/${cleanRepo}/manifests/${encodeURIComponent(tag)}`;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), MANIFEST_TIMEOUT_MS);

  try {
    const res = await fetch(url, {
      method: 'HEAD',
      headers: { Accept: MANIFEST_ACCEPT },
      signal: controller.signal,
    });
    if (res.status === 200) {
      cache.set(key, true);
      return true;
    }
    if (res.status === 404) {
      cache.set(key, false);
      return false;
    }
    // 401/429/5xx/redirects → undetermined; don't cache.
    return null;
  } catch {
    // Network error / abort / DNS failure → undetermined.
    return null;
  } finally {
    clearTimeout(timer);
  }
}

/**
 * Convenience wrapper accepting a full reference like
 * `mcr.microsoft.com/openjdk/jdk:17-azurelinux`. Returns `null` for refs that
 * are not on MCR or carry no tag (nothing to verify).
 */
export async function mcrRefExists(ref: string): Promise<boolean | null> {
  const at = ref.indexOf('@');
  const withoutDigest = at >= 0 ? ref.slice(0, at) : ref;
  if (!/^mcr\.microsoft\.com\//i.test(withoutDigest)) return null;
  const lastColon = withoutDigest.lastIndexOf(':');
  const lastSlash = withoutDigest.lastIndexOf('/');
  if (lastColon <= lastSlash) return null; // no tag
  const repo = withoutDigest.slice(0, lastColon);
  const tag = withoutDigest.slice(lastColon + 1);
  return mcrManifestExists(repo, tag);
}

/** Test hook: clear the in-process manifest cache. */
export function __clearMcrCache(): void {
  cache.clear();
}
