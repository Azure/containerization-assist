import { promises as fs } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse as parseYaml } from 'yaml';
import { MAX_FAILURE_DETAIL_CHARS } from './log-config.js';

const execFileP = promisify(execFile);

export interface CheckContext {
  artifactDir: string;
}

export interface CheckResult {
  name: string;
  passed: boolean;
  message: string;
  details?: string;
}

export interface Check {
  name: string;
  run(ctx: CheckContext): Promise<CheckResult>;
}

async function readDockerfile(artifactDir: string): Promise<string | null> {
  try {
    return await fs.readFile(join(artifactDir, 'Dockerfile'), 'utf8');
  } catch {
    return null;
  }
}

// Directories never scanned for k8s manifests: vendored libs, build output, and
// CI config whose stray .yaml/.yml files would otherwise trip the label check.
const MANIFEST_SCAN_SKIP_DIRS = new Set([
  '.git',
  '.github',
  '.idea',
  '.vscode',
  'node_modules',
  'bower_components',
  'vendor',
  'target',
  'build',
  'dist',
  'out',
  '.gradle',
  '.mvn',
]);

async function listManifestFiles(artifactDir: string): Promise<string[]> {
  const found: string[] = [];
  async function walk(dir: string): Promise<void> {
    const entries = await fs.readdir(dir, { withFileTypes: true });
    for (const e of entries) {
      if (e.isDirectory()) {
        if (MANIFEST_SCAN_SKIP_DIRS.has(e.name)) continue;
        await walk(join(dir, e.name));
      } else if (e.name.endsWith('.yaml') || e.name.endsWith('.yml')) {
        found.push(join(dir, e.name));
      }
    }
  }
  await walk(artifactDir);
  return found;
}

const MCR_REGISTRY_PATTERN = /\bmcr\.microsoft\.com\//i;
const WHY_NOT_MCR_PATTERN = /^\s*#\s*WHY-NOT-MCR\s*:/i;

// Versions MCR actually ships per stack. A public-registry base whose stack +
// version IS covered here fails (MCR has an equivalent). A recognized stack
// whose version is NOT covered (a genuine coverage gap) passes; a recognized
// stack whose version cannot be parsed is treated strictly as covered (fails)
// rather than silently passing on missing information.
const MCR_COVERAGE = {
  java: {
    repo: 'openjdk/jdk',
    versions: new Set([8, 11, 17, 21, 25]),
    suggestion: 'mcr.microsoft.com/openjdk/jdk:<V>-azurelinux or :<V>-distroless',
  },
  node: {
    repo: 'azurelinux/base/nodejs',
    versions: new Set([20, 24]),
    suggestion: 'mcr.microsoft.com/azurelinux/base/nodejs:<V> or .../distroless/nodejs:<V>',
  },
  python: {
    repo: 'azurelinux/base/python',
    versions: new Set(['3.12']),
    suggestion: 'mcr.microsoft.com/azurelinux/base/python:<V> or .../distroless/python:<V>',
  },
  dotnet: {
    repo: 'dotnet/sdk',
    versions: new Set(['8.0', '9.0', '10.0']),
    suggestion: 'mcr.microsoft.com/dotnet/<sdk|aspnet|runtime>:<V>-azurelinux3.0',
  },
} as const;

const MCR_CATALOG_PATH = fileURLToPath(
  new URL('../../knowledge/catalogs/mcr-base-images.json', import.meta.url),
);
let mcrCatalog: Map<string, Set<string>> | null | undefined;

async function loadMcrCatalog(): Promise<Map<string, Set<string>> | null> {
  if (mcrCatalog !== undefined) return mcrCatalog;
  try {
    const raw = await fs.readFile(MCR_CATALOG_PATH, 'utf8');
    const data = JSON.parse(raw) as { repos?: Record<string, string[]> };
    const map = new Map<string, Set<string>>();
    for (const [repo, tags] of Object.entries(data.repos ?? {})) map.set(repo, new Set(tags));
    mcrCatalog = map;
  } catch {
    mcrCatalog = null;
  }
  return mcrCatalog;
}

async function getMcrTags(repo: string): Promise<Set<string> | null> {
  const catalog = await loadMcrCatalog();
  return catalog?.get(repo) ?? null;
}

function canonicalizeTag(tag: string): string {
  // Drop a trailing architecture qualifier (mirrors isNoiseTag in the refresh
  // script, which removes these so the catalog stores only canonical tags).
  // Covers versioned arch suffixes too (`arm64v8`, `arm32v7`), which MCR uses.
  const noArch = tag.replace(/-(amd64|arm64(v8)?|arm32(v[0-9]+)?|ppc64le|s390x)$/i, '');
  // Collapse a leading 3+ part numeric version to major.minor (`8.0.11` → `8.0`,
  // `8.0.11-azurelinux3.0` → `8.0-azurelinux3.0`). The refresh script drops
  // 3-part numeric tags, so the catalog only ever stores the 1-/2-part family.
  return noArch.replace(/^(\d+\.\d+)\.\d+/, '$1');
}

/**
 * Does the catalog contain a pullable tag matching `tag`? The catalog is
 * intentionally lossy — the refresh script strips arch-suffixed and 3+ part
 * numeric tags as noise — so an exact `.has()` mis-flags real-but-more-specific
 * tags (e.g. `dotnet/sdk:8.0.11`, `nodejs:20.14.0`) as hallucinated. We instead
 * match against the tag's canonical family, and treat a bare major (`17`) as
 * present when any variant (`17-azurelinux`, `17.x`) is published. Genuine
 * hallucinations (fake distro suffixes, nonexistent versions) still fail to
 * match and are correctly rejected.
 */
function tagExistsInCatalog(tags: Set<string>, tag: string): boolean {
  if (tags.has(tag)) return true;
  const canon = canonicalizeTag(tag);
  if (canon !== tag && tags.has(canon)) return true;
  if (/^\d+$/.test(canon)) {
    for (const t of tags) {
      if (t === canon || t.startsWith(`${canon}-`) || t.startsWith(`${canon}.`)) return true;
    }
  }
  return false;
}

// Public-registry repos that map onto an MCR-covered stack (plain JDK/JRE, Node,
// Python, .NET runtimes). Excludes builders/app servers (maven, gradle, tomcat,
// wildfly, …) which have no MCR equivalent.
const JAVA_JDK_REPO =
  /^(?:eclipse-temurin|openjdk|amazoncorretto|adoptopenjdk|ibmjava|sapmachine|azul\/zulu-openjdk|bellsoft\/liberica-openjdk-(?:alpine|debian))$/i;
const NODE_REPO = /^node$/i;
const PYTHON_REPO = /^python$/i;
const DOTNET_REPO = /^(?:mcr\.microsoft\.com\/)?dotnet\/(?:sdk|aspnet|runtime)$/i;

// Public-registry repos with no MCR equivalent — using them without a
// WHY-NOT-MCR annotation is fine (real coverage gaps). REPO patterns only.
// Kept broad on purpose: a legitimate base that works should never be flagged
// "not using a Microsoft image" just because the list was too narrow.
const NO_MCR_EQUIVALENT_REPOS: ReadonlyArray<{ pattern: RegExp; reason: string }> = [
  // JVM builders & app servers — MCR ships JDK/JRE bases but no build tools or servers.
  { pattern: /^maven$/i, reason: 'Maven builder — MCR ships no Maven-with-JDK image' },
  { pattern: /^gradle$/i, reason: 'Gradle builder — MCR ships no Gradle image' },
  {
    pattern: /^(?:sbt|hseeberger\/scala-sbt)$/i,
    reason: 'sbt/Scala builder — MCR ships no sbt image',
  },
  { pattern: /^tomcat$/i, reason: 'Tomcat servlet container — MCR ships no Tomcat' },
  {
    pattern: /^(?:jboss|quay\.io\/wildfly)\/wildfly$/i,
    reason: 'WildFly app server — MCR ships no app server',
  },
  {
    pattern: /^payara\/(?:server-full|server-web|micro|server)$/i,
    reason: 'Payara app server — MCR ships no app server',
  },
  { pattern: /^tomee$/i, reason: 'TomEE app server — MCR ships no app server' },
  {
    pattern:
      /^(?:websphere-liberty|ibmcom\/websphere-liberty|icr\.io\/appcafe\/websphere-liberty)$/i,
    reason: 'WebSphere Liberty — MCR ships no app server',
  },
  {
    pattern: /^(?:open-liberty|icr\.io\/appcafe\/open-liberty)$/i,
    reason: 'Open Liberty app server — MCR ships no app server',
  },
  {
    pattern: /^(?:jetty|eclipse\/jetty)$/i,
    reason: 'Jetty servlet container — MCR ships no Jetty',
  },
  {
    pattern: /^(?:glassfish|oracle\/glassfish)$/i,
    reason: 'GlassFish app server — MCR ships no app server',
  },

  // Web / static / proxy servers — MCR ships no general-purpose web/edge server.
  {
    pattern: /^(?:nginx|nginxinc\/nginx-unprivileged|bitnami\/nginx)$/i,
    reason: 'nginx web/static server — MCR ships no nginx',
  },
  { pattern: /^httpd$/i, reason: 'Apache httpd — MCR ships no httpd' },
  { pattern: /^caddy$/i, reason: 'Caddy web server — MCR ships no Caddy' },
  {
    pattern: /^(?:haproxy|envoyproxy\/envoy|traefik)$/i,
    reason: 'Proxy/edge server — MCR ships no equivalent',
  },

  // Language runtimes / toolchains with no MCR equivalent.
  { pattern: /^golang$/i, reason: 'Go toolchain — MCR ships no Go image' },
  { pattern: /^rust$/i, reason: 'Rust toolchain — MCR ships no Rust image' },
  { pattern: /^php$/i, reason: 'PHP — MCR ships no PHP image' },
  { pattern: /^composer$/i, reason: 'PHP composer builder — MCR ships no composer image' },
  { pattern: /^ruby$/i, reason: 'Ruby — MCR ships no Ruby image' },
  { pattern: /^(?:elixir|erlang)$/i, reason: 'Elixir/Erlang — MCR ships no equivalent' },
  {
    pattern: /^(?:perl|swift|haskell|clojure|julia|deno|denoland\/deno|oven\/bun)$/i,
    reason: 'Other language runtime — MCR ships no equivalent',
  },

  // Hardened / minimal bases — already security-focused, no MCR equivalent.
  {
    pattern: /^gcr\.io\/distroless\//i,
    reason: 'Google distroless — minimal runtime, no MCR equivalent',
  },
  { pattern: /^cgr\.dev\/chainguard\//i, reason: 'Chainguard hardened image — no MCR equivalent' },
  { pattern: /^scratch$/i, reason: 'FROM scratch — no base image at all' },

  // Generic OS bases — not a stack image; no direct MCR equivalent.
  {
    pattern: /^(?:ubuntu|debian|alpine|busybox)$/i,
    reason: 'Generic OS base — not a stack image, no direct MCR equivalent',
  },
  {
    pattern: /^(?:fedora|rockylinux|almalinux|centos|amazonlinux|opensuse\/leap)$/i,
    reason: 'Generic OS base — not a stack image, no direct MCR equivalent',
  },
];

interface ParsedFrom {
  /** raw text of the FROM line */
  raw: string;
  /** repository portion (everything before the tag/digest), e.g. `eclipse-temurin` */
  repo: string;
  /** tag portion (everything after `:`), or null if untagged */
  tag: string | null;
}

function parseFromLine(line: string): ParsedFrom | null {
  // FROM [--platform=...] image[:tag|@digest] [AS name]
  const m = /^\s*FROM\s+(?:--platform=\S+\s+)?(\S+)/i.exec(line);
  if (!m) return null;
  let ref = m[1];
  // Strip digest (anything after @)
  const at = ref.indexOf('@');
  if (at >= 0) ref = ref.slice(0, at);
  // Tag is everything after the LAST `:` that comes after the LAST `/`
  // (avoids splitting `host:port/repo`).
  const lastSlash = ref.lastIndexOf('/');
  const lastColon = ref.lastIndexOf(':');
  let repo: string;
  let tag: string | null;
  if (lastColon > lastSlash) {
    repo = ref.slice(0, lastColon);
    tag = ref.slice(lastColon + 1);
  } else {
    repo = ref;
    tag = null;
  }
  return { raw: line, repo, tag };
}

/** Pull the major version (single int) from a Java tag, e.g. `17-jdk-alpine` → 17, `1.8.0_345` → 8. */
function extractJavaMajor(tag: string | null): number | null {
  if (!tag) return null;
  // `1.8`-style → 8; otherwise leading int.
  const oneDot = /^1\.(\d+)(?:[.\-_].*)?$/.exec(tag);
  if (oneDot) return parseInt(oneDot[1], 10);
  const leading = /^(\d+)(?:[-.][\w.-]*)?$/.exec(tag);
  if (leading) return parseInt(leading[1], 10);
  return null;
}

/** Pull the major version (single int) from a Node tag, e.g. `20-alpine` → 20. */
function extractNodeMajor(tag: string | null): number | null {
  if (!tag) return null;
  const m = /^(\d+)(?:[-.][\w.-]*)?$/.exec(tag);
  return m ? parseInt(m[1], 10) : null;
}

/** Pull `major.minor` from a Python/.NET tag, e.g. `3.11-slim` → `3.11`, `8.0-azurelinux3.0` → `8.0`. */
function extractMajorMinor(tag: string | null): string | null {
  if (!tag) return null;
  const m = /^(\d+\.\d+)(?:[-.][\w.-]*)?$/.exec(tag);
  return m ? m[1] : null;
}

type Coverage =
  | { kind: 'mcr' }
  | { kind: 'mcr-missing' }
  | { kind: 'mcr-equivalent-exists'; suggestion: string }
  | { kind: 'no-mcr-equivalent'; reason: string }
  | { kind: 'unknown' };

async function coverageFor<K>(
  version: K | null,
  entry: { repo: string; versions: ReadonlySet<K>; suggestion: string },
  placeholder: string,
  label: string,
): Promise<Coverage> {
  if (version === null) {
    return {
      kind: 'mcr-equivalent-exists',
      suggestion: entry.suggestion.replace(/<V>/g, placeholder),
    };
  }
  const tags = await getMcrTags(entry.repo);
  const covered = tags ? tagExistsInCatalog(tags, String(version)) : entry.versions.has(version);
  if (covered) {
    return {
      kind: 'mcr-equivalent-exists',
      suggestion: entry.suggestion.replace(/<V>/g, String(version)),
    };
  }
  return {
    kind: 'no-mcr-equivalent',
    reason: `${label} ${version} not published on MCR (${entry.repo})`,
  };
}

async function classifyBase(parsed: ParsedFrom): Promise<Coverage> {
  const { tag } = parsed;
  const repo = parsed.repo.replace(/^docker\.io\//i, '').replace(/^library\//i, '');

  if (MCR_REGISTRY_PATTERN.test(parsed.repo)) {
    const mcrRepo = parsed.repo.replace(/^mcr\.microsoft\.com\//i, '');
    const tags = await getMcrTags(mcrRepo);
    if (tags === null) return { kind: 'mcr' };
    return tagExistsInCatalog(tags, tag ?? 'latest') ? { kind: 'mcr' } : { kind: 'mcr-missing' };
  }

  if (JAVA_JDK_REPO.test(repo))
    return coverageFor(extractJavaMajor(tag), MCR_COVERAGE.java, '<major>', 'Java');
  if (NODE_REPO.test(repo))
    return coverageFor(extractNodeMajor(tag), MCR_COVERAGE.node, '<major>', 'Node');
  if (PYTHON_REPO.test(repo))
    return coverageFor(extractMajorMinor(tag), MCR_COVERAGE.python, '<major.minor>', 'Python');
  if (DOTNET_REPO.test(repo))
    return coverageFor(extractMajorMinor(tag), MCR_COVERAGE.dotnet, '<major.minor>', '.NET');

  for (const { pattern, reason } of NO_MCR_EQUIVALENT_REPOS) {
    if (pattern.test(repo)) return { kind: 'no-mcr-equivalent', reason };
  }

  return { kind: 'unknown' };
}

function hasWhyNotMcrAnnotation(lines: string[], fromLineIdx: number): boolean {
  let j = fromLineIdx - 1;
  while (j >= 0 && lines[j].trim() === '') j--;
  while (j >= 0 && /^\s*#/.test(lines[j])) {
    if (WHY_NOT_MCR_PATTERN.test(lines[j])) return true;
    j--;
  }
  return false;
}

export const requiresAzureBaseImage: Check = {
  name: 'requires-azure-base',
  async run({ artifactDir }) {
    const dockerfile = await readDockerfile(artifactDir);
    if (dockerfile === null) {
      return { name: this.name, passed: false, message: 'Dockerfile not found' };
    }
    const lines = dockerfile.split('\n');
    const offending: string[] = [];
    const allowed: string[] = [];
    let fromCount = 0;
    for (let i = 0; i < lines.length; i++) {
      if (!/^\s*FROM\s/i.test(lines[i])) continue;
      fromCount++;
      const parsed = parseFromLine(lines[i]);
      if (!parsed) continue;
      const coverage = await classifyBase(parsed);
      switch (coverage.kind) {
        case 'mcr':
          break;
        case 'mcr-missing':
          offending.push(
            `${parsed.raw.trim()}  → MCR image/tag not found in the registry (hallucinated?); use a real mcr.microsoft.com tag`,
          );
          break;
        case 'no-mcr-equivalent':
          allowed.push(`${parsed.raw.trim()}  // ${coverage.reason}`);
          break;
        case 'unknown':
          // Strict default: an unrecognized base cannot be confirmed as a
          // genuine MCR coverage gap, so fail it unless explicitly justified.
          if (hasWhyNotMcrAnnotation(lines, i)) {
            allowed.push(
              `${parsed.raw.trim()}  // unrecognized base — justified by WHY-NOT-MCR annotation`,
            );
          } else {
            offending.push(
              `${parsed.raw.trim()}  → unrecognized base; use an mcr.microsoft.com base or add a "# WHY-NOT-MCR:" annotation`,
            );
          }
          break;
        case 'mcr-equivalent-exists':
          if (hasWhyNotMcrAnnotation(lines, i)) {
            allowed.push(`${parsed.raw.trim()}  // overridden by WHY-NOT-MCR annotation`);
          } else {
            offending.push(`${parsed.raw.trim()}  → use ${coverage.suggestion}`);
          }
          break;
      }
    }
    if (fromCount === 0) {
      return { name: this.name, passed: false, message: 'Dockerfile has no FROM directive' };
    }
    if (offending.length > 0) {
      return {
        name: this.name,
        passed: false,
        message: `${offending.length} FROM line(s) must use an MCR base (or carry a "# WHY-NOT-MCR:" annotation)`,
        details:
          offending.join('\n') +
          (allowed.length
            ? `\n--- allowed (no MCR equivalent / annotated):\n${allowed.join('\n')}`
            : ''),
      };
    }
    const summary = allowed.length
      ? `All FROM lines OK (${fromCount} total; ${allowed.length} non-MCR but no MCR equivalent or annotated)`
      : `All ${fromCount} FROM line(s) use mcr.microsoft.com`;
    return {
      name: this.name,
      passed: true,
      message: summary,
      details: allowed.length ? allowed.join('\n') : undefined,
    };
  },
};

const REQUIRED_DOCKERFILE_LABEL = 'com.azure.containerizationassist.createdby';
const REQUIRED_K8S_LABELS = ['app.kubernetes.io/name', 'app.kubernetes.io/managed-by'];

export const hasRequiredLabels: Check = {
  name: 'has-required-labels',
  async run({ artifactDir }) {
    const missing: string[] = [];

    const dockerfile = await readDockerfile(artifactDir);
    if (dockerfile === null) {
      missing.push('Dockerfile not found');
    } else if (!new RegExp(`^\\s*LABEL\\s+${REQUIRED_DOCKERFILE_LABEL}\\s*=`, 'm').test(dockerfile)) {
      missing.push(`Dockerfile is missing LABEL ${REQUIRED_DOCKERFILE_LABEL}`);
    }

    const manifestPaths = await listManifestFiles(artifactDir);
    for (const path of manifestPaths) {
      const text = await fs.readFile(path, 'utf8');
      const docs = text.split(/^---\s*$/m).filter((d) => d.trim());
      for (const doc of docs) {
        let parsed: unknown;
        try {
          parsed = parseYaml(doc);
        } catch {
          // Skip non-k8s YAML that slipped past the directory filter.
          continue;
        }
        if (!parsed || typeof parsed !== 'object') continue;
        // Only enforce labels on real Kubernetes objects (have apiVersion + kind).
        const obj = parsed as {
          apiVersion?: unknown;
          kind?: unknown;
          metadata?: { labels?: Record<string, string> };
        };
        if (typeof obj.apiVersion !== 'string' || typeof obj.kind !== 'string') continue;
        const labels = obj.metadata?.labels ?? {};
        for (const required of REQUIRED_K8S_LABELS) {
          if (!labels[required]) {
            missing.push(`${path}: missing label '${required}'`);
          }
        }
      }
    }

    if (missing.length > 0) {
      return {
        name: this.name,
        passed: false,
        message: `${missing.length} required label issue(s)`,
        details: missing.join('\n'),
      };
    }
    return { name: this.name, passed: true, message: 'All required labels present' };
  },
};

export const dockerBuilds: Check = {
  name: 'docker-builds',
  async run({ artifactDir }) {
    const tag = `agent-eval-${Date.now()}:check`;
    const platform = process.env.AGENT_EVAL_BUILD_PLATFORM?.trim() || 'linux/amd64';
    const buildArgs = ['buildx', 'build', '--platform', platform, '--load', '-t', tag, artifactDir];
    try {
      await execFileP('docker', buildArgs, { maxBuffer: 16 * 1024 * 1024 });
    } catch (err) {
      const e = err as { code?: string; stderr?: string; message?: string };
      if (e.code === 'ENOENT') {
        return { name: this.name, passed: false, message: 'docker not available on PATH' };
      }
      return {
        name: this.name,
        passed: false,
        message: 'docker build failed',
        details: (e.stderr ?? e.message ?? '').slice(-MAX_FAILURE_DETAIL_CHARS),
      };
    }
    try {
      await execFileP('docker', ['rmi', tag]);
    } catch {
      // ignore cleanup failures
    }
    return { name: this.name, passed: true, message: 'docker build succeeded' };
  },
};

export const ALL_CHECKS: readonly Check[] = [
  requiresAzureBaseImage,
  hasRequiredLabels,
  dockerBuilds,
];

export function selectChecks(spec: string): Check[] {
  const trimmed = spec.trim();
  if (trimmed === 'all') return [...ALL_CHECKS];
  if (trimmed === 'none' || trimmed === '') return [];
  const requested = trimmed
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  const byName = new Map(ALL_CHECKS.map((c) => [c.name, c]));
  const selected: Check[] = [];
  for (const name of requested) {
    const check = byName.get(name);
    if (!check) {
      throw new Error(
        `Unknown check '${name}'. Available: ${ALL_CHECKS.map((c) => c.name).join(', ')}`,
      );
    }
    selected.push(check);
  }
  return selected;
}

export async function runChecks(checks: Check[], ctx: CheckContext): Promise<CheckResult[]> {
  return Promise.all(checks.map((c) => c.run(ctx)));
}
