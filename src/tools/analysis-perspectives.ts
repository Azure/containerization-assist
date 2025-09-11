/**
 * Analysis Perspectives - Simple Enhancement for Repository Analysis
 *
 * Provides different analysis perspectives (security, performance, comprehensive)
 * that integrate seamlessly with the existing analyze-repo tool without complex types.
 */

import type { AnalysisPerspective, PerspectiveConfig } from './types';

/**
 * Analysis perspective configurations
 */
export const ANALYSIS_PERSPECTIVES: Record<AnalysisPerspective, PerspectiveConfig> = {
  comprehensive: {
    perspective: 'comprehensive',
    emphasis: ['complete coverage', 'detailed analysis', 'thorough dependency review'],
    additionalChecks: [
      'architecture patterns',
      'deployment readiness',
      'scalability considerations',
      'monitoring hooks',
    ],
  },
  'security-focused': {
    perspective: 'security-focused',
    emphasis: ['security vulnerabilities', 'compliance requirements', 'access controls'],
    additionalChecks: [
      'vulnerable dependencies',
      'hardcoded secrets',
      'insecure configurations',
      'privilege escalation risks',
      'network security',
    ],
  },
  'performance-focused': {
    perspective: 'performance-focused',
    emphasis: ['performance bottlenecks', 'resource optimization', 'scalability'],
    additionalChecks: [
      'resource usage patterns',
      'caching opportunities',
      'database query optimization',
      'memory management',
      'CPU intensive operations',
    ],
  },
};
