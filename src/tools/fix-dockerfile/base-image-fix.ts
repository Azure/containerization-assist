/**
 * Base-image correction for fix-dockerfile.
 *
 * The single most common reason an agent-generated Dockerfile fails to build in
 * the pipeline is a bad base image: either a hallucinated `mcr.microsoft.com`
 * tag that does not exist, or a public (Docker Hub) image that is unreachable
 * in-network but has a verified MCR equivalent. This module detects those cases
 * and rewrites ONLY the offending image token on each `FROM` line, preserving
 * comments, `--platform` flags, `AS <stage>` aliases, and every other line —
 * unlike the lossy full-reconstruction fixer.
 */

import { suggestMcrFix, type BuildStage } from '@/knowledge/base-image-catalog';
import { mcrRefExists } from '@/validation/mcr-registry';

export interface BaseImageSubstitution {
  /** 1-based line number of the rewritten FROM instruction. */
  line: number;
  stage: BuildStage;
  original: string;
  replacement: string;
  reason: 'hallucinated-mcr-tag' | 'non-mcr-with-equivalent';
}

export interface BaseImageFixResult {
  substitutions: BaseImageSubstitution[];
  /** The corrected Dockerfile, or null when nothing was changed. */
  fixedDockerfile: string | null;
}

const FROM_RE = /^(\s*FROM\s+(?:--platform=\S+\s+)?)(\S+)(.*)$/i;

interface FromLine {
  idx: number; // index into the lines array
  prefix: string;
  image: string;
  rest: string;
}

function collectFromLines(lines: string[]): FromLine[] {
  const froms: FromLine[] = [];
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (line === undefined) continue;
    const m = FROM_RE.exec(line);
    if (m?.[1] === undefined || m[2] === undefined) continue;
    froms.push({ idx: i, prefix: m[1], image: m[2], rest: m[3] ?? '' });
  }
  return froms;
}

/** Strip a trailing `@sha256:...` digest from an image ref. */
function stripDigest(image: string): string {
  const at = image.indexOf('@');
  return at >= 0 ? image.slice(0, at) : image;
}

function isMcrRef(image: string): boolean {
  return /^mcr\.microsoft\.com\//i.test(image);
}

/**
 * Analyze a Dockerfile and, where a base image is provably wrong (hallucinated
 * MCR tag) or replaceable (non-MCR with a verified MCR equivalent), produce a
 * corrected Dockerfile via surgical per-line substitution.
 *
 * Stage selection: in a multi-stage build the final `FROM` is treated as the
 * runtime stage (smaller distroless image); all earlier stages and any
 * single-stage build use the build image (full SDK), which both compiles and
 * runs safely.
 */
export async function fixBaseImages(dockerfile: string): Promise<BaseImageFixResult> {
  const lines = dockerfile.split('\n');
  const froms = collectFromLines(lines);
  if (froms.length === 0) return { substitutions: [], fixedDockerfile: null };

  const multiStage = froms.length > 1;
  const substitutions: BaseImageSubstitution[] = [];

  for (let p = 0; p < froms.length; p++) {
    const from = froms[p];
    if (from === undefined) continue;
    const stage: BuildStage = multiStage && p === froms.length - 1 ? 'runtime' : 'build';
    const image = stripDigest(from.image);

    let reason: BaseImageSubstitution['reason'];
    if (isMcrRef(image)) {
      // An MCR-shaped ref only needs fixing if the tag does not actually exist.
      const exists = await mcrRefExists(image);
      if (exists !== false) continue; // real (true) or undetermined (null) → leave it
      reason = 'hallucinated-mcr-tag';
    } else {
      reason = 'non-mcr-with-equivalent';
    }

    const fix = suggestMcrFix(image);
    if (!fix) continue; // no verified MCR replacement for this stack+version

    const replacement = stage === 'runtime' ? fix.runtime : fix.build;
    if (replacement === image) continue; // already canonical

    lines[from.idx] = `${from.prefix}${replacement}${from.rest}`;
    substitutions.push({
      line: from.idx + 1,
      stage,
      original: from.image,
      replacement,
      reason,
    });
  }

  if (substitutions.length === 0) return { substitutions: [], fixedDockerfile: null };
  return { substitutions, fixedDockerfile: lines.join('\n') };
}
