/**
 * Scoring functions for specialized content types
 */

import type { ScoringContext } from './base-scorer';

/**
 * Simple scoring function for Azure Container Apps manifests
 */
export function scoreACAManifest(
  content: string,
  _context?: ScoringContext,
): Record<string, number> {
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
export function scoreBaseImageRecommendation(
  content: string,
  _context?: ScoringContext,
): Record<string, number> {
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
export function scoreRepositoryAnalysis(content: string, _context?: ScoringContext): number {
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
export function scoreACAConversion(content: string, _context?: ScoringContext): number {
  let score = 0;
  if (content.includes('apiVersion: apps/v1')) score += 25;
  if (content.includes('kind: Deployment')) score += 20;
  if (content.includes('kind: Service')) score += 20;
  if (content.includes('metadata:') && content.includes('labels:')) score += 15;
  if (content.includes('spec:') && content.includes('selector:')) score += 20;
  return Math.min(score, 100);
}
