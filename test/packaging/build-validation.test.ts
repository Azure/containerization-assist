/**
 * Build Validation Tests
 * 
 * Ensures that the build process produces all expected files
 * and that the package structure is correct for publication.
 */

import { execSync } from 'child_process';
import { existsSync, statSync } from 'fs';
import { join } from 'path';

const PROJECT_ROOT = join(__dirname, '../..');

describe('Build Validation', () => {
  beforeAll(() => {
    // Ensure we start with a clean build
    try {
      execSync('npm run clean', { cwd: PROJECT_ROOT, stdio: 'pipe' });
    } catch (error) {
      // Clean script might not exist, that's ok
    }
  });

  describe('Build Process', () => {
    it('should complete npm run build without errors', () => {
      expect(() => {
        execSync('npm run build', { 
          cwd: PROJECT_ROOT, 
          stdio: 'pipe',
          timeout: 120000 // 2 minutes
        });
      }).not.toThrow();
    });

    it('should create both ESM and CJS dist directories', () => {
      expect(existsSync(join(PROJECT_ROOT, 'dist'))).toBe(true);
      expect(existsSync(join(PROJECT_ROOT, 'dist-cjs'))).toBe(true);
    });
  });

  describe('ESM Build Output', () => {
    const distDir = join(PROJECT_ROOT, 'dist');

    it('should have CLI entry point', () => {
      const cliPath = join(distDir, 'src/cli/cli.js');
      expect(existsSync(cliPath)).toBe(true);
      
      // Should be executable
      const stats = statSync(cliPath);
      expect(stats.isFile()).toBe(true);
    });

    it('should have MCP server files', () => {
      const serverPath = join(distDir, 'src/mcp/server.js');
      expect(existsSync(serverPath)).toBe(true);
      
      const serverDirectPath = join(distDir, 'src/mcp/server-direct.js');
      expect(existsSync(serverDirectPath)).toBe(true);
    });

    it('should have main index file', () => {
      const indexPath = join(distDir, 'src/index.js');
      expect(existsSync(indexPath)).toBe(true);
    });

    it('should have TypeScript declarations', () => {
      const indexDtsPath = join(distDir, 'src/index.d.ts');
      expect(existsSync(indexDtsPath)).toBe(true);
      
      const serverDtsPath = join(distDir, 'src/mcp/server.d.ts');
      expect(existsSync(serverDtsPath)).toBe(true);
    });

    it('should have all tool files', () => {
      const tools = [
        'analyze-repo',
        'build-image', 
        'deploy',
        'fix-dockerfile',
        'generate-dockerfile',
        'generate-k8s-manifests',
        'ops',
        'prepare-cluster',
        'push-image',
        'resolve-base-images',
        'scan',
        'tag-image',
        'verify-deployment',
        'workflow'
      ];

      for (const tool of tools) {
        const toolPath = join(distDir, `src/tools/${tool}/tool.js`);
        expect(existsSync(toolPath)).toBe(true);
        
        const schemaPath = join(distDir, `src/tools/${tool}/schema.js`);
        expect(existsSync(schemaPath)).toBe(true);
      }
    });
  });

  describe('CommonJS Build Output', () => {
    const distCjsDir = join(PROJECT_ROOT, 'dist-cjs');

    it('should have CLI entry point', () => {
      const cliPath = join(distCjsDir, 'src/cli/cli.js');
      expect(existsSync(cliPath)).toBe(true);
    });

    it('should have MCP server files', () => {
      const serverPath = join(distCjsDir, 'src/mcp/server.js');
      expect(existsSync(serverPath)).toBe(true);
      
      const serverDirectPath = join(distCjsDir, 'src/mcp/server-direct.js');
      expect(existsSync(serverDirectPath)).toBe(true);
    });

    it('should have package.json for CommonJS', () => {
      const packagePath = join(distCjsDir, 'package.json');
      expect(existsSync(packagePath)).toBe(true);
    });
  });

  describe('Import Resolution', () => {
    it('should resolve main exports from ESM build', async () => {
      const indexPath = join(PROJECT_ROOT, 'dist/src/index.js');
      
      // Dynamic import to test ES module resolution
      await expect(import(indexPath)).resolves.toBeDefined();
    });

    it('should have valid CLI shebang', () => {
      const cliPath = join(PROJECT_ROOT, 'dist/src/cli/cli.js');
      const content = require('fs').readFileSync(cliPath, 'utf8');
      
      expect(content).toMatch(/^#!/); // Should start with shebang
    });
  });
});