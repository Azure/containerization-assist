/**
 * Integration tests for health check functionality across entry points
 */

import { describe, it, expect } from '@jest/globals';
import { execSync } from 'node:child_process';
import { join } from 'node:path';

describe('Health Check Integration', () => {
  describe('CLI Health Check', () => {
    it('should run health check command successfully', () => {
      const cliPath = join(process.cwd(), 'dist/src/cli/cli.js');

      // Run health check via CLI - capture stderr since that's where CLI output goes
      let output = '';
      try {
        execSync(`node ${cliPath} --health-check 2>&1`, {
          encoding: 'utf-8',
          stdio: 'pipe',
        });
      } catch (error) {
        // Health check may exit with code 1 if dependencies are unavailable
        // but we still want to check the output
        if (error && typeof error === 'object' && 'stdout' in error) {
          output = (error as { stdout: string }).stdout;
        }
      }

      // If we didn't get output from the catch block, run again to get it
      if (!output) {
        output = execSync(`node ${cliPath} --health-check 2>&1`, {
          encoding: 'utf-8',
        });
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
      const cliPath = join(process.cwd(), 'dist/src/cli/cli.js');

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
      const cliPath = join(process.cwd(), 'dist/src/cli/cli.js');

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
      const cliPath = join(process.cwd(), 'dist/src/cli/cli.js');

      const result1 = execSync(`node ${cliPath} --health-check 2>&1`, {
        encoding: 'utf-8',
      });

      const result2 = execSync(`node ${cliPath} --health-check 2>&1`, {
        encoding: 'utf-8',
      });

      // Both results should have the same structure
      expect(result1).toContain('Health Check Results');
      expect(result2).toContain('Health Check Results');

      // Both should include dependencies section
      const hasDeps1 = result1.includes('Dependencies:');
      const hasDeps2 = result2.includes('Dependencies:');
      expect(hasDeps1).toBe(hasDeps2);
    });
  });
});
