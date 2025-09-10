/**
 * Workflow Configuration
 *
 * Simple configuration-based workflow definitions
 * instead of complex procedural planning logic
 */

interface WorkflowStep {
  toolName: string;
  description: string;
  required: boolean;
  skipIf?: string; // Condition name to check for skipping
}

interface WorkflowDefinition {
  name: string;
  description: string;
  steps: WorkflowStep[];
  estimatedDurationSeconds: number;
}

/**
 * Workflow definitions - simple configuration instead of complex logic
 */
const WORKFLOW_DEFINITIONS: Record<string, WorkflowDefinition> = {
  containerization: {
    name: 'containerization',
    description: 'Complete containerization workflow from analysis to deployment',
    steps: [
      {
        toolName: 'analyze-repo',
        description: 'Analyzing repository structure and dependencies',
        required: true,
        skipIf: 'analysis_completed',
      },
      {
        toolName: 'generate-dockerfile',
        description: 'Generating optimized Dockerfile',
        required: true,
      },
      {
        toolName: 'build-image',
        description: 'Building Docker image',
        required: false,
        skipIf: 'no_build',
      },
      {
        toolName: 'scan',
        description: 'Scanning image for vulnerabilities',
        required: false,
        skipIf: 'no_scan',
      },
      {
        toolName: 'push',
        description: 'Pushing image to registry',
        required: false,
        skipIf: 'no_push',
      },
    ],
    estimatedDurationSeconds: 150,
  },

  deployment: {
    name: 'deployment',
    description: 'Deploy application to Kubernetes cluster',
    steps: [
      {
        toolName: 'generate-k8s-manifests',
        description: 'Generating Kubernetes manifests',
        required: true,
      },
      {
        toolName: 'prepare-cluster',
        description: 'Preparing cluster for deployment',
        required: true,
      },
      {
        toolName: 'deploy',
        description: 'Deploying application to cluster',
        required: true,
      },
      {
        toolName: 'verify-deployment',
        description: 'Verifying deployment health',
        required: false,
      },
    ],
    estimatedDurationSeconds: 120,
  },

  security: {
    name: 'security',
    description: 'Security analysis and remediation workflow',
    steps: [
      {
        toolName: 'analyze-repo',
        description: 'Analyzing repository for security issues',
        required: true,
      },
      {
        toolName: 'scan',
        description: 'Scanning for vulnerabilities',
        required: true,
      },
      {
        toolName: 'fix-dockerfile',
        description: 'Fixing security issues in Dockerfile',
        required: false,
        skipIf: 'no_vulnerabilities',
      },
    ],
    estimatedDurationSeconds: 90,
  },

  optimization: {
    name: 'optimization',
    description: 'Optimize Docker images for size and performance',
    steps: [
      {
        toolName: 'analyze-repo',
        description: 'Analyzing repository',
        required: true,
      },
      {
        toolName: 'resolve-base-images',
        description: 'Resolving optimal base images',
        required: true,
      },
      {
        toolName: 'generate-dockerfile',
        description: 'Generating optimized Dockerfile',
        required: true,
      },
      {
        toolName: 'build-image',
        description: 'Building optimized image',
        required: false,
      },
    ],
    estimatedDurationSeconds: 120,
  },
};

/**
 * Get workflow steps with runtime conditions applied
 */
function _getWorkflowSteps(
  workflowName: string,
  params: Record<string, unknown>,
  sessionState?: Record<string, unknown>,
): WorkflowStep[] {
  const definition = WORKFLOW_DEFINITIONS[workflowName];
  if (!definition) {
    return [];
  }

  return definition.steps
    .filter((step) => {
      // Check skip conditions
      if (step.skipIf) {
        switch (step.skipIf) {
          case 'analysis_completed': {
            const completedSteps = sessionState?.completed_steps;
            if (Array.isArray(completedSteps) && completedSteps.includes('analyze-repo')) {
              return false;
            }
            break;
          }
          case 'no_build':
            if (params.buildImage === false) {
              return false;
            }
            break;
          case 'no_scan':
            if (params.scanImage === false || params.buildImage === false) {
              return false;
            }
            break;
          case 'no_push':
            if (!params.pushImage || !params.registry || params.buildImage === false) {
              return false;
            }
            break;
          case 'no_vulnerabilities':
            // This would be checked at runtime, include for now
            break;
        }
      }
      return true;
    })
    .map((step) => ({
      ...step,
      parameters: { ...params, sessionId: params.sessionId },
    }));
}

/**
 * Get workflow by name
 */
function _getWorkflow(name: string): WorkflowDefinition | undefined {
  return WORKFLOW_DEFINITIONS[name];
}

/**
 * Generate workflow recommendations based on results
 */
function _generateRecommendations(
  workflowType: string,
  results: Record<string, unknown>[],
): string[] {
  const recommendations: string[] = [
    'Review generated artifacts for accuracy',
    'Test container functionality before production deployment',
  ];

  switch (workflowType) {
    case 'containerization':
      if (
        results.some((r) => {
          const vulnerabilities = (r as { vulnerabilities?: unknown[] })?.vulnerabilities;
          return Array.isArray(vulnerabilities) && vulnerabilities.length > 0;
        })
      ) {
        recommendations.push('Address security vulnerabilities before deployment');
      }
      recommendations.push('Configure CI/CD pipeline for automated builds');
      break;

    case 'deployment':
      recommendations.push('Monitor deployment health metrics');
      recommendations.push('Set up alerts for critical issues');
      recommendations.push('Configure autoscaling based on load patterns');
      break;

    case 'security':
      recommendations.push('Enable security scanning in CI/CD pipeline');
      recommendations.push('Regularly update base images and dependencies');
      recommendations.push('Implement runtime security monitoring');
      break;

    case 'optimization': {
      const imageSizes = results
        .filter((r) => (r as { imageSize?: unknown }).imageSize)
        .map((r) => (r as { imageSize?: number }).imageSize as number);
      if (imageSizes.length > 1) {
        const firstSize = imageSizes[0];
        const lastSize = imageSizes[imageSizes.length - 1];
        if (firstSize && lastSize) {
          const reduction = ((firstSize - lastSize) / firstSize) * 100;
          recommendations.push(
            `Image size reduced by ${reduction.toFixed(1)}% through optimization`,
          );
        }
      }
      recommendations.push('Consider using distroless images for further size reduction');
      break;
    }
  }

  return recommendations;
}
