import {
  parseImageRef,
  splitMcrRef,
  detectStack,
  canonicalMcrRef,
  suggestMcrFix,
  KNOWN_GOOD_MCR_REFS,
  MCR_BASE_CATALOG,
} from '@/knowledge/base-image-catalog';

describe('base-image-catalog parseImageRef', () => {
  it('parses an MCR ref with tag', () => {
    const p = parseImageRef('mcr.microsoft.com/openjdk/jdk:8-azurelinux');
    expect(p.repo).toBe('mcr.microsoft.com/openjdk/jdk');
    expect(p.tag).toBe('8-azurelinux');
    expect(p.isMcr).toBe(true);
    expect(p.digest).toBeNull();
  });

  it('does not treat a host:port as a tag', () => {
    const p = parseImageRef('localhost:5000/team/app');
    expect(p.repo).toBe('localhost:5000/team/app');
    expect(p.tag).toBeNull();
  });

  it('captures a digest', () => {
    const p = parseImageRef('node@sha256:abcd');
    expect(p.repo).toBe('node');
    expect(p.digest).toBe('sha256:abcd');
  });

  it('splitMcrRef strips the mcr host', () => {
    expect(splitMcrRef('mcr.microsoft.com/openjdk/jdk:17-azurelinux')).toEqual({
      repo: 'openjdk/jdk',
      tag: '17-azurelinux',
    });
  });
});

describe('base-image-catalog detectStack', () => {
  it.each([
    ['eclipse-temurin:17-jdk', 'java', '17'],
    ['mcr.microsoft.com/java/jre:8u372-zulu-ubuntu', 'java', '8'],
    ['openjdk:11-jre-slim', 'java', '11'],
    ['node:20-alpine', 'node', '20'],
    ['python:3.12-slim', 'python', '3.12'],
    ['mcr.microsoft.com/dotnet/sdk:8.0', 'dotnet', '8.0'],
  ])('detects %s as %s %s', (ref, stack, version) => {
    expect(detectStack(ref)).toEqual({ stack, version });
  });

  it.each(['golang:1.21-alpine', 'maven:3.9-eclipse-temurin-17', 'tomcat:9-jdk11', 'nginx:1.25'])(
    'returns null for non-MCR stack %s',
    (ref) => {
      expect(detectStack(ref)).toBeNull();
    },
  );
});

describe('base-image-catalog canonicalMcrRef / suggestMcrFix', () => {
  it('resolves a covered java version', () => {
    expect(canonicalMcrRef('java', '8', 'build')).toBe(
      'mcr.microsoft.com/openjdk/jdk:8-azurelinux',
    );
    expect(canonicalMcrRef('java', '8', 'runtime')).toBe(
      'mcr.microsoft.com/openjdk/jdk:8-distroless',
    );
  });

  it('returns null for a node version MCR does not publish (18/22)', () => {
    expect(canonicalMcrRef('node', '18')).toBeNull();
    expect(canonicalMcrRef('node', '22')).toBeNull();
    expect(canonicalMcrRef('node', '20')).toBe('mcr.microsoft.com/azurelinux/base/nodejs:20');
  });

  it('suggests a fix for a hallucinated MCR java tag', () => {
    const fix = suggestMcrFix('mcr.microsoft.com/java/jre:8u372-zulu-ubuntu');
    expect(fix).not.toBeNull();
    expect(fix!.stack).toBe('java');
    expect(fix!.version).toBe('8');
    expect(fix!.build).toBe('mcr.microsoft.com/openjdk/jdk:8-azurelinux');
  });

  it('does not suggest a fix when the exact version is absent (no silent major bump)', () => {
    expect(suggestMcrFix('python:3.11-slim')).toBeNull();
  });
});

// Live guard: proves the shipped catalog contains only real, pullable tags.
// Skips automatically when MCR is unreachable so it never breaks offline CI.
describe('base-image-catalog live manifest verification', () => {
  const refs = [...KNOWN_GOOD_MCR_REFS];

  async function manifestStatus(ref: string): Promise<number | null> {
    const slash = ref.indexOf('/');
    const path = ref.slice(slash + 1); // drop host
    const lastColon = path.lastIndexOf(':');
    const repo = path.slice(0, lastColon);
    const tag = path.slice(lastColon + 1);
    try {
      const res = await fetch(`https://mcr.microsoft.com/v2/${repo}/manifests/${tag}`, {
        method: 'HEAD',
        headers: {
          Accept:
            'application/vnd.docker.distribution.manifest.list.v2+json,application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.v2+json',
        },
        signal: AbortSignal.timeout(8000),
      });
      return res.status;
    } catch {
      return null; // offline / blocked
    }
  }

  it('every catalog ref returns 200 from MCR (or is skipped offline)', async () => {
    const first = await manifestStatus(refs[0]);
    if (first === null) {
      console.warn('MCR unreachable — skipping live catalog verification');
      return;
    }
    for (const ref of refs) {
      const status = await manifestStatus(ref);
      if (status === null) continue; // transient; don't fail the suite offline
      expect({ ref, status }).toEqual({ ref, status: 200 });
    }
  }, 60000);
});

describe('base-image-catalog shape', () => {
  it('every version entry has build+runtime refs on mcr.microsoft.com', () => {
    for (const stack of Object.values(MCR_BASE_CATALOG)) {
      for (const [, v] of Object.entries(stack.versions)) {
        expect(v.build.startsWith('mcr.microsoft.com/')).toBe(true);
        expect(v.runtime.startsWith('mcr.microsoft.com/')).toBe(true);
      }
      expect(stack.versions[stack.defaultVersion]).toBeDefined();
    }
  });
});
