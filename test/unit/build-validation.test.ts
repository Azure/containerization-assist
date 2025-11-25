import { describe, it, expect, beforeAll } from '@jest/globals';
import { existsSync, readdirSync, statSync, readFileSync } from 'fs';
import { join } from 'path';
import { execSync } from 'child_process';

/**
 * Build Validation Tests
 *
 * These tests ensure that critical runtime resources (prompts and knowledge data)
 * are properly included in the built package. This prevents issues where the
 * published npm package is missing required files.
 */
describe('Build Output Validation', () => {
  const rootDir = process.cwd();
  const distDir = join(rootDir, 'dist');
  const distCjsDir = join(rootDir, 'dist-cjs');

  // Build the project before running tests if dist doesn't exist
  beforeAll(() => {
    if (!existsSync(distDir)) {
      console.log('Building project for validation tests...');
      execSync('npm run build', { stdio: 'inherit' });
    }
  });

  /**
   * Read and parse package.json
   */
  function getPackageJson() {
    const packageJsonPath = join(rootDir, 'package.json');
    const packageJsonContent = readFileSync(packageJsonPath, 'utf-8');
    return JSON.parse(packageJsonContent);
  }

  /**
   * Extract export paths from package.json
   */
  function getExportPaths(packageJson: Record<string, unknown>) {
    const exports = packageJson.exports as Record<string, unknown>;
    const paths: Array<{ name: string; esm?: string; cjs?: string; types?: string }> = [];

    for (const [name, config] of Object.entries(exports)) {
      if (typeof config === 'object' && config !== null) {
        const exportConfig = config as Record<string, unknown>;
        paths.push({
          name,
          esm: typeof exportConfig.import === 'string' ? exportConfig.import : undefined,
          cjs: typeof exportConfig.require === 'string' ? exportConfig.require : undefined,
          types: typeof exportConfig.types === 'string' ? exportConfig.types : undefined,
        });
      }
    }

    return paths;
  }

  describe('Build directories exist', () => {
    it('should have dist/ directory', () => {
      expect(existsSync(distDir)).toBe(true);
    });

    it('should have dist-cjs/ directory', () => {
      expect(existsSync(distCjsDir)).toBe(true);
    });
  });

  describe('Module format validation', () => {
    it('ESM files should use ES module syntax', () => {
      const indexPath = join(distDir, 'src/index.js');
      if (existsSync(indexPath)) {
        const content = readFileSync(indexPath, 'utf-8');
        // ESM should have 'export' or 'import' statements
        expect(content).toMatch(/export\s+{|import\s+{/);
        // ESM should not have "use strict" at the top (TypeScript ESM doesn't add it)
        const firstLines = content.split('\n').slice(0, 10).join('\n');
        expect(firstLines).not.toMatch(/^"use strict";/);
      }
    });

    it('CJS files should use CommonJS syntax', () => {
      const indexPath = join(distCjsDir, 'src/index.js');
      if (existsSync(indexPath)) {
        const content = readFileSync(indexPath, 'utf-8');
        // CJS should have "use strict" and require/exports
        expect(content).toMatch(/"use strict"/);
        expect(content).toMatch(/require\(|exports\./);
      }
    });
  });

  describe('Declaration files and source maps', () => {
    const keyFiles = ['src/index', 'src/mcp/mcp-server', 'src/tools/index', 'src/types/index'];

    for (const file of keyFiles) {
      it(`should have .d.ts for ${file}`, () => {
        const declPath = join(distDir, `${file}.d.ts`);
        expect(existsSync(declPath)).toBe(true);
      });

      it(`should have .d.ts.map for ${file}`, () => {
        const mapPath = join(distDir, `${file}.d.ts.map`);
        expect(existsSync(mapPath)).toBe(true);
      });

      it(`should have .js.map for ${file} in ESM build`, () => {
        const mapPath = join(distDir, `${file}.js.map`);
        expect(existsSync(mapPath)).toBe(true);
      });

      it(`should have .js.map for ${file} in CJS build`, () => {
        const mapPath = join(distCjsDir, `${file}.js.map`);
        expect(existsSync(mapPath)).toBe(true);
      });
    }
  });

  describe('Binary files', () => {
    it('should have executable CLI in dist/', () => {
      const cliPath = join(distDir, 'src/cli/cli.js');
      expect(existsSync(cliPath)).toBe(true);
    });

    it('CLI should have proper shebang', () => {
      const cliPath = join(distDir, 'src/cli/cli.js');
      if (existsSync(cliPath)) {
        const content = readFileSync(cliPath, 'utf-8');
        expect(content).toMatch(/^#!/);
      }
    });
  });

  describe('ESM Build (dist)', () => {

    describe('Knowledge Data Directory', () => {
      // Knowledge data is located in top-level knowledge/packs/ directory
      const knowledgeDataDir = join(rootDir, 'knowledge', 'packs');

      it('should have knowledge data in top-level knowledge/packs directory', () => {
        expect(existsSync(knowledgeDataDir)).toBe(true);
      });

      it('should include all knowledge pack files', () => {
        const expectedPacks = [
          'starter-pack.json',
          'nodejs-pack.json',
          'python-pack.json',
          'java-pack.json',
          'dotnet-pack.json',
          'go-pack.json',
          'kubernetes-pack.json',
          'security-pack.json'
        ];

        const files = readdirSync(knowledgeDataDir);

        expectedPacks.forEach(pack => {
          expect(files).toContain(pack);
        });
      });

      it('should have valid JSON content in knowledge files', () => {
        const files = readdirSync(knowledgeDataDir).filter(f => f.endsWith('.json'));

        files.forEach(file => {
          const filePath = join(knowledgeDataDir, file);
          const content = readFileSync(filePath, 'utf-8');
          // Should be valid JSON
          expect(() => JSON.parse(content)).not.toThrow();
        });
      });
    });
  });

  describe('CommonJS Build (dist-cjs)', () => {
    // Knowledge data is no longer duplicated in dist-cjs
    // It's in the top-level knowledge/packs/ directory only
  });

  describe('Package Integrity', () => {
    it('should have knowledge data files with reasonable sizes', () => {
      // Knowledge data is now in top-level knowledge/packs/ directory
      const knowledgeDataDir = join(rootDir, 'knowledge', 'packs');
      const files = readdirSync(knowledgeDataDir).filter(f => f.endsWith('.json'));

      files.forEach(file => {
        const filePath = join(knowledgeDataDir, file);
        const stats = statSync(filePath);

        // Each knowledge pack should be at least 1KB but not more than 100KB
        expect(stats.size).toBeGreaterThan(1000);
        expect(stats.size).toBeLessThan(100000);
      });
    });
  });

  describe('Runtime Loading Validation', () => {
    it('should be able to find knowledge data at runtime', () => {
      const knowledgeDataDir = join(rootDir, 'knowledge', 'packs');
      expect(existsSync(knowledgeDataDir)).toBe(true);

      // Verify it contains knowledge pack files
      const files = readdirSync(knowledgeDataDir);
      const jsonFiles = files.filter(f => f.endsWith('.json'));

      // Should have knowledge pack files
      expect(jsonFiles.length).toBeGreaterThan(0);
    });
  });
});