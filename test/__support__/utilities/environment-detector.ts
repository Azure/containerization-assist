/**
 * Universal Environment Detector
 * Detects available services and capabilities for integration testing
 */

import { exec } from 'child_process';
import { promisify } from 'util';
import Docker from 'dockerode';
import { promises as fs } from 'fs';

const execAsync = promisify(exec);

export interface EnvironmentCapabilities {
  docker: {
    available: boolean;
    version?: string;
    socketPath?: string;
    error?: string;
  };
  registry: {
    available: boolean;
    host?: string;
    port?: number;
    error?: string;
  };
  trivy: {
    available: boolean;
    version?: string;
    binaryPath?: string;
    containerFallback: boolean;
    error?: string;
  };
  kubernetes: {
    available: boolean;
    context?: string;
    version?: string;
    type?: 'kind' | 'minikube' | 'docker-desktop' | 'remote' | 'unknown';
    error?: string;
  };
  ai: {
    available: boolean;
    service?: string;
    error?: string;
  };
  platform: {
    os: string;
    ci: boolean;
    skipIntegration: boolean;
  };
}

export interface DetectionOptions {
  timeout?: number; // Detection timeout in milliseconds (default: 5000)
  skipDocker?: boolean;
  skipRegistry?: boolean;
  skipTrivy?: boolean;
  skipKubernetes?: boolean;
  skipAi?: boolean;
}

const DEFAULT_TIMEOUT = 5000;
const REGISTRY_DEFAULT_HOST = 'localhost';
const REGISTRY_DEFAULT_PORT = 5000;

/**
 * Detect Docker daemon availability and configuration
 */
async function detectDocker(timeout: number = DEFAULT_TIMEOUT): Promise<EnvironmentCapabilities['docker']> {
  try {
    // Determine socket path based on platform
    const platform = process.platform;
    const defaultSocketPath = platform === 'win32' 
      ? '//./pipe/docker_engine' 
      : '/var/run/docker.sock';
    
    const socketPath = process.env.DOCKER_SOCKET || defaultSocketPath;

    // Test socket accessibility first
    if (platform !== 'win32') {
      try {
        await fs.access(socketPath);
      } catch (error) {
        return {
          available: false,
          error: `Docker socket not accessible: ${socketPath}`
        };
      }
    }

    // Create Docker client and test connection
    const docker = new Docker({ 
      socketPath,
      timeout
    });

    const timeoutPromise = new Promise<never>((_, reject) => {
      setTimeout(() => reject(new Error('Docker connection timeout')), timeout);
    });

    const versionInfo = await Promise.race([
      docker.version(),
      timeoutPromise
    ]);

    return {
      available: true,
      version: versionInfo.Version,
      socketPath
    };
  } catch (error) {
    return {
      available: false,
      error: `Docker unavailable: ${error instanceof Error ? error.message : 'Unknown error'}`
    };
  }
}

/**
 * Detect local Docker registry availability
 */
async function detectRegistry(
  host: string = REGISTRY_DEFAULT_HOST,
  port: number = REGISTRY_DEFAULT_PORT,
  timeout: number = DEFAULT_TIMEOUT
): Promise<EnvironmentCapabilities['registry']> {
  try {
    const registryUrl = `http://${host}:${port}/v2/`;
    
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    const response = await fetch(registryUrl, {
      signal: controller.signal,
      method: 'GET'
    });

    clearTimeout(timeoutId);

    if (response.ok || response.status === 401) { // 401 is expected for registry
      return {
        available: true,
        host,
        port
      };
    } else {
      return {
        available: false,
        error: `Registry responded with status ${response.status}`
      };
    }
  } catch (error) {
    return {
      available: false,
      error: `Registry unavailable at ${host}:${port}: ${error instanceof Error ? error.message : 'Unknown error'}`
    };
  }
}

/**
 * Detect Trivy security scanner availability
 */
async function detectTrivy(timeout: number = DEFAULT_TIMEOUT): Promise<EnvironmentCapabilities['trivy']> {
  // First try to find Trivy binary
  try {
    const { stdout } = await execAsync('trivy version', { timeout });
    const versionMatch = stdout.match(/Version:\s*(.+)/);
    const version = versionMatch ? versionMatch[1].trim() : 'unknown';

    return {
      available: true,
      version,
      binaryPath: 'trivy',
      containerFallback: false
    };
  } catch (binaryError) {
    // Binary not available, check if Docker is available for container fallback
    const dockerCapabilities = await detectDocker(timeout);
    
    if (dockerCapabilities.available) {
      try {
        // Test if we can run Trivy container
        const { stdout } = await execAsync(
          'docker run --rm aquasec/trivy:latest version',
          { timeout }
        );
        
        const versionMatch = stdout.match(/Version:\s*(.+)/);
        const version = versionMatch ? versionMatch[1].trim() : 'unknown';

        return {
          available: true,
          version,
          binaryPath: 'docker',
          containerFallback: true
        };
      } catch (containerError) {
        return {
          available: false,
          error: `Trivy binary and container both unavailable`,
          containerFallback: false
        };
      }
    } else {
      return {
        available: false,
        error: `Trivy binary not found and Docker unavailable for container fallback`,
        containerFallback: false
      };
    }
  }
}

/**
 * Detect Kubernetes cluster availability and type
 */
async function detectKubernetes(timeout: number = DEFAULT_TIMEOUT): Promise<EnvironmentCapabilities['kubernetes']> {
  try {
    // Check if kubectl is available
    const { stdout: versionOutput } = await execAsync('kubectl version --client=true', { timeout });
    const versionMatch = versionOutput.match(/GitVersion:"(.+?)"/);
    const version = versionMatch ? versionMatch[1] : 'unknown';

    // Try to get cluster info to confirm connectivity
    try {
      const { stdout: contextOutput } = await execAsync('kubectl config current-context', { timeout });
      const context = contextOutput.trim();

      // Determine cluster type based on context
      let type: EnvironmentCapabilities['kubernetes']['type'] = 'unknown';
      if (context.includes('kind-')) {
        type = 'kind';
      } else if (context.includes('minikube')) {
        type = 'minikube';
      } else if (context.includes('docker-desktop') || context.includes('docker-for-desktop')) {
        type = 'docker-desktop';
      } else if (context.includes('gke_') || context.includes('eks_') || context.includes('aks_')) {
        type = 'remote';
      }

      // Test cluster connectivity
      await execAsync('kubectl cluster-info --request-timeout=2s', { timeout: Math.min(timeout, 3000) });

      return {
        available: true,
        context,
        version,
        type
      };
    } catch (clusterError) {
      return {
        available: false,
        error: `kubectl available but cluster not accessible: ${clusterError instanceof Error ? clusterError.message : 'Unknown error'}`
      };
    }
  } catch (error) {
    return {
      available: false,
      error: `Kubernetes/kubectl unavailable: ${error instanceof Error ? error.message : 'Unknown error'}`
    };
  }
}

/**
 * Detect AI service availability (simplified check)
 */
async function detectAi(): Promise<EnvironmentCapabilities['ai']> {
  // Check for AI service configuration (MCP-based only)
  const aiServiceUrl = process.env.AI_SERVICE_URL;

  if (aiServiceUrl) {
    return {
      available: true,
      service: 'custom'
    };
  }

  // MCP SDK provides AI capabilities through the host
  return {
    available: true,
    service: 'mcp-host'
  };
}

/**
 * Detect platform and CI environment
 */
function detectPlatform(): EnvironmentCapabilities['platform'] {
  return {
    os: process.platform,
    ci: !!(
      process.env.CI ||
      process.env.CONTINUOUS_INTEGRATION ||
      process.env.BUILD_NUMBER ||
      process.env.GITHUB_ACTIONS ||
      process.env.GITLAB_CI ||
      process.env.JENKINS_URL
    ),
    skipIntegration: process.env.SKIP_INTEGRATION_TESTS === 'true'
  };
}

/**
 * Main environment detection function
 */
export async function detectEnvironment(options: DetectionOptions = {}): Promise<EnvironmentCapabilities> {
  const {
    timeout = DEFAULT_TIMEOUT,
    skipDocker = false,
    skipRegistry = false,
    skipTrivy = false,
    skipKubernetes = false,
    skipAi = false
  } = options;

  const platform = detectPlatform();

  // Skip all integration detection if explicitly disabled
  if (platform.skipIntegration) {
    return {
      docker: { available: false, error: 'Integration tests disabled' },
      registry: { available: false, error: 'Integration tests disabled' },
      trivy: { available: false, error: 'Integration tests disabled', containerFallback: false },
      kubernetes: { available: false, error: 'Integration tests disabled' },
      ai: { available: false, error: 'Integration tests disabled' },
      platform
    };
  }

  // Run detection in parallel for speed
  const [docker, registry, trivy, kubernetes, ai] = await Promise.allSettled([
    skipDocker ? Promise.resolve({ available: false, error: 'Skipped' } as EnvironmentCapabilities['docker']) : detectDocker(timeout),
    skipRegistry ? Promise.resolve({ available: false, error: 'Skipped' } as EnvironmentCapabilities['registry']) : detectRegistry(REGISTRY_DEFAULT_HOST, REGISTRY_DEFAULT_PORT, timeout),
    skipTrivy ? Promise.resolve({ available: false, error: 'Skipped', containerFallback: false } as EnvironmentCapabilities['trivy']) : detectTrivy(timeout),
    skipKubernetes ? Promise.resolve({ available: false, error: 'Skipped' } as EnvironmentCapabilities['kubernetes']) : detectKubernetes(timeout),
    skipAi ? Promise.resolve({ available: false, error: 'Skipped' } as EnvironmentCapabilities['ai']) : detectAi()
  ]);

  return {
    docker: docker.status === 'fulfilled' ? docker.value : { available: false, error: 'Detection failed' },
    registry: registry.status === 'fulfilled' ? registry.value : { available: false, error: 'Detection failed' },
    trivy: trivy.status === 'fulfilled' ? trivy.value : { available: false, error: 'Detection failed', containerFallback: false },
    kubernetes: kubernetes.status === 'fulfilled' ? kubernetes.value : { available: false, error: 'Detection failed' },
    ai: ai.status === 'fulfilled' ? ai.value : { available: false, error: 'Detection failed' },
    platform
  };
}

/**
 * Create a summary report of environment capabilities
 */
export function createEnvironmentReport(capabilities: EnvironmentCapabilities): string {
  const lines = [
    '=== Integration Test Environment Report ===',
    '',
    `Platform: ${capabilities.platform.os}`,
    `CI Environment: ${capabilities.platform.ci ? 'Yes' : 'No'}`,
    `Integration Tests: ${capabilities.platform.skipIntegration ? 'DISABLED' : 'ENABLED'}`,
    '',
    '--- Service Availability ---',
    `🐳 Docker: ${capabilities.docker.available ? '✅ Available' : '❌ Unavailable'} ${capabilities.docker.version ? `(v${capabilities.docker.version})` : ''}`,
    `${capabilities.docker.available ? `   Socket: ${capabilities.docker.socketPath}` : `   Error: ${capabilities.docker.error}`}`,
    '',
    `📦 Registry: ${capabilities.registry.available ? '✅ Available' : '❌ Unavailable'}`,
    `${capabilities.registry.available ? `   Endpoint: ${capabilities.registry.host}:${capabilities.registry.port}` : `   Error: ${capabilities.registry.error}`}`,
    '',
    `🛡️  Trivy: ${capabilities.trivy.available ? '✅ Available' : '❌ Unavailable'} ${capabilities.trivy.version ? `(v${capabilities.trivy.version})` : ''}`,
    `${capabilities.trivy.available ? `   Mode: ${capabilities.trivy.containerFallback ? 'Container' : 'Binary'}` : `   Error: ${capabilities.trivy.error}`}`,
    '',
    `☸️  Kubernetes: ${capabilities.kubernetes.available ? '✅ Available' : '❌ Unavailable'} ${capabilities.kubernetes.version ? `(v${capabilities.kubernetes.version})` : ''}`,
    `${capabilities.kubernetes.available ? `   Context: ${capabilities.kubernetes.context} (${capabilities.kubernetes.type})` : `   Error: ${capabilities.kubernetes.error}`}`,
    '',
    `🤖 AI Service: ${capabilities.ai.available ? '✅ Available' : '❌ Unavailable'}`,
    `${capabilities.ai.available ? `   Type: ${capabilities.ai.service}` : `   Error: ${capabilities.ai.error}`}`,
    '',
    '=== Recommended Test Execution ===',
    capabilities.docker.available ? '✅ Run Docker integration tests' : '⏭️  Skip Docker tests (use mocks)',
    capabilities.registry.available ? '✅ Run Registry integration tests' : '⏭️  Skip Registry tests (use mocks)',
    capabilities.trivy.available ? `✅ Run Security scan tests ${capabilities.trivy.containerFallback ? '(container mode)' : '(binary mode)'}` : '⏭️  Skip Security tests (use mocks)',
    capabilities.kubernetes.available ? `✅ Run Kubernetes tests (${capabilities.kubernetes.type})` : '⏭️  Skip Kubernetes tests (use mocks)',
    capabilities.ai.available ? '✅ Run AI workflow tests' : '⏭️  Skip AI tests (use mocks)',
    ''
  ];

  return lines.join('\n');
}

/**
 * Utility function for conditional test execution
 */
export function createConditionalDescribe(requirements: Array<keyof Omit<EnvironmentCapabilities, 'platform'>>) {
  return (name: string, fn: () => void, capabilities?: EnvironmentCapabilities) => {
    if (!capabilities) {
      // If capabilities not provided, we'll detect at runtime
      return describe(name, () => {
        let envCapabilities: EnvironmentCapabilities;
        
        beforeAll(async () => {
          envCapabilities = await detectEnvironment({ timeout: 3000 });
        });

        const shouldSkip = () => {
          if (!envCapabilities) return true;
          return requirements.some(req => !envCapabilities[req].available);
        };

        describe('Environment Check', () => {
          test('should have required services available', () => {
            if (shouldSkip()) {
              console.log(`Skipping ${name} - Required services not available:`, 
                requirements.filter(req => !envCapabilities[req].available).join(', '));
              return;
            }
          });
        });

        if (!shouldSkip()) {
          fn();
        }
      });
    } else {
      // Use provided capabilities
      const shouldSkip = requirements.some(req => !capabilities[req].available);
      
      if (shouldSkip) {
        const missing = requirements.filter(req => !capabilities[req].available);
        console.log(`Skipping ${name} - Missing services: ${missing.join(', ')}`);
        return describe.skip(name, fn);
      } else {
        return describe(name, fn);
      }
    }
  };
}