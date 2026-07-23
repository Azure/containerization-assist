import { promises as fs } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';

/**
 * Maven CI reachability shim (Track B).
 *
 * The eval builds the agent-generated Dockerfile inside a network-isolated
 * pipeline where Maven Central (`repo.maven.apache.org`) is BLOCKED. Left alone,
 * every `mvn`/`mvnw` build fails at dependency download — an *environment*
 * artifact, not a signal about the agent. That failure hits bare/mcp/skills
 * equally and flattens the comparison.
 *
 * This module neutralizes that artifact identically for all modes: before a
 * `docker build`, it (1) mounts a `settings.xml` that mirrors Maven Central to
 * the reachable Azure Artifacts feed (`Kubernetes_PublicPackages`, which proxies
 * Central) as a BuildKit secret at `/root/.m2/settings.xml`, and (2) repoints the
 * Maven Wrapper distribution + supplies wrapper credentials so `./mvnw` can
 * bootstrap. The OAuth token arrives only via BuildKit secrets (tmpfs) and is
 * never baked into an image layer.
 *
 * Entirely gated behind AGENT_EVAL_MAVEN_MIRROR — local/dev runs are untouched.
 */

// Azure Artifacts feed that already proxies Maven Central (verified reachable
// from inside `docker build` with the pipeline's System.AccessToken).
const DEFAULT_FEED_URL =
  'https://pkgs.dev.azure.com/AzureContainerUpstream/Kubernetes/_packaging/Kubernetes_PublicPackages/maven/v1';

// BuildKit secret ids + in-container mount targets.
const SETTINGS_SECRET_ID = 'm2settings';
const SETTINGS_TARGET = '/root/.m2/settings.xml';
const TOKEN_SECRET_ID = 'aztoken';
const TOKEN_TARGET = '/tmp/.aztoken';

// Marker proving a Dockerfile was already transformed (keeps the rewrite
// idempotent when both build paths prep the same working dir).
const INJECT_MARKER = `id=${SETTINGS_SECRET_ID}`;

export interface MavenMirrorConfig {
  feedUrl: string;
  token: string;
}

/** Resolve the mirror config from env, or null when disabled / unconfigured. */
export function mavenMirrorConfig(env: NodeJS.ProcessEnv = process.env): MavenMirrorConfig | null {
  const flag = (env.AGENT_EVAL_MAVEN_MIRROR ?? '').trim().toLowerCase();
  if (flag !== '1' && flag !== 'true' && flag !== 'yes') return null;
  const token = (env.AGENT_EVAL_MAVEN_TOKEN ?? env.SYSTEM_ACCESSTOKEN ?? '').trim();
  if (!token) return null;
  const feedUrl = (env.AGENT_EVAL_MAVEN_FEED_URL ?? DEFAULT_FEED_URL).trim();
  return { feedUrl, token };
}

/** Maven settings.xml mirroring every external repo to the feed, with creds. */
export function buildCiSettingsXml(cfg: MavenMirrorConfig): string {
  const url = escapeXml(cfg.feedUrl);
  const pw = escapeXml(cfg.token);
  return `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.0.0">
  <mirrors>
    <mirror>
      <id>ci-feed</id>
      <name>Azure Artifacts Maven Central mirror</name>
      <mirrorOf>external:*</mirrorOf>
      <url>${url}</url>
    </mirror>
  </mirrors>
  <servers>
    <server>
      <id>ci-feed</id>
      <username>build</username>
      <password>${pw}</password>
    </server>
  </servers>
</settings>
`;
}

function escapeXml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

const MVN_INVOKE = /(^|[\s;&|(])(?:\.?\/)?mvnw?\b/;
const MVNW_INVOKE = /(^|[\s;&|(])(?:\.?\/)?mvnw\b/;

/**
 * Add BuildKit secret mounts to every `RUN` that invokes `mvn`/`mvnw`, so Maven
 * reads the mirror settings.xml (and `mvnw` can authenticate its bootstrap
 * download). Pure + idempotent for unit testing. Handles shell-form RUN with
 * backslash line continuations; exec-form (`RUN ["..."]`) is left untouched.
 */
export function transformDockerfileForMaven(content: string): { content: string; changed: boolean } {
  if (content.includes(INJECT_MARKER)) return { content, changed: false };

  const lines = content.split('\n');
  let changed = false;

  for (let i = 0; i < lines.length; i++) {
    const m = /^(\s*)RUN\s+/i.exec(lines[i]);
    if (!m) continue;

    // Gather the full logical instruction across backslash continuations.
    const start = i;
    let end = i;
    while (end < lines.length && /\\\s*$/.test(lines[end])) end++;
    const instruction = lines.slice(start, end + 1).join('\n');

    // Skip exec-form RUN (JSON array) — prefixing would corrupt it.
    const afterRun = instruction.slice(instruction.toUpperCase().indexOf('RUN') + 3).trimStart();
    if (afterRun.startsWith('[')) {
      i = end;
      continue;
    }

    if (MVN_INVOKE.test(instruction)) {
      const isWrapper = MVNW_INVOKE.test(instruction);
      const mounts = isWrapper
        ? `--mount=type=secret,id=${SETTINGS_SECRET_ID},target=${SETTINGS_TARGET} ` +
          `--mount=type=secret,id=${TOKEN_SECRET_ID},target=${TOKEN_TARGET} `
        : `--mount=type=secret,id=${SETTINGS_SECRET_ID},target=${SETTINGS_TARGET} `;
      const envPrefix = isWrapper
        ? `MVNW_USERNAME=build MVNW_PASSWORD="$(cat ${TOKEN_TARGET})" `
        : '';
      const indent = m[1];
      lines[start] = lines[start].replace(
        /^(\s*)RUN\s+/i,
        `${indent}RUN ${mounts}${envPrefix}`,
      );
      changed = true;
    }
    i = end;
  }

  return { content: lines.join('\n'), changed };
}

/**
 * Repoint Maven Wrapper `distributionUrl` (and `wrapperUrl`) from blocked Maven
 * Central to the reachable feed. Returns the count of files rewritten.
 */
export async function repointMavenWrapper(buildDir: string, cfg: MavenMirrorConfig): Promise<number> {
  const candidates = [
    join(buildDir, '.mvn', 'wrapper', 'maven-wrapper.properties'),
  ];
  let rewritten = 0;
  for (const file of candidates) {
    let text: string;
    try {
      text = await fs.readFile(file, 'utf8');
    } catch {
      continue;
    }
    const next = text.replace(
      /^(\s*(?:distributionUrl|wrapperUrl)\s*=\s*)https?:\/\/repo\.maven\.apache\.org\/maven2\/(.*)$/gim,
      (_full, prefix: string, path: string) => `${prefix}${cfg.feedUrl}/${path}`,
    );
    if (next !== text) {
      await fs.writeFile(file, next, 'utf8');
      rewritten++;
    }
  }
  return rewritten;
}

export interface MavenBuildPrep {
  /** Extra CLI args to splice into the `docker build` invocation. */
  secretArgs: string[];
  /** Remove host-side temp secret files. */
  cleanup(): Promise<void>;
}

/**
 * Prepare `buildDir` for a Maven-aware `docker build`. No-op (returns null) when
 * the mirror is disabled, no token is present, or the dir is not a Maven
 * project. Otherwise transforms the Dockerfile, repoints the wrapper, writes the
 * secret files, and returns the `--secret` args + a cleanup handle.
 */
export async function prepareMavenBuild(
  buildDir: string,
  env: NodeJS.ProcessEnv = process.env,
): Promise<MavenBuildPrep | null> {
  const cfg = mavenMirrorConfig(env);
  if (!cfg) return null;

  // Only act on Maven projects.
  try {
    await fs.access(join(buildDir, 'pom.xml'));
  } catch {
    return null;
  }

  const dockerfilePath = join(buildDir, 'Dockerfile');
  let dockerfile: string;
  try {
    dockerfile = await fs.readFile(dockerfilePath, 'utf8');
  } catch {
    return null;
  }

  const { content, changed } = transformDockerfileForMaven(dockerfile);
  if (changed) await fs.writeFile(dockerfilePath, content, 'utf8');
  await repointMavenWrapper(buildDir, cfg);

  // Write secrets to a private temp dir (mode 0600) — never into the context.
  const secretDir = await fs.mkdtemp(join(tmpdir(), 'agent-eval-mvn-'));
  const settingsPath = join(secretDir, 'settings.xml');
  const tokenPath = join(secretDir, 'aztoken');
  await fs.writeFile(settingsPath, buildCiSettingsXml(cfg), { mode: 0o600 });
  await fs.writeFile(tokenPath, cfg.token, { mode: 0o600 });

  return {
    secretArgs: [
      '--secret',
      `id=${SETTINGS_SECRET_ID},src=${settingsPath}`,
      '--secret',
      `id=${TOKEN_SECRET_ID},src=${tokenPath}`,
    ],
    async cleanup() {
      await fs.rm(secretDir, { recursive: true, force: true });
    },
  };
}
