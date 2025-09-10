import { Result, Success, Failure } from '../../types';

/**
 * Standard success response shape for all tools
 * Ensures consistent structure across all tool responses
 */
export interface StandardToolResponse<T = unknown> {
  ok: boolean;
  sessionId?: string;
  data?: T;
  message?: string;
}

/**
 * Dockerfile-specific response shape
 */
export interface DockerfileResponse {
  ok: boolean;
  sessionId?: string;
  dockerfile: string;
  path: string;
}

/**
 * Kubernetes manifest response shape
 */
export interface ManifestResponse {
  ok: boolean;
  sessionId?: string;
  manifest: string;
  kind: string;
}

/**
 * Analysis response shape
 */
export interface AnalysisResponse {
  ok: boolean;
  sessionId?: string;
  analysis: {
    framework: string;
    language: string;
    dependencies: string[];
    recommendations: string[];
  };
}

/**
 * Scan response shape
 */
export interface ScanResponse {
  ok: boolean;
  sessionId?: string;
  vulnerabilities: {
    critical: number;
    high: number;
    medium: number;
    low: number;
    total: number;
  };
  summary: string;
}

/**
 * Deployment response shape
 */
export interface DeploymentResponse {
  ok: boolean;
  sessionId?: string;
  deployed: boolean;
  resources: string[];
  status: string;
}

/**
 * Core response formatter that wraps Result<T> in standardized response shape
 * @param result - The Result<T> to format
 * @param sessionId - Optional session ID to include in response
 * @returns Formatted standard response
 */
export function formatStandardResponse<T>(
  result: Result<T>,
  sessionId?: string,
): Result<StandardToolResponse<T>> {
  if (result.ok) {
    const response: StandardToolResponse<T> = {
      ok: true,
      data: result.value,
      message: 'Operation completed successfully',
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return Success(response);
  }
  return Failure(result.error);
}

/**
 * Detects Kubernetes resource kind from YAML content
 * @param yaml - YAML content to analyze
 * @returns Detected Kubernetes kind or 'Unknown'
 */
export function detectK8sKind(yaml: string): string {
  const kindMatch = yaml.match(/^kind:\s*(.+)$/m);
  return kindMatch ? kindMatch[1]?.trim() || 'Unknown' : 'Unknown';
}

/**
 * Tool-specific response formatters for different output types
 */
export const responseFormatters = {
  /**
   * Format Dockerfile generation response
   */
  dockerfile: (content: string, sessionId?: string): DockerfileResponse => {
    const response: DockerfileResponse = {
      ok: true,
      dockerfile: content,
      path: '/app/Dockerfile',
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return response;
  },

  /**
   * Format Kubernetes manifest response
   */
  manifest: (yaml: string, sessionId?: string): ManifestResponse => {
    const response: ManifestResponse = {
      ok: true,
      manifest: yaml,
      kind: detectK8sKind(yaml),
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return response;
  },

  /**
   * Format repository analysis response
   */
  analysis: (
    framework: string,
    language: string,
    dependencies: string[],
    recommendations: string[],
    sessionId?: string,
  ): AnalysisResponse => {
    const response: AnalysisResponse = {
      ok: true,
      analysis: {
        framework,
        language,
        dependencies,
        recommendations,
      },
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return response;
  },

  /**
   * Format security scan response
   */
  scan: (
    vulnerabilities: { critical: number; high: number; medium: number; low: number },
    summary: string,
    sessionId?: string,
  ): ScanResponse => {
    const response: ScanResponse = {
      ok: true,
      vulnerabilities: {
        ...vulnerabilities,
        total:
          vulnerabilities.critical +
          vulnerabilities.high +
          vulnerabilities.medium +
          vulnerabilities.low,
      },
      summary,
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return response;
  },

  /**
   * Format deployment response
   */
  deployment: (
    deployed: boolean,
    resources: string[],
    status: string,
    sessionId?: string,
  ): DeploymentResponse => {
    const response: DeploymentResponse = {
      ok: true,
      deployed,
      resources,
      status,
    };
    if (sessionId) {
      response.sessionId = sessionId;
    }
    return response;
  },
};
