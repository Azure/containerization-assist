/**
 * Kubeconfig discovery and validation utilities
 *
 * Simplifies kubeconfig detection with fast-fail error handling
 */

import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import * as k8s from '@kubernetes/client-node';
import { Success, Failure, type Result } from '@/types';

export interface KubeconfigInfo {
  path: string;
  contextName: string;
  clusterName: string;
  user: string;
}

/**
 * Discover kubeconfig location following kubectl's logic:
 * 1. KUBECONFIG environment variable
 * 2. ~/.kube/config
 * 3. In-cluster config (not applicable for single-user scenarios)
 */
export function discoverKubeconfigPath(): Result<string> {
  // Check KUBECONFIG environment variable
  const kubeconfigEnv = process.env.KUBECONFIG;
  if (kubeconfigEnv) {
    // Handle multiple paths separated by colon (kubectl behavior)
    const paths = kubeconfigEnv.split(path.delimiter);
    for (const configPath of paths) {
      if (fs.existsSync(configPath)) {
        return Success(configPath);
      }
    }
    return Failure(`KUBECONFIG environment variable is set but file not found: ${kubeconfigEnv}`, {
      message: 'Kubeconfig file not found',
      hint: 'KUBECONFIG environment variable points to a non-existent file',
      resolution:
        'Verify the path in KUBECONFIG is correct, or unset it to use the default ~/.kube/config location.',
      details: { kubeconfigEnv },
    });
  }

  // Check default location
  const defaultPath = path.join(os.homedir(), '.kube', 'config');
  if (fs.existsSync(defaultPath)) {
    return Success(defaultPath);
  }

  return Failure('Kubeconfig not found', {
    message: 'No kubeconfig file found',
    hint: 'Neither KUBECONFIG environment variable nor ~/.kube/config exists',
    resolution:
      'Set up kubectl configuration: run `kubectl config view` to check current config, or configure access to your cluster using cloud provider CLI (e.g., `aws eks update-kubeconfig`, `gcloud container clusters get-credentials`, or `az aks get-credentials`).',
    details: { defaultPath },
  });
}

/**
 * Validate that a kubeconfig file is readable and parseable
 */
export function validateKubeconfig(configPath: string): Result<KubeconfigInfo> {
  try {
    // Check file exists
    if (!fs.existsSync(configPath)) {
      return Failure(`Kubeconfig file does not exist: ${configPath}`, {
        message: 'Kubeconfig file not found',
        hint: `File does not exist at path: ${configPath}`,
        resolution: 'Verify the kubeconfig path is correct and the file exists.',
        details: { configPath },
      });
    }

    // Check file is readable
    try {
      fs.accessSync(configPath, fs.constants.R_OK);
    } catch (error) {
      return Failure(`Kubeconfig file is not readable: ${configPath}`, {
        message: 'Cannot read kubeconfig file',
        hint: 'Insufficient permissions to read the kubeconfig file',
        resolution: `Check file permissions: \`chmod 600 ${configPath}\` or verify file ownership.`,
        details: { configPath, error: String(error) },
      });
    }

    // Parse the kubeconfig
    const kc = new k8s.KubeConfig();
    try {
      kc.loadFromFile(configPath);
    } catch (error) {
      return Failure(`Failed to parse kubeconfig: ${configPath}`, {
        message: 'Invalid kubeconfig format',
        hint: 'The kubeconfig file could not be parsed',
        resolution:
          'Verify the file is valid YAML. Run `kubectl config view` to check for syntax errors.',
        details: { configPath, error: String(error) },
      });
    }

    // Get current context info
    const currentContext = kc.getCurrentContext();
    if (!currentContext) {
      return Failure('No current context set in kubeconfig', {
        message: 'No active Kubernetes context',
        hint: 'The kubeconfig file has no current context configured',
        resolution:
          'Set a context: `kubectl config use-context <context-name>` or `kubectl config get-contexts` to list available contexts.',
        details: { configPath },
      });
    }

    const cluster = kc.getCurrentCluster();
    const user = kc.getCurrentUser();

    return Success({
      path: configPath,
      contextName: currentContext,
      clusterName: cluster?.name || 'unknown',
      user: user?.name || 'unknown',
    });
  } catch (error) {
    return Failure(`Kubeconfig validation failed: ${String(error)}`, {
      message: 'Failed to validate kubeconfig',
      hint: 'An unexpected error occurred while validating the kubeconfig',
      resolution: 'Check the kubeconfig file format and permissions.',
      details: { configPath, error: String(error) },
    });
  }
}

/**
 * Discover and validate kubeconfig in one step
 */
export function discoverAndValidateKubeconfig(): Result<KubeconfigInfo> {
  const pathResult = discoverKubeconfigPath();
  if (!pathResult.ok) {
    return pathResult;
  }

  return validateKubeconfig(pathResult.value);
}

/**
 * Check if running in-cluster (Kubernetes pod)
 * Not applicable for single-user scenarios but included for completeness
 */
export function isInCluster(): boolean {
  return (
    fs.existsSync('/var/run/secrets/kubernetes.io/serviceaccount/token') &&
    fs.existsSync('/var/run/secrets/kubernetes.io/serviceaccount/ca.crt')
  );
}
