/**
 * Workflow hints helper - Shared logic for suggesting next steps in the containerization workflow
 *
 * This module provides a consistent way for tools to surface contextual recommendations
 * and progress breadcrumbs without duplicating logic.
 */

import type { SessionFacade } from '@/app/orchestrator-types';

/**
 * Workflow hint structure
 */
export interface WorkflowHint {
  nextStep: string;
  message: string;
}

/**
 * Workflow context from session state
 */
interface WorkflowContext {
  hasAnalysis: boolean;
  hasDockerfile: boolean;
  hasImage: boolean;
  hasScanned: boolean;
  hasTagged: boolean;
  hasPushed: boolean;
  hasManifests: boolean;
  hasPreparedCluster: boolean;
  hasDeployed: boolean;
  hasVerified: boolean;
  sessionId: string | undefined;
}

/**
 * Extract workflow context from session
 */
function getWorkflowContext(session?: SessionFacade, sessionId?: string): WorkflowContext {
  if (!session) {
    return {
      hasAnalysis: false,
      hasDockerfile: false,
      hasImage: false,
      hasScanned: false,
      hasTagged: false,
      hasPushed: false,
      hasManifests: false,
      hasPreparedCluster: false,
      hasDeployed: false,
      hasVerified: false,
      sessionId,
    };
  }

  // Get results from session
  const results = session.get('results');
  const hasResult = (toolName: string): boolean => {
    return !!(results && typeof results === 'object' && toolName in results);
  };

  return {
    hasAnalysis: hasResult('analyze-repo'),
    hasDockerfile: hasResult('generate-dockerfile'),
    hasImage: hasResult('build-image'),
    hasScanned: hasResult('scan-image'),
    hasTagged: hasResult('tag-image'),
    hasPushed: hasResult('push-image'),
    hasManifests: hasResult('generate-k8s-manifests'),
    hasPreparedCluster: hasResult('prepare-cluster'),
    hasDeployed: hasResult('deploy'),
    hasVerified: hasResult('verify-deploy'),
    sessionId,
  };
}

/**
 * Get next recommended step after image build
 */
export function getPostBuildHint(
  session?: SessionFacade,
  sessionId?: string,
  optimizationSuggestions?: string,
): WorkflowHint {
  const ctx = getWorkflowContext(session, sessionId);

  if (!ctx.hasScanned) {
    return {
      nextStep: 'scan-image',
      message: ctx.sessionId
        ? `Image built successfully. Use "scan-image" with sessionId ${ctx.sessionId} to check for security vulnerabilities.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`
        : `Image built successfully. Use "scan-image" to check for security vulnerabilities.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`,
    };
  }

  if (!ctx.hasTagged) {
    return {
      nextStep: 'tag-image',
      message: ctx.sessionId
        ? `Image built successfully. Use "tag-image" with sessionId ${ctx.sessionId} to add version tags.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`
        : `Image built successfully. Use "tag-image" to add version tags.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`,
    };
  }

  if (!ctx.hasPushed) {
    return {
      nextStep: 'push-image',
      message: ctx.sessionId
        ? `Image built successfully. Use "push-image" with sessionId ${ctx.sessionId} to push to a registry, or "generate-k8s-manifests" to create deployment manifests.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`
        : `Image built successfully. Use "push-image" to push to a registry, or "generate-k8s-manifests" to create deployment manifests.${optimizationSuggestions ? ' Review AI optimization suggestions for improvements.' : ''}`,
    };
  }

  return {
    nextStep: 'generate-k8s-manifests',
    message: ctx.sessionId
      ? `Image built and ready. Use "generate-k8s-manifests" with sessionId ${ctx.sessionId} to create Kubernetes deployment manifests.`
      : 'Image built and ready. Use "generate-k8s-manifests" to create Kubernetes deployment manifests.',
  };
}

/**
 * Get next recommended step after security scan
 */
export function getPostScanHint(
  passed: boolean,
  criticalCount: number,
  highCount: number,
  session?: SessionFacade,
  sessionId?: string,
): WorkflowHint {
  const ctx = getWorkflowContext(session, sessionId);

  if (!passed) {
    // Security issues found - suggest remediation
    if (criticalCount > 0 || highCount > 0) {
      return {
        nextStep: 'fix-dockerfile',
        message: `Security scan found ${criticalCount} critical and ${highCount} high severity vulnerabilities. Use "fix-dockerfile" to address security issues in your base images and dependencies.`,
      };
    } else {
      return {
        nextStep: 'generate-dockerfile',
        message: `Security scan found vulnerabilities. Consider regenerating your Dockerfile with more secure base images using "generate-dockerfile".`,
      };
    }
  }

  // Scan passed - suggest next deployment steps
  if (!ctx.hasTagged) {
    return {
      nextStep: 'tag-image',
      message: ctx.sessionId
        ? `Security scan passed! Use "tag-image" with sessionId ${ctx.sessionId} to add version tags.`
        : 'Security scan passed! Use "tag-image" to add version tags.',
    };
  }

  if (!ctx.hasPushed) {
    return {
      nextStep: 'push-image',
      message: ctx.sessionId
        ? `Security scan passed! Use "push-image" with sessionId ${ctx.sessionId} to push to a registry, or proceed with deployment.`
        : 'Security scan passed! Use "push-image" to push to a registry, or proceed with deployment.',
    };
  }

  return {
    nextStep: 'generate-k8s-manifests',
    message: ctx.sessionId
      ? `Security scan passed! Use "generate-k8s-manifests" with sessionId ${ctx.sessionId} to create deployment manifests.`
      : 'Security scan passed! Use "generate-k8s-manifests" to create deployment manifests.',
  };
}

/**
 * Get next recommended step after deployment
 */
export function getPostDeployHint(
  endpoints: Array<{ url?: string }>,
  session?: SessionFacade,
  sessionId?: string,
  deploymentAnalysis?: string,
): WorkflowHint {
  const ctx = getWorkflowContext(session, sessionId);

  const endpointMsg =
    endpoints.length > 0 && endpoints[0]?.url
      ? `, or access your application at: ${endpoints[0].url}`
      : '';

  if (!ctx.hasVerified) {
    return {
      nextStep: 'verify-deploy',
      message: ctx.sessionId
        ? `Application deployed successfully. Use "verify-deploy" with sessionId ${ctx.sessionId} to check deployment status${endpointMsg}.${deploymentAnalysis ? ' Review AI deployment analysis for optimization recommendations.' : ''}`
        : `Application deployed successfully. Use "verify-deploy" to check deployment status${endpointMsg}.${deploymentAnalysis ? ' Review AI deployment analysis for optimization recommendations.' : ''}`,
    };
  }

  return {
    nextStep: 'verify-deploy',
    message: `Application deployed successfully${endpointMsg}.${deploymentAnalysis ? ' Review AI deployment analysis for optimization recommendations.' : ''}`,
  };
}
