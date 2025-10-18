/**
 * Kubernetes error handling utilities with actionable guidance
 */

import type { ErrorGuidance } from '@/types';
import { createErrorGuidanceBuilder, customPattern, type ErrorPattern } from '@/lib/error-guidance';

/**
 * Helper to extract message from error (handles both Error instances and plain objects)
 */
function getErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  const errorObj = error as { message?: string };
  if (errorObj && typeof errorObj.message === 'string') {
    return errorObj.message;
  }
  return String(error);
}

/**
 * Kubernetes error patterns in order of specificity
 */
const k8sErrorPatterns: ErrorPattern[] = [
  // Kubeconfig issues
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('kubeconfig') || msg.includes('config file');
    },
    {
      message: 'Kubernetes configuration not found',
      hint: 'Unable to locate or read kubeconfig file',
      resolution:
        'Set KUBECONFIG environment variable or ensure ~/.kube/config exists. Run `kubectl config view` to verify.',
    },
  ),

  // Connection issues - ECONNREFUSED
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return (
        msg.includes('econnrefused') ||
        msg.includes('connection refused') ||
        msg.includes('connect econnrefused')
      );
    },
    {
      message: 'Cannot connect to Kubernetes cluster',
      hint: 'Connection to Kubernetes API server was refused',
      resolution:
        'Verify cluster is running: `kubectl cluster-info`. Check API server address in kubeconfig and ensure network connectivity.',
    },
  ),

  // Timeout issues
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('etimedout') || msg.includes('timeout');
    },
    {
      message: 'Kubernetes operation timed out',
      hint: 'The API server did not respond in time',
      resolution:
        'Check cluster connectivity and load. Verify firewall rules allow access to the API server. Try `kubectl get nodes` to test connectivity.',
    },
  ),

  // Authentication issues
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('unauthorized') || msg.includes('401') || msg.includes('authentication');
    },
    {
      message: 'Kubernetes authentication failed',
      hint: 'Invalid or expired credentials',
      resolution:
        'Refresh cluster credentials. For cloud providers: re-authenticate (e.g., `aws eks update-kubeconfig`, `gcloud container clusters get-credentials`).',
    },
  ),

  // Authorization issues
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('forbidden') || msg.includes('403') || msg.includes('authorization');
    },
    {
      message: 'Kubernetes authorization failed',
      hint: 'Your user/service account lacks required permissions',
      resolution:
        'Verify RBAC permissions with `kubectl auth can-i <verb> <resource>`. Contact cluster administrator to grant necessary roles.',
    },
  ),

  // Resource not found
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('not found') || msg.includes('404');
    },
    {
      message: 'Kubernetes resource not found',
      hint: 'The requested resource does not exist in the cluster',
      resolution:
        'Verify resource name and namespace. Use `kubectl get <resource> -n <namespace>` to list available resources.',
    },
  ),

  // Namespace issues
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('namespace') && msg.includes('does not exist');
    },
    {
      message: 'Kubernetes namespace does not exist',
      hint: 'The target namespace has not been created',
      resolution:
        'Create the namespace: `kubectl create namespace <name>` or ensure it exists before deploying resources.',
    },
  ),

  // Resource conflicts
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('already exists') || msg.includes('conflict');
    },
    {
      message: 'Kubernetes resource already exists',
      hint: 'A resource with this name already exists',
      resolution:
        'Use a different name, delete the existing resource, or use `kubectl apply` instead of `create` to update it.',
    },
  ),

  // Validation errors
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('invalid') || msg.includes('validation');
    },
    {
      message: 'Kubernetes resource validation failed',
      hint: 'The resource specification is invalid',
      resolution:
        'Check the manifest against Kubernetes API documentation. Use `kubectl apply --dry-run=client` to validate syntax.',
    },
  ),

  // API version mismatch
  customPattern(
    (error: unknown) => {
      const msg = getErrorMessage(error).toLowerCase();
      return msg.includes('no matches for kind') || msg.includes('api version');
    },
    {
      message: 'Kubernetes API version not supported',
      hint: 'The resource type or API version is not available in this cluster',
      resolution:
        'Check cluster version with `kubectl version` and update API versions in manifests. Some resources may require cluster upgrades.',
    },
  ),
];

/**
 * Default guidance when no pattern matches
 */
function defaultK8sGuidance(error: unknown): ErrorGuidance {
  const message = getErrorMessage(error);

  return {
    message: message || 'Kubernetes operation failed',
    hint: 'An error occurred during the Kubernetes operation',
    resolution:
      'Run `kubectl get events --sort-by=.lastTimestamp` to see recent cluster events. Check resource logs with `kubectl logs`.',
    details: { originalError: message },
  };
}

/**
 * Extract error with actionable guidance for Kubernetes operations
 *
 * @param error - The error to extract guidance from
 * @param operation - Optional operation context to include in the message
 */
export function extractK8sErrorGuidance(error: unknown, operation?: string): ErrorGuidance {
  const baseExtractor = createErrorGuidanceBuilder(k8sErrorPatterns, defaultK8sGuidance);
  const guidance = baseExtractor(error);

  // If operation context is provided and error is "not found", include it in the message
  if (operation && guidance.message.includes('resource not found')) {
    return {
      ...guidance,
      message: `Kubernetes resource not found (${operation})`,
    };
  }

  return guidance;
}
