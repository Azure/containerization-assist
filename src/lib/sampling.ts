/**
 * Sampling Types and Core Interface
 * Provides cohesive sampling functionality for MCP tools
 */

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
