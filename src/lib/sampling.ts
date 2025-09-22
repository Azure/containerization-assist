/**
 * Sampling Types and Core Interface
 * Provides cohesive sampling functionality for MCP tools
 */

import { createCache } from '@/lib/cache';
import crypto from 'crypto';

export interface SamplingCandidate {
  /** Unique identifier for the candidate */
  id: string;
  /** Generated content (Dockerfile, YAML, etc.) */
  content: string;
  /** Overall score (0-100) */
  score: number;
  /** Breakdown of scoring components */
  scoreBreakdown: Record<string, number>;
  /** Rank among all candidates (1-based) */
  rank?: number;
}

export interface SamplingOptions {
  /** Enable multi-candidate sampling */
  enableSampling?: boolean;
  /** Maximum candidates to generate (1-10) */
  maxCandidates?: number;
  /** Score threshold for early stopping (0-100) */
  earlyStopThreshold?: number;
  /** Include score breakdown in response */
  includeScoreBreakdown?: boolean;
  /** Return all candidates, not just winner */
  returnAllCandidates?: boolean;
  /** Use caching for repeated requests */
  useCache?: boolean;
}

export interface SamplingResult<T> {
  /** The winning candidate with highest score */
  winner: T & {
    score: number;
    scoreBreakdown?: Record<string, number>;
    rank?: number;
  };
  /** All candidates if returnAllCandidates=true */
  allCandidates?: SamplingCandidate[];
  /** Sampling execution metadata */
  samplingMetadata?: {
    /** Whether early stopping was triggered */
    stoppedEarly?: boolean;
    /** Number of candidates actually generated */
    candidatesGenerated: number;
    /** Score of the winning candidate */
    winnerScore: number;
    /** Total time spent on sampling */
    samplingDuration?: number;
  };
}

export interface ScoringWeights {
  [key: string]: number;
}

// Create cache instance for sampling
const samplingCache = createCache<SamplingCandidate>('sampling', {
  ttlMs: 15 * 60 * 1000, // 15 minutes
  maxSize: 100,
});

/**
 * Simple sampling function that generates and scores candidates
 */
export async function sampleCandidates(
  generate: () => Promise<string>,
  score: (content: string) => number | Record<string, number>,
  options: {
    count?: number;
    stopAt?: number;
    includeBreakdown?: boolean;
  } = {},
): Promise<SamplingCandidate> {
  const config = {
    count: options.count ?? 3,
    stopAt: options.stopAt ?? 95,
    includeBreakdown: options.includeBreakdown ?? false,
  };

  const candidates: SamplingCandidate[] = [];

  for (let i = 0; i < config.count; i++) {
    const content = await generate();
    const scoreResult = score(content);

    const totalScore =
      typeof scoreResult === 'number'
        ? scoreResult
        : Object.values(scoreResult).reduce((sum, val) => sum + val, 0) /
          Object.keys(scoreResult).length;

    const candidate: SamplingCandidate = {
      id: crypto.randomBytes(8).toString('hex'),
      content,
      score: totalScore,
      scoreBreakdown: typeof scoreResult === 'object' ? scoreResult : { overall: scoreResult },
    };

    candidates.push(candidate);

    // Early stopping if we hit a high score
    if (totalScore >= config.stopAt) {
      break;
    }
  }

  // Sort by score and assign ranks
  candidates.sort((a, b) => b.score - a.score);
  candidates.forEach((candidate, index) => {
    candidate.rank = index + 1;
  });

  return (
    candidates[0] || {
      id: crypto.randomBytes(8).toString('hex'),
      content: '',
      score: 0,
      scoreBreakdown: { overall: 0 },
      rank: 1,
    }
  );
}

/**
 * Sample with caching support
 */
export async function sampleWithCache(
  key: string,
  generate: () => Promise<string>,
  score: (content: string) => number | Record<string, number>,
  options: {
    count?: number;
    stopAt?: number;
    includeBreakdown?: boolean;
    useCache?: boolean;
  } = {},
): Promise<SamplingCandidate> {
  // Check cache first if enabled
  if (options.useCache !== false) {
    const cached = samplingCache.get(key);
    if (cached) {
      return cached;
    }
  }

  const best = await sampleCandidates(generate, score, options);

  // Cache the result
  if (options.useCache !== false) {
    samplingCache.set(key, best);
  }

  return best;
}

/**
 * Full sampling with result metadata
 */
export async function sample<T extends { content: string }>(
  generate: () => Promise<string>,
  score: (content: string) => number | Record<string, number>,
  transform: (candidate: SamplingCandidate) => T,
  options: SamplingOptions = {},
): Promise<SamplingResult<T>> {
  const startTime = Date.now();

  const config = {
    maxCandidates: options.maxCandidates ?? 3,
    earlyStopThreshold: options.earlyStopThreshold ?? 95,
    includeScoreBreakdown: options.includeScoreBreakdown ?? false,
    returnAllCandidates: options.returnAllCandidates ?? false,
  };

  const candidates: SamplingCandidate[] = [];
  let stoppedEarly = false;

  // Generate candidates
  for (let i = 0; i < config.maxCandidates; i++) {
    const content = await generate();
    const scoreResult = score(content);

    const totalScore =
      typeof scoreResult === 'number'
        ? scoreResult
        : Object.values(scoreResult).reduce((sum, val) => sum + val, 0) /
          Object.keys(scoreResult).length;

    const candidate: SamplingCandidate = {
      id: crypto.randomBytes(8).toString('hex'),
      content,
      score: totalScore,
      scoreBreakdown: typeof scoreResult === 'object' ? scoreResult : { overall: scoreResult },
    };

    candidates.push(candidate);

    // Early stopping
    if (totalScore >= config.earlyStopThreshold) {
      stoppedEarly = true;
      break;
    }
  }

  // Sort and rank
  candidates.sort((a, b) => b.score - a.score);
  candidates.forEach((candidate, index) => {
    candidate.rank = index + 1;
  });

  const winner = candidates[0] || {
    id: crypto.randomBytes(8).toString('hex'),
    content: '',
    score: 0,
    scoreBreakdown: { overall: 0 },
    rank: 1,
  };
  const transformedWinner = transform(winner);

  const result: SamplingResult<T> = {
    winner: {
      ...transformedWinner,
      score: winner.score,
      ...(config.includeScoreBreakdown && { scoreBreakdown: winner.scoreBreakdown }),
      ...(winner.rank !== undefined && { rank: winner.rank }),
    },
    ...(config.returnAllCandidates && { allCandidates: candidates }),
    samplingMetadata: {
      stoppedEarly,
      candidatesGenerated: candidates.length,
      winnerScore: winner.score,
      samplingDuration: Date.now() - startTime,
    },
  };

  return result;
}

/**
 * Simple scoring function for Dockerfiles
 */
export function scoreDockerfile(content: string): Record<string, number> {
  const scores: Record<string, number> = {
    size: 0,
    security: 0,
    bestPractices: 0,
    caching: 0,
  };

  // Size optimizations
  if (content.includes('--no-cache')) scores.size += 20;
  if (content.includes('&& rm -rf')) scores.size += 15;
  if (/FROM .+:alpine/.test(content)) scores.size += 25;
  if (content.includes('multi-stage')) scores.size += 20;

  // Security
  if (!/USER root/i.test(content) && /USER \w+/i.test(content)) scores.security += 30;
  if (!content.includes('sudo')) scores.security += 20;
  if (content.includes('--chown=')) scores.security += 20;
  if (!/ADD .* \/$/m.test(content)) scores.security += 15;

  // Best practices
  if (content.includes('HEALTHCHECK')) scores.bestPractices += 25;
  if (content.includes('LABEL')) scores.bestPractices += 15;
  if (!/RUN .* && .* && .* && .* &&/m.test(content)) scores.bestPractices += 20;
  if (content.includes('COPY --from=')) scores.bestPractices += 20;

  // Caching optimization
  const copyCommands = (content.match(/COPY/g) || []).length;
  const runCommands = (content.match(/RUN/g) || []).length;
  if (copyCommands > 0 && runCommands > 0) {
    const firstCopy = content.indexOf('COPY');
    const firstRun = content.indexOf('RUN');
    if (firstCopy < firstRun) scores.caching += 30;
  }
  if (content.includes('COPY package*.json')) scores.caching += 25;
  if (content.includes('COPY go.mod go.sum')) scores.caching += 25;

  // Normalize scores to 0-100
  Object.keys(scores).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for Kubernetes manifests
 */
export function scoreKubernetesManifest(content: string): Record<string, number> {
  const scores: Record<string, number> = {
    resources: 0,
    security: 0,
    reliability: 0,
    observability: 0,
  };

  // Resource management
  if (content.includes('resources:') && content.includes('limits:')) scores.resources += 30;
  if (content.includes('requests:')) scores.resources += 30;
  if (!/cpu: [0-9]+[^m]/.test(content)) scores.resources += 20; // Prefer millicores
  if (!/memory: [0-9]+G/.test(content)) scores.resources += 20; // Avoid large memory

  // Security
  if (content.includes('securityContext:')) scores.security += 25;
  if (content.includes('runAsNonRoot: true')) scores.security += 25;
  if (content.includes('readOnlyRootFilesystem: true')) scores.security += 25;
  if (content.includes('allowPrivilegeEscalation: false')) scores.security += 25;

  // Reliability
  if (content.includes('replicas:') && /replicas:\s*[2-9]/.test(content)) scores.reliability += 30;
  if (content.includes('livenessProbe:')) scores.reliability += 25;
  if (content.includes('readinessProbe:')) scores.reliability += 25;
  if (content.includes('strategy:')) scores.reliability += 20;

  // Observability
  if (content.includes('prometheus.io/scrape')) scores.observability += 30;
  if (content.includes('labels:')) scores.observability += 20;
  if (content.includes('annotations:')) scores.observability += 20;
  if (content.match(/env:[\s\S]*LOG_LEVEL/)) scores.observability += 30;

  // Normalize scores to 0-100
  Object.keys(scores).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Combined scoring function that detects content type
 */
export function scoreContent(content: string): number | Record<string, number> {
  // Detect content type and use appropriate scorer
  if (content.includes('FROM ') && content.includes('RUN ')) {
    const scores = scoreDockerfile(content);
    return scores;
  } else if (content.includes('apiVersion:') && content.includes('kind:')) {
    const scores = scoreKubernetesManifest(content);
    return scores;
  }

  // Default scoring for unknown content
  return 50;
}
