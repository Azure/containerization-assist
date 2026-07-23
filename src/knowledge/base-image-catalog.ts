/**
 * Minimal, manifest-verified catalog of Microsoft Container Registry (MCR) base
 * images, plus pure helpers to parse image references and map a recognised
 * stack+version onto a canonical, real MCR tag.
 *
 * Why this exists: the agent frequently emits MCR-shaped base tags that do not
 * actually exist (e.g. `mcr.microsoft.com/java/jre:8u372-zulu-ubuntu`), which
 * pass a naive "looks like MCR" check but fail the real build with
 * `image not found`. This module provides the canonical, verified references so
 * both the eval checks and the fix-dockerfile tool can substitute a tag that
 * genuinely pulls.
 *
 * Every tag below was verified to return HTTP 200 from
 * `https://mcr.microsoft.com/v2/<repo>/manifests/<tag>`. Do NOT add a version
 * without checking the manifest first — notably MCR publishes only Node major
 * `20` and Python `3.12` on Azure Linux (18/22 and 3.11 do NOT exist).
 *
 * This is intentionally a typed module (not a JSON file + loader) so it is safe
 * under the ESM+CJS dual build and requires no runtime filesystem access.
 */

export type Stack = 'java' | 'node' | 'python' | 'dotnet';

/** A canonical MCR reference pair for a single language version. */
export interface McrVersionRefs {
  /** Full MCR ref suitable for a build stage (contains the toolchain/SDK). */
  build: string;
  /** Full MCR ref suitable for a slim runtime stage (distroless where available). */
  runtime: string;
}

export interface McrStackCatalog {
  stack: Stack;
  displayName: string;
  /** Verified canonical refs keyed by the language version string. */
  versions: Record<string, McrVersionRefs>;
  /** Default version to use when the source version is unknown/unparseable. */
  defaultVersion: string;
}

/**
 * Verified MCR catalog. Java uses a single-major tag (`8`, not `1.8`/`8u362`);
 * Node/Python use Azure Linux base+distroless; .NET uses `-azurelinux3.0`
 * across sdk/aspnet/runtime.
 */
export const MCR_BASE_CATALOG: Record<Stack, McrStackCatalog> = {
  java: {
    stack: 'java',
    displayName: 'Java (OpenJDK)',
    defaultVersion: '21',
    versions: {
      '8': {
        build: 'mcr.microsoft.com/openjdk/jdk:8-azurelinux',
        runtime: 'mcr.microsoft.com/openjdk/jdk:8-distroless',
      },
      '11': {
        build: 'mcr.microsoft.com/openjdk/jdk:11-azurelinux',
        runtime: 'mcr.microsoft.com/openjdk/jdk:11-distroless',
      },
      '17': {
        build: 'mcr.microsoft.com/openjdk/jdk:17-azurelinux',
        runtime: 'mcr.microsoft.com/openjdk/jdk:17-distroless',
      },
      '21': {
        build: 'mcr.microsoft.com/openjdk/jdk:21-azurelinux',
        runtime: 'mcr.microsoft.com/openjdk/jdk:21-distroless',
      },
      '25': {
        build: 'mcr.microsoft.com/openjdk/jdk:25-azurelinux',
        runtime: 'mcr.microsoft.com/openjdk/jdk:25-distroless',
      },
    },
  },
  node: {
    stack: 'node',
    displayName: 'Node.js',
    defaultVersion: '20',
    versions: {
      // Azure Linux publishes only Node 20 (18 and 22 return 404).
      '20': {
        build: 'mcr.microsoft.com/azurelinux/base/nodejs:20',
        runtime: 'mcr.microsoft.com/azurelinux/distroless/nodejs:20',
      },
    },
  },
  python: {
    stack: 'python',
    displayName: 'Python',
    defaultVersion: '3.12',
    versions: {
      // Azure Linux publishes only Python 3.12 (3.11 returns 404).
      '3.12': {
        build: 'mcr.microsoft.com/azurelinux/base/python:3.12',
        runtime: 'mcr.microsoft.com/azurelinux/distroless/python:3.12',
      },
    },
  },
  dotnet: {
    stack: 'dotnet',
    displayName: '.NET',
    defaultVersion: '8.0',
    versions: {
      '8.0': {
        build: 'mcr.microsoft.com/dotnet/sdk:8.0-azurelinux3.0',
        runtime: 'mcr.microsoft.com/dotnet/aspnet:8.0-azurelinux3.0',
      },
      '9.0': {
        build: 'mcr.microsoft.com/dotnet/sdk:9.0-azurelinux3.0',
        runtime: 'mcr.microsoft.com/dotnet/aspnet:9.0-azurelinux3.0',
      },
      '10.0': {
        build: 'mcr.microsoft.com/dotnet/sdk:10.0-azurelinux3.0',
        runtime: 'mcr.microsoft.com/dotnet/aspnet:10.0-azurelinux3.0',
      },
    },
  },
};

/** Every canonical ref we ship, for fast "is this a tag we know is real" checks. */
export const KNOWN_GOOD_MCR_REFS: ReadonlySet<string> = new Set(
  Object.values(MCR_BASE_CATALOG).flatMap((s) =>
    Object.values(s.versions).flatMap((v) => [v.build, v.runtime]),
  ),
);

export interface ParsedImageRef {
  /** The full reference minus any digest, e.g. `mcr.microsoft.com/openjdk/jdk`. */
  repo: string;
  /** Tag after the last `:` (when present and not a registry port), else null. */
  tag: string | null;
  /** Digest after `@`, else null. */
  digest: string | null;
  /** True when the repo is hosted on mcr.microsoft.com. */
  isMcr: boolean;
}

const MCR_HOST = 'mcr.microsoft.com/';

/**
 * Parse an image reference into repo/tag/digest. Handles `--platform` prefixes
 * elsewhere; pass only the image token here (e.g. the first arg after FROM).
 */
export function parseImageRef(image: string): ParsedImageRef {
  let ref = image.trim();
  let digest: string | null = null;
  const at = ref.indexOf('@');
  if (at >= 0) {
    digest = ref.slice(at + 1) || null;
    ref = ref.slice(0, at);
  }
  // Tag is the segment after the last `:` that follows the last `/`, so we
  // don't mistake a `host:port` for a tag.
  const lastSlash = ref.lastIndexOf('/');
  const lastColon = ref.lastIndexOf(':');
  let repo = ref;
  let tag: string | null = null;
  if (lastColon > lastSlash) {
    repo = ref.slice(0, lastColon);
    tag = ref.slice(lastColon + 1) || null;
  }
  return {
    repo,
    tag,
    digest,
    isMcr: repo.toLowerCase().startsWith(MCR_HOST),
  };
}

/** Split an MCR ref into its registry-relative repo path and tag. */
export function splitMcrRef(ref: string): { repo: string; tag: string | null } {
  const parsed = parseImageRef(ref);
  const repo = parsed.repo.toLowerCase().startsWith(MCR_HOST)
    ? parsed.repo.slice(MCR_HOST.length)
    : parsed.repo;
  return { repo, tag: parsed.tag };
}

// --- Stack + version detection ---------------------------------------------

const JAVA_REPO = new RegExp(
  `(?:^|/)(?:${[
    'openjdk',
    'jdk',
    'jre',
    'java',
    'eclipse-temurin',
    'temurin',
    'amazoncorretto',
    'adoptopenjdk',
    'ibmjava',
    'sapmachine',
    'zulu-openjdk',
    'liberica-openjdk[\\w-]*',
  ].join('|')})$`,
  'i',
);
const NODE_REPO = /(?:^|\/)(?:node|nodejs)$/i;
const PYTHON_REPO = /(?:^|\/)python$/i;
const DOTNET_REPO = /(?:^|\/)dotnet\/(?:sdk|aspnet|runtime)$/i;

/** Java major from a tag: `17-jdk` → 17, `1.8.0_345` → 8, `8u362-zulu` → 8. */
function javaMajor(tag: string | null): string | null {
  if (!tag) return null;
  const oneDot = /^1\.(\d+)(?:[._-].*)?$/.exec(tag);
  if (oneDot) return oneDot[1] ?? null;
  const leading = /^(\d+)(?:u\d+)?(?:[.\-_].*)?$/.exec(tag);
  return leading ? (leading[1] ?? null) : null;
}

/** Node major from a tag: `20-alpine` → 20. */
function nodeMajor(tag: string | null): string | null {
  if (!tag) return null;
  const m = /^(\d+)(?:[.\-_].*)?$/.exec(tag);
  return m ? (m[1] ?? null) : null;
}

/** Python/.NET major.minor from a tag: `3.11-slim` → `3.11`, `8.0-x` → `8.0`. */
function majorMinor(tag: string | null): string | null {
  if (!tag) return null;
  const m = /^(\d+\.\d+)(?:[.\-_].*)?$/.exec(tag);
  return m ? (m[1] ?? null) : null;
}

export interface DetectedStack {
  stack: Stack;
  /** The version string as it maps into the catalog (may be absent from it). */
  version: string | null;
}

/**
 * Best-effort detection of the language stack and version from any base image
 * reference (MCR or public). Returns null for unrecognised stacks (Go, Rust,
 * Maven builders, app servers, generic OS bases, …) — those have no MCR
 * equivalent and must not be "fixed" into one.
 */
export function detectStack(ref: string): DetectedStack | null {
  const { repo, tag } = splitMcrRef(ref);
  const normalized = repo.replace(/^docker\.io\//i, '').replace(/^library\//i, '');
  if (DOTNET_REPO.test(normalized)) return { stack: 'dotnet', version: majorMinor(tag) };
  if (NODE_REPO.test(normalized)) return { stack: 'node', version: nodeMajor(tag) };
  if (PYTHON_REPO.test(normalized)) return { stack: 'python', version: majorMinor(tag) };
  if (JAVA_REPO.test(normalized)) return { stack: 'java', version: javaMajor(tag) };
  return null;
}

export type BuildStage = 'build' | 'runtime';

/**
 * The canonical, verified MCR ref for a stack+version, or null when this exact
 * version is not published on MCR. Never silently changes the major version — a
 * missing version returns null so the caller can decide (e.g. flag a coverage
 * gap) rather than substituting a different runtime.
 */
export function canonicalMcrRef(
  stack: Stack,
  version: string | null,
  stage: BuildStage = 'build',
): string | null {
  const cat = MCR_BASE_CATALOG[stack];
  const entry = version ? cat.versions[version] : undefined;
  if (!entry) return null;
  return entry[stage];
}

export interface McrFixSuggestion {
  stack: Stack;
  version: string;
  build: string;
  runtime: string;
}

/**
 * Given any base image reference, suggest a canonical MCR replacement when the
 * stack+version is covered by the verified catalog. Returns null when the stack
 * is unrecognised or the specific version is not on MCR (no safe substitution).
 */
export function suggestMcrFix(ref: string): McrFixSuggestion | null {
  const detected = detectStack(ref);
  if (!detected) return null;
  const { stack, version } = detected;
  if (!version) return null;
  const build = canonicalMcrRef(stack, version, 'build');
  const runtime = canonicalMcrRef(stack, version, 'runtime');
  if (!build || !runtime) return null;
  return { stack, version, build, runtime };
}
