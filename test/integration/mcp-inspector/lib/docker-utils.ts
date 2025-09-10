/**
 * Docker Utilities for Integration Testing
 * Provides Docker operations for real containerization testing
 */

import { spawn } from 'child_process';
import { writeFile, unlink, mkdtemp } from 'fs/promises';
import { join } from 'path';
import { tmpdir } from 'os';

export interface BuildConfig {
  dockerfile: string;
  context: string;
  tag: string;
  args?: Record<string, string>;
  platform?: string;
}

export interface BuildResult {
  success: boolean;
  imageId?: string;
  imageTag: string;
  buildLog: string;
  error?: string;
  duration: number;
}

export interface RunConfig {
  image: string;
  timeout?: number;
  expectedLogs?: string[];
  ports?: Array<{ host: number; container: number }>;
  env?: Record<string, string>;
  detached?: boolean;
}

export interface RunResult {
  success: boolean;
  containerId?: string;
  logs: string;
  error?: string;
  duration: number;
}

export interface ImageInfo {
  id: string;
  tag: string;
  size: number;
  created: string;
}

export class DockerUtils {
  private createdImages: Set<string> = new Set();
  private runningContainers: Set<string> = new Set();
  private tempFiles: Set<string> = new Set();

  /**
   * Check if Docker is available in the environment
   */
  static async isDockerAvailable(): Promise<boolean> {
    try {
      const result = await DockerUtils.execCommand('docker', ['--version']);
      return result.exitCode === 0;
    } catch {
      return false;
    }
  }

  /**
   * Build a Docker image from Dockerfile content and context
   */
  async buildImage(config: BuildConfig): Promise<BuildResult> {
    const startTime = performance.now();
    
    try {
      // Create temporary Dockerfile if needed
      let dockerfilePath: string;
      let dockerfileContent = config.dockerfile;
      
      // Fix common test fixture issues
      if (dockerfileContent.includes('npm ci --only=production')) {
        dockerfileContent = dockerfileContent.replace('npm ci --only=production', 'npm install --production');
      }
      
      if (dockerfileContent.includes('\n') || !dockerfileContent.startsWith('FROM')) {
        const tempDir = await mkdtemp(join(tmpdir(), 'dockerfile-'));
        dockerfilePath = join(tempDir, 'Dockerfile');
        await writeFile(dockerfilePath, dockerfileContent);
        this.tempFiles.add(dockerfilePath);
      } else {
        dockerfilePath = dockerfileContent;
      }

      // Build Docker command
      const buildArgs = [
        'build',
        '-t', config.tag,
        '-f', dockerfilePath
      ];

      // Add build args
      if (config.args) {
        Object.entries(config.args).forEach(([key, value]) => {
          buildArgs.push('--build-arg', `${key}=${value}`);
        });
      }

      // Add platform if specified
      if (config.platform) {
        buildArgs.push('--platform', config.platform);
      }

      // Add context
      buildArgs.push(config.context);

      const result = await DockerUtils.execCommand('docker', buildArgs, { timeout: 120000 });
      const duration = performance.now() - startTime;

      if (result.exitCode === 0) {
        // Get image ID
        const inspectResult = await DockerUtils.execCommand('docker', ['images', config.tag, '--format', '{{.ID}}']);
        const imageId = inspectResult.stdout.trim();

        this.createdImages.add(config.tag);
        if (imageId) {
          this.createdImages.add(imageId);
        }

        return {
          success: true,
          imageId,
          imageTag: config.tag,
          buildLog: result.stdout + result.stderr,
          duration
        };
      } else {
        return {
          success: false,
          imageTag: config.tag,
          buildLog: result.stdout + result.stderr,
          error: result.stderr || 'Build failed with unknown error',
          duration
        };
      }
    } catch (error) {
      const duration = performance.now() - startTime;
      return {
        success: false,
        imageTag: config.tag,
        buildLog: '',
        error: error instanceof Error ? error.message : String(error),
        duration
      };
    }
  }

  /**
   * Run a container and optionally wait for expected logs
   */
  async runContainer(config: RunConfig): Promise<RunResult> {
    const startTime = performance.now();
    
    try {
      const runArgs = ['run'];
      
      if (config.detached) {
        runArgs.push('-d');
      } else {
        runArgs.push('--rm');
      }

      // Add port mappings
      if (config.ports) {
        config.ports.forEach(port => {
          runArgs.push('-p', `${port.host}:${port.container}`);
        });
      }

      // Add environment variables
      if (config.env) {
        Object.entries(config.env).forEach(([key, value]) => {
          runArgs.push('-e', `${key}=${value}`);
        });
      }

      runArgs.push(config.image);

      const timeout = config.timeout || 30000;
      const result = await DockerUtils.execCommand('docker', runArgs, { timeout });
      const duration = performance.now() - startTime;

      if (result.exitCode === 0) {
        const containerId = config.detached ? result.stdout.trim() : undefined;
        
        if (containerId) {
          this.runningContainers.add(containerId);
        }

        // Check for expected logs if specified
        if (config.expectedLogs && config.expectedLogs.length > 0) {
          const logs = result.stdout + result.stderr;
          const hasExpectedLogs = config.expectedLogs.every(expected => 
            logs.includes(expected)
          );
          
          if (!hasExpectedLogs) {
            return {
              success: false,
              containerId,
              logs,
              error: `Expected logs not found: ${config.expectedLogs.join(', ')}`,
              duration
            };
          }
        }

        return {
          success: true,
          containerId,
          logs: result.stdout + result.stderr,
          duration
        };
      } else {
        return {
          success: false,
          logs: result.stdout + result.stderr,
          error: result.stderr || 'Container run failed',
          duration
        };
      }
    } catch (error) {
      const duration = performance.now() - startTime;
      return {
        success: false,
        logs: '',
        error: error instanceof Error ? error.message : String(error),
        duration
      };
    }
  }

  /**
   * Get information about an image
   */
  async getImageInfo(imageTag: string): Promise<ImageInfo | null> {
    try {
      const result = await DockerUtils.execCommand('docker', [
        'inspect', imageTag, '--format',
        '{{.Id}}|{{join .RepoTags ","}}|{{.Size}}|{{.Created}}'
      ]);

      if (result.exitCode === 0) {
        const [id, tags, size, created] = result.stdout.trim().split('|');
        return {
          id: id.replace('sha256:', '').substring(0, 12),
          tag: imageTag,
          size: parseInt(size) || 0,
          created
        };
      }
      
      return null;
    } catch {
      return null;
    }
  }

  /**
   * Get logs from a running container
   */
  async getContainerLogs(containerId: string): Promise<string> {
    try {
      const result = await DockerUtils.execCommand('docker', ['logs', containerId]);
      return result.stdout + result.stderr;
    } catch {
      return '';
    }
  }

  /**
   * Stop and remove a container
   */
  async stopContainer(containerId: string): Promise<boolean> {
    try {
      await DockerUtils.execCommand('docker', ['stop', containerId]);
      await DockerUtils.execCommand('docker', ['rm', containerId]);
      this.runningContainers.delete(containerId);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Build image from Dockerfile content (convenience method)
   */
  async buildFromDockerfile(dockerfileContent: string, tag: string, context: string = '.'): Promise<BuildResult> {
    return this.buildImage({
      dockerfile: dockerfileContent,
      context,
      tag
    });
  }

  /**
   * Clean up all created resources
   */
  async cleanup(): Promise<void> {
    const cleanupPromises = [];

    // Stop and remove containers
    for (const containerId of this.runningContainers) {
      cleanupPromises.push(this.stopContainer(containerId));
    }

    // Remove images
    for (const imageId of this.createdImages) {
      cleanupPromises.push(this.removeImage(imageId));
    }

    // Clean up temp files
    for (const filePath of this.tempFiles) {
      cleanupPromises.push(
        unlink(filePath).catch(() => {}) // Ignore errors
      );
    }

    await Promise.allSettled(cleanupPromises);
    
    this.runningContainers.clear();
    this.createdImages.clear();
    this.tempFiles.clear();
  }

  /**
   * Remove a Docker image
   */
  private async removeImage(imageId: string): Promise<boolean> {
    try {
      await DockerUtils.execCommand('docker', ['rmi', '-f', imageId]);
      return true;
    } catch {
      return false;
    }
  }

  /**
   * Execute a command and return the result
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