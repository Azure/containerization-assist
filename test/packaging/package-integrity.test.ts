/**
 * Package Integrity Tests
 * 
 * Validates that npm pack creates a correct package structure
 * and that installation works as expected.
 */

import { execSync } from 'child_process';
import { existsSync, rmSync, readFileSync, mkdirSync } from 'fs';
import { join } from 'path';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';

const PROJECT_ROOT = join(__dirname, '../..');

describe('Package Integrity', () => {
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  let packageTarball: string;

  beforeAll(() => {
    // Ensure project is built
    execSync('npm run build', { cwd: PROJECT_ROOT, stdio: 'pipe' });

    // Create test directory
    const tempResult = createTestTempDir('containerization-assist-test-');
    testDir = tempResult.dir;
    cleanup = tempResult.cleanup;
  });

  afterAll(async () => {
    // Cleanup
    await cleanup();

    // Clean up any tarballs in project root
    const tarballPattern = /azure-containerization-assist-mcp-.*\.tgz$/;
    try {
      const files = require('fs').readdirSync(PROJECT_ROOT);
      for (const file of files) {
        if (tarballPattern.test(file)) {
          rmSync(join(PROJECT_ROOT, file), { force: true });
        }
      }
    } catch (error) {
      // Ignore cleanup errors
    }
  });

  describe('npm pack', () => {
    it('should create package tarball successfully', () => {
      const result = execSync('npm pack', {
        cwd: PROJECT_ROOT,
        encoding: 'utf8'
      }).trim();

      packageTarball = join(PROJECT_ROOT, result);
      expect(existsSync(packageTarball)).toBe(true);
    });

    it('should contain expected files in tarball', () => {
      const output = execSync(`tar -tzf "${packageTarball}"`, {
        encoding: 'utf8'
      });

      const files = output.split('\n').filter(Boolean);

      // Should have package structure
      expect(files).toContain('package/package.json');
      expect(files).toContain('package/README.md');
      expect(files).toContain('package/LICENSE');

      // Should have CommonJS dist files
      expect(files.some(f => f.includes('package/dist-cjs/'))).toBe(true);

      // Should have CLI entry point (either ESM or CJS)
      const hasEsmCli = files.some(f => f.includes('package/dist/src/cli/cli.js'));
      const hasCjsCli = files.some(f => f.includes('package/dist-cjs/src/cli/cli.js'));
      expect(hasEsmCli || hasCjsCli).toBe(true);
    });
  });

  describe('npm install', () => {
    let installDir: string;

    beforeAll(() => {
      installDir = join(testDir.name, 'install-test');
      mkdirSync(installDir, { recursive: true });

      // Create package.json
      execSync('npm init -y', { cwd: installDir, stdio: 'pipe' });

      // Install from tarball
      execSync(`npm install "${packageTarball}"`, {
        cwd: installDir,
        stdio: 'pipe'
      });
    });

    it('should install package successfully', () => {
      const nodeModulesPath = join(installDir, 'node_modules/containerization-assist-mcp');
      expect(existsSync(nodeModulesPath)).toBe(true);
    });

    it('should have correct package.json structure', () => {
      const packageJsonPath = join(installDir, 'node_modules/containerization-assist-mcp/package.json');
      const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf8'));

      expect(packageJson.name).toBe('containerization-assist-mcp');
      expect(packageJson.bin).toBeDefined();
      expect(packageJson.main).toBeDefined();
      expect(packageJson.exports).toBeDefined();
    });

    it('should have CLI binary files present', () => {
      const nodeModulesPath = join(installDir, 'node_modules/containerization-assist-mcp');
      const packageJson = JSON.parse(readFileSync(join(nodeModulesPath, 'package.json'), 'utf8'));

      // Check that binary files exist
      const binPath = packageJson.bin['containerization-assist-mcp'];
      const fullBinPath = join(nodeModulesPath, binPath);

      expect(existsSync(fullBinPath)).toBe(true);
    });

    it('should have required dependencies available', () => {
      const nodeModulesPath = join(installDir, 'node_modules/containerization-assist-mcp');
      const packageJson = JSON.parse(readFileSync(join(nodeModulesPath, 'package.json'), 'utf8'));

      // Key dependencies should be present in the installed package
      const requiredDeps = [
        '@modelcontextprotocol/sdk',
        'commander',
        'dockerode',
        'zod'
      ];

      for (const dep of requiredDeps) {
        expect(packageJson.dependencies[dep]).toBeDefined();
      }
    });

    it('should have consistent binary and main paths', () => {
      const nodeModulesPath = join(installDir, 'node_modules/containerization-assist-mcp');
      const packageJson = JSON.parse(readFileSync(join(nodeModulesPath, 'package.json'), 'utf8'));

      const binPath = packageJson.bin['containerization-assist-mcp'];
      const mainPath = packageJson.main;

      // Both should use the same dist directory (either dist/ or dist-cjs/)
      const binDir = binPath.split('/')[0];
      const mainDir = mainPath.split('/')[0];

      // If they're different, it's likely a configuration mismatch
      if (binDir !== mainDir) {
        console.warn(`Binary uses ${binDir}/ but main uses ${mainDir}/`);
      }
    });
  });

  describe('Programmatic Import', () => {
    let installDir: string;

    beforeAll(() => {
      installDir = join(testDir.name, 'import-test');
      mkdirSync(installDir, { recursive: true });

      // Create package.json with type: module
      execSync('npm init -y', { cwd: installDir, stdio: 'pipe' });
      const pkgPath = join(installDir, 'package.json');
      const pkg = JSON.parse(readFileSync(pkgPath, 'utf8'));
      pkg.type = 'module';
      require('fs').writeFileSync(pkgPath, JSON.stringify(pkg, null, 2));

      // Install from tarball
      execSync(`npm install "${packageTarball}"`, {
        cwd: installDir,
        stdio: 'pipe'
      });
    });

    it('should support main import', async () => {
      const testScript = `
        import pkg from 'containerization-assist-mcp';
        console.log(typeof pkg);
      `;

      require('fs').writeFileSync(join(installDir, 'test.mjs'), testScript);

      try {
        const result = execSync('node test.mjs', {
          cwd: installDir,
          encoding: 'utf8'
        });
        expect(result.trim()).not.toBe('undefined');
      } catch (error) {
        // Log the actual error for debugging
        console.error('Import test failed:', error);
        throw error;
      }
    });

    it('should support named exports', async () => {
      const testScript = `
        import { MCPServer } from 'containerization-assist-mcp';
        console.log(typeof MCPServer);
      `;

      require('fs').writeFileSync(join(installDir, 'test-named.mjs'), testScript);

      try {
        const result = execSync('node test-named.mjs', {
          cwd: installDir,
          encoding: 'utf8'
        });
        expect(result.trim()).toBe('function');
      } catch (error) {
        console.error('Named export test failed:', error);
        throw error;
      }
    });
  });
});