import { describe, it, expect, beforeAll } from '@jest/globals';
import { existsSync, readdirSync, statSync } from 'fs';
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

  describe('ESM Build (dist)', () => {
    describe('AI Module', () => {
      const aiDir = join(distDir, 'src', 'ai');

      it('should include AI directory in dist', () => {
        expect(existsSync(aiDir)).toBe(true);
      });

      it('should include prompt template files', () => {
        // Check for prompt template module in AI directory
        const expectedPromptFiles = [
          'prompt-templates.js'
        ];

        const files = readdirSync(aiDir).filter(item => {
          return item.endsWith('.js') || item.endsWith('.d.ts');
        });

        expectedPromptFiles.forEach(file => {
          expect(files).toContain(file);
        });
      });

      it('should include TypeScript declaration files', () => {
        const files = readdirSync(aiDir);
        const declarationFiles = files.filter(f => f.endsWith('.d.ts'));

        // Should have declaration files for type safety
        expect(declarationFiles.length).toBeGreaterThan(0);

        // Check for specific declaration files
        expect(declarationFiles).toContain('prompt-templates.d.ts');
      });

      it('should include critical AI modules', () => {
        const criticalModules = [
          join(aiDir, 'prompt-templates.js'),
          join(aiDir, 'prompt-engine.js')
        ];

        criticalModules.forEach(moduleFile => {
          expect(existsSync(moduleFile)).toBe(true);

          // Also check for corresponding declaration files
          const declarationFile = moduleFile.replace('.js', '.d.ts');
          expect(existsSync(declarationFile)).toBe(true);
        });
      });
    });

    describe('Knowledge Data Directory', () => {
      // Knowledge data has been moved to top-level knowledge/packs/ directory
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
          expect(() => {
            require(filePath);
          }).not.toThrow();
        });
      });
    });
  });

  describe('CommonJS Build (dist-cjs)', () => {
    describe('AI Module', () => {
      const aiDir = join(distCjsDir, 'src', 'ai');

      it('should include AI directory in dist-cjs', () => {
        expect(existsSync(aiDir)).toBe(true);
      });

      it('should include prompt template files', () => {
        // Check for prompt template module in AI directory
        const expectedPromptFiles = [
          'prompt-templates.js'
        ];

        const files = readdirSync(aiDir).filter(item => {
          return item.endsWith('.js') || item.endsWith('.d.ts');
        });

        expectedPromptFiles.forEach(file => {
          expect(files).toContain(file);
        });
      });
    });

    // Knowledge data is no longer duplicated in dist-cjs
    // It's in the top-level knowledge/packs/ directory only
  });

  describe('Package Integrity', () => {
    it('should have consistent TypeScript modules between ESM and CommonJS builds', () => {
      const esmAiDir = join(distDir, 'src', 'ai');
      const cjsAiDir = join(distCjsDir, 'src', 'ai');

      // Count JavaScript files (compiled TypeScript)
      const countJsFiles = (dir: string): number => {
        const items = readdirSync(dir);
        return items.filter(item => item.endsWith('.js')).length;
      };

      const esmAiCount = countJsFiles(esmAiDir);
      const cjsAiCount = countJsFiles(cjsAiDir);

      expect(esmAiCount).toBe(cjsAiCount);
      expect(esmAiCount).toBeGreaterThan(0);

      // Should have at least prompt-templates and prompt-engine
      expect(esmAiCount).toBeGreaterThanOrEqual(2);
    });

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
    it('should be able to find AI modules at runtime', () => {
      const possibleAiDirs = [
        join(rootDir, 'src', 'ai'),
        join(distDir, 'src', 'ai'),
        join(distCjsDir, 'src', 'ai')
      ];

      const foundAiDir = possibleAiDirs.find(dir => existsSync(dir));
      expect(foundAiDir).toBeDefined();

      // Verify it contains TypeScript modules
      if (foundAiDir) {
        const files = readdirSync(foundAiDir);
        const tsFiles = files.filter(f => f.endsWith('.ts') || f.endsWith('.js'));

        // Should have TypeScript source files or compiled JS files
        expect(tsFiles.length).toBeGreaterThan(0);

        // Check for key modules
        const hasTemplates = files.some(f => f.includes('prompt-templates'));

        expect(hasTemplates).toBe(true);
      }
    });
  });
});