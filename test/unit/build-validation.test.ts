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
    describe('Prompts Directory', () => {
      const promptsDir = join(distDir, 'src', 'prompts');

      it('should include prompts directory in dist', () => {
        expect(existsSync(promptsDir)).toBe(true);
      });

      it('should include TypeScript prompt files', () => {
        // Check for TypeScript prompt files (templates module)
        const expectedPromptFiles = [
          'templates.js'
        ];

        const files = readdirSync(promptsDir).filter(item => {
          return item.endsWith('.js') || item.endsWith('.d.ts');
        });

        expectedPromptFiles.forEach(file => {
          expect(files).toContain(file);
        });
      });

      it('should include TypeScript declaration files', () => {
        const files = readdirSync(promptsDir);
        const declarationFiles = files.filter(f => f.endsWith('.d.ts'));

        // Should have declaration files for type safety
        expect(declarationFiles.length).toBeGreaterThan(0);

        // Check for specific declaration files
        expect(declarationFiles).toContain('templates.d.ts');
      });

      it('should include critical TypeScript prompt modules', () => {
        const criticalModules = [
          join(promptsDir, 'templates.js')
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
      const knowledgeDataDir = join(distDir, 'src', 'knowledge', 'data');

      it('should include knowledge data directory in dist', () => {
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
    describe('Prompts Directory', () => {
      const promptsDir = join(distCjsDir, 'src', 'prompts');

      it('should include prompts directory in dist-cjs', () => {
        expect(existsSync(promptsDir)).toBe(true);
      });

      it('should include TypeScript prompt files', () => {
        // Check for TypeScript prompt files (templates module)
        const expectedPromptFiles = [
          'templates.js'
        ];

        const files = readdirSync(promptsDir).filter(item => {
          return item.endsWith('.js') || item.endsWith('.d.ts');
        });

        expectedPromptFiles.forEach(file => {
          expect(files).toContain(file);
        });
      });

      it('should include JSON prompt files in CommonJS build', () => {
        const categories = readdirSync(promptsDir).filter(item => {
          const itemPath = join(promptsDir, item);
          return statSync(itemPath).isDirectory();
        });

        categories.forEach(category => {
          const categoryPath = join(promptsDir, category);
          const files = readdirSync(categoryPath);
          const jsonFiles = files.filter(f => f.endsWith('.json'));
          
          expect(jsonFiles.length).toBeGreaterThan(0);
        });
      });
    });

    describe('Knowledge Data Directory', () => {
      const knowledgeDataDir = join(distCjsDir, 'src', 'knowledge', 'data');

      it('should include knowledge data directory in dist-cjs', () => {
        expect(existsSync(knowledgeDataDir)).toBe(true);
      });

      it('should include all knowledge pack files in CommonJS build', () => {
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
    });
  });

  describe('Package Integrity', () => {
    it('should have consistent TypeScript modules between ESM and CommonJS builds', () => {
      const esmPromptsDir = join(distDir, 'src', 'prompts');
      const cjsPromptsDir = join(distCjsDir, 'src', 'prompts');

      // Count JavaScript files (compiled TypeScript)
      const countJsFiles = (dir: string): number => {
        const items = readdirSync(dir);
        return items.filter(item => item.endsWith('.js')).length;
      };

      const esmPromptCount = countJsFiles(esmPromptsDir);
      const cjsPromptCount = countJsFiles(cjsPromptsDir);

      expect(esmPromptCount).toBe(cjsPromptCount);
      expect(esmPromptCount).toBeGreaterThan(0);

      // Should have at least templates
      expect(esmPromptCount).toBeGreaterThanOrEqual(1);
    });

    it('should have knowledge data files with reasonable sizes', () => {
      const knowledgeDataDir = join(distDir, 'src', 'knowledge', 'data');
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
    it('should be able to find prompts modules at runtime', () => {
      const possiblePromptDirs = [
        join(rootDir, 'src', 'prompts'),
        join(distDir, 'src', 'prompts'),
        join(distCjsDir, 'src', 'prompts')
      ];

      const foundPromptDir = possiblePromptDirs.find(dir => existsSync(dir));
      expect(foundPromptDir).toBeDefined();
      
      // Verify it contains TypeScript modules
      if (foundPromptDir) {
        const files = readdirSync(foundPromptDir);
        const tsFiles = files.filter(f => f.endsWith('.ts') || f.endsWith('.js'));

        // Should have TypeScript source files or compiled JS files
        expect(tsFiles.length).toBeGreaterThan(0);

        // Check for key modules
        const hasTemplates = files.some(f => f.startsWith('templates'));

        expect(hasTemplates).toBe(true);
      }
    });
  });
});