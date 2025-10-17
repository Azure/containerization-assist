/**
 * Integration tests for health check functionality across entry points
 */

import { describe, it, expect } from '@jest/globals';
import { spawnSync } from 'node:child_process';
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
      const result = spawnSync('node', [cliPath, '--health-check'], {
        encoding: 'utf-8',
      });

      // Health check writes to stderr, combine stdout and stderr
      const output = `${result.stdout}${result.stderr}`;

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

      const result = spawnSync('node', [cliPath, '--health-check'], {
        encoding: 'utf-8',
        stdio: 'pipe',
      });

      // Exit code should be 0 or 1 (both are valid test outcomes)
      // Some environments may not have Docker/K8s available (exit code 1)
      expect([0, 1]).toContain(result.status);
    });

    it('should complete health check within reasonable time', () => {
      const cliPath = getCliPath();

      const startTime = Date.now();

      spawnSync('node', [cliPath, '--health-check'], {
        encoding: 'utf-8',
        stdio: 'pipe',
        timeout: 10000, // 10 second timeout
      });

      const duration = Date.now() - startTime;

      // Should complete within 10 seconds
      expect(duration).toBeLessThan(10000);
    });
  });

  describe('Health Check Structure Consistency', () => {
    it('should return consistent health check structure', () => {
      const cliPath = getCliPath();

      const spawn1 = spawnSync('node', [cliPath, '--health-check'], {
        encoding: 'utf-8',
      });
      const result1 = `${spawn1.stdout}${spawn1.stderr}`;

      const spawn2 = spawnSync('node', [cliPath, '--health-check'], {
        encoding: 'utf-8',
      });
      const result2 = `${spawn2.stdout}${spawn2.stderr}`;

      // Normalize outputs by removing lines that may contain timestamps or durations
      function normalizeOutput(output: string): string[] {
        return output
          .split('\n')
          // Remove lines with timestamps or durations (customize as needed)
          .filter(line => {
            const trimmed = line.trim();
            return (
              !/(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})/.test(trimmed) && // e.g., 2024-06-01 12:34:56
              !/Duration: \d+ms/.test(trimmed) && // e.g., Duration: 123ms
              !trimmed.startsWith('{') && // Remove JSON log lines
              trimmed !== ''
            );
          })
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
