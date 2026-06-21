import { promises as fs } from 'node:fs';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { join } from 'node:path';
import { parse as parseYaml } from 'yaml';

const execFileP = promisify(execFile);

export interface CheckContext {
  artifactDir: string;
  fixtureDir: string;
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

// Directories we never recurse into when hunting for k8s manifests. These are
// vendored libraries, build artefacts, or CI config — all of which contain
// `.yaml`/`.yml` files (eslint configs, travis configs, mkdocs configs,
// GitHub workflows, etc.) that would otherwise be flagged as "missing
// app.kubernetes.io/name". Keep this list conservative: false-negatives are
// safer than false-positives for this check.
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

// Canonical MCR coverage. Versions verified against the live MCR registry
// tags API (e.g. https://mcr.microsoft.com/v2/openjdk/jdk/tags/list).
// Keep this table general — it describes WHAT MCR ships, not WHICH fixture
// we are testing. The check below uses it both ways:
//   (a) public-registry image whose stack+version IS in this table → fail
//       (MCR has an equivalent, so a non-MCR base is unjustified).
//   (b) public-registry image whose stack is NOT in this table, OR whose
//       version is OUT OF RANGE → pass (genuine MCR coverage gap).
const MCR_COVERAGE = {
  java: { versions: new Set([8, 11, 17, 21, 25]), suggestion: 'mcr.microsoft.com/openjdk/jdk:<V>-azurelinux or :<V>-distroless' },
  node: { versions: new Set([18, 20, 22]), suggestion: 'mcr.microsoft.com/azurelinux/base/nodejs:<V> or .../distroless/nodejs:<V>' },
  python: { versions: new Set(['3.11', '3.12']), suggestion: 'mcr.microsoft.com/azurelinux/base/python:<V> or .../distroless/python:<V>' },
  dotnet: { versions: new Set(['8.0', '9.0']), suggestion: 'mcr.microsoft.com/dotnet/<sdk|aspnet|runtime>:<V>-azurelinux3.0' },
} as const;

// Public-registry image repos that map onto a stack we know MCR covers.
// Plain JDK/JRE images, Node runtimes, Python runtimes, .NET runtimes.
// NOTE: this does NOT include `maven:*`, `gradle:*`, `tomcat:*`,
// `jboss/wildfly:*`, app servers, etc. — those have no MCR equivalent.
const JAVA_JDK_REPO = /^(?:eclipse-temurin|openjdk|amazoncorretto|adoptopenjdk|ibmjava|sapmachine|azul\/zulu-openjdk|bellsoft\/liberica-openjdk-(?:alpine|debian))$/i;
const NODE_REPO = /^node$/i;
const PYTHON_REPO = /^python$/i;
const DOTNET_REPO = /^(?:mcr\.microsoft\.com\/)?dotnet\/(?:sdk|aspnet|runtime)$/i;

// Public-registry repos where MCR has no equivalent. Using these without
// a WHY-NOT-MCR annotation is fine — they represent real coverage gaps.
// Keep these as REPO patterns (no tag/version constraint).
const NO_MCR_EQUIVALENT_REPOS: ReadonlyArray<{ pattern: RegExp; reason: string }> = [
  { pattern: /^maven$/i, reason: 'Maven builder — MCR ships no Maven-with-JDK image' },
  { pattern: /^gradle$/i, reason: 'Gradle builder — MCR ships no Gradle image' },
  { pattern: /^tomcat$/i, reason: 'Tomcat servlet container — MCR ships no Tomcat' },
  { pattern: /^(?:jboss|quay\.io\/wildfly)\/wildfly$/i, reason: 'WildFly app server — MCR ships no app server' },
  { pattern: /^payara\/(?:server-full|server-web|micro|server)$/i, reason: 'Payara app server — MCR ships no app server' },
  { pattern: /^tomee$/i, reason: 'TomEE app server — MCR ships no app server' },
  { pattern: /^(?:websphere-liberty|ibmcom\/websphere-liberty|icr\.io\/appcafe\/websphere-liberty)$/i, reason: 'WebSphere Liberty — MCR ships no app server' },
  { pattern: /^(?:jetty|eclipse\/jetty)$/i, reason: 'Jetty servlet container — MCR ships no Jetty' },
  { pattern: /^golang$/i, reason: 'Go toolchain — MCR ships no Go image' },
  { pattern: /^rust$/i, reason: 'Rust toolchain — MCR ships no Rust image' },
  { pattern: /^php$/i, reason: 'PHP — MCR ships no PHP image' },
  { pattern: /^composer$/i, reason: 'PHP composer builder — MCR ships no composer image' },
  { pattern: /^ruby$/i, reason: 'Ruby — MCR ships no Ruby image' },
  { pattern: /^gcr\.io\/distroless\//i, reason: 'Google distroless — standard Go/Rust runtime, no MCR equivalent' },
  { pattern: /^scratch$/i, reason: 'FROM scratch — no base image at all' },
];

interface ParsedFrom {
  /** zero-indexed line in the Dockerfile */
  lineNo: number;
  /** raw text of the FROM line */
  raw: string;
  /** repository portion (everything before the tag/digest), e.g. `eclipse-temurin` */
  repo: string;
  /** tag portion (everything after `:`), or null if untagged */
  tag: string | null;
}

function parseFromLine(line: string, lineNo: number): ParsedFrom | null {
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
  return { lineNo, raw: line, repo, tag };
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

/** Pull `major.minor` from a Python tag, e.g. `3.11-slim` → `3.11`. */
function extractPythonMajorMinor(tag: string | null): string | null {
  if (!tag) return null;
  const m = /^(\d+\.\d+)(?:[-.][\w.-]*)?$/.exec(tag);
  return m ? m[1] : null;
}

/** Pull `major.minor` from a .NET tag, e.g. `8.0-azurelinux3.0` → `8.0`. */
function extractDotnetMajorMinor(tag: string | null): string | null {
  if (!tag) return null;
  const m = /^(\d+\.\d+)(?:[-.][\w.-]*)?$/.exec(tag);
  return m ? m[1] : null;
}

type Coverage =
  | { kind: 'mcr' }
  | { kind: 'mcr-equivalent-exists'; suggestion: string }
  | { kind: 'no-mcr-equivalent'; reason: string }
  | { kind: 'unknown' };

function classifyBase(parsed: ParsedFrom): Coverage {
  const { repo, tag } = parsed;
  // 1. Already on MCR → pass.
  if (MCR_REGISTRY_PATTERN.test(repo)) return { kind: 'mcr' };

  // 2. Java JDK/JRE bases — MCR covers 8, 11, 17, 21, 25.
  if (JAVA_JDK_REPO.test(repo)) {
    const major = extractJavaMajor(tag);
    if (major !== null && MCR_COVERAGE.java.versions.has(major)) {
      return {
        kind: 'mcr-equivalent-exists',
        suggestion: MCR_COVERAGE.java.suggestion.replace(/<V>/g, String(major)),
      };
    }
    return {
      kind: 'no-mcr-equivalent',
      reason: `Java ${major ?? '(unparsed)'} not in MCR (publishes 8/11/17/21/25)`,
    };
  }

  // 3. Node — MCR covers 18, 20, 22.
  if (NODE_REPO.test(repo)) {
    const major = extractNodeMajor(tag);
    if (major !== null && MCR_COVERAGE.node.versions.has(major)) {
      return {
        kind: 'mcr-equivalent-exists',
        suggestion: MCR_COVERAGE.node.suggestion.replace(/<V>/g, String(major)),
      };
    }
    return {
      kind: 'no-mcr-equivalent',
      reason: `Node ${major ?? '(unparsed)'} not in MCR (publishes 18/20/22)`,
    };
  }

  // 4. Python — MCR covers 3.11, 3.12.
  if (PYTHON_REPO.test(repo)) {
    const mm = extractPythonMajorMinor(tag);
    if (mm !== null && (MCR_COVERAGE.python.versions as Set<string>).has(mm)) {
      return {
        kind: 'mcr-equivalent-exists',
        suggestion: MCR_COVERAGE.python.suggestion.replace(/<V>/g, mm),
      };
    }
    return {
      kind: 'no-mcr-equivalent',
      reason: `Python ${mm ?? '(unparsed)'} not in MCR (publishes 3.11/3.12)`,
    };
  }

  // 5. .NET (non-MCR variants, e.g. bitnami) — MCR covers 8.0, 9.0.
  if (DOTNET_REPO.test(repo)) {
    const mm = extractDotnetMajorMinor(tag);
    if (mm !== null && (MCR_COVERAGE.dotnet.versions as Set<string>).has(mm)) {
      return {
        kind: 'mcr-equivalent-exists',
        suggestion: MCR_COVERAGE.dotnet.suggestion.replace(/<V>/g, mm),
      };
    }
    return {
      kind: 'no-mcr-equivalent',
      reason: `.NET ${mm ?? '(unparsed)'} not in MCR (publishes 8.0/9.0)`,
    };
  }

  // 6. Stacks with no MCR equivalent at all (Maven, Tomcat, Wildfly, Go, …)
  for (const { pattern, reason } of NO_MCR_EQUIVALENT_REPOS) {
    if (pattern.test(repo)) return { kind: 'no-mcr-equivalent', reason };
  }

  // 7. Anything else — unknown stack. Be lenient: report but don't fail.
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
      const parsed = parseFromLine(lines[i], i);
      if (!parsed) continue;
      const coverage = classifyBase(parsed);
      switch (coverage.kind) {
        case 'mcr':
          break;
        case 'no-mcr-equivalent':
          allowed.push(`${parsed.raw.trim()}  // ${coverage.reason}`);
          break;
        case 'unknown':
          // Lenient default: unknown stacks (custom internal registries,
          // niche bases) are allowed but recorded for visibility.
          allowed.push(`${parsed.raw.trim()}  // unknown stack — not flagged`);
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
        message: `${offending.length} FROM line(s) use a non-MCR base while MCR has an equivalent`,
        details: offending.join('\n') + (allowed.length ? `\n--- allowed (no MCR equivalent / annotated):\n${allowed.join('\n')}` : ''),
      };
    }
    const summary = allowed.length
      ? `All FROM lines OK (${fromCount} total; ${allowed.length} non-MCR but no MCR equivalent or annotated)`
      : `All ${fromCount} FROM line(s) use mcr.microsoft.com`;
    return { name: this.name, passed: true, message: summary, details: allowed.length ? allowed.join('\n') : undefined };
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
    } else if (!new RegExp(`LABEL[\\s\\S]*?${REQUIRED_DOCKERFILE_LABEL}`).test(dockerfile)) {
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
          // Non-k8s YAML files (eslint configs, mkdocs configs, etc.) may
          // still slip past the directory filter — silently skip parse errors
          // rather than treating them as label violations.
          continue;
        }
        if (!parsed || typeof parsed !== 'object') continue;
        // Only enforce labels on actual Kubernetes objects (must have both
        // `apiVersion` and `kind`). Skips application config YAML, rule
        // files, etc. that happen to live in the working tree.
        const obj = parsed as { apiVersion?: unknown; kind?: unknown; metadata?: { labels?: Record<string, string> } };
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
    try {
      await execFileP('docker', ['build', '-t', tag, artifactDir], { maxBuffer: 16 * 1024 * 1024 });
    } catch (err) {
      const e = err as { code?: string; stderr?: string; message?: string };
      if (e.code === 'ENOENT') {
        return { name: this.name, passed: false, message: 'docker not available on PATH' };
      }
      return {
        name: this.name,
        passed: false,
        message: 'docker build failed',
        details: (e.stderr ?? e.message ?? '').slice(-2000),
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

export const ALL_CHECKS: readonly Check[] = [requiresAzureBaseImage, hasRequiredLabels, dockerBuilds];

export function selectChecks(spec: string): Check[] {
  const trimmed = spec.trim();
  if (trimmed === 'all') return [...ALL_CHECKS];
  if (trimmed === 'none' || trimmed === '') return [];
  const requested = trimmed.split(',').map((s) => s.trim()).filter(Boolean);
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
