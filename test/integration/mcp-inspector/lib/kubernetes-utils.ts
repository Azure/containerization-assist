/**
 * Kubernetes Utilities for Integration Testing
 * Provides Kubernetes operations for deployment testing
 */

import { spawn } from 'child_process';
import { writeFile, unlink } from 'fs/promises';
import { join } from 'path';
import tmp from 'tmp';
import type { DeployResult, K8sManifest, KubernetesValidationResult } from '../../../../src/types/result-types.js';

// Extended interface for Kubernetes deployment results  
export interface KubernetesDeployResult extends DeployResult {
  deploymentName: string;
  namespace: string;
  duration: number;
}

export interface ClusterInfo {
  available: boolean;
  version?: string;
  context?: string;
  namespaces: string[];
}

export class KubernetesUtils {
  private createdResources: Array<{ kind: string; name: string; namespace: string }> = [];
  private tempFiles: Set<string> = new Set();

  /**
   * Check if Kubernetes cluster is available
   */
  static async isKubernetesAvailable(): Promise<boolean> {
    try {
      const result = await KubernetesUtils.execCommand('kubectl', ['version', '--client=true']);
      return result.exitCode === 0;
    } catch {
      return false;
    }
  }

  /**
   * Check if cluster is accessible
   */
  static async isClusterAvailable(): Promise<boolean> {
    try {
      const result = await KubernetesUtils.execCommand('kubectl', ['cluster-info'], { timeout: 10000 });
      return result.exitCode === 0;
    } catch {
      return false;
    }
  }

  /**
   * Get cluster information
   */
  async getClusterInfo(): Promise<ClusterInfo> {
    try {
      // Get cluster version
      const versionResult = await KubernetesUtils.execCommand('kubectl', ['version', '--short']);
      const version = versionResult.exitCode === 0 ? versionResult.stdout : undefined;

      // Get current context
      const contextResult = await KubernetesUtils.execCommand('kubectl', ['config', 'current-context']);
      const context = contextResult.exitCode === 0 ? contextResult.stdout.trim() : undefined;

      // Get namespaces
      const nsResult = await KubernetesUtils.execCommand('kubectl', ['get', 'namespaces', '-o', 'name']);
      const namespaces = nsResult.exitCode === 0 
        ? nsResult.stdout.split('\n').map(n => n.replace('namespace/', '')).filter(Boolean)
        : [];

      return {
        available: await KubernetesUtils.isClusterAvailable(),
        ...(version?.split('\n')[1] && { version: version.split('\n')[1] }),
        ...(context && { context }),
        namespaces
      };
    } catch {
      return {
        available: false,
        namespaces: []
      };
    }
  }

  /**
   * Validate Kubernetes manifests against cluster schema
   */
  async validateManifests(manifests: K8sManifest[]): Promise<KubernetesValidationResult[]> {
    const results: KubernetesValidationResult[] = [];

    for (const manifest of manifests) {
      let tempDirResult: tmp.DirResult | null = null;
      try {
        // Create temporary file for manifest
        tempDirResult = tmp.dirSync({ prefix: 'k8s-manifest-', unsafeCleanup: true, keep: false });
        const manifestPath = join(tempDirResult.name, `${manifest.kind}-${manifest.metadata.name}.yaml`);

        await writeFile(manifestPath, this.manifestToYaml(manifest));
        this.tempFiles.add(manifestPath);

        // Use kubectl to validate
        const result = await KubernetesUtils.execCommand('kubectl', [
          'apply', '--dry-run=client', '--validate=true', '-f', manifestPath
        ]);

        const validation: KubernetesValidationResult = {
          isValid: result.exitCode === 0,
          errors: [],
          warnings: [],
          manifest
        };

        if (result.exitCode !== 0) {
          validation.errors.push(result.stderr);
        }

        // Parse warnings from stderr
        if (result.stderr.includes('Warning:')) {
          const warnings = result.stderr.split('\n')
            .filter(line => line.includes('Warning:'))
            .map(line => line.replace(/.*Warning:\s*/, ''));
          validation.warnings = warnings;
        }

        results.push(validation);

        // Clean up temp directory for this manifest
        if (tempDirResult) {
          tempDirResult.removeCallback();
          tempDirResult = null;
        }
      } catch (error) {
        results.push({
          isValid: false,
          errors: [error instanceof Error ? error.message : String(error)],
          warnings: [],
          manifest
        });
      } finally {
        // Ensure cleanup even on error
        if (tempDirResult) {
          try {
            tempDirResult.removeCallback();
          } catch {
            // Ignore cleanup errors
          }
        }
      }
    }

    return results;
  }

  /**
   * Perform dry-run deployment
   */
  async dryRunDeploy(manifests: K8sManifest[], namespace = 'default'): Promise<KubernetesDeployResult> {
    const startTime = performance.now();
    let tempDirResult: tmp.DirResult | null = null;

    try {
      // Create temporary file with all manifests
      tempDirResult = tmp.dirSync({ prefix: 'k8s-deploy-', unsafeCleanup: true, keep: false });
      const manifestsPath = join(tempDirResult.name, 'manifests.yaml');

      const yamlContent = manifests.map(m => this.manifestToYaml(m)).join('---\n');
      await writeFile(manifestsPath, yamlContent);
      this.tempFiles.add(manifestsPath);

      const result = await KubernetesUtils.execCommand('kubectl', [
        'apply', '--dry-run=server', '-f', manifestsPath, '-n', namespace
      ]);

      const duration = performance.now() - startTime;

      const deployResult: KubernetesDeployResult = {
        success: result.exitCode === 0,
        deploymentName: manifests.find(m => m.kind === 'Deployment')?.metadata.name || 'unknown',
        namespace,
        resources: manifests.map(m => ({ kind: m.kind, name: m.metadata.name, namespace, status: 'deployed' })),
        duration
      };

      if (result.exitCode !== 0 && result.stderr) {
        deployResult.error = result.stderr;
      }

      return deployResult;
    } catch (error) {
      return {
        success: false,
        deploymentName: 'unknown',
        namespace,
        resources: [],
        error: error instanceof Error ? error.message : String(error),
        duration: performance.now() - startTime
      };
    } finally {
      // Clean up temp directory
      if (tempDirResult) {
        try {
          tempDirResult.removeCallback();
        } catch {
          // Ignore cleanup errors
        }
      }
    }
  }

  /**
   * Create namespace if it doesn't exist
   */
  async ensureNamespace(namespace: string): Promise<boolean> {
    try {
      // Check if namespace exists
      const checkResult = await KubernetesUtils.execCommand('kubectl', ['get', 'namespace', namespace]);
      
      if (checkResult.exitCode === 0) {
        return true; // Namespace already exists
      }

      // Create namespace
      const createResult = await KubernetesUtils.execCommand('kubectl', ['create', 'namespace', namespace]);
      
      if (createResult.exitCode === 0) {
        this.createdResources.push({ kind: 'Namespace', name: namespace, namespace: '' });
        return true;
      }

      return false;
    } catch {
      return false;
    }
  }

  /**
   * Deploy manifests to cluster (actual deployment - use with caution in tests)
   */
  async deploy(manifests: K8sManifest[], namespace = 'default', wait = false): Promise<KubernetesDeployResult> {
    const startTime = performance.now();
    let tempDirResult: tmp.DirResult | null = null;

    try {
      // Ensure namespace exists
      await this.ensureNamespace(namespace);

      // Create temporary file with all manifests
      tempDirResult = tmp.dirSync({ prefix: 'k8s-deploy-', unsafeCleanup: true, keep: false });
      const manifestsPath = join(tempDirResult.name, 'manifests.yaml');

      const yamlContent = manifests.map(m => this.manifestToYaml(m)).join('---\n');
      await writeFile(manifestsPath, yamlContent);
      this.tempFiles.add(manifestsPath);

      const args = ['apply', '-f', manifestsPath, '-n', namespace];
      if (wait) {
        args.push('--wait');
      }

      const result = await KubernetesUtils.execCommand('kubectl', args);
      const duration = performance.now() - startTime;

      if (result.exitCode === 0) {
        // Track created resources for cleanup
        manifests.forEach(manifest => {
          this.createdResources.push({
            kind: manifest.kind,
            name: manifest.metadata.name,
            namespace: manifest.metadata.namespace || namespace
          });
        });
      }

      const deployResult: KubernetesDeployResult = {
        success: result.exitCode === 0,
        deploymentName: manifests.find(m => m.kind === 'Deployment')?.metadata.name || 'unknown',
        namespace,
        resources: manifests.map(m => ({ kind: m.kind, name: m.metadata.name, namespace, status: 'deployed' })),
        duration
      };

      if (result.exitCode !== 0 && result.stderr) {
        deployResult.error = result.stderr;
      }

      return deployResult;
    } catch (error) {
      return {
        success: false,
        deploymentName: 'unknown',
        namespace,
        resources: [],
        error: error instanceof Error ? error.message : String(error),
        duration: performance.now() - startTime
      };
    } finally {
      // Clean up temp directory
      if (tempDirResult) {
        try {
          tempDirResult.removeCallback();
        } catch {
          // Ignore cleanup errors
        }
      }
    }
  }

  /**
   * Get deployment status
   */
  async getDeploymentStatus(deploymentName: string, namespace = 'default'): Promise<{
    ready: boolean;
    replicas: { desired: number; ready: number; available: number };
    conditions: Array<{ type: string; status: string; reason?: string }>;
  } | null> {
    try {
      const result = await KubernetesUtils.execCommand('kubectl', [
        'get', 'deployment', deploymentName, '-n', namespace, '-o', 'json'
      ]);

      if (result.exitCode === 0) {
        const deployment = JSON.parse(result.stdout);
        const status = deployment.status || {};
        
        return {
          ready: status.readyReplicas === status.replicas,
          replicas: {
            desired: status.replicas || 0,
            ready: status.readyReplicas || 0,
            available: status.availableReplicas || 0
          },
          conditions: status.conditions || []
        };
      }

      return null;
    } catch {
      return null;
    }
  }

  /**
   * Clean up all created resources
   */
  async cleanup(): Promise<void> {
    const cleanupPromises = [];

    // Delete created resources in reverse order
    const resourcesReverse = [...this.createdResources].reverse();
    
    for (const resource of resourcesReverse) {
      cleanupPromises.push(this.deleteResource(resource));
    }

    // Clean up temp files
    for (const filePath of this.tempFiles) {
      cleanupPromises.push(
        unlink(filePath).catch(() => {}) // Ignore errors
      );
    }

    await Promise.allSettled(cleanupPromises);
    
    this.createdResources.length = 0;
    this.tempFiles.clear();
  }

  /**
   * Convert manifest object to YAML string
   */
  private manifestToYaml(manifest: K8sManifest): string {
    // Simple YAML serialization for K8s manifests
    // In a real implementation, you might want to use a proper YAML library
    return [
      `apiVersion: ${manifest.apiVersion}`,
      `kind: ${manifest.kind}`,
      `metadata:`,
      `  name: ${manifest.metadata.name}`,
      manifest.metadata.namespace ? `  namespace: ${manifest.metadata.namespace}` : '',
      manifest.metadata.labels ? this.objectToYaml('  labels', manifest.metadata.labels, 2) : '',
      `spec:`,
      this.objectToYaml('', manifest.spec, 1)
    ].filter(Boolean).join('\n');
  }

  /**
   * Convert object to YAML format (simple implementation)
   */
  private objectToYaml(prefix: string, obj: any, indent: number): string {
    const spaces = '  '.repeat(indent);
    const lines = [];
    
    if (prefix) {
      lines.push(prefix + ':');
    }

    for (const [key, value] of Object.entries(obj)) {
      if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
        lines.push(`${spaces}${key}:`);
        lines.push(this.objectToYaml('', value, indent + 1));
      } else if (Array.isArray(value)) {
        lines.push(`${spaces}${key}:`);
        value.forEach(item => {
          if (typeof item === 'object') {
            lines.push(`${spaces}- `);
            lines.push(this.objectToYaml('', item, indent + 1).replace(new RegExp(`^${'  '.repeat(indent + 1)}`, 'gm'), `${spaces}  `));
          } else {
            lines.push(`${spaces}- ${item}`);
          }
        });
      } else {
        lines.push(`${spaces}${key}: ${value}`);
      }
    }

    return lines.join('\n');
  }

  /**
   * Delete a Kubernetes resource
   */
  private async deleteResource(resource: { kind: string; name: string; namespace: string }): Promise<boolean> {
    try {
      const args = ['delete', resource.kind.toLowerCase(), resource.name];
      if (resource.namespace) {
        args.push('-n', resource.namespace);
      }
      
      await KubernetesUtils.execCommand('kubectl', args);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Execute a kubectl command and return the result
   */
  private static async execCommand(
    command: string, 
    args: string[], 
    options: { timeout?: number } = {}
  ): Promise<{ stdout: string; stderr: string; exitCode: number }> {
    return new Promise((resolve, reject) => {
      const child = spawn(command, args, {
        stdio: 'pipe',
        env: { ...process.env }
      });

      let stdout = '';
      let stderr = '';

      child.stdout?.on('data', (data) => {
        stdout += data.toString();
      });

      child.stderr?.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('close', (code) => {
        resolve({
          stdout,
          stderr,
          exitCode: code || 0
        });
      });

      child.on('error', (error) => {
        reject(error);
      });

      // Handle timeout
      if (options.timeout) {
        setTimeout(() => {
          child.kill('SIGTERM');
          reject(new Error(`Command timed out after ${options.timeout}ms`));
        }, options.timeout);
      }
    });
  }
}