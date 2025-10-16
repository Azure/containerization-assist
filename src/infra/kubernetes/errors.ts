/**
 * Kubernetes error handling utilities with actionable guidance
 */

import type { ErrorGuidance } from '@/types';

/**
 * Extract error with actionable guidance for Kubernetes operations
 */
export function extractK8sErrorGuidance(error: unknown, operation?: string): ErrorGuidance {
  const details: Record<string, unknown> = {};

  if (error instanceof Error) {
    const errorMessage = error.message.toLowerCase();

    // Kubeconfig issues
    if (errorMessage.includes('kubeconfig') || errorMessage.includes('config file')) {
      return {
        message: 'Kubernetes configuration not found',
        hint: 'Unable to locate or read kubeconfig file',
        resolution:
          'Set KUBECONFIG environment variable or ensure ~/.kube/config exists. Run `kubectl config view` to verify.',
        details: { originalError: error.message },
      };
    }

    // Connection issues
    if (
      errorMessage.includes('econnrefused') ||
      errorMessage.includes('connection refused') ||
      errorMessage.includes('connect econnrefused')
    ) {
      return {
        message: 'Cannot connect to Kubernetes cluster',
        hint: 'Connection to Kubernetes API server was refused',
        resolution:
          'Verify cluster is running: `kubectl cluster-info`. Check API server address in kubeconfig and ensure network connectivity.',
        details: { originalError: error.message },
      };
    }

    if (errorMessage.includes('etimedout') || errorMessage.includes('timeout')) {
      return {
        message: 'Kubernetes operation timed out',
        hint: 'The API server did not respond in time',
        resolution:
          'Check cluster connectivity and load. Verify firewall rules allow access to the API server. Try `kubectl get nodes` to test connectivity.',
        details: { originalError: error.message },
      };
    }

    // Authentication issues
    if (
      errorMessage.includes('unauthorized') ||
      errorMessage.includes('401') ||
      errorMessage.includes('authentication')
    ) {
      return {
        message: 'Kubernetes authentication failed',
        hint: 'Invalid or expired credentials',
        resolution:
          'Refresh cluster credentials. For cloud providers: re-authenticate (e.g., `aws eks update-kubeconfig`, `gcloud container clusters get-credentials`).',
        details: { originalError: error.message },
      };
    }

    // Authorization issues
    if (
      errorMessage.includes('forbidden') ||
      errorMessage.includes('403') ||
      errorMessage.includes('authorization')
    ) {
      return {
        message: 'Kubernetes authorization failed',
        hint: 'Your user/service account lacks required permissions',
        resolution:
          'Verify RBAC permissions with `kubectl auth can-i <verb> <resource>`. Contact cluster administrator to grant necessary roles.',
        details: { originalError: error.message },
      };
    }

    // Resource not found
    if (errorMessage.includes('not found') || errorMessage.includes('404')) {
      const resource = operation ? ` (${operation})` : '';
      return {
        message: `Kubernetes resource not found${resource}`,
        hint: 'The requested resource does not exist in the cluster',
        resolution:
          'Verify resource name and namespace. Use `kubectl get <resource> -n <namespace>` to list available resources.',
        details: { originalError: error.message },
      };
    }

    // Namespace issues
    if (errorMessage.includes('namespace') && errorMessage.includes('does not exist')) {
      return {
        message: 'Kubernetes namespace does not exist',
        hint: 'The target namespace has not been created',
        resolution:
          'Create the namespace: `kubectl create namespace <name>` or ensure it exists before deploying resources.',
        details: { originalError: error.message },
      };
    }

    // Resource conflicts
    if (errorMessage.includes('already exists') || errorMessage.includes('conflict')) {
      return {
        message: 'Kubernetes resource already exists',
        hint: 'A resource with this name already exists',
        resolution:
          'Use a different name, delete the existing resource, or use `kubectl apply` instead of `create` to update it.',
        details: { originalError: error.message },
      };
    }

    // Validation errors
    if (errorMessage.includes('invalid') || errorMessage.includes('validation')) {
      return {
        message: 'Kubernetes resource validation failed',
        hint: 'The resource specification is invalid',
        resolution:
          'Check the manifest against Kubernetes API documentation. Use `kubectl apply --dry-run=client` to validate syntax.',
        details: { originalError: error.message },
      };
    }

    // API version mismatch
    if (errorMessage.includes('no matches for kind') || errorMessage.includes('api version')) {
      return {
        message: 'Kubernetes API version not supported',
        hint: 'The resource type or API version is not available in this cluster',
        resolution:
          'Check cluster version with `kubectl version` and update API versions in manifests. Some resources may require cluster upgrades.',
        details: { originalError: error.message },
      };
    }

    // General error
    return {
      message: error.message || 'Kubernetes operation failed',
      hint: 'An error occurred during the Kubernetes operation',
      resolution:
        'Run `kubectl get events --sort-by=.lastTimestamp` to see recent cluster events. Check resource logs with `kubectl logs`.',
      details,
    };
  }

  // Non-Error object
  return {
    message: String(error) || 'Unknown Kubernetes error',
    hint: 'An unexpected error occurred',
    resolution:
      'Check cluster status with `kubectl cluster-info` and verify your kubeconfig is valid.',
    details,
  };
}

