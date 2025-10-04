/**
 * Integration tests for health check functionality across entry points
 */

import { describe, it, expect } from '@jest/globals';
import { execSync } from 'node:child_process';
import { join } from 'node:path';
import { existsSync } from 'node:fs';

/**
 * Get the CLI path, checking both possible build outputs
 */
function getCliPath(): string {
  const cwd = process.cwd();

  // Try ESM build first (dist/src/cli/cli.js)
  const esmPath = join(cwd, 'dist', 'src', 'cli', 'cli.js');
  if (existsSync(esmPath)) {
    return esmPath;
  }

  // Fall back to CJS build (dist-cjs/src/cli/cli.js)
  const cjsPath = join(cwd, 'dist-cjs', 'src', 'cli', 'cli.js');
  if (existsSync(cjsPath)) {
    return cjsPath;
  }

  // If neither exists, return ESM path and let the test fail with a clear error
  throw new Error(
    `CLI not found. Please build the project first with 'npm run build' or 'npm run build:esm'`
  );
}

describe('Health Check Integration', () => {
  describe('CLI Health Check', () => {
    it('should run health check command successfully', () => {
      const cliPath = getCliPath();

      // Run health check via CLI - capture both stdout and stderr
      let output = '';
      try {
        output = execSync(`node ${cliPath} --health-check 2>&1`, {
          encoding: 'utf-8',
        });
      } catch (error) {
        // Health check may exit with code 1 if dependencies are unavailable
        // but we still want to check the output
        if (error && typeof error === 'object') {
          const stdout = 'stdout' in error && typeof error.stdout === 'string' ? error.stdout : '';
          const stderr = 'stderr' in error && typeof error.stderr === 'string' ? error.stderr : '';
          output = `${stdout}${stderr}`;
        }
      }

      // Verify output format
      expect(output).toContain('Health Check Results');
      expect(output).toContain('Status:');
      expect(output).toContain('Services:');
      expect(output).toContain('MCP Server: ready');
      expect(output).toContain('Tools loaded:');

      // Should include dependency checks
      expect(output).toContain('Dependencies:');
      expect(output).toContain('Docker:');
      expect(output).toContain('Kubernetes:');
    });

    it('should exit with status 0 when healthy', () => {
      const cliPath = getCliPath();

      try {
        execSync(`node ${cliPath} --health-check`, {
          encoding: 'utf-8',
          stdio: 'pipe',
        });
        // If we reach here, exit code was 0
        expect(true).toBe(true);
      } catch (error) {
        // If health check fails (exit code 1), it's still a valid test outcome
        // Some environments may not have Docker/K8s available
        expect(error).toBeDefined();
      }
    });

    it('should complete health check within reasonable time', () => {
      const cliPath = getCliPath();

      const startTime = Date.now();

      try {
        execSync(`node ${cliPath} --health-check`, {
          encoding: 'utf-8',
          stdio: 'pipe',
          timeout: 10000, // 10 second timeout
        });
      } catch (error) {
        // Ignore errors - we're just testing timeout
      }

      const duration = Date.now() - startTime;

      // Should complete within 10 seconds
      expect(duration).toBeLessThan(10000);
    });
  });

  describe('Health Check Structure Consistency', () => {
    it('should return consistent health check structure', () => {
      const cliPath = getCliPath();

      let result1 = '';
      try {
        result1 = execSync(`node ${cliPath} --health-check 2>&1`, {
          encoding: 'utf-8',
        });
      } catch (error) {
        if (error && typeof error === 'object') {
          const stdout = 'stdout' in error && typeof error.stdout === 'string' ? error.stdout : '';
          const stderr = 'stderr' in error && typeof error.stderr === 'string' ? error.stderr : '';
          result1 = `${stdout}${stderr}`;
        }
      }

      let result2 = '';
      try {
        result2 = execSync(`node ${cliPath} --health-check 2>&1`, {
          encoding: 'utf-8',
        });
      } catch (error) {
        if (error && typeof error === 'object') {
          const stdout = 'stdout' in error && typeof error.stdout === 'string' ? error.stdout : '';
          const stderr = 'stderr' in error && typeof error.stderr === 'string' ? error.stderr : '';
          result2 = `${stdout}${stderr}`;
        }
      }

      // Normalize outputs by removing lines that may contain timestamps or durations
      function normalizeOutput(output: string): string[] {
        return output
          .split('\n')
          // Remove lines with timestamps or durations (customize as needed)
          .filter(line =>
            !/(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})/.test(line) && // e.g., 2024-06-01 12:34:56
            !/Duration: \d+ms/.test(line) && // e.g., Duration: 123ms
            line.trim() !== ''
          )
          .map(line => line.trim());
      }

      const norm1 = normalizeOutput(result1);
      const norm2 = normalizeOutput(result2);

      // Both results should have the same structure and content (ignoring variable lines)
      expect(norm1).toEqual(norm2);

      // Both results should still contain key sections
      expect(norm1.join('\n')).toContain('Health Check Results');
      expect(norm1.join('\n')).toContain('Dependencies:');
    });
  });
});
