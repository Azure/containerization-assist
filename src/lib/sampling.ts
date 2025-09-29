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
  const scores = {
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
  (Object.keys(scores) as Array<keyof typeof scores>).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for Kubernetes manifests
 */
export function scoreKubernetesManifest(content: string): Record<string, number> {
  const scores = {
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
  (Object.keys(scores) as Array<keyof typeof scores>).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for Helm charts
 */
export function scoreHelmChart(content: string): Record<string, number> {
  const scores = {
    chartStructure: 0,
    templating: 0,
    values: 0,
    bestPractices: 0,
  };

  // Chart.yaml structure
  if (content.includes('apiVersion: v2')) scores.chartStructure += 25;
  if (content.includes('name:') && content.includes('version:')) scores.chartStructure += 25;
  if (content.includes('description:')) scores.chartStructure += 20;
  if (content.includes('type: application')) scores.chartStructure += 15;
  if (content.includes('appVersion:')) scores.chartStructure += 15;

  // Template quality
  if (content.includes('{{- if')) scores.templating += 20; // Conditional logic
  if (content.includes('{{- range')) scores.templating += 15; // Loops
  if (content.includes('{{- with')) scores.templating += 15; // Context
  if (content.includes('{{- include')) scores.templating += 20; // Template includes
  if (content.includes('{{- toYaml')) scores.templating += 15; // YAML helpers
  if (content.includes('{{- end }}')) scores.templating += 15; // Proper closing

  // Values.yaml quality
  if (content.includes('# Default values') || content.includes('# -- ')) scores.values += 25;
  if (content.includes('replicaCount:')) scores.values += 20;
  if (content.includes('image:') && content.includes('tag:')) scores.values += 20;
  if (content.includes('service:') && content.includes('port:')) scores.values += 15;
  if (content.includes('resources:')) scores.values += 20;

  // Best practices
  if (content.includes('securityContext:')) scores.bestPractices += 25;
  if (content.includes('podSecurityContext:')) scores.bestPractices += 20;
  if (content.includes('livenessProbe:') || content.includes('readinessProbe:'))
    scores.bestPractices += 20;
  if (content.includes('nodeSelector:') || content.includes('affinity:'))
    scores.bestPractices += 15;
  if (content.includes('tolerations:')) scores.bestPractices += 10;
  if (content.includes('serviceAccount:')) scores.bestPractices += 10;

  // Normalize scores to 0-100
  (Object.keys(scores) as Array<keyof typeof scores>).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for Azure Container Apps manifests
 */
export function scoreACAManifest(content: string): Record<string, number> {
  const scores = {
    structure: 0,
    configuration: 0,
    scaling: 0,
    security: 0,
  };

  // ACA structure
  if (content.includes('Microsoft.App/containerApps')) scores.structure += 30;
  if (content.includes('properties:')) scores.structure += 20;
  if (content.includes('configuration:')) scores.structure += 25;
  if (content.includes('template:')) scores.structure += 25;

  // Configuration quality
  if (content.includes('ingress:')) scores.configuration += 25;
  if (content.includes('registries:')) scores.configuration += 20;
  if (content.includes('secrets:')) scores.configuration += 20;
  if (content.includes('dapr:')) scores.configuration += 15;
  if (content.includes('environmentVariables:')) scores.configuration += 20;

  // Scaling configuration
  if (content.includes('scale:')) scores.scaling += 30;
  if (content.includes('minReplicas:') && content.includes('maxReplicas:')) scores.scaling += 30;
  if (content.includes('rules:')) scores.scaling += 25;
  if (content.includes('http:') || content.includes('cpu:') || content.includes('memory:'))
    scores.scaling += 15;

  // Security best practices
  if (content.includes('allowInsecure: false')) scores.security += 25;
  if (content.includes('managedIdentity:')) scores.security += 20;
  if (!content.includes('allowInsecure: true')) scores.security += 15;
  if (content.includes('activeRevisionsMode: single')) scores.security += 20;
  if (content.includes('transport: http2')) scores.security += 10;
  if (content.includes('corsPolicy:')) scores.security += 10;

  // Normalize scores to 0-100
  (Object.keys(scores) as Array<keyof typeof scores>).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for base image recommendations
 */
export function scoreBaseImageRecommendation(content: string): Record<string, number> {
  const scores = {
    specificity: 0,
    security: 0,
    optimization: 0,
    maintenance: 0,
  };

  // Avoid generic/latest tags - prefer specific versions
  if (!/latest|generic/i.test(content)) scores.specificity += 30;
  if (/\d+\.\d+/.test(content)) scores.specificity += 25; // Has version numbers
  if (/alpine|slim|distroless/.test(content)) scores.specificity += 25;
  if (content.includes('SHA256:') || content.includes('@sha256:')) scores.specificity += 20; // Digest pinning

  // Security considerations
  if (/distroless|scratch/.test(content)) scores.security += 30; // Minimal attack surface
  if (/alpine/.test(content)) scores.security += 25; // Small, security-focused
  if (!/ubuntu:|centos:|amazonlinux:/.test(content)) scores.security += 15; // Avoid large distros
  if (content.includes('vulnerability scan: clean') || content.includes('no known vulnerabilities'))
    scores.security += 30;
  if (content.includes('signed') || content.includes('official')) scores.security += 10;

  // Size/performance optimization
  if (/alpine|slim|micro/.test(content)) scores.optimization += 30;
  if (content.includes('multi-stage') || content.includes('builder')) scores.optimization += 20;
  if (content.includes('size:') && /MB|mb/.test(content)) scores.optimization += 15;
  if (!/FROM .+:.+-.+-.+/.test(content)) scores.optimization += 15; // Avoid overly complex tags
  if (content.includes('compressed') || content.includes('optimized')) scores.optimization += 10;

  // Maintenance and support
  if (content.includes('LTS') || content.includes('stable')) scores.maintenance += 25;
  if (content.includes('official') || content.includes('maintained')) scores.maintenance += 20;
  if (!content.includes('deprecated') && !content.includes('EOL')) scores.maintenance += 25;
  if (content.includes('updated') || content.includes('recent')) scores.maintenance += 15;
  if (content.includes('supported') || content.includes('community')) scores.maintenance += 15;

  // Normalize scores to 0-100
  (Object.keys(scores) as Array<keyof typeof scores>).forEach((key) => {
    scores[key] = Math.min(100, scores[key] || 0);
  });

  return scores;
}

/**
 * Simple scoring function for repository analysis
 */
export function scoreRepositoryAnalysis(content: string): number {
  let score = 0;
  if (content.includes('framework:')) score += 25;
  if (content.includes('dependencies:')) score += 20;
  if (content.includes('buildCommands:')) score += 20;
  if (content.includes('dockerStrategy:')) score += 15;
  if (content.includes('portDetection:')) score += 10;
  if (content.includes('language:')) score += 10;
  return Math.min(score, 100);
}

/**
 * Simple scoring function for ACA to K8s conversion
 */
export function scoreACAConversion(content: string): number {
  let score = 0;
  if (content.includes('apiVersion: apps/v1')) score += 25;
  if (content.includes('kind: Deployment')) score += 20;
  if (content.includes('kind: Service')) score += 20;
  if (content.includes('metadata:') && content.includes('labels:')) score += 15;
  if (content.includes('spec:') && content.includes('selector:')) score += 20;
  return Math.min(score, 100);
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
  } else if (content.includes('Microsoft.App/containerApps')) {
    const scores = scoreACAManifest(content);
    return scores;
  } else if (
    content.includes('Chart.yaml') ||
    (content.includes('apiVersion: v2') &&
      content.includes('name:') &&
      content.includes('version:'))
  ) {
    const scores = scoreHelmChart(content);
    return scores;
  } else if (
    content.toLowerCase().includes('base image') ||
    content.toLowerCase().includes('docker image') ||
    /FROM\s+[\w.-]+:[\w.-]+/.test(content)
  ) {
    const scores = scoreBaseImageRecommendation(content);
    return scores;
  }

  // Default scoring for unknown content
  return 50;
}
