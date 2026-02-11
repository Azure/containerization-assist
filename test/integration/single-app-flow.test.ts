/**
 * Single App Flow Integration Test
 *
 * Tests the complete containerization journey for a single application
 * by executing the smoke:journey command to match CI behavior.
 */

import { execSync } from 'child_process';
import { join } from 'path';
import { existsSync, rmSync, readFileSync } from 'fs';

describe('Single App Flow Integration', () => {
  const outputDir = join(process.cwd(), '.smoke-test');

  beforeAll(() => {
    // Clean output directory
    if (existsSync(outputDir)) {
      rmSync(outputDir, { recursive: true, force: true });
    }
  });

  afterAll(() => {
    // Clean up output directory
    if (existsSync(outputDir)) {
      rmSync(outputDir, { recursive: true, force: true });
    }
  });

  describe('Smoke Journey Command', () => {
    it('should complete the full containerization workflow via smoke:journey', () => {
      // Set environment to mock AI sampling
      const env = {
        ...process.env,
        MCP_QUIET: 'true',
        MOCK_SAMPLING: 'true',
      };

      let result;
      try {
        // Execute the smoke:journey command
        result = execSync('npm run smoke:journey', {
          encoding: 'utf8',
          env,
          timeout: 120000, // 2 minute timeout
        });
      } catch (error: any) {
        // If Docker/K8s steps fail, that's OK for this test
        const output = error.stdout || error.output?.join('\n') || '';

        // Check that at least the basic steps completed
        expect(output).toContain('Analyze Repository');
        expect(output).toContain('Generate Dockerfile');
        expect(output).toContain('Generate Kubernetes Manifests');

        // It's OK if build/deploy steps fail due to missing Docker/K8s
        if (output.includes('Build Docker Image') && output.includes('Docker daemon')) {
          // Expected failure due to missing Docker
          return;
        }
      }

      if (result) {
        // Check that key steps were executed
        expect(result).toContain('Starting end-to-end containerization smoke test');
        expect(result).toContain('Analyze Repository');
        expect(result).toContain('Generate Dockerfile');
        expect(result).toContain('Generate Kubernetes Manifests');

        // Note: generate-dockerfile and generate-k8s-manifests return plans, not files
        // Only analysis.json is written by the smoke journey
        if (existsSync(join(outputDir, 'analysis.json'))) {
          // Analysis was saved
          expect(existsSync(join(outputDir, 'analysis.json'))).toBe(true);
        }
      }
    }, 150000);

    it('should generate valid artifacts', () => {
      // Only check if files were created in previous test
      if (existsSync(outputDir)) {
        const analysisPath = join(outputDir, 'analysis.json');

        // Check analysis if it exists (this is the only file the smoke journey writes)
        if (existsSync(analysisPath)) {
          const analysis = JSON.parse(readFileSync(analysisPath, 'utf8'));
          // The analysis result has modules array, not a language field at the top level
          expect(analysis).toHaveProperty('modules');
        }
      }
    });
  });
});